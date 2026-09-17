package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/tenant"
	"go-stock/backend/webmode"

	"gorm.io/gorm"
)

func oauthWebTenant(t *testing.T, id uint, endpoint string) *tenant.Runtime {
	t.Helper()
	g, err := db.Open(filepath.Join(t.TempDir(), "tenant.db"))
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := g.DB()
	t.Cleanup(func() { sql.Close() })
	if err := g.AutoMigrate(&models.MCPServer{}); err != nil {
		t.Fatal(err)
	}
	if err := g.Create(&models.MCPServer{URL: endpoint, Name: "fixture", AuthType: "oauth"}).Error; err != nil {
		t.Fatal(err)
	}
	return &tenant.Runtime{UserID: id, DB: g}
}

func oauthWebProvider(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/oauth-protected-resource":
			json.NewEncoder(w).Encode(map[string]any{"authorization_servers": []string{srv.URL}})
		case "/.well-known/oauth-authorization-server":
			json.NewEncoder(w).Encode(map[string]any{"authorization_endpoint": srv.URL + "/authorize", "token_endpoint": srv.URL + "/token", "registration_endpoint": srv.URL + "/register"})
		case "/register":
			json.NewEncoder(w).Encode(map[string]any{"client_id": "fixture-client"})
		default:
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestMCPWebOAuthInitialStatusFailure(t *testing.T) {
	webmode.Enable()
	t.Cleanup(webmode.Disable)
	t.Setenv("WEB_PUBLIC_ORIGIN", "https://stocks.example.test")
	provider := oauthWebProvider(t)
	rt := oauthWebTenant(t, 998, provider.URL)
	old := db.Dao
	db.Dao = rt.DB
	t.Cleanup(func() { db.Dao = old })
	injected := errors.New("injected initial status failure")
	if err := rt.DB.Callback().Update().Before("gorm:update").Register("test:initial", func(tx *gorm.DB) {
		if values, ok := tx.Statement.Dest.(map[string]any); ok && values["status"] == "testing" {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	tenant.Bind(rt)
	auth, err := NewMCPServerApi().startWebOAuth(context.Background(), 1)
	tenant.Unbind()
	key := webOAuthKey{998, 1}
	webOAuthMu.Lock()
	flow := webOAuthFlows[key]
	if flow != nil {
		flow.timer.Stop()
		delete(webOAuthFlows, key)
	}
	webOAuthMu.Unlock()
	if !errors.Is(err, injected) || auth != "" || flow != nil {
		t.Fatalf("initial write failure: auth=%q err=%v orphan=%v", auth, err, flow != nil)
	}
}

func TestMCPWebOAuthTimerWriteRetry(t *testing.T) {
	rt := oauthWebTenant(t, 1000, "https://provider.test")
	key := webOAuthKey{1000, 1}
	var attempts atomic.Int32
	if err := rt.DB.Callback().Update().Before("gorm:update").Register("test:timer", func(tx *gorm.DB) {
		if values, ok := tx.Statement.Dest.(map[string]any); ok && values["status"] == "unauthorized" {
			if attempts.Add(1) == 1 {
				tx.AddError(errors.New("injected timer failure"))
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	synctest.Test(t, func(t *testing.T) {
		flow := &webOAuthFlow{key: key, rt: rt, state: "timer", expires: time.Now().Add(webOAuthTTL)}
		webOAuthMu.Lock()
		webOAuthFlows[key] = flow
		flow.timer = time.AfterFunc(webOAuthTTL, func() { expireWebOAuth(flow) })
		webOAuthMu.Unlock()
		defer func() { flow.timer.Stop(); webOAuthMu.Lock(); delete(webOAuthFlows, key); webOAuthMu.Unlock() }()
		time.Sleep(webOAuthTTL)
		synctest.Wait()
		if attempts.Load() != 1 {
			t.Fatalf("timer attempts = %d", attempts.Load())
		}
		time.Sleep(time.Second)
		synctest.Wait()
		var server models.MCPServer
		if err := rt.DB.First(&server, 1).Error; err != nil {
			t.Fatal(err)
		}
		webOAuthMu.Lock()
		remaining := webOAuthFlows[key]
		if remaining != nil {
			remaining.timer.Stop()
			delete(webOAuthFlows, key)
		}
		webOAuthMu.Unlock()
		if attempts.Load() != 2 || remaining != nil || server.Status != "unauthorized" || server.TestResult != "授权超时未完成，请重试" {
			t.Fatalf("timer recovery: attempts=%d remaining=%v status=%s result=%s", attempts.Load(), remaining != nil, server.Status, server.TestResult)
		}
	})
}

func TestMCPWebOAuthTerminalWriteRetry(t *testing.T) {
	for _, expired := range []bool{false, true} {
		for _, outcome := range []string{"recover", "replace", "exhaust"} {
			t.Run(fmt.Sprintf("expired=%v/%s", expired, outcome), func(t *testing.T) {
				rt := oauthWebTenant(t, 999, "https://provider.test")
				if err := rt.DB.Model(&models.MCPServer{}).Where("id = ?", 1).Update("status", "testing").Error; err != nil {
					t.Fatal(err)
				}
				attempts := 0
				if err := rt.DB.Callback().Update().Before("gorm:update").Register("test:terminal", func(tx *gorm.DB) {
					values, ok := tx.Statement.Dest.(map[string]any)
					if !ok || values["status"] != "unauthorized" {
						return
					}
					attempts++
					if attempts == 1 || outcome == "exhaust" {
						tx.AddError(errors.New("injected terminal failure"))
					}
				}); err != nil {
					t.Fatal(err)
				}
				synctest.Test(t, func(t *testing.T) {
					key := webOAuthKey{999, 1}
					flow := &webOAuthFlow{key: key, rt: rt, state: "retry", expires: time.Now().Add(time.Minute), timer: time.AfterFunc(time.Hour, func() {})}
					if expired {
						flow.expires = time.Now().Add(-time.Second)
					}
					webOAuthMu.Lock()
					webOAuthFlows[key] = flow
					webOAuthMu.Unlock()
					defer func() { flow.timer.Stop(); webOAuthMu.Lock(); delete(webOAuthFlows, key); webOAuthMu.Unlock() }()
					done := make(chan struct{})
					go func() {
						defer close(done)
						HandleWebMCPOAuthCallback(httptest.NewRecorder(), httptest.NewRequest("GET", MCPOAuthCallbackPath+"?state=retry&error=denied", nil), 999)
					}()
					synctest.Wait()
					if attempts != 1 {
						t.Fatalf("first attempts = %d", attempts)
					}
					webOAuthMu.Lock()
					retained := webOAuthFlows[key] == flow && flow.state == ""
					webOAuthMu.Unlock()
					if !retained {
						t.Fatal("failed terminal write discarded flow instead of retaining consumed reconciliation")
					}
					w := httptest.NewRecorder()
					HandleWebMCPOAuthCallback(w, httptest.NewRequest("GET", MCPOAuthCallbackPath+"?state=retry&code=replay", nil), 999)
					if w.Code != 400 {
						t.Fatal("retry reenabled consumed state")
					}
					var replacement *webOAuthFlow
					if outcome == "replace" {
						replacement = &webOAuthFlow{key: key, rt: rt, state: "new"}
						webOAuthMu.Lock()
						webOAuthFlows[key] = replacement
						err := rt.DB.Model(&models.MCPServer{}).Where("id = ?", 1).Updates(map[string]any{"status": "testing", "test_result": "new authorization"}).Error
						webOAuthMu.Unlock()
						if err != nil {
							t.Fatal(err)
						}
					}
					<-done
					var server models.MCPServer
					if err := rt.DB.First(&server, 1).Error; err != nil {
						t.Fatal(err)
					}
					webOAuthMu.Lock()
					remaining := webOAuthFlows[key]
					webOAuthMu.Unlock()
					switch outcome {
					case "recover":
						if attempts != 2 || server.Status != "unauthorized" || remaining != nil {
							t.Fatalf("recovery: attempts=%d status=%s remaining=%v", attempts, server.Status, remaining != nil)
						}
					case "replace":
						if attempts != 1 || remaining != replacement || replacement.state != "new" || server.TestResult != "new authorization" {
							t.Fatal("stale retry overwrote replacement")
						}
					case "exhaust":
						if attempts != 3 || remaining != nil {
							t.Fatalf("unbounded retry/leak: attempts=%d remaining=%v", attempts, remaining != nil)
						}
					}
				})
			})
		}
	}
}

func TestMCPWebOAuthRejectsUnsafeOrigin(t *testing.T) {
	for _, origin := range []string{"", "//stocks.test", "https://user:pass@stocks.test", "https://stocks.test/path", "https://stocks.test?x=1", "https://stocks.test#fragment", "http://stocks.test", "https://stocks.test#", "https://stocks.test:0", "https://stocks.test:65536"} {
		t.Run(origin, func(t *testing.T) {
			t.Setenv("WEB_PUBLIC_ORIGIN", origin)
			if got, err := webOAuthRedirectURI(); err == nil {
				t.Fatalf("unsafe origin accepted: %q -> %s", origin, got)
			}
		})
	}
	for _, origin := range []string{"https://stocks.test", "http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080"} {
		t.Setenv("WEB_PUBLIC_ORIGIN", origin)
		if _, err := webOAuthRedirectURI(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMCPWebOAuthExpiredCallbackReleasesFlow(t *testing.T) {
	rt := oauthWebTenant(t, 991, "https://provider.test")
	if err := rt.DB.Model(&models.MCPServer{}).Where("id = ?", 1).Updates(map[string]any{"status": "testing", "test_result": "waiting"}).Error; err != nil {
		t.Fatal(err)
	}
	key := webOAuthKey{userID: 991, serverID: 1}
	flow := &webOAuthFlow{key: key, rt: rt, state: "expired", expires: time.Now().Add(-time.Second), timer: time.AfterFunc(time.Hour, func() {})}
	webOAuthMu.Lock()
	webOAuthFlows[key] = flow
	webOAuthMu.Unlock()
	defer func() { flow.timer.Stop(); webOAuthMu.Lock(); delete(webOAuthFlows, key); webOAuthMu.Unlock() }()
	w := httptest.NewRecorder()
	HandleWebMCPOAuthCallback(w, httptest.NewRequest("GET", MCPOAuthCallbackPath+"?state=expired&code=unused", nil), 991)
	if w.Code != 400 {
		t.Fatalf("expired state accepted: %d", w.Code)
	}
	var server models.MCPServer
	if err := rt.DB.First(&server, 1).Error; err != nil {
		t.Fatal(err)
	}
	if server.Status != "unauthorized" || server.TestResult != "授权超时未完成，请重试" {
		t.Fatalf("expired callback left status=%q result=%q", server.Status, server.TestResult)
	}
	webOAuthMu.Lock()
	defer webOAuthMu.Unlock()
	if webOAuthFlows[key] != nil {
		t.Fatal("expired callback retained flow and timer")
	}
}

func TestMCPWebOAuthRejectsReplayWhileExchangePending(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		json.NewEncoder(w).Encode(map[string]string{"access_token": "fixture"})
	}))
	defer srv.Close()
	rt := oauthWebTenant(t, 993, srv.URL)
	key := webOAuthKey{993, 1}
	flow := &webOAuthFlow{key: key, rt: rt, state: "once", expires: time.Now().Add(time.Minute), timer: time.AfterFunc(time.Hour, func() {}), config: MCPAuthConfig{TokenURL: srv.URL}}
	webOAuthMu.Lock()
	webOAuthFlows[key] = flow
	webOAuthMu.Unlock()
	done := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		HandleWebMCPOAuthCallback(w, httptest.NewRequest("GET", MCPOAuthCallbackPath+"?state=once&code=fixture", nil), 993)
		done <- w.Code
	}()
	<-started
	w := httptest.NewRecorder()
	HandleWebMCPOAuthCallback(w, httptest.NewRequest("GET", MCPOAuthCallbackPath+"?state=once&code=fixture", nil), 993)
	close(release)
	if w.Code != 400 {
		t.Fatalf("concurrent replay accepted: %d", w.Code)
	}
	if code := <-done; code != 200 {
		t.Fatalf("original exchange: %d", code)
	}
}

func TestMCPWebOAuthDenialCannotReflectHTML(t *testing.T) {
	rt := oauthWebTenant(t, 994, "https://provider.test")
	key := webOAuthKey{994, 1}
	flow := &webOAuthFlow{key: key, rt: rt, state: "denial", expires: time.Now().Add(time.Minute), timer: time.AfterFunc(time.Hour, func() {})}
	webOAuthMu.Lock()
	webOAuthFlows[key] = flow
	webOAuthMu.Unlock()
	w := httptest.NewRecorder()
	HandleWebMCPOAuthCallback(w, httptest.NewRequest("GET", MCPOAuthCallbackPath+"?state=denial&error="+url.QueryEscape("<script>alert(1)</script>"), nil), 994)
	if w.Code != 400 || strings.Contains(w.Body.String(), "<script>") {
		t.Fatalf("unsafe denial response: %s", w.Body.String())
	}
	webOAuthMu.Lock()
	defer webOAuthMu.Unlock()
	if webOAuthFlows[key] != nil {
		t.Fatal("denial not consumed")
	}
}

func TestMCPWebOAuthFailedCallbackPersistsTerminalStatus(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		status      int
	}{
		{"denial", "&error=access_denied", 400},
		{"missing_code", "", 400},
		{"exchange_failure", "&code=bad", 502},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "invalid grant", http.StatusBadRequest)
			}))
			defer srv.Close()
			rt := oauthWebTenant(t, 995, srv.URL)
			other := oauthWebTenant(t, 996, srv.URL)
			for _, runtime := range []*tenant.Runtime{rt, other} {
				if err := runtime.DB.Model(&models.MCPServer{}).Where("id = ?", 1).Updates(map[string]any{"status": "testing", "test_result": "waiting"}).Error; err != nil {
					t.Fatal(err)
				}
			}
			// Callback must use the captured runtime, not the current tenant.
			tenant.Bind(other)
			defer tenant.Unbind()
			key := webOAuthKey{995, 1}
			flow := &webOAuthFlow{key: key, rt: rt, state: "failure", expires: time.Now().Add(time.Minute), timer: time.AfterFunc(time.Hour, func() {}), config: MCPAuthConfig{TokenURL: srv.URL}}
			webOAuthMu.Lock()
			webOAuthFlows[key] = flow
			webOAuthMu.Unlock()
			defer func() { flow.timer.Stop(); webOAuthMu.Lock(); delete(webOAuthFlows, key); webOAuthMu.Unlock() }()
			w := httptest.NewRecorder()
			HandleWebMCPOAuthCallback(w, httptest.NewRequest("GET", MCPOAuthCallbackPath+"?state=failure"+tc.query, nil), 995)
			if w.Code != tc.status {
				t.Fatalf("callback status = %d, want %d", w.Code, tc.status)
			}
			var server models.MCPServer
			if err := rt.DB.First(&server, 1).Error; err != nil {
				t.Fatal(err)
			}
			if server.Status != "unauthorized" || server.TestResult != strings.TrimSpace(w.Body.String()) {
				t.Fatalf("consumed failure left status=%q result=%q; want unauthorized and %q", server.Status, server.TestResult, strings.TrimSpace(w.Body.String()))
			}
			var untouched models.MCPServer
			if err := other.DB.First(&untouched, 1).Error; err != nil {
				t.Fatal(err)
			}
			if untouched.Status != "testing" || untouched.TestResult != "waiting" {
				t.Fatalf("callback changed another tenant: %+v", untouched)
			}
			webOAuthMu.Lock()
			remaining := webOAuthFlows[key]
			webOAuthMu.Unlock()
			if remaining != nil {
				t.Fatal("failed callback retained consumed flow")
			}
		})
	}
}

