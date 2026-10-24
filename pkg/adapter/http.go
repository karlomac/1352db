package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// HTTPAdapter talks to a cdb-compatible HTTP endpoint.
type HTTPAdapter struct {
	baseURL string
	dbName  string
	client  *http.Client
}

func NewHTTPAdapter(baseURL string, dbName string) *HTTPAdapter {
	return &HTTPAdapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		dbName:  dbName,
		client:  http.DefaultClient,
	}
}

func (h *HTTPAdapter) endpoint(path string) string {
	return h.baseURL + "/" + h.dbName + path
}

func (h *HTTPAdapter) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, h.endpoint(path), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return resp, nil
}

func (h *HTTPAdapter) Info(ctx context.Context) (Info, error) {
	resp, err := h.do(ctx, http.MethodGet, "", nil)
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return Info{}, err
	}
	var info Info
	if err := json.Unmarshal(b, &info); err != nil {
		return Info{}, err
	}
	if info.Adapter == "" {
		info.Adapter = "http"
	}
	return info, nil
}

func (h *HTTPAdapter) Get(ctx context.Context, id string, rev string) (Document, error) {
	path := "/" + url.PathEscape(id)
	if rev != "" {
		path += "?rev=" + url.QueryEscape(rev)
	}
	resp, err := h.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return Document{}, err
	}
	defer resp.Body.Close()
	var doc Document
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func (h *HTTPAdapter) Put(ctx context.Context, doc Document) (string, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	if doc.ID == "" {
		resp, err := h.do(ctx, http.MethodPost, "", strings.NewReader(string(b)))
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		var r struct {
			Rev string `json:"rev"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return "", err
		}
		return r.Rev, nil
	}
	path := "/" + url.PathEscape(doc.ID)
	resp, err := h.do(ctx, http.MethodPut, path, strings.NewReader(string(b)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r struct {
		Rev string `json:"rev"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	return r.Rev, nil
}

func (h *HTTPAdapter) BulkDocs(ctx context.Context, docs []Document, newEdits bool) ([]BulkDocResult, error) {
	body := map[string]any{"docs": docs, "new_edits": newEdits}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := h.do(ctx, http.MethodPost, "/_bulk_docs", strings.NewReader(string(b)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var raw []struct {
		ID     string `json:"id"`
		Rev    string `json:"rev"`
		Error  string `json:"error"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	res := make([]BulkDocResult, 0, len(raw))
	for _, r := range raw {
		msg := r.Reason
		res = append(res, BulkDocResult{ID: r.ID, Rev: r.Rev, Error: r.Error, Message: msg})
	}
	return res, nil
}

func (h *HTTPAdapter) Remove(ctx context.Context, id string, rev string) (string, error) {
	if rev == "" {
		d, err := h.Get(ctx, id, "")
		if err != nil {
			return "", err
		}
		rev = d.Rev
	}
	path := "/" + url.PathEscape(id) + "?rev=" + url.QueryEscape(rev)
	resp, err := h.do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r struct {
		Rev string `json:"rev"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.Rev == "" {
		r.Rev = rev
	}
	return r.Rev, nil
}

func (h *HTTPAdapter) BulkGet(ctx context.Context, reqs []BulkGetRequest) (BulkGetResponse, error) {
	type bgReq struct {
		ID  string `json:"id"`
		Rev string `json:"rev,omitempty"`
	}
	docs := make([]bgReq, 0, len(reqs))
	for _, r := range reqs {
		docs = append(docs, bgReq{ID: r.ID, Rev: r.Rev})
	}
	b, err := json.Marshal(map[string]any{"docs": docs})
	if err != nil {
		return BulkGetResponse{}, err
	}
	resp, err := h.do(ctx, http.MethodPost, "/_bulk_get", strings.NewReader(string(b)))
	if err != nil {
		return BulkGetResponse{}, err
	}
	defer resp.Body.Close()
	var raw struct {
		Results []struct {
			ID   string            `json:"id"`
			Docs []json.RawMessage `json:"docs"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return BulkGetResponse{}, err
	}
	respOut := BulkGetResponse{Results: []BulkGetResult{}}
	for _, r := range raw.Results {
		gr := BulkGetResult{ID: r.ID, Docs: []BulkGetDoc{}}
		for _, dRaw := range r.Docs {
			var okW struct {
				OK *json.RawMessage `json:"ok"`
			}
			if err := json.Unmarshal(dRaw, &okW); err == nil && okW.OK != nil {
				var doc Document
				_ = json.Unmarshal(*okW.OK, &doc)
				gr.Docs = append(gr.Docs, BulkGetDoc{OK: &doc})
				continue
			}
			be := &BulkGetError{}
			var topLevel struct {
				Error   string `json:"error"`
				Reason  string `json:"reason"`
				Missing string `json:"missing"`
				Message string `json:"message"`
			}
			_ = json.Unmarshal(dRaw, &topLevel)
			if topLevel.Error != "" {
				be.Error = topLevel.Error
			}
			if topLevel.Reason != "" {
				be.Message = topLevel.Reason
			}
			if topLevel.Missing != "" {
				be.Missing = topLevel.Missing
			}
			if topLevel.Message != "" {
				be.Message = topLevel.Message
			}
			// nested error object
			var nested struct {
				Error json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal(dRaw, &nested); err == nil && len(nested.Error) > 0 {
				var es string
				if json.Unmarshal(nested.Error, &es) == nil {
					if be.Error == "" {
						be.Error = es
					}
				} else {
					var eo map[string]any
					if json.Unmarshal(nested.Error, &eo) == nil {
						if v, ok := eo["error"].(string); ok && be.Error == "" {
							be.Error = v
						}
						if v, ok := eo["reason"].(string); ok && be.Message == "" {
							be.Message = v
						}
						if v, ok := eo["missing"].(string); ok && be.Missing == "" {
							be.Missing = v
						}
					}
				}
			}
			gr.Docs = append(gr.Docs, BulkGetDoc{Error: be})
		}
		respOut.Results = append(respOut.Results, gr)
	}
	return respOut, nil
}

func (h *HTTPAdapter) Changes(ctx context.Context, opts ChangesOpts) (<-chan Change, error) {
	ch := make(chan Change, 64)
	go func() {
		defer close(ch)
		q := url.Values{}
		since := opts.Since
		if since == "" {
			since = "0"
		}
		q.Set("since", since)
		limit := opts.Limit
		if limit <= 0 {
			limit = opts.BatchSize
		}
		if limit > 0 {
			q.Set("limit", fmt.Sprintf("%d", limit))
		}
		if opts.IncludeDocs {
			q.Set("include_docs", "true")
		}
		path := "/_changes?" + q.Encode()
		resp, err := h.do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		var raw struct {
			Results []Change `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return
		}
		for _, c := range raw.Results {
			select {
			case ch <- c:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (h *HTTPAdapter) RevsDiff(ctx context.Context, diff map[string][]string) (map[string]RevsDiffResult, error) {
	b, err := json.Marshal(diff)
	if err != nil {
		return nil, err
	}
	resp, err := h.do(ctx, http.MethodPost, "/_revs_diff", strings.NewReader(string(b)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var raw map[string]struct {
		Missing  []string `json:"missing"`
		Possible []string `json:"possible"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	res := map[string]RevsDiffResult{}
	for k, v := range raw {
		res[k] = RevsDiffResult{Missing: v.Missing, Possible: v.Possible}
	}
	return res, nil
}

func (h *HTTPAdapter) Close() error { return nil }

func (i *Info) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for k, v := range raw {
		switch k {
		case "db_name":
			_ = json.Unmarshal(v, &i.DBName)
		case "update_seq":
			var s string
			if err := json.Unmarshal(v, &s); err == nil {
				i.Seq = s
			} else {
				var n json.Number
				if err := json.Unmarshal(v, &n); err == nil {
					i.Seq = n.String()
				} else {
					var f float64
					if err := json.Unmarshal(v, &f); err == nil {
						if f == float64(int64(f)) {
							i.Seq = fmt.Sprintf("%d", int64(f))
						} else {
							i.Seq = fmt.Sprintf("%g", f)
						}
					}
				}
			}
		case "compact_running":
			_ = json.Unmarshal(v, &i.CompactRunning)
		case "active_tasks":
			_ = json.Unmarshal(v, &i.ActiveTasks)
		case "adapter":
			_ = json.Unmarshal(v, &i.Adapter)
		}
	}
	return nil
}

// Document JSON handling preserving extra fields.
func (d Document) MarshalJSON() ([]byte, error) {
	m := map[string]any{}
	if d.ID != "" {
		m["_id"] = d.ID
	}
	if d.Rev != "" {
		m["_rev"] = d.Rev
	}
	if d.Deleted {
		m["_deleted"] = true
	}
	if len(d.Revs) > 0 {
		m["_revs"] = d.Revs
	}
	if len(d.Revisions) > 0 {
		m["_revs_info"] = d.Revisions
	}
	if len(d.Attachments) > 0 {
		m["_attachments"] = d.Attachments
	}
	if d.Seq != "" {
		m["_seq"] = d.Seq
	}
	for k, v := range d.Extra {
		m[k] = v
	}
	return json.Marshal(m)
}

func (d *Document) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for k, v := range raw {
		switch k {
		case "_id":
			_ = json.Unmarshal(v, &d.ID)
		case "_rev":
			_ = json.Unmarshal(v, &d.Rev)
		case "_deleted":
			_ = json.Unmarshal(v, &d.Deleted)
		case "_revs":
			_ = json.Unmarshal(v, &d.Revs)
		case "_revs_info":
			_ = json.Unmarshal(v, &d.Revisions)
		case "_attachments":
			_ = json.Unmarshal(v, &d.Attachments)
		case "_seq":
			var s string
			if err := json.Unmarshal(v, &s); err == nil {
				d.Seq = s
			} else {
				var n json.Number
				if err := json.Unmarshal(v, &n); err == nil {
					d.Seq = n.String()
				} else {
					var f float64
					if err := json.Unmarshal(v, &f); err == nil {
						if f == float64(int64(f)) {
							d.Seq = fmt.Sprintf("%d", int64(f))
						} else {
							d.Seq = fmt.Sprintf("%g", f)
						}
					}
				}
			}
		default:
			if d.Extra == nil {
				d.Extra = map[string]any{}
			}
			var val any
			_ = json.Unmarshal(v, &val)
			d.Extra[k] = val
		}
	}
	return nil
}
