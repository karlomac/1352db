# Prototype Report: pdb Core Rewrite (Go)

## Start time
2026-10-23T18:42Z (session start). Coding completed 2026-10-23T19:14Z.

## What was built
Go module `github.com/stonefish/pdb-go` implementing:
- `pkg/adapter` — storage backend interface + MemoryAdapter + HTTPAdapter (cdb-compatible REST).
- `pkg/collate` — cdb collation (port of pdb-collate).
- `pkg/merge` — rev tree traversal, leaves, winning rev, conflicts.
- `pkg/replication` — pull replication engine (changes → revsDiff → bulkGet → bulkDocs new_edits:false → checkpoint), batched, idempotent.
- `internal/mockcouch` — in-memory cdb-compatible HTTP server for interop tests.

## Tests
- `go test ./...` passes (collate, merge, replication incl. HTTP interop).
- Benchmark: 1000 docs replicated in ~13.9ms (~72k docs/s) on EPYC.

## Go vs Rust decision
Consistent with rewrite-evaluation.md recommendation:
- Server/edge: Go + SQLite (PocketBase style) — strong HTTP/cdb client libs, easy cross-compile, fast builds.
- Browser: Rust core to WASM for size/no-GC; Go WASM too large.
- Incremental path confirmed: storage adapter → replication engine → query engine.

## Limitations / shortcuts
- shortcut: checkpoint reads from `_local` docs not yet implemented; `Opts.Since` used as resume point. Upgrade when implementing full checkpointer (compareReplicationHistory).
- shortcut: continuous/live replication and backoff not implemented; single-round pull only.
- shortcut: `batches_limit` pipelining not implemented; serial batches.
- shortcut: collation string comparison uses rune code points, not full cdb UTF-8 byte ordering; upgrade for view correctness.
- shortcut: merge winning-rev tie-break may differ from cdb on edge cases; needs interop suite.
- shortcut: attachments travel as base64 in `_attachments` (preserved on roundtrip) but no streaming.

## Next steps
1. Implement checkpointer (compareReplicationHistory) for durable resume.
2. Add push replication and continuous mode with longpoll + heartbeat.
3. SQLite adapter (modernc.org/sqlite) for PocketBase-style single binary.
4. Rust/WASM core for browser target; keep Go for server.
5. cdb interop test suite against real cdb.
## Additional builds (this pass)
- `pkg/checkpointer` — durable checkpoints in `_local` docs (compareHistory).
- Push/Pull helpers, continuous mode with exponential backoff.
- `pkg/adapter/sqlite.go` — SQLite adapter (modernc.org/sqlite, pure Go).
- `rust/` — pdb-core crate (collate + merge) building to native and wasm32-unknown-unknown.
