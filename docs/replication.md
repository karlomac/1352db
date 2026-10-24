# Replication protocol

Implements cdb-compatible pull/push replication.

## Flow (one round)
1. Generate replication id from source/target + opts.
2. `GetCheckpoint()` reads `_local/<replid>` docs on both ends and picks resume seq via `compareHistory` (CouchDB algorithm).
3. `src.Changes({since, limit: batch_size})` in batches, up to `batches_limit` pipelined (serial here).
4. `target.RevsDiff(diff)` finds missing revisions.
5. `src.BulkGet(missing)` fetches full docs (with attachments inline base64).
6. `target.BulkDocs(docs, new_edits:false)` writes replicated revs.
7. Emit `change` per batch; `WriteCheckpoint(seq, session)`.
8. On error: backoff + retry (continuous mode).

## Continuous mode
`replication.Continuous(ctx, src, target, opts, onResult)` loops rounds with exponential backoff (1s→30s) on failure and 1s idle between successful rounds.

## Checkpoint document (`_local/<replid>`)
```
{
  "session_id": "<uuid>",
  "last_seq": "42",
  "version": 1,
  "replicator": "pdb",
  "history": [{"last_seq":"42","session_id":"..."}, ...]  // max 5
}
```
`compareHistory` walks source/target history to find a shared session id and resume from the matching seq; falls back to 0.

## Conflict resolution
`merge.FindWinningRev` picks winner by: longest branch (depth), then highest generation, then lexicographically largest rev. Conflicts = remaining leaves.

## Attachments
Travel inline as base64 in `_attachments`. `getDocs` (JS) fetches missing attachments from source or reuses target's when unchanged; this prototype preserves `_attachments` on roundtrip. Streaming not implemented.