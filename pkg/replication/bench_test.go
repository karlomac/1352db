package replication

import (
	"context"
	"fmt"
	"testing"

	"github.com/stonefish/pdb-go/pkg/adapter"
)

func BenchmarkReplication1000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		src := adapter.NewMemoryAdapter()
		tgt := adapter.NewMemoryAdapter()
		for j := 0; j < 1000; j++ {
			src.Put(context.Background(), adapter.Document{ID: fmt.Sprintf("doc%d", j), Extra: map[string]any{"n": j}})
		}
		r := New(src, tgt, Options{BatchSize: 100})
		r.Run(context.Background())
	}
}
