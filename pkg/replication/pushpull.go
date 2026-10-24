package replication

import (
	"context"

	"github.com/stonefish/pdb-go/pkg/adapter"
)

// Pull replicates from src to target (pull replication).
func Pull(ctx context.Context, src, target adapter.Adapter, opts Options) Result {
	return New(src, target, opts).Run(ctx)
}

// Push replicates from src (local) to target (remote).
func Push(ctx context.Context, src, target adapter.Adapter, opts Options) Result {
	return New(src, target, opts).Run(ctx)
}