func TestMCPWebOAuthFailedCallbackCannotOverwriteReplacement(t *testing.T) {
	webmode.Enable()
	t.Cleanup(webmode.Disable)
	t.Setenv("WEB_PUBLIC_ORIGIN", "https://stocks.example.test")
	started, release := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		http.Error(w, "invalid grant", http.StatusBadRequest)
	}))
	defer srv.Close()
	provider := oauthWebProvider(t)
	rt := oauthWebTenant(t, 997, provider.URL)
	old := db.Dao
	db.Dao = rt.DB
	t.Cleanup(func() { db.Dao = old })
	key := webOAuthKey{997, 1}
	flow := &webOAuthFlow{key: key, rt: rt, state: "old", expires: time.Now().Add(time.Minute), timer: time.AfterFunc(time.Hour, func() {}), config: MCPAuthConfig{TokenURL: srv.URL}}
	webOAuthMu.Lock()
	webOAuthFlows[key] = flow
	webOAuthMu.Unlock()
	defer func() {
		flow.timer.Stop()
		webOAuthMu.Lock()
		defer webOAuthMu.Unlock()
		if f := webOAuthFlows[key]; f != nil {
			f.timer.Stop()
			delete(webOAuthFlows, key)
		}
	}()
	done := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		HandleWebMCPOAuthCallback(w, httptest.NewRequest("GET", MCPOAuthCallbackPath+"?state=old&code=bad", nil), 997)
		done <- w.Code
	}()
	<-started
	tenant.Bind(rt)
	auth, err := NewMCPServerApi().startWebOAuth(context.Background(), 1)
	tenant.Unbind()
	// Always release the HTTP handler before any fatal assertion.
	close(release)
	code := <-done
	if err != nil {
		t.Fatal(err)
	}
	if code != 502 {
		t.Fatalf("old exchange status = %d", code)
	}
	u, err := url.Parse(auth)
	if err != nil {
		t.Fatal(err)
	}
	webOAuthMu.Lock()
	replacement := webOAuthFlows[key]
	validReplacement := replacement != nil && replacement != flow && replacement.state == u.Query().Get("state")
	webOAuthMu.Unlock()
	if !validReplacement {
		t.Fatal("failed callback removed or consumed replacement")
	}
	var server models.MCPServer
	if err := rt.DB.First(&server, 1).Error; err != nil {
		t.Fatal(err)
	}
	if server.Status != "testing" || server.TestResult != "等待浏览器完成授权..." {
		t.Fatalf("stale callback overwrote replacement: status=%q result=%q", server.Status, server.TestResult)
	}
	cfg, err := decryptAuthConfig(server.AuthConfig)
	if err != nil || cfg.TokenURL != provider.URL+"/token" {
		t.Fatalf("replacement credentials changed: cfg=%+v err=%v", cfg, err)
	}
}

