package webdownload

import (
	"strings"
	"testing"

	"go-stock/backend/tenant"
)

func TestPutTakeRoundTrip(t *testing.T) {
	resetStore()
	id := Put("a.txt", []byte("hello"))
	if id == "" {
		t.Fatal("put failed")
	}
	name, content, ok := Take(id)
	if !ok || name != "a.txt" || string(content) != "hello" {
		t.Fatalf("take: ok=%v name=%q content=%q", ok, name, content)
	}
	if _, _, ok := Take(id); ok {
		t.Fatal("second take should miss")
	}
}

func TestPutRejectsOversizedItem(t *testing.T) {
	resetStore()
	if id := Put("big.bin", make([]byte, maxDownloadBytes+1)); id != "" {
		t.Fatal("oversize item should be rejected")
	}
}

func TestPutEvictsWhenUserQuotaExceeded(t *testing.T) {
	resetStore()
	tenant.Bind(&tenant.Runtime{UserID: 7})
	t.Cleanup(tenant.Unbind)

	first := Put("one.bin", make([]byte, 20<<20))
	if first == "" {
		t.Fatal("first put failed")
	}
	second := Put("two.bin", make([]byte, 20<<20))
	if second == "" {
		t.Fatal("second put should evict and succeed")
	}
	if _, _, ok := Take(first); ok {
		t.Fatal("first item should have been evicted for user quota")
	}
	if _, content, ok := Take(second); !ok || len(content) != 20<<20 {
		t.Fatal("second item should remain")
	}
}

func TestPutIsolatesTenantQuota(t *testing.T) {
	resetStore()
	tenant.Bind(&tenant.Runtime{UserID: 1})
	a := Put("a.bin", make([]byte, 20<<20))
	tenant.Unbind()
	tenant.Bind(&tenant.Runtime{UserID: 2})
	b := Put("b.bin", make([]byte, 20<<20))
	tenant.Unbind()
	if a == "" || b == "" {
		t.Fatal("both tenants should be able to cache 20MiB")
	}
	if _, _, ok := Take(a); !ok {
		t.Fatal("tenant 1 item should still be present")
	}
	if _, _, ok := Take(b); !ok {
		t.Fatal("tenant 2 item should still be present")
	}
}

func TestPutAndFormatToken(t *testing.T) {
	resetStore()
	got := PutAndFormat("报表.xlsx", []byte("x"))
	if !strings.HasPrefix(got, Prefix) || !strings.Contains(got, ":报表.xlsx") {
		t.Fatalf("token=%q", got)
	}
}
