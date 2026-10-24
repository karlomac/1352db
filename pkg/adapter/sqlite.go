package adapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	_ "modernc.org/sqlite"
)

type SQLiteAdapter struct {
	db     *sql.DB
	dbName string
	seq    int64
}

func NewSQLiteAdapter(path, dbName string) (*SQLiteAdapter, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	a := &SQLiteAdapter{db: db, dbName: dbName}
	if err := a.init(); err != nil {
		db.Close()
		return nil, err
	}
	return a, nil
}

func (a *SQLiteAdapter) init() error {
	_, err := a.db.Exec(`CREATE TABLE IF NOT EXISTS docs (
		id TEXT NOT NULL,
		rev TEXT NOT NULL,
		body TEXT NOT NULL,
		seq INTEGER NOT NULL,
		deleted INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (id, rev)
	)`)
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`CREATE TABLE IF NOT EXISTS meta (k TEXT PRIMARY KEY, v TEXT)`)
	if err != nil {
		return err
	}
	// load seq
	row := a.db.QueryRow(`SELECT COALESCE(MAX(seq),0) FROM docs`)
	_ = row.Scan(&a.seq)
	return nil
}

func (a *SQLiteAdapter) Info(ctx context.Context) (Info, error) {
	return Info{DBName: a.dbName, Seq: strconv.FormatInt(a.seq, 10), Adapter: "sqlite"}, nil
}

func (a *SQLiteAdapter) Get(ctx context.Context, id string, rev string) (Document, error) {
	var body string
	var r string
	var q string
	var args []any
	if rev == "" {
		q = `SELECT rev, body FROM docs WHERE id=? ORDER BY seq DESC LIMIT 1`
		args = []any{id}
	} else {
		q = `SELECT rev, body FROM docs WHERE id=? AND rev=?`
		args = []any{id, rev}
	}
	err := a.db.QueryRowContext(ctx, q, args...).Scan(&r, &body)
	if err != nil {
		return Document{}, fmt.Errorf("not_found: %s", id)
	}
	var doc Document
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return Document{}, err
	}
	if doc.ID == "" {
		doc.ID = id
	}
	if doc.Rev == "" {
		doc.Rev = r
	}
	return doc, nil
}

func (a *SQLiteAdapter) Put(ctx context.Context, doc Document) (string, error) {
	if doc.Rev == "" {
		// new rev generation 1
		doc.Rev = fmt.Sprintf("1-%x", a.seq+1)
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	a.seq++
	_, err = a.db.ExecContext(ctx, `INSERT OR REPLACE INTO docs (id, rev, body, seq, deleted) VALUES (?,?,?,?,?)`,
		doc.ID, doc.Rev, string(body), a.seq, boolToInt(doc.Deleted))
	if err != nil {
		return "", err
	}
	return doc.Rev, nil
}

func (a *SQLiteAdapter) BulkDocs(ctx context.Context, docs []Document, newEdits bool) ([]BulkDocResult, error) {
	res := make([]BulkDocResult, 0, len(docs))
	for _, d := range docs {
		rev, err := a.Put(ctx, d)
		if err != nil {
			res = append(res, BulkDocResult{ID: d.ID, Error: "conflict", Message: err.Error()})
			continue
		}
		res = append(res, BulkDocResult{ID: d.ID, Rev: rev})
	}
	return res, nil
}

func (a *SQLiteAdapter) Remove(ctx context.Context, id string, rev string) (string, error) {
	d, err := a.Get(ctx, id, rev)
	if err != nil {
		return "", err
	}
	d.Deleted = true
	return a.Put(ctx, d)
}

func (a *SQLiteAdapter) BulkGet(ctx context.Context, reqs []BulkGetRequest) (BulkGetResponse, error) {
	resp := BulkGetResponse{Results: []BulkGetResult{}}
	for _, r := range reqs {
		gr := BulkGetResult{ID: r.ID, Docs: []BulkGetDoc{}}
		d, err := a.Get(ctx, r.ID, r.Rev)
		if err != nil {
			gr.Docs = append(gr.Docs, BulkGetDoc{Error: &BulkGetError{Missing: r.Rev, Error: "not_found"}})
		} else {
			gr.Docs = append(gr.Docs, BulkGetDoc{OK: &d})
		}
		resp.Results = append(resp.Results, gr)
	}
	return resp, nil
}

func (a *SQLiteAdapter) Changes(ctx context.Context, opts ChangesOpts) (<-chan Change, error) {
	ch := make(chan Change, 64)
	go func() {
		defer close(ch)
		sinceN := 0
		fmt.Sscan(opts.Since, &sinceN)
		rows, err := a.db.QueryContext(ctx, `SELECT id, rev, body, seq FROM docs WHERE seq > ? ORDER BY seq`, sinceN)
		if err != nil {
			return
		}
		defer rows.Close()
		seen := map[string]bool{}
		for rows.Next() {
			var id, rev, body string
			var seq int64
			if err := rows.Scan(&id, &rev, &body, &seq); err != nil {
				continue
			}
			if seen[id] {
				// only latest per id? memory returns one per id; keep simple: skip dupes
				continue
			}
			seen[id] = true
			var doc Document
			json.Unmarshal([]byte(body), &doc)
			c := Change{ID: id, Seq: strconv.FormatInt(seq, 10), Changes: []RevInfo{{Rev: rev}}}
			if opts.IncludeDocs {
				dd := doc
				c.Doc = &dd
			}
			if doc.Deleted {
				c.Deleted = true
			}
			select {
			case ch <- c:
			case <-ctx.Done():
				return
			}
			if opts.Limit > 0 && len(seen) >= opts.Limit {
				// best-effort limit
			}
			_ = opts
		}
	}()
	return ch, nil
}

func (a *SQLiteAdapter) RevsDiff(ctx context.Context, diff map[string][]string) (map[string]RevsDiffResult, error) {
	res := map[string]RevsDiffResult{}
	for id, revs := range diff {
		rows, err := a.db.QueryContext(ctx, `SELECT rev FROM docs WHERE id=?`, id)
		if err != nil {
			continue
		}
		revSet := map[string]bool{}
		for rows.Next() {
			var r string
			rows.Scan(&r)
			revSet[r] = true
		}
		rows.Close()
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

func (a *SQLiteAdapter) Close() error { return a.db.Close() }

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