func TestMCPDesktopOAuthKeepsLoopback(t *testing.T) {
	webmode.Disable()
	provider := oauthWebProvider(t)
	rt := oauthWebTenant(t, 0, provider.URL)
	old := db.Dao
	db.Dao = rt.DB
	t.Cleanup(func() { db.Dao = old })
	auth, err := NewMCPServerApi().StartOAuth(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	oauthFlowMu.Lock()
	flow := oauthFlows[1]
	oauthFlowMu.Unlock()
	defer cleanupFlow(1, flow)
	u, _ := url.Parse(auth)
	if !strings.HasPrefix(u.Query().Get("redirect_uri"), "http://localhost:1896") {
		t.Fatal("desktop loopback changed")
	}
}

func TestMCPWebOAuthUsesConfiguredPublicCallback(t *testing.T) {
	webmode.Enable()
	t.Cleanup(webmode.Disable)
	t.Setenv("WEB_PUBLIC_ORIGIN", "https://stocks.example.test")
	provider := oauthWebProvider(t)
	rt := oauthWebTenant(t, 11, provider.URL)
	old := db.Dao
	db.Dao = rt.DB
	t.Cleanup(func() { db.Dao = old })
	tenant.Bind(rt)
	defer tenant.Unbind()
	auth, err := NewMCPServerApi().StartOAuth(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(auth)
	t.Cleanup(func() {
		webOAuthMu.Lock()
		defer webOAuthMu.Unlock()
		key := webOAuthKey{rt.UserID, 1}
		if f := webOAuthFlows[key]; f != nil {
			f.timer.Stop()
			delete(webOAuthFlows, key)
		}
	})
	if got := u.Query().Get("redirect_uri"); got != "https://stocks.example.test/api/mcp/oauth/callback" {
		t.Fatalf("web callback is not reachable through published HTTP server: %s", got)
	}
}
