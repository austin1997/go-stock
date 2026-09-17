package webauth

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Shorten the production deadline without mutable package globals. Reads still
// go through a real HTTP server and TCP connection, not a simulated Body.
type shortBodyDeadlineWriter struct {
	http.ResponseWriter
	timeout time.Duration
}

func (w shortBodyDeadlineWriter) SetReadDeadline(deadline time.Time) error {
	if !deadline.IsZero() {
		deadline = time.Now().Add(w.timeout)
	}
	return http.NewResponseController(w.ResponseWriter).SetReadDeadline(deadline)
}

func TestAuthSlowBodyDeadline(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		limited bool
		prefix  string
		status  int
	}{
		{"register", HandleRegister, false, `{"username":"`, http.StatusRequestTimeout},
		{"login", HandleLogin, false, `{"username":"`, http.StatusRequestTimeout},
		{"limited-login", HandleLogin, true, `{"username":"`, http.StatusTooManyRequests},
		{"complete-json-unfinished-body", func(w http.ResponseWriter, r *http.Request) {
			var body loginBody
			if decodeAuthJSON(w, r, &body) {
				writeAuthJSON(w, http.StatusOK, body)
			}
		}, false, `{}`, http.StatusRequestTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := loginLimiter
			limit := 10
			if tc.limited {
				limit = 0
			}
			loginLimiter = newRateLimiter(limit, time.Minute)
			defer func() { loginLimiter = old }()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tc.handler(shortBodyDeadlineWriter{w, 60 * time.Millisecond}, r)
			}))
			defer server.Close()
			conn, err := net.Dial("tcp", server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			_, err = fmt.Fprintf(conn, "POST / HTTP/1.1\r\nHost: localhost\r\nContent-Length: 1000\r\n\r\n%s", tc.prefix)
			if err != nil {
				t.Fatal(err)
			}
			// Keep trickling bytes faster than the timeout: the deadline must be
			// absolute, not refreshed for each byte or only enforced at EOF.
			done := make(chan struct{})
			defer close(done)
			go func() {
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-done:
						return
					case <-ticker.C:
						if _, err := io.WriteString(conn, " "); err != nil {
							return
						}
					}
				}
			}()
			resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
			if err != nil {
				t.Fatalf("slow body did not receive bounded HTTP response: %v", err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.status {
				t.Fatalf("status=%d want=%d body=%s", resp.StatusCode, tc.status, body)
			}
		})
	}
}

// A hijacked connection does not get net/http's next-request deadline reset.
// Reading after the body budget expires proves the body deadline was cleared,
// including for future WebSocket users of the shared decoding seam.
func TestAuthBodyDeadlineClearedBeforeHijack(t *testing.T) {
	result := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body loginBody
		if !decodeAuthJSON(shortBodyDeadlineWriter{w, 40 * time.Millisecond}, r, &body) {
			result <- fmt.Errorf("decode failed")
			return
		}
		time.Sleep(100 * time.Millisecond)
		conn, rw, err := http.NewResponseController(w).Hijack()
		if err != nil {
			result <- err
			return
		}
		defer conn.Close()
		_, err = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: test\r\n\r\n")
		if err == nil {
			err = rw.Flush()
		}
		if err == nil {
			var line string
			line, err = rw.ReadString('\n')
			if err == nil && line != "ping\n" {
				err = fmt.Errorf("unexpected ping: %q", line)
			}
		}
		result <- err
	}))
	defer server.Close()
	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, "POST / HTTP/1.1\r\nHost: localhost\r\nContent-Length: 2\r\n\r\n{}"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if _, err := io.WriteString(conn, "ping\n"); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatalf("deadline survived body consumption: %v", err)
	}
}

func TestAuthBodyDeadlineAllowsKeepAlive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body loginBody
		if decodeAuthJSON(shortBodyDeadlineWriter{w, 40 * time.Millisecond}, r, &body) {
			time.Sleep(100 * time.Millisecond)
			writeAuthJSON(w, http.StatusOK, body)
		}
	}))
	defer server.Close()
	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	for i := 0; i < 2; i++ {
		if _, err := io.WriteString(conn, "POST / HTTP/1.1\r\nHost: localhost\r\nContent-Length: 2\r\n\r\n{}"); err != nil {
			t.Fatal(err)
		}
		resp, err := http.ReadResponse(reader, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK || resp.Close {
			t.Fatalf("status=%d close=%v", resp.StatusCode, resp.Close)
		}
	}
}

func TestAuthBodySizeAndValidJSON(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"valid", `{"username":"alice"}`, http.StatusOK},
		{"oversize", `{"username":"` + strings.Repeat("a", int(maxAuthJSONBytes)) + `"}`, http.StatusBadRequest},
		{"oversize-trailing-space", `{}` + strings.Repeat(" ", int(maxAuthJSONBytes)), http.StatusBadRequest},
		{"invalid", `{`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body loginBody
				if decodeAuthJSON(w, r, &body) {
					writeAuthJSON(w, http.StatusOK, body)
				}
			}))
			defer server.Close()
			resp, err := server.Client().Post(server.URL, "application/json", strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.status {
				t.Fatalf("status=%d want=%d", resp.StatusCode, tc.status)
			}
		})
	}
}
