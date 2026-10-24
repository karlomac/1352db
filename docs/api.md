# API reference

## adapter
```go
mem := adapter.NewMemoryAdapter()
http := adapter.NewHTTPAdapter("http://localhost:5984", "mydb")
sqlite, err := adapter.NewSQLiteAdapter("/data/db.sqlite", "mydb")
```

## replication
```go
res := replication.Pull(ctx, src, tgt, replication.Options{
    BatchSize: 100,
    Checkpoint: "both", // "both"|"source"|"target"|"none"
})
// res.DocsRead, res.DocsWritten, res.LastSeq, res.OK, res.Errors
```
Push: `replication.Push(ctx, src, tgt, opts)` (same engine, src→tgt).
Continuous: `replication.Continuous(ctx, src, tgt, opts, func(r replication.Result){})`.

## checkpointer
```go
cp := checkpointer.New(src, tgt, "repl-id", checkpointer.DefaultOpts())
seq, _ := cp.GetCheckpoint()
cp.WriteCheckpoint("42", sessionID)
```

## collate
```go
cmp := collate.Compare(a, b) // -1/0/1
collate.Less(a,b); collate.Equal(a,b)
```

## merge
```go
tree := merge.BuildTree(revs)
winner, conflicts := merge.FindWinningRev(tree)
info := merge.TreeInfoFromRevs(revs)
gen, hash := merge.ParseRev("3-abc")
```

## Rust (pdb-core)
```rust
use pdb_core::collate::compare;
use pdb_core::merge::find_winning_rev;
```
Build: `cargo build --release` (native) or `--target wasm32-unknown-unknown` (WASM).<|end|>