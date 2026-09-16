package data

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/tenant"
	"go-stock/backend/webmode"
)

func TestTimeoutRoundTripperUsesTenantTimeoutInWebMode(t *testing.T) {
	webmode.Enable()
	t.Cleanup(webmode.Disable)
	tenant.Bind(&tenant.Runtime{UserID: 1, HTTPTimeout: 40 * time.Millisecond})
	t.Cleanup(tenant.Unbind)

	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	rt := &timeoutRoundTripper{base: http.DefaultTransport}
	began := time.Now()
	_, err = rt.RoundTrip(req)
	elapsed := time.Since(began)
	if err == nil {
		t.Fatal("expected tenant timeout")
	}
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("elapsed %v, tenant timeout should have fired first", elapsed)
	}
	select {
	case <-started:
	default:
		t.Fatal("server should have received the request")
	}
}

func TestTimeoutRoundTripperKeepsContextUntilBodyClose(t *testing.T) {
	webmode.Enable()
	t.Cleanup(webmode.Disable)
	tenant.Bind(&tenant.Runtime{UserID: 1, HTTPTimeout: 120 * time.Millisecond})
	t.Cleanup(tenant.Unbind)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(400 * time.Millisecond)
		_, _ = w.Write([]byte("late-body"))
	}))
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	rt := &timeoutRoundTripper{base: http.DefaultTransport}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("headers should succeed: %v", err)
	}
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	if err == nil {
		t.Fatal("expected body read to fail after tenant timeout")
	}
}

func TestTimeoutRoundTripperReadsBodyWithinTimeout(t *testing.T) {
	webmode.Enable()
	t.Cleanup(webmode.Disable)
	tenant.Bind(&tenant.Runtime{UserID: 1, HTTPTimeout: 2 * time.Second})
	t.Cleanup(tenant.Unbind)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	rt := &timeoutRoundTripper{base: http.DefaultTransport}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Fatalf("body=%q", body)
	}
}

func TestCreateHTTPClientWithTimeoutDoesNotMutateSharedClient(t *testing.T) {
	webmode.Enable()
	t.Cleanup(webmode.Disable)
	tenant.Bind(&tenant.Runtime{UserID: 1, ProxyOn: true, ProxyURL: "http://127.0.0.1:9"})
	t.Cleanup(tenant.Unbind)

	beforeTimeout := SharedHTTPClient.GetClient().Timeout
	c := CreateHTTPClientWithTimeout(3 * time.Second)
	if c == SharedHTTPClient {
		t.Fatal("expected a request-local client")
	}
	if SharedHTTPClient.GetClient().Timeout != beforeTimeout {
		t.Fatal("SharedHTTPClient timeout should stay unchanged")
	}
}
