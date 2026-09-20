package agent

import (
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/tenant"
	"go-stock/backend/webmode"
)

func TestRestrictWebUploadPath(t *testing.T) {
	dir := t.TempDir()
	upload := filepath.Join(dir, "tmp")
	if err := os.MkdirAll(upload, 0o755); err != nil {
		t.Fatal(err)
	}
	okFile := filepath.Join(upload, "doc.md")
	if err := os.WriteFile(okFile, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "secret.env")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	webmode.Enable()
	t.Cleanup(webmode.Disable)
	rt := &tenant.Runtime{UserID: 2, Root: dir}
	tenant.Bind(rt)
	t.Cleanup(tenant.Unbind)

	got, err := restrictWebUploadPath(okFile)
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("expected resolved path")
	}
	if _, err := restrictWebUploadPath(outside); err == nil {
		t.Fatal("outside path should be rejected")
	}
	if _, err := restrictWebUploadPath(filepath.Join(upload, "..", "secret.env")); err == nil {
		t.Fatal("escaped path should be rejected")
	}

	link := filepath.Join(upload, "link.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := restrictWebUploadPath(link); err == nil {
		t.Fatal("symlink should be rejected")
	}
	if _, err := restrictWebUploadPath(upload); err == nil {
		t.Fatal("directory should be rejected")
	}
}

func TestCurrentUserKeyUsesWebTenant(t *testing.T) {
	rt := &tenant.Runtime{UserID: 42, Root: t.TempDir()}
	tenant.Bind(rt)
	t.Cleanup(tenant.Unbind)
	if got := CurrentUserKey(""); got != "web:42" {
		t.Fatalf("got %q", got)
	}
}

func TestMemoryTenantKeyFollowsRoot(t *testing.T) {
	dir := t.TempDir()
	rt := &tenant.Runtime{UserID: 7, Root: dir}
	tenant.Bind(rt)
	t.Cleanup(tenant.Unbind)
	if got := memoryTenantKey(); got != dir {
		t.Fatalf("key=%q root=%q", got, dir)
	}
}
