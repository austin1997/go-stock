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

var errUserDisabled = fmt.Errorf("账号已禁用")

type userRuntime struct {
	once    sync.Once
	mu      sync.Mutex
	stopped bool
	err     error
	userID  uint
	app     *App
	gdb     *gorm.DB
	rt      *tenant.Runtime
}

func (item *userRuntime) markStopped() {
	item.mu.Lock()
	item.stopped = true
	item.mu.Unlock()
}

func (item *userRuntime) snapshot() (app *App, rt *tenant.Runtime, err error, stopped bool) {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.app, item.rt, item.err, item.stopped
}

func (item *userRuntime) setErr(err error) {
	item.mu.Lock()
	item.err = err
	item.mu.Unlock()
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
		item.mu.Lock()
		stopped := item.stopped
		item.mu.Unlock()
		if stopped {
			item.setErr(errUserDisabled)
			return
		}

		prepared, err := prepareUserWorkspace(userID)
		if err != nil {
			item.setErr(err)
			return
		}
		tenant.Bind(prepared.rt)
		defer tenant.Unbind()

		app := NewApp()
		ctx := tenant.WithRuntime(context.Background(), prepared.rt)
		app.startup(ctx)
		app.domReady(ctx)

		item.mu.Lock()
		if item.stopped {
			item.mu.Unlock()
			app.StopBackground()
			item.setErr(errUserDisabled)
			return
		}
		item.app = app
		item.gdb = prepared.gdb
		item.rt = prepared.rt
		item.mu.Unlock()

		go func() {
			tenant.Bind(prepared.rt)
			defer tenant.Unbind()
			_ = db.ClearExpiredStockTransactionCache()
		}()
		logger.SugaredLogger.Infof("user workspace ready: id=%d root=%s", userID, prepared.rt.Root)
	})
	app, _, err, stopped := item.snapshot()
	if err != nil || stopped || app == nil {
		m.mu.Lock()
		if m.items[userID] == item {
			delete(m.items, userID)
		}
		m.mu.Unlock()
		if err == nil {
			err = errUserDisabled
		}
		return nil, err
	}
	return item, nil
}

func (m *runtimeManager) stop(userID uint) {
	m.mu.Lock()
	item := m.items[userID]
	if item != nil {
		item.markStopped()
		delete(m.items, userID)
	}
	m.mu.Unlock()
	m.halt(item, false, nil)
}

func (m *runtimeManager) shutdown(ctx context.Context) {
	m.mu.Lock()
	items := make([]*userRuntime, 0, len(m.items))
	for _, item := range m.items {
		if item != nil {
			item.markStopped()
		}
		items = append(items, item)
	}
	m.items = map[uint]*userRuntime{}
	m.mu.Unlock()
	for _, item := range items {
		m.halt(item, true, ctx)
	}
}

func (m *runtimeManager) halt(item *userRuntime, shutdown bool, ctx context.Context) {
	if item == nil {
		return
	}
	item.markStopped()
	item.once.Do(func() {
		item.setErr(errUserDisabled)
	})
	app, rt, _, _ := item.snapshot()
	if app == nil {
		return
	}
	if rt != nil {
		tenant.Bind(rt)
		defer tenant.Unbind()
	}
	app.StopBackground()
	if shutdown {
		app.shutdown(ctx)
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
