package agent

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/philippgille/chromem-go"

	"go-stock/backend/tenant"
)

func TestEmbeddingCacheUnboundWorkerTenantIsolation(t *testing.T) {
	InvalidateEmbeddingCache()
	t.Cleanup(InvalidateEmbeddingCache)

	type embeddingOwner struct {
		name    string
		uid     uint
		vector  []float32
		calls   int64
		wrapped chromem.EmbeddingFunc
	}
	owners := []*embeddingOwner{
		{name: "desktop", vector: []float32{1, 0, 0}},
		{name: "tenant-1", uid: 101, vector: []float32{0, 1, 0}},
		{name: "tenant-2", uid: 202, vector: []float32{0, 0, 1}},
	}
	for _, owner := range owners {
		// KB collections retain this callback after creation; embedding workers
		// launched with plain go do not retain the caller's tenant binding.
		func() {
			if owner.uid != 0 {
				tenant.Bind(&tenant.Runtime{UserID: owner.uid})
				defer tenant.Unbind()
			}
			owner.wrapped = wrapEmbedFuncWithCache(func(context.Context, string) ([]float32, error) {
				atomic.AddInt64(&owner.calls, 1)
				return append([]float32(nil), owner.vector...), nil
			}, "same-model")
		}()
	}

	// Seed legacy cache first, then both tenants, and revisit all owners. All
	// callbacks use identical model/text, but only their own cached vector.
	for round := 0; round < 2; round++ {
		for _, owner := range owners {
			type result struct {
				uid    uint
				vector []float32
				err    error
			}
			done := make(chan result, 1)
			go func() {
				vector, err := owner.wrapped(context.Background(), "same text")
				done <- result{uid: tenant.UserID(), vector: vector, err: err}
			}()
			select {
			case got := <-done:
				if got.uid != 0 {
					t.Fatalf("worker unexpectedly bound to tenant %d", got.uid)
				}
				if got.err != nil {
					t.Fatalf("%s round %d: %v", owner.name, round, got.err)
				}
				if !reflect.DeepEqual(got.vector, owner.vector) {
					t.Errorf("%s round %d embedding = %v, want %v", owner.name, round, got.vector, owner.vector)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("embedding worker did not finish")
			}
		}
	}
	for _, owner := range owners {
		if calls := atomic.LoadInt64(&owner.calls); calls != 1 {
			t.Errorf("%s embedding calls = %d, want 1 (second call must hit its own cache)", owner.name, calls)
		}
	}
}
