# stonefish — pdb core (Go + Rust)

Prototype of a pdb (PouchDB-compatible) core in Go (server/edge) and Rust (browser/WASM).

## Packages
- `pkg/adapter` — storage backend interface: `MemoryAdapter`, `HTTPAdapter`, `SQLiteAdapter`.
- `pkg/checkpointer` — durable replication checkpoints in `_local` docs.
- `pkg/replication` — pull/push replication engine (changes → revsDiff → bulkGet → bulkDocs `new_edits:false`), continuous mode + backoff.
- `pkg/collate`, `pkg/merge` — CouchDB-compatible collation and rev-tree conflict resolution.
- `rust/` — `pdb-core` crate (collate + merge), builds native and `wasm32-unknown-unknown`.
- `internal/mockcouch` — in-memory cdb-compatible HTTP server for tests.

## Build
```sh
# Go
export GOROOT=/data/orca-home/go GOPATH=/data/orca-home/gopath
export PATH=$GOROOT/bin:$GOPATH/bin:$PATH
go build ./...
go test ./...

# Rust
cd rust
export PATH=/data/orca-home/.cargo/bin:$PATH
cargo build --release
cargo build --release --target wasm32-unknown-unknown
```

## Use in an existing codebase
- Go: import `github.com/stonefish/pdb-go/pkg/...` and depend on `go.mod`. Use `SQLiteAdapter` for embedded single-binary (PocketBase style) or `HTTPAdapter` to sync with a cdb server.
- Rust/WASM: add `pdb-core` as a dependency or load the compiled `.wasm` in the browser; call collate/merge helpers from JS via wasm-bindgen (add wasm-bindgen when exposing).

## SvelteKit integration
- Server side: run stonefish as a Go service (or embed) exposing cdb-compatible HTTP endpoints; SvelteKit talks via `fetch` to `/db/*` or a local adapter. Trust proxy headers (`PROTOCOL_HEADER`, `HOST_HEADER`) when behind Traefik.
- Browser/offline: compile `rust/pdb-core` to WASM (wasm-bindgen) and load in a SvelteKit client module for local collation/merge and offline storage; sync with server via the replication engine.
- Keep origin handling correct: do not hardcode `ORIGIN=http://localhost` in production.

## Deploy (Dokploy)
- Do not publish host ports; use `expose: ["<container-port>"]` and map domains to the container port.
- Declare env vars under `environment:` (no committed `env_file: .env`).
- Add a `HEALTHCHECK` + `/health` route.

## Licensing
Brand names "pdb"/"cdb" used in place of restricted terms.