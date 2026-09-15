//go:build goweb
// +build goweb

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gorm.io/gorm"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/tenant"
	"go-stock/backend/webauth"
)

type userRuntime struct {
	once   sync.Once
	err    error
	userID uint
	app    *App
	gdb    *gorm.DB
	rt     *tenant.Runtime
}

type runtimeManager struct {
	mu    sync.Mutex
	items map[uint]*userRuntime
}

func newRuntimeManager() *runtimeManager {
	return &runtimeManager{items: map[uint]*userRuntime{}}
}

func (m *runtimeManager) get(userID uint) (*userRuntime, error) {
	m.mu.Lock()
	item, ok := m.items[userID]
	if !ok {
		item = &userRuntime{userID: userID}
		m.items[userID] = item
	}
	m.mu.Unlock()

	item.once.Do(func() {
		prepared, err := prepareUserWorkspace(userID)
		if err != nil {
			item.err = err
			return
		}
		tenant.Bind(prepared.rt)
		defer tenant.Unbind()

		app := NewApp()
		ctx := tenant.WithRuntime(context.Background(), prepared.rt)
		app.startup(ctx)
		app.domReady(ctx)
		go func() {
			tenant.Bind(prepared.rt)
			defer tenant.Unbind()
			_ = db.ClearExpiredStockTransactionCache()
		}()
		item.app = app
		item.gdb = prepared.gdb
		item.rt = prepared.rt
		logger.SugaredLogger.Infof("user workspace ready: id=%d root=%s", userID, prepared.rt.Root)
	})
	if item.err != nil {
		m.mu.Lock()
		if m.items[userID] == item {
			delete(m.items, userID)
		}
		m.mu.Unlock()
		return nil, item.err
	}
	return item, nil
}

func (m *runtimeManager) shutdown(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range m.items {
		if item == nil || item.app == nil {
			continue
		}
		tenant.Bind(item.rt)
		item.app.shutdown(ctx)
		tenant.Unbind()
	}
}

type preparedWorkspace struct {
	gdb *gorm.DB
	rt  *tenant.Runtime
}

func prepareUserWorkspace(userID uint) (*preparedWorkspace, error) {
	if err := webauth.EnsureWorkspaceDirs(userID); err != nil {
		return nil, err
	}
	if err := webauth.MigrateLegacyIfNeeded(userID); err != nil {
		logger.SugaredLogger.Warnf("legacy workspace migrate: %v", err)
	}
	if err := webauth.CopyDefaultSkills(userID); err != nil {
		logger.SugaredLogger.Warnf("copy default skills: %v", err)
	}

	gdb, err := db.Open(webauth.StockDBPath(userID))
	if err != nil {
		return nil, fmt.Errorf("open user db: %w", err)
	}
	root, err := filepath.Abs(webauth.WorkspaceRoot(userID))
	if err != nil {
		root = webauth.WorkspaceRoot(userID)
	}
	rt := &tenant.Runtime{UserID: userID, DB: gdb, Root: root}
	tenant.Bind(rt)
	defer tenant.Unbind()
	db.AutoMigrate()
	AutoMigrate()
	return &preparedWorkspace{gdb: gdb, rt: rt}, nil
}

func userUploadDir(userID uint) string {
	dir := filepath.Join(webauth.WorkspaceRoot(userID), "tmp")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}
