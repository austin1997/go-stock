//go:build goweb
// +build goweb

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	log "go-stock/backend/logger"
	"go-stock/backend/machineid"
	"go-stock/backend/webauth"
	"go-stock/backend/webmode"
)

func main() {
	webmode.Enable()

	checkDir("data")
	checkDir("data/tmp")
	checkDir("data/users")
	checkDir("logs")
	checkDir("memory")
	checkDir("skills")

	machineid.Init(BuildKey)
	data.SponsorDecryptKeyHex = BuildKey
	data.SetAppIcon(icon)
	db.InitTenantShell("data/.web_shell.db")
	if err := webauth.Init("data/auth.db"); err != nil {
		log.SugaredLogger.Fatalf("auth db: %v", err)
	}
	if u, err := webauth.BootstrapAdminFromEnv(); err != nil {
		log.SugaredLogger.Fatalf("bootstrap admin: %v", err)
	} else if u != nil {
		log.SugaredLogger.Infof("admin user ready: %s", u.Username)
	}
	data.InitAnalyzeSentiment()

	log.SugaredLogger.Info("starting go-stock web...")
	log.SugaredLogger.Infof("version: %s  commit: %s", Version, VersionCommit)

	addr := os.Getenv("WEB_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	staticDir := os.Getenv("WEB_STATIC_DIR")
	if staticDir == "" {
		staticDir = "frontend/dist"
	}

	srv := newWebServer(staticDir)
	errCh := make(chan error, 1)
	go func() {
		log.SugaredLogger.Infof("go-stock web listening on %s", addr)
		errCh <- srv.ListenAndServe(addr)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.SugaredLogger.Fatal(err)
		}
	case <-sigCh:
		log.SugaredLogger.Info("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}
}
