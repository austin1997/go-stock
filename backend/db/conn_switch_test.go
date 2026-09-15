package db

import (
	"context"
	"path/filepath"
	"testing"

	"go-stock/backend/tenant"
)

type switchRow struct {
	ID    uint   `gorm:"primarykey"`
	Value string `gorm:"size:64"`
}

func (switchRow) TableName() string { return "switch_row" }

func TestConnPoolSwitchesByTenant(t *testing.T) {
	dir := t.TempDir()
	InitTenantShell(filepath.Join(dir, "shell.db"))

	aDB, err := Open(filepath.Join(dir, "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	bDB, err := Open(filepath.Join(dir, "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := aDB.AutoMigrate(&switchRow{}); err != nil {
		t.Fatal(err)
	}
	if err := bDB.AutoMigrate(&switchRow{}); err != nil {
		t.Fatal(err)
	}

	rtA := &tenant.Runtime{UserID: 1, DB: aDB, Root: filepath.Join(dir, "a")}
	rtB := &tenant.Runtime{UserID: 2, DB: bDB, Root: filepath.Join(dir, "b")}

	tenant.Bind(rtA)
	if err := Dao.Create(&switchRow{Value: "alice"}).Error; err != nil {
		t.Fatal(err)
	}
	tenant.Unbind()

	tenant.Bind(rtB)
	if err := Dao.Create(&switchRow{Value: "bob"}).Error; err != nil {
		t.Fatal(err)
	}
	var bob []switchRow
	if err := Dao.Find(&bob).Error; err != nil {
		t.Fatal(err)
	}
	if len(bob) != 1 || bob[0].Value != "bob" {
		t.Fatalf("user B saw %+v", bob)
	}
	tenant.Unbind()

	tenant.Bind(rtA)
	var alice []switchRow
	if err := Dao.Find(&alice).Error; err != nil {
		t.Fatal(err)
	}
	if len(alice) != 1 || alice[0].Value != "alice" {
		t.Fatalf("user A saw %+v", alice)
	}
	tenant.Unbind()
}

func TestConnPoolUsesContextWhenUnbound(t *testing.T) {
	dir := t.TempDir()
	InitTenantShell(filepath.Join(dir, "shell.db"))

	aDB, err := Open(filepath.Join(dir, "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := aDB.AutoMigrate(&switchRow{}); err != nil {
		t.Fatal(err)
	}
	rtA := &tenant.Runtime{UserID: 1, DB: aDB, Root: filepath.Join(dir, "a")}
	ctx := tenant.WithRuntime(context.Background(), rtA)

	if err := Dao.WithContext(ctx).Create(&switchRow{Value: "from-ctx"}).Error; err != nil {
		t.Fatal(err)
	}
	var rows []switchRow
	if err := aDB.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Value != "from-ctx" {
		t.Fatalf("context tenant missed write: %+v", rows)
	}
}
