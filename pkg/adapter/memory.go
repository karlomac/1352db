package adapter

import (
	"context"
	"fmt"
	"sync"
)

// MemoryAdapter is an in-memory storage backend implementing Adapter.
type MemoryAdapter struct {
	mu    sync.RWMutex
	docs  map[string]map[string]Document // id -> rev -> doc
	revs  map[string][]string            // id -> rev list
	order []string                       // insertion order for changes
	seq   int
}

func NewMemoryAdapter() *MemoryAdapter {
	return &MemoryAdapter{docs: map[string]map[string]Document{}, revs: map[string][]string{}}
}

func (m *MemoryAdapter) Info(ctx context.Context) (Info, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Info{DBName: "memory", Seq: fmt.Sprintf("%d", m.seq), Adapter: "memory"}, nil
}

func (m *MemoryAdapter) Get(ctx context.Context, id string, rev string) (Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	revisions, ok := m.docs[id]
	if !ok {
		return Document{}, errNotFound(id)
	}
	if rev == "" {
		// current winning rev
		if len(m.revs[id]) == 0 {
			return Document{}, errNotFound(id)
		}
		rev = m.revs[id][len(m.revs[id])-1]
	}
	d, ok := revisions[rev]
	if !ok {
		return Document{}, errNotFound(id)
	}
	return d, nil
}

func (m *MemoryAdapter) Put(ctx context.Context, doc Document) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.docs[doc.ID] == nil {
		m.docs[doc.ID] = map[string]Document{}
	}
	if doc.Rev == "" {
		// new doc
		doc.Rev = fmt.Sprintf("1-%s", shortHash(doc.ID))
	} else {
		// ensure parent exists
	}
	m.docs[doc.ID][doc.Rev] = doc
	m.revs[doc.ID] = appendUniqueRev(m.revs[doc.ID], doc.Rev)
	m.seq++
	doc.Seq = fmt.Sprintf("%d", m.seq)
	m.docs[doc.ID][doc.Rev] = doc
	m.order = append(m.order, doc.ID)
	return doc.Rev, nil
}

func (m *MemoryAdapter) BulkDocs(ctx context.Context, docs []Document, newEdits bool) ([]BulkDocResult, error) {
	res := make([]BulkDocResult, 0, len(docs))
	for _, d := range docs {
		rev, err := m.Put(ctx, d)
		if err != nil {
			res = append(res, BulkDocResult{ID: d.ID, Error: "conflict", Message: err.Error()})
			continue
		}
		res = append(res, BulkDocResult{ID: d.ID, Rev: rev})
	}
	return res, nil
}

func (m *MemoryAdapter) Remove(ctx context.Context, id string, rev string) (string, error) {
	d, err := m.Get(ctx, id, rev)
	if err != nil {
		return "", err
	}
	d.Deleted = true
	return m.Put(ctx, d)
}

func (m *MemoryAdapter) BulkGet(ctx context.Context, reqs []BulkGetRequest) (BulkGetResponse, error) {
	resp := BulkGetResponse{Results: []BulkGetResult{}}
	for _, r := range reqs {
		gr := BulkGetResult{ID: r.ID, Docs: []BulkGetDoc{}}
		d, err := m.Get(ctx, r.ID, r.Rev)
		if err != nil {
			gr.Docs = append(gr.Docs, BulkGetDoc{Error: &BulkGetError{Missing: r.Rev, Error: "not_found"}})
		} else {
			gr.Docs = append(gr.Docs, BulkGetDoc{OK: &d})
		}
		resp.Results = append(resp.Results, gr)
	}
	return resp, nil
}

func (m *MemoryAdapter) Changes(ctx context.Context, opts ChangesOpts) (<-chan Change, error) {
	ch := make(chan Change, 64)
	go func() {
		defer close(ch)
		m.mu.RLock()
		defer m.mu.RUnlock()
		seen := map[string]bool{}
		for _, id := range m.order {
			if seen[id] {
				continue
			}
			seen[id] = true
			revs := m.revs[id]
			if len(revs) == 0 {
				continue
			}
			rev := revs[len(revs)-1]
			d := m.docs[id][rev]
			// since filter
			if opts.Since != "" {
				if compareSeq(d.Seq, opts.Since) <= 0 {
					continue
				}
			}
			c := Change{ID: id, Seq: d.Seq, Changes: []RevInfo{{Rev: rev}}}
			if opts.IncludeDocs {
				dd := d
				c.Doc = &dd
			}
			if d.Deleted {
				c.Deleted = true
			}
			select {
			case ch <- c:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (m *MemoryAdapter) RevsDiff(ctx context.Context, diff map[string][]string) (map[string]RevsDiffResult, error) {
	res := map[string]RevsDiffResult{}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for id, revs := range diff {
		revSet := map[string]bool{}
		for _, r := range m.revs[id] {
			revSet[r] = true
		}
		var missing []string
		for _, r := range revs {
			if !revSet[r] {
				missing = append(missing, r)
			}
		}
		if len(missing) > 0 {
			res[id] = RevsDiffResult{Missing: missing}
		}
	}
	return res, nil
}

func appendUniqueRev(slice []string, v string) []string {
	for _, x := range slice {
		if x == v {
			return slice
		}
	}
	return append(slice, v)
}

func (m *MemoryAdapter) Close() error { return nil }

func errNotFound(id string) error {
	return fmt.Errorf("not_found: %s", id)
}

func shortHash(s string) string {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return fmt.Sprintf("%x", h)
}

func compareSeq(a, b string) int {
	// numeric compare if both parse
	ia, ib := 0, 0
	fmt.Sscanf(a, "%d", &ia)
	fmt.Sscanf(b, "%d", &ib)
	if ia < ib {
		return -1
	}
	if ia > ib {
		return 1
	}
	return 0
}
