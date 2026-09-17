package data

import (
	"path/filepath"
	"testing"

	"go-stock/backend/tenant"
)

func TestTenantDataFileUsesWorkspace(t *testing.T) {
	if got := tenantDataFile("hot_money_seats.json"); got != filepath.Join("data", "hot_money_seats.json") {
		t.Fatalf("desktop path=%s", got)
	}
	root := t.TempDir()
	tenant.Bind(&tenant.Runtime{UserID: 3, Root: root})
	t.Cleanup(tenant.Unbind)
	want := filepath.Join(root, "data", "hot_money_seats.json")
	if got := tenantDataFile("hot_money_seats.json"); got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if tenantCacheKey("keydept:all") != "3:keydept:all" {
		t.Fatalf("cache key=%s", tenantCacheKey("keydept:all"))
	}
}
