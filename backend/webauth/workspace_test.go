package webauth

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestMigrateLegacyMovesWALSidecars(t *testing.T) {
	old := authDB
	authDB = nil
	t.Cleanup(func() { authDB = old })

	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("data", "stock.db"), []byte("main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("data", "stock.db-wal"), []byte("wal"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("data", "stock.db-shm"), []byte("shm"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateLegacyIfNeeded(1); err != nil {
		t.Fatal(err)
	}
	dst := StockDBPath(1)
	for _, path := range []string{dst, dst + "-wal", dst + "-shm"} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing migrated file %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join("data", "stock.db.migrated")); err != nil {
		t.Fatal("expected migration marker")
	}
}

func TestMigrateLegacyOnlyFirstUser(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("WEB_ALLOW_REGISTER", "")
	t.Setenv("WEB_SETUP_SECRET", "setup-secret")
	if err := Init(filepath.Join(dir, "auth.db")); err != nil {
		t.Fatal(err)
	}
	first, err := Register("alice", "secret1", "setup-secret")
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateUser("bob", "secret2", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("data", "stock.db"), []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join("data", "stock.db.migrated"))

	if err := MigrateLegacyIfNeeded(second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(StockDBPath(second.ID)); err == nil {
		t.Fatal("second user must not receive legacy database")
	}
	if err := MigrateLegacyIfNeeded(first.ID); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(StockDBPath(first.ID))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "legacy" {
		t.Fatalf("first user db = %q", got)
	}
}

func TestRegisterSerializesFirstAdmin(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("WEB_ALLOW_REGISTER", "")
	t.Setenv("WEB_SETUP_SECRET", "setup-secret")
	if err := Init(filepath.Join(dir, "auth.db")); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	users := make(chan *User, 2)
	wg.Add(2)
	for i, name := range []string{"alice", "bob"} {
		name := name
		_ = i
		go func() {
			defer wg.Done()
			u, err := Register(name, "secret1", "setup-secret")
			if err != nil {
				errs <- err
				return
			}
			users <- u
		}()
	}
	wg.Wait()
	close(errs)
	close(users)

	var created []*User
	for u := range users {
		created = append(created, u)
	}
	if len(created) != 1 {
		t.Fatalf("expected exactly one successful first registration, got %d", len(created))
	}
	if !created[0].IsAdmin {
		t.Fatal("first user should be admin")
	}
	var n int
	for range errs {
		n++
	}
	if n != 1 {
		t.Fatalf("expected one rejected racer, got %d errors", n)
	}
}

func TestMigrateLegacyStopsWhenCopyFails(t *testing.T) {
	old := authDB
	authDB = nil
	t.Cleanup(func() { authDB = old })

	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("data", "stock.db"), []byte("main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("memory", 0o755); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join("memory", "blocked.bin")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o644) })

	err := MigrateLegacyIfNeeded(1)
	if err == nil {
		t.Fatal("expected memory copy failure")
	}
	if _, statErr := os.Stat(filepath.Join("data", "stock.db.migrated")); statErr == nil {
		t.Fatal("marker must not be written after failed memory copy")
	}
}

func TestEnforceTmpQuotaEvictsOldest(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < MaxUserTmpFiles; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%02d.txt", i))
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, time.Now().Add(-time.Duration(MaxUserTmpFiles-i)*time.Second), time.Now().Add(-time.Duration(MaxUserTmpFiles-i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if err := EnforceTmpQuota(dir, 1); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != MaxUserTmpFiles-1 {
		t.Fatalf("got %d files, want %d", len(entries), MaxUserTmpFiles-1)
	}
	if _, err := os.Stat(filepath.Join(dir, "00.txt")); err == nil {
		t.Fatal("oldest file should have been evicted")
	}
}
