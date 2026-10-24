package mockcouch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
)

// Store is an in-memory document store.
type Store struct {
	mu    sync.Mutex
	docs  map[string]map[string]any // id -> rev -> doc
	revs  map[string][]string
	order []string
	seq   int
}

func NewStore() *Store {
	return &Store{docs: map[string]map[string]any{}, revs: map[string][]string{}}
}

func (s *Store) nextRev(id string) string {
	s.seq++
	// simple rev: gen-hash
	return fmt.Sprintf("%d-%x", len(s.revs[id])+1, s.seq)
}

func (s *Store) putDoc(doc map[string]any, newEdits bool) map[string]any {
	id, _ := doc["_id"].(string)
	rev, _ := doc["_rev"].(string)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.docs[id] == nil {
		s.docs[id] = map[string]any{}
	}
	if !newEdits {
		if rev == "" {
			rev = s.nextRev(id)
			doc["_rev"] = rev
		}
		s.seq++
		doc["_seq"] = strconv.Itoa(s.seq)
		s.docs[id][rev] = cloneMap(doc)
		s.revs[id] = appendUnique(s.revs[id], rev)
		s.order = append(s.order, id)
		return map[string]any{"ok": true, "id": id, "rev": rev}
	}
	// new edit
	newRev := s.nextRev(id)
	doc["_rev"] = newRev
	s.seq++
	doc["_seq"] = strconv.Itoa(s.seq)
	s.docs[id][newRev] = cloneMap(doc)
	s.revs[id] = appendUnique(s.revs[id], newRev)
	s.order = append(s.order, id)
	return map[string]any{"ok": true, "id": id, "rev": newRev}
}

func appendUnique(slice []string, v string) []string {
	for _, x := range slice {
		if x == v {
			return slice
		}
	}
	return append(slice, v)
}

func cloneMap(m map[string]any) map[string]any {
	b, _ := json.Marshal(m)
	var out map[string]any
	json.Unmarshal(b, &out)
	return out
}

// Server wraps httptest.Server.
type Server struct {
	*httptest.Server
	Store *Store
}

