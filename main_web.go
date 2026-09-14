//go:build goweb
// +build goweb

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	log "go-stock/backend/logger"
	"go-stock/backend/machineid"
	"go-stock/backend/webaddr"
	"go-stock/backend/webauth"
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

	addr := webaddr.FromEnv(os.Getenv("WEB_ADDR"))
	staticDir := os.Getenv("WEB_STATIC_DIR")
	if staticDir == "" {
		staticDir = "frontend/dist"
	}

	token, generated, err := webauth.ResolveToken("data/.web_auth_token")
	if err != nil {
		log.SugaredLogger.Fatalf("web auth token: %v", err)
	}
	if generated {
		log.SugaredLogger.Info("网页访问口令已写入 data/.web_auth_token，打开页面后用该口令登录；也可用环境变量 WEB_AUTH_TOKEN 指定")
	} else if strings.TrimSpace(os.Getenv("WEB_AUTH_TOKEN")) != "" {
		log.SugaredLogger.Info("网页访问口令来自 WEB_AUTH_TOKEN")
	} else {
		log.SugaredLogger.Info("网页访问口令来自 data/.web_auth_token")
	}
	if !webaddr.IsLoopback(addr) {
		log.SugaredLogger.Warn("WEB_ADDR 非本机回环，请确保仅受信网络可访问，且已设置访问口令")
	}

	srv := newWebServer(app, staticDir, webauth.New(token))
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
