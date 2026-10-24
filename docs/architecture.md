# Architecture

## Module layout
```
github.com/stonefish/pdb-go
├── pkg/adapter        storage backend interface + impls
├── pkg/checkpointer   durable replication checkpoints
├── pkg/collate        CouchDB-compatible collation
├── pkg/merge          rev-tree traversal + conflict resolution
├── pkg/replication    pull/push replication engine
├── internal/mockcouch in-memory cdb-compatible HTTP server (tests)
└── rust/              pdb-core crate (collate + merge), native + WASM
```

## Adapter interface
All backends implement `adapter.Adapter`:
- `Info(ctx) (Info, error)`
- `Get(ctx, id, rev) (Document, error)` — empty rev = current winning rev
- `Put(ctx, doc) (string, error)` — returns new rev; honors `doc.Rev` when provided (new_edits:false)
- `BulkDocs(ctx, docs, newEdits) ([]BulkDocResult, error)`
- `Remove(ctx, id, rev) (string, error)`
- `BulkGet(ctx, reqs) (BulkGetResponse, error)`
- `Changes(ctx, opts) (<-chan Change, error)`
- `RevsDiff(ctx, diff) (map[string]RevsDiffResult, error)`
- `Close() error`

## Backends
- **MemoryAdapter**: in-memory, thread-safe (RWMutex). Good for tests and ephemeral.
- **HTTPAdapter**: cdb-compatible REST. Base URL + db name.
- **SQLiteAdapter**: pure-Go SQLite (modernc.org/sqlite). Single file, embedded.

## Document model
`adapter.Document` carries `_id`, `_rev`, `_deleted`, `_attachments`, plus `Extra` map for arbitrary fields. Custom JSON marshal/unmarshal preserves `_`-prefixed and user fields.