func StartServer() *Server {
	st := NewStore()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	s := &Server{Server: srv, Store: st}

	dbName := "testdb"
	// info
	mux.HandleFunc("/"+dbName, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			st.mu.Lock()
			seq := st.seq
			st.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{
				"db_name":    dbName,
				"update_seq": strconv.Itoa(seq),
				"adapter":    "memory",
			})
			return
		}
		w.WriteHeader(404)
	})
	// changes
	mux.HandleFunc("/"+dbName+"/_changes", func(w http.ResponseWriter, r *http.Request) {
		since := r.URL.Query().Get("since")
		limitStr := r.URL.Query().Get("limit")
		limit := 0
		if limitStr != "" {
			fmt.Sscan(limitStr, &limit)
		}
		includeDocs := r.URL.Query().Get("include_docs") == "true"
		st.mu.Lock()
		defer st.mu.Unlock()
		sinceN := 0
		fmt.Sscan(since, &sinceN)
		seen := map[string]bool{}
		var results []map[string]any
		count := 0
		for _, id := range st.order {
			if seen[id] {
				continue
			}
			seen[id] = true
			revs := st.revs[id]
			if len(revs) == 0 {
				continue
			}
			rev := revs[len(revs)-1]
			doc, _ := st.docs[id][rev].(map[string]any)
			seqStr, _ := doc["_seq"].(string)
			seqN := 0
			fmt.Sscan(seqStr, &seqN)
			if seqN <= sinceN {
				continue
			}
			entry := map[string]any{
				"id":      id,
				"seq":     seqStr,
				"changes": []map[string]any{{"rev": rev}},
			}
			if includeDocs {
				entry["doc"] = doc
			}
			if del, _ := doc["_deleted"].(bool); del {
				entry["deleted"] = true
			}
			results = append(results, entry)
			count++
			if limit > 0 && count >= limit {
				break
			}
		}
		json.NewEncoder(w).Encode(map[string]any{
			"results":  results,
			"last_seq": strconv.Itoa(st.seq),
		})
	})
	// revs_diff
	mux.HandleFunc("/"+dbName+"/_revs_diff", func(w http.ResponseWriter, r *http.Request) {
		var diff map[string][]string
		json.NewDecoder(r.Body).Decode(&diff)
		res := map[string]map[string]any{}
		st.mu.Lock()
		defer st.mu.Unlock()
		for id, revs := range diff {
			revSet := map[string]bool{}
			for _, rv := range st.revs[id] {
				revSet[rv] = true
			}
			var missing []string
			for _, rv := range revs {
				if !revSet[rv] {
					missing = append(missing, rv)
				}
			}
			if len(missing) > 0 {
				res[id] = map[string]any{"missing": missing}
			}
		}
		json.NewEncoder(w).Encode(res)
	})
	// bulk_get
	mux.HandleFunc("/"+dbName+"/_bulk_get", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Docs []struct {
				ID  string `json:"id"`
				Rev string `json:"rev"`
			} `json:"docs"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		st.mu.Lock()
		defer st.mu.Unlock()
		var results []map[string]any
		for _, req := range body.Docs {
			gr := map[string]any{"id": req.ID, "docs": []any{}}
			revs := st.revs[req.ID]
			targetRev := req.Rev
			if targetRev == "" && len(revs) > 0 {
				targetRev = revs[len(revs)-1]
			}
			if d, ok := st.docs[req.ID][targetRev]; ok {
				gr["docs"] = []any{map[string]any{"ok": d}}
			} else {
				gr["docs"] = []any{map[string]any{"error": map[string]any{"missing": req.Rev, "error": "not_found"}}}
			}
			results = append(results, gr)
		}
		json.NewEncoder(w).Encode(map[string]any{"results": results})
	})
	// bulk_docs
	mux.HandleFunc("/"+dbName+"/_bulk_docs", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Docs     []map[string]any `json:"docs"`
			NewEdits *bool            `json:"new_edits"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		newEdits := true
		if body.NewEdits != nil {
			newEdits = *body.NewEdits
		}
		var results []map[string]any
		for _, d := range body.Docs {
			res := s.Store.putDoc(d, newEdits)
			results = append(results, res)
		}
		json.NewEncoder(w).Encode(results)
	})
	// doc get/put/delete
	mux.HandleFunc("/"+dbName+"/", func(w http.ResponseWriter, r *http.Request) {
		// strip db prefix
		p := strings.TrimPrefix(r.URL.Path, "/"+dbName+"/")
		if p == "" || strings.HasPrefix(p, "_") {
			// already handled? changes etc handled above by more specific? ServeMux longest match wins, but _changes has explicit. _bulk handled. fallback.
			w.WriteHeader(404)
			return
		}
		id := p
		switch r.Method {
		case http.MethodGet:
			rev := r.URL.Query().Get("rev")
			st.mu.Lock()
			revs := st.revs[id]
			targetRev := rev
			if targetRev == "" && len(revs) > 0 {
				targetRev = revs[len(revs)-1]
			}
			d, ok := st.docs[id][targetRev].(map[string]any)
			st.mu.Unlock()
			if !ok {
				w.WriteHeader(404)
				json.NewEncoder(w).Encode(map[string]any{"error": "not_found"})
				return
			}
			json.NewEncoder(w).Encode(d)
		case http.MethodPut:
			var doc map[string]any
			json.NewDecoder(r.Body).Decode(&doc)
			if doc["_id"] == nil {
				doc["_id"] = id
			}
			res := s.Store.putDoc(doc, true)
			json.NewEncoder(w).Encode(res)
		case http.MethodDelete:
			rev := r.URL.Query().Get("rev")
			st.mu.Lock()
			revs := st.revs[id]
			targetRev := rev
			if targetRev == "" && len(revs) > 0 {
				targetRev = revs[len(revs)-1]
			}
			d, ok := st.docs[id][targetRev].(map[string]any)
			st.mu.Unlock()
			if !ok {
				w.WriteHeader(404)
				return
			}
			d["_deleted"] = true
			res := s.Store.putDoc(d, true)
			json.NewEncoder(w).Encode(res)
		default:
			w.WriteHeader(405)
		}
	})

	return s
}
