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
	"go-stock/backend/webmode"
)

func main() {
	webmode.Enable()

	checkDir("data")
	checkDir("data/tmp")
	checkDir("logs")
	checkDir("memory")
	checkDir("skills")

	machineid.Init(BuildKey)
	data.SponsorDecryptKeyHex = BuildKey
	data.SetAppIcon(icon)
	db.Init("")
	data.InitAnalyzeSentiment()
	AutoMigrate()

	log.SugaredLogger.Info("starting go-stock web...")
	log.SugaredLogger.Infof("version: %s  commit: %s", Version, VersionCommit)

	app := NewApp()
	ctx := context.Background()
	app.startup(ctx)
	app.domReady(ctx)

	addr := os.Getenv("WEB_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	staticDir := os.Getenv("WEB_STATIC_DIR")
	if staticDir == "" {
		staticDir = "frontend/dist"
	}

	srv := newWebServer(app, staticDir)
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
		app.shutdown(ctx)
	}
}
