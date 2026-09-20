package webcors

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testHandler() http.Handler {
	return Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
}

func TestForeignOriginRPCIsForbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/rpc", strings.NewReader(`{"method":"GetConfig"}`))
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Id", "attacker")

	rr := httptest.NewRecorder()
	testHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
	if ao := rr.Header().Get("Access-Control-Allow-Origin"); ao != "" {
		t.Fatalf("reflected Access-Control-Allow-Origin %q", ao)
	}
	if strings.Contains(rr.Body.String(), `"ok"`) {
		t.Fatalf("handler ran for foreign origin: %s", rr.Body.String())
	}
}

func TestForeignOriginPreflightIsForbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/rpc", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type, x-client-id")

	rr := httptest.NewRecorder()
	testHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
	if ao := rr.Header().Get("Access-Control-Allow-Origin"); ao != "" {
		t.Fatalf("preflight reflected origin %q", ao)
	}
}

func TestLoopbackOriginAllowedWithCredentials(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/rpc", strings.NewReader(`{}`))
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	testHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Allow-Origin = %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Allow-Credentials = %q, want true", got)
	}
}

func TestSameOriginLANAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Host = "192.168.1.10:8080"
	req.Header.Set("Origin", "http://192.168.1.10:8080")

	rr := httptest.NewRecorder()
	testHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://192.168.1.10:8080" {
		t.Fatalf("Allow-Origin = %q", got)
	}
}

func TestNoOriginPasses(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Host = "127.0.0.1:8080"

	rr := httptest.NewRecorder()
	testHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestOriginAllowedRejectsUserinfo(t *testing.T) {
	if OriginAllowed("http://evil.example@127.0.0.1:8080", "127.0.0.1:8080") {
		t.Fatal("userinfo origin must be rejected")
	}
}

func TestOriginAllowedRejectsNullAndNonHTTP(t *testing.T) {
	if OriginAllowed("null", "127.0.0.1:8080") {
		t.Fatal("null origin must be rejected")
	}
	if OriginAllowed("ftp://localhost", "127.0.0.1:8080") {
		t.Fatal("non-http origin must be rejected")
	}
}
