// Package adapter defines the storage backend interface used by the
// replication engine. It mirrors the pdb adapter contract so the
// same replication code works against any backend.
package adapter

import "context"

// Attachment describes an attachment on a document.
type Attachment struct {
	Content     string `json:"content,omitempty"` // base64 when inline
	ContentType string `json:"content_type,omitempty"`
	Digest      string `json:"digest,omitempty"`
	Length      int    `json:"length,omitempty"`
	RevPos      int    `json:"rev_pos,omitempty"`
	Stub        bool   `json:"stub,omitempty"`
}

// Change describes a single entry in a changes feed.
type Change struct {
	ID      string    `json:"id"`
	Changes []RevInfo `json:"changes"`
	Seq     string    `json:"seq"`
	Deleted bool      `json:"deleted,omitempty"`
	Doc     *Document `json:"doc,omitempty"`
}

// RevInfo is a single revision reference in a change.
type RevInfo struct {
	Rev string `json:"rev"`
}

// BulkDocResult is the response to a bulkDocs write.
type BulkDocResult struct {
	ID      string `json:"id"`
	Rev     string `json:"rev,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// BulkGetRequest is a single request in a bulk_get call.
type BulkGetRequest struct {
	ID  string `json:"id"`
	Rev string `json:"rev"`
}

// BulkGetResult is the result for one id in a bulk_get response.
type BulkGetResult struct {
	ID   string       `json:"id"`
	Docs []BulkGetDoc `json:"docs"`
}

// BulkGetDoc is a single doc entry in a bulk_get result.
type BulkGetDoc struct {
	OK    *Document     `json:"ok,omitempty"`
	Error *BulkGetError `json:"error,omitempty"`
}

// BulkGetError describes a missing/failed doc in bulk_get.
type BulkGetError struct {
	Missing string `json:"missing,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// BulkGetResponse is the full bulk_get response.
type BulkGetResponse struct {
	Results []BulkGetResult `json:"results"`
}

// RevsDiffResult describes which revisions are missing and which are conflicted.
type RevsDiffResult struct {
	Missing  []string `json:"missing"`
	Possible []string `json:"possible,omitempty"`
}

// Document is a cdb/pdb document.
type Document struct {
	ID          string                `json:"_id"`
	Rev         string                `json:"_rev,omitempty"`
	Deleted     bool                  `json:"_deleted,omitempty"`
	Revs        []string              `json:"_revs,omitempty"`
	Revisions   map[string][]string   `json:"_revs_info,omitempty"`
	Attachments map[string]Attachment `json:"_attachments,omitempty"`
	Seq         string                `json:"_seq,omitempty"`
	Extra       map[string]any        `json:"-"`
}

// ChangesOpts configures a changes feed.
type ChangesOpts struct {
	Since       string
	Limit       int
	BatchSize   int
	Style       string
	IncludeDocs bool
	DocIDs      []string
	Filter      string
	Selector    string
	QueryParams map[string]string
	View        string
	ReturnDocs  bool
	Heartbeat   int
	Timeout     int
	Live        bool
}

// Info describes database metadata.
type Info struct {
	DBName         string   `json:"db_name"`
	Seq            string   `json:"update_seq"`
	CompactRunning bool     `json:"compact_running,omitempty"`
	ActiveTasks    []string `json:"active_tasks,omitempty"`
	Adapter        string   `json:"adapter"`
}

// Adapter is the storage backend interface.
type Adapter interface {
	Info(ctx context.Context) (Info, error)
	Get(ctx context.Context, id string, rev string) (Document, error)
	Put(ctx context.Context, doc Document) (string, error)
	BulkDocs(ctx context.Context, docs []Document, newEdits bool) ([]BulkDocResult, error)
	Remove(ctx context.Context, id string, rev string) (string, error)
	BulkGet(ctx context.Context, reqs []BulkGetRequest) (BulkGetResponse, error)
	Changes(ctx context.Context, opts ChangesOpts) (<-chan Change, error)
	RevsDiff(ctx context.Context, diff map[string][]string) (map[string]RevsDiffResult, error)
	Close() error
}
