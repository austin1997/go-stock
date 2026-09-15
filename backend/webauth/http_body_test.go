package webauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleRegisterRejectsOversizedBody(t *testing.T) {
	body := strings.Repeat("a", int(maxAuthJSONBytes)+8)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	HandleRegister(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("register oversized: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleLoginRejectsOversizedBody(t *testing.T) {
	body := strings.Repeat("a", int(maxAuthJSONBytes)+8)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	HandleLogin(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("login oversized: code=%d body=%s", rec.Code, rec.Body.String())
	}
}
