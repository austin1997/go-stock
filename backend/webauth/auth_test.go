package webauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTokenFromEnv(t *testing.T) {
	t.Setenv("WEB_AUTH_TOKEN", "from-env")
	got, generated, err := ResolveToken(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if generated || got != "from-env" {
		t.Fatalf("got %q generated=%v", got, generated)
	}
}

func TestResolveTokenPersistsFile(t *testing.T) {
	t.Setenv("WEB_AUTH_TOKEN", "")
	path := filepath.Join(t.TempDir(), ".web_auth_token")
	first, generated, err := ResolveToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if !generated || len(first) < 32 {
		t.Fatalf("first token generated=%v len=%d", generated, len(first))
	}
	second, generated, err := ResolveToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if generated || second != first {
		t.Fatalf("expected persisted token, generated=%v", generated)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %o", info.Mode().Perm())
	}
}

func TestLoginAndRPCSession(t *testing.T) {
	auth := New("secret-token")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", auth.HandleLogin)
	mux.HandleFunc("/api/auth/status", auth.HandleStatus)
	mux.HandleFunc("/api/rpc", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	})
	h := auth.Middleware(mux)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/rpc", strings.NewReader(`{"method":"GetConfig"}`)))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated RPC status = %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"wrong"}`))
	login.RemoteAddr = "127.0.0.1:9"
	h.ServeHTTP(rr, login)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	login = httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"secret-token"}`))
	login.RemoteAddr = "127.0.0.1:9"
	h.ServeHTTP(rr, login)
	if rr.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", rr.Code, rr.Body.String())
	}
	cookie := sessionCookie(rr)
	if cookie == "" {
		t.Fatal("missing session cookie")
	}

	rr = httptest.NewRecorder()
	rpc := httptest.NewRequest(http.MethodPost, "/api/rpc", strings.NewReader(`{"method":"GetConfig"}`))
	rpc.AddCookie(&http.Cookie{Name: CookieName, Value: cookie})
	h.ServeHTTP(rr, rpc)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"ok"`) {
		t.Fatalf("authed RPC status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestBearerMasterToken(t *testing.T) {
	auth := New("secret-token")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/rpc", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	h := auth.Middleware(mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/rpc", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	auth := New("secret-token")
	for i := 0; i < loginMaxHits; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"x"}`))
		req.RemoteAddr = "10.0.0.8:1"
		auth.HandleLogin(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d", i, rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"secret-token"}`))
	req.RemoteAddr = "10.0.0.8:1"
	auth.HandleLogin(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limit status = %d", rr.Code)
	}
}

func TestHealthAndStatusBypassAuth(t *testing.T) {
	auth := New("secret-token")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/api/auth/status", auth.HandleStatus)
	h := auth.Middleware(mux)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("health status = %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/auth/status", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["authenticated"] != false || body["required"] != true {
		t.Fatalf("status body = %#v", body)
	}
}

func sessionCookie(rr *httptest.ResponseRecorder) string {
	for _, c := range rr.Result().Cookies() {
		if c.Name == CookieName {
			return c.Value
		}
	}
	return ""
}
