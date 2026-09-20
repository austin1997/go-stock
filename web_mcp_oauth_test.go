//go:build goweb

package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/tenant"
	"go-stock/backend/webauth"
	"go-stock/backend/webmode"
)

func TestWebMCPOAuthPublishedCallback(t *testing.T) {
	webmode.Enable()
	defer webmode.Disable()
	oldDB := db.Dao
	db.InitTenantShell(filepath.Join(t.TempDir(), "shell.db"))
	defer func() { sql, _ := db.Dao.DB(); sql.Close(); db.Dao = oldDB }()
	authPath := filepath.Join(t.TempDir(), "auth.db")
	if err := webauth.Init(authPath); err != nil {
		t.Fatal(err)
	}
	authDB, err := db.Open(authPath)
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := authDB.DB()
	defer sql.Close()
	for _, id := range []uint{41, 42} {
		if err := authDB.Create(&webauth.User{ID: id, Username: map[uint]string{41: "alice", 42: "bob"}[id]}).Error; err != nil {
			t.Fatal(err)
		}
	}
	sessions := map[uint]string{}
	for _, id := range []uint{41, 42} {
		s, e := webauth.CreateSession(id)
		if e != nil {
			t.Fatal(e)
		}
		sessions[id] = s.Token
	}
	var exchanges atomic.Int32
	challenges := map[string]string{}
	var provider *httptest.Server
	provider = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-protected-resource":
			json.NewEncoder(w).Encode(map[string]any{"authorization_servers": []string{provider.URL}})
		case "/.well-known/oauth-authorization-server":
			json.NewEncoder(w).Encode(map[string]string{"authorization_endpoint": provider.URL + "/authorize", "token_endpoint": provider.URL + "/token", "registration_endpoint": provider.URL + "/register"})
		case "/register":
			json.NewEncoder(w).Encode(map[string]string{"client_id": "local-fixture"})
		case "/token":
			r.ParseForm()
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if base64.RawURLEncoding.EncodeToString(sum[:]) != challenges[r.Form.Get("code")] {
				t.Error("PKCE verifier not bound to flow")
			}
			if r.Form.Get("redirect_uri") != tPublicCallback {
				t.Errorf("wrong exchange redirect: %s", r.Form.Get("redirect_uri"))
			}
			exchanges.Add(1)
			json.NewEncoder(w).Encode(map[string]string{"access_token": "token-" + r.Form.Get("code"), "token_type": "Bearer"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()
	s := newWebServer(t.TempDir())
	published := httptest.NewServer(s.http.Handler)
	defer published.Close()
	t.Setenv("WEB_PUBLIC_ORIGIN", "https://stocks.example.test")
	runtimes := map[uint]*tenant.Runtime{}
	states := map[uint]string{}
	for _, id := range []uint{41, 42} {
		g, e := db.Open(filepath.Join(t.TempDir(), "tenant.db"))
		if e != nil {
			t.Fatal(e)
		}
		sq, _ := g.DB()
		defer sq.Close()
		g.AutoMigrate(&models.MCPServer{})
		g.Create(&models.MCPServer{Name: "fixture", URL: provider.URL, AuthType: "oauth"})
		rt := &tenant.Runtime{UserID: id, DB: g}
		runtimes[id] = rt
		tenant.Bind(rt)
		auth, e := data.NewMCPServerApi().StartOAuth(context.Background(), 1)
		tenant.Unbind()
		if e != nil {
			t.Fatal(e)
		}
		u, _ := url.Parse(auth)
		states[id] = u.Query().Get("state")
		challenges[map[uint]string{41: "alice", 42: "bob"}[id]] = u.Query().Get("code_challenge")
		if u.Query().Get("redirect_uri") != tPublicCallback {
			t.Fatal("wrong public redirect")
		}
	}
	callback := func(user uint, query string, want int) {
		t.Helper()
		req, _ := http.NewRequest("GET", published.URL+"/api/mcp/oauth/callback?"+query, nil)
		req.Host = "attacker.invalid"
		req.Header.Set("X-Forwarded-Host", "attacker.invalid")
		if user != 0 {
			req.AddCookie(&http.Cookie{Name: webauth.CookieName, Value: sessions[user]})
		}
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != want {
			t.Fatalf("callback status=%d want=%d body=%s", resp.StatusCode, want, body)
		}
		if strings.Contains(string(body), "token-alice") {
			t.Fatal("token leaked")
		}
	}
	callback(0, "state="+states[41]+"&code=alice", 401)
	callback(42, "state="+states[41]+"&code=alice", 400)
	callback(41, "state=wrong&error=denied", 400)
	callback(41, "state="+states[41]+"&code=alice", 200)
	callback(41, "state="+states[41]+"&code=alice", 400)
	callback(42, "state="+states[42]+"&code=bob", 200)
	if exchanges.Load() != 2 {
		t.Fatalf("exchanges=%d", exchanges.Load())
	}
	for _, id := range []uint{41, 42} {
		var server models.MCPServer
		runtimes[id].DB.First(&server, 1)
		if server.AuthConfig == "" {
			t.Fatal("token not saved")
		}
		tenant.Bind(runtimes[id])
		cfg, e := data.NewMCPServerApi().LoadAuthConfig(1)
		tenant.Unbind()
		if e != nil {
			t.Fatal(e)
		}
		if cfg.AccessToken != "token-"+map[uint]string{41: "alice", 42: "bob"}[id] {
			t.Fatalf("cross-tenant token: %+v", cfg)
		}
	}
}

const tPublicCallback = "https://stocks.example.test/api/mcp/oauth/callback"
