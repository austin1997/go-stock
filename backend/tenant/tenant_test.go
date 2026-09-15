package tenant

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestBindUnbind(t *testing.T) {
	rt := &Runtime{UserID: 7, Root: "/tmp/u7"}
	if Current() != nil {
		t.Fatal("expected no tenant")
	}
	Bind(rt)
	if UserID() != 7 || Root() != "/tmp/u7" {
		t.Fatalf("got uid=%d root=%s", UserID(), Root())
	}
	Unbind()
	if Current() != nil {
		t.Fatal("expected unbound")
	}
}

func TestNestedBind(t *testing.T) {
	rt := &Runtime{UserID: 1}
	Bind(rt)
	Bind(rt)
	Unbind()
	if UserID() != 1 {
		t.Fatal("nested unbind should keep binding")
	}
	Unbind()
	if Current() != nil {
		t.Fatal("expected fully unbound")
	}
}

func TestContextRoundTrip(t *testing.T) {
	rt := &Runtime{UserID: 3}
	ctx := WithRuntime(context.Background(), rt)
	got := FromContext(ctx)
	if got == nil || got.UserID != 3 {
		t.Fatalf("context runtime: %+v", got)
	}
}

func TestGoInherits(t *testing.T) {
	rt := &Runtime{UserID: 9}
	Bind(rt)
	defer Unbind()

	var wg sync.WaitGroup
	wg.Add(1)
	var got uint
	Go(func() {
		defer wg.Done()
		got = UserID()
	})
	wg.Wait()
	if got != 9 {
		t.Fatalf("inherited uid=%d", got)
	}
}

func TestGoContextFallsBackToContext(t *testing.T) {
	if Current() != nil {
		t.Fatal("expected unbound caller")
	}
	rt := &Runtime{UserID: 11}
	ctx := WithRuntime(context.Background(), rt)
	var wg sync.WaitGroup
	wg.Add(1)
	var got uint
	GoContext(ctx, func() {
		defer wg.Done()
		got = UserID()
	})
	wg.Wait()
	if got != 11 {
		t.Fatalf("context uid=%d", got)
	}
}

func TestSetHTTPTimeout(t *testing.T) {
	rt := &Runtime{UserID: 4}
	Bind(rt)
	defer Unbind()
	SetHTTPTimeout(12 * time.Second)
	if HTTPTimeout() != 12*time.Second {
		t.Fatalf("timeout=%v", HTTPTimeout())
	}
}
