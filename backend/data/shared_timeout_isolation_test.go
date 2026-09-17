package data

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"go-stock/backend/tenant"
	"go-stock/backend/webmode"
)

type isolationTransport func(*http.Request) (*http.Response, error)

func (f isolationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSharedTimeoutConcurrentNewsIsolation(t *testing.T) {
	webmode.Enable()
	defer webmode.Disable()
	old := SharedHTTPClient
	defer func() { SharedHTTPClient = old }()
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	var policyDeadline, unrelatedDeadline time.Time
	SharedHTTPClient = resty.NewWithClient(&http.Client{Transport: &timeoutRoundTripper{base: isolationTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "policy.test" {
			policyDeadline, _ = r.Context().Deadline()
			close(entered)
			<-release
		}
		if r.URL.Host == "unrelated.test" {
			unrelatedDeadline, _ = r.Context().Deadline()
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"errno":0,"data":{"roll_data":[]}}`)), Request: r}, nil
	})}})
	before := time.Now()
	go func() {
		tenant.Bind(&tenant.Runtime{UserID: 101, HTTPTimeout: time.Minute})
		defer tenant.Unbind()
		_, err := fetchGovPage("https://policy.test/")
		done <- err
	}()
	<-entered
	tenant.Bind(&tenant.Runtime{UserID: 102, HTTPTimeout: 45 * time.Second})
	defer tenant.Unbind()
	NewMarketNewsApi().GetNewTelegraph(2)
	if got := SharedHTTPClient.GetClient().Timeout; got != 0 {
		t.Errorf("shared client timeout mutated to %v", got)
	}
	// An unrelated request must retain this tenant's budget.
	_, err := SharedHTTPClient.R().Get("https://unrelated.test/")
	if err != nil {
		t.Error(err)
	}
	if unrelatedDeadline.Before(before.Add(44 * time.Second)) {
		t.Errorf("unrelated tenant deadline capped by another request: %v", unrelatedDeadline.Sub(before))
	}
	if policyDeadline.Before(before.Add(14*time.Second)) || policyDeadline.After(time.Now().Add(15*time.Second)) {
		t.Errorf("policy deadline = %v", policyDeadline.Sub(before))
	}
	close(release)
	if err := <-done; err != nil {
		t.Error(err)
	}
}

func TestNoRequestTimeSharedTimeoutMutation(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || file == "httpclient.go" || file == "openai_tools.go" || file == "plaza_proxy.go" || file == "wallstreetcn_api.go" {
			continue
		}
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "//") && strings.Contains(line, "SetTimeout(") {
				t.Errorf("request-time timeout mutation at %s:%d: %s", file, i+1, strings.TrimSpace(line))
			}
		}
	}
}

func TestDesktopTimeoutUpdateKeepsSharedClientImmutable(t *testing.T) {
	webmode.Disable()
	old := requestTimeout()
	defer UpdateHTTPClientTimeout(old)
	before := SharedHTTPClient.GetClient().Timeout
	UpdateHTTPClientTimeout(7 * time.Second)
	if got := SharedHTTPClient.GetClient().Timeout; got != before {
		t.Errorf("desktop update mutated shared client: %v -> %v", before, got)
	}
	if got := requestTimeout(); got != 7*time.Second {
		t.Errorf("desktop budget = %v", got)
	}
}

func TestPrivateTimeoutClientPreservesBounds(t *testing.T) {
	webmode.Enable()
	defer webmode.Disable()
	tenant.Bind(&tenant.Runtime{UserID: 103, HTTPTimeout: 3 * time.Second})
	defer tenant.Unbind()
	for _, tc := range []struct {
		name    string
		timeout time.Duration
		parent  time.Duration
		want    time.Duration
	}{
		{"tenant", 10 * time.Second, 0, 3 * time.Second},
		{"operation", time.Second, 0, time.Second},
		{"caller", 10 * time.Second, time.Second, time.Second},
		{"zero", 0, 0, 3 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tenant.Bind(&tenant.Runtime{UserID: 103, HTTPTimeout: 3 * time.Second})
			defer tenant.Unbind()
			before := time.Now()
			base := resty.NewWithClient(&http.Client{Transport: &timeoutRoundTripper{base: isolationTransport(func(r *http.Request) (*http.Response, error) {
				deadline, ok := r.Context().Deadline()
				if !ok || deadline.Before(before.Add(tc.want)) || deadline.After(time.Now().Add(tc.want)) {
					t.Errorf("unexpected deadline %v", deadline)
				}
				return &http.Response{StatusCode: 200, Body: http.NoBody, Header: http.Header{}, Request: r}, nil
			})}})
			ctx := context.Background()
			if tc.parent > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.parent)
				defer cancel()
			}
			if _, err := clientWithTimeout(base, tc.timeout).R().SetContext(ctx).Get("https://bounds.test/"); err != nil {
				t.Fatal(err)
			}
			if base.GetClient().Timeout != 0 {
				t.Fatal("base client mutated")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	entered := make(chan struct{})
	base := resty.NewWithClient(&http.Client{Transport: isolationTransport(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})})
	done := make(chan error, 1)
	go func() {
		_, err := clientWithTimeout(base, time.Minute).R().SetContext(ctx).Get("https://cancel.test/")
		done <- err
	}()
	<-entered
	cancel()
	if err := <-done; err == nil {
		t.Fatal("caller cancellation lost")
	}
}

// Caller context semantics must not be weakened to work around shared timeouts.
func TestTimeoutRoundTripperPreservesExplicitDeadline(t *testing.T) {
	parent, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()
	expected, _ := parent.Deadline()
	rt := &timeoutRoundTripper{base: isolationTransport(func(r *http.Request) (*http.Response, error) {
		got, _ := r.Context().Deadline()
		if !got.Equal(expected) {
			t.Errorf("deadline = %v, want %v", got, expected)
		}
		return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
	})}
	req, _ := http.NewRequestWithContext(parent, "GET", "https://deadline.test/", nil)
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
}
