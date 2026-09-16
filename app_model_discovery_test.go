package main

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// Keep this independent of the implementation constant so raising the limit
// cannot silently weaken the regression test.
const discoveryTestBodyLimit = 1 << 20

func modelDiscoveryServer(t *testing.T, body, encoding string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing discovery authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		if encoding != "identity" {
			w.Header().Set("Content-Encoding", "gzip")
		}
		// No Content-Length: the limit must also apply to streamed responses.
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		if encoding != "identity" {
			zw := gzip.NewWriter(w)
			_, _ = io.WriteString(zw, body)
			_ = zw.Close()
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchAiModelInfoResponseLimit(t *testing.T) {
	const payload = `{"id":"test-model","context_length":8192,"max_output_tokens":512}`
	for _, encoding := range []string{"identity", "gzip-auto", "gzip-explicit"} {
		t.Run(encoding, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				size int
			}{
				{"normal", len(payload)},
				{"at_limit", discoveryTestBodyLimit},
				{"over_limit", discoveryTestBodyLimit + 1},
			} {
				t.Run(tc.name, func(t *testing.T) {
					srv := modelDiscoveryServer(t, strings.Repeat(" ", tc.size-len(payload))+payload, encoding)
					headers := ""
					if encoding == "gzip-explicit" {
						headers = `{"Accept-Encoding":"gzip"}`
					}
					got := (&App{}).FetchAiModelInfo(srv.URL, "test-key", "test-model", headers)
					if got == nil {
						t.Fatal("expected model info fallback")
					}
					if tc.size > discoveryTestBodyLimit {
						if got.Source == "api" || got.ContextWindow != 0 || got.MaxTokens != 0 {
							t.Fatalf("oversized decompressed detail accepted: %+v", got)
						}
					} else if got.Source != "api" || got.ContextWindow != 8192 || got.MaxTokens != 512 {
						t.Fatalf("valid detail rejected: %+v", got)
					}
				})
			}
		})
	}
}

func TestFetchAiModelInfoOversizedResponseKeepsBuiltinFallback(t *testing.T) {
	body := strings.Repeat(" ", discoveryTestBodyLimit) +
		`{"id":"gpt-4o","context_length":1,"max_output_tokens":1}`
	srv := modelDiscoveryServer(t, body, "gzip-auto")
	app := &App{}
	// An absent API key uses exactly the same built-in fallback without HTTP.
	want := app.FetchAiModelInfo(srv.URL, "", "gpt-4o", "")
	got := app.FetchAiModelInfo(srv.URL, "test-key", "gpt-4o", "")
	if want == nil || want.Source != "builtin" || want.ContextWindow <= 0 {
		t.Fatalf("expected built-in fixture: %+v", want)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("oversized response should preserve fallback: got %+v, want %+v", got, want)
	}
}

func TestFetchAiModelsResponseLimit(t *testing.T) {
	const payload = `{"data":[{"id":"test-model"},{"id":" "}]}`
	for _, encoding := range []string{"identity", "gzip-auto", "gzip-explicit"} {
		t.Run(encoding, func(t *testing.T) {
			for _, size := range []int{len(payload), discoveryTestBodyLimit, discoveryTestBodyLimit + 1} {
				name := "normal"
				if size == discoveryTestBodyLimit {
					name = "at_limit"
				} else if size > discoveryTestBodyLimit {
					name = "over_limit"
				}
				t.Run(name, func(t *testing.T) {
					body := strings.Repeat(" ", size-len(payload)) + payload
					srv := modelDiscoveryServer(t, body, encoding)
					headers := ""
					if encoding == "gzip-explicit" {
						// Explicit Accept-Encoding bypasses net/http's automatic
						// decompression and exercises Resty's gzip handling.
						headers = `{"Accept-Encoding":"gzip"}`
					}
					got := (&App{}).FetchAiModels(srv.URL, "test-key", headers)
					if size > discoveryTestBodyLimit {
						if len(got) != 0 {
							t.Fatalf("oversized decompressed response accepted: %v", got)
						}
					} else if !reflect.DeepEqual(got, []string{"test-model"}) {
						t.Fatalf("valid response rejected: %v", got)
					}
				})
			}
		})
	}
}
