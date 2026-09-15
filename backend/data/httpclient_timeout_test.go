package data

import (
	"context"
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
