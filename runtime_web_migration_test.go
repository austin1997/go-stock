//go:build goweb
// +build goweb

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/webauth"
)

func TestRuntimeInitializationAbortsOnLegacyMigrationFailureAndCanRetry(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("GO_STOCK_ROOT_DIR", dir)
	db.InitTenantShell(filepath.Join(dir, "shell.db"))

	if err := webauth.Init(filepath.Join(dir, "auth.db")); err != nil {
		t.Fatal(err)
	}
	user, err := webauth.CreateUser("alice", "secret1", true)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join("memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("memory", "legacy.txt"), []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	defaultSkill := filepath.Join(dir, "skills-default", "public", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(defaultSkill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defaultSkill, []byte("public"), 0o644); err != nil {
		t.Fatal(err)
	}

	marker := filepath.Join("data", "stock.db.migrated")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(marker), marker); err != nil {
		t.Fatal(err)
	}

	manager := newRuntimeManager()
	failedItem, err := manager.getItem(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.get(user.ID)
	if err == nil || !strings.Contains(err.Error(), "write migration marker") {
		t.Fatalf("initialization error = %v, want migration marker failure", err)
	}
	app, rt, gdb, _, _ := failedItem.snapshot()
	if app != nil || rt != nil || gdb != nil {
		t.Fatalf("failed initialization published runtime state: app=%p rt=%p db=%p", app, rt, gdb)
	}
	if _, err := os.Stat(filepath.Join(webauth.WorkspaceRoot(user.ID), "skills", "public", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("default skills copied after migration failure: %v", err)
	}
	if _, err := os.Stat(webauth.StockDBPath(user.ID)); !os.IsNotExist(err) {
		t.Fatalf("database opened after migration failure: %v", err)
	}
	manager.mu.Lock()
	_, retained := manager.items[user.ID]
	manager.mu.Unlock()
	if retained {
		t.Fatal("failed runtime item retained; retry would reuse consumed sync.Once")
	}

	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareUserWorkspace(user.ID)
	if err != nil {
		t.Fatalf("retry workspace preparation: %v", err)
	}
	t.Cleanup(func() { closeTenantDB(prepared.gdb) })
	if prepared.rt == nil || prepared.gdb == nil {
		t.Fatal("retry did not prepare database/runtime")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("retry did not write migration marker: %v", err)
	}
	if _, err := os.Stat(filepath.Join(webauth.WorkspaceRoot(user.ID), "skills", "public", "SKILL.md")); err != nil {
		t.Fatalf("retry did not copy default skills: %v", err)
	}
	if _, err := os.Stat(webauth.StockDBPath(user.ID)); err != nil {
		t.Fatalf("retry did not open database: %v", err)
	}
}
