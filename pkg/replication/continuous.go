package replication

import (
	"context"
	"time"

	"github.com/stonefish/pdb-go/pkg/adapter"
)

// Continuous runs replication repeatedly until ctx is cancelled, applying
// exponential backoff on errors.
func Continuous(ctx context.Context, src, target adapter.Adapter, opts Options, onResult func(Result)) {
	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		res := New(src, target, opts).Run(ctx)
		if onResult != nil {
			onResult(res)
		}
		if !res.OK {
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		// wait briefly before next round
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}
