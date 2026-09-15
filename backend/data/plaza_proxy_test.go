package data

import (
	"net"
	"testing"
)

func TestResolvePlazaProxyURLAcceptsOfficialPath(t *testing.T) {
	got, err := ResolvePlazaProxyURL("/auth/register")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://go-stock.sparkmemory.top/api/auth/register"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolvePlazaProxyURLRejectsUnsafePaths(t *testing.T) {
	cases := []string{
		"",
		"auth/register",
		"//evil.example/pwn",
		"/../secret",
		"/auth/../../etc/passwd",
		"https://127.0.0.1/latest/meta-data",
		"/auth/register?x=1",
		"/auth/register#frag",
		"/auth/%2e%2e/admin",
	}
	for _, path := range cases {
		if _, err := ResolvePlazaProxyURL(path); err == nil {
			t.Errorf("path %q should be rejected", path)
		}
	}
}

func TestForbiddenIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true},
		{"100.64.0.1", true},
		{"::1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
	}
	for _, tt := range cases {
		if got := forbiddenIP(net.ParseIP(tt.ip)); got != tt.blocked {
			t.Errorf("forbiddenIP(%s)=%v want %v", tt.ip, got, tt.blocked)
		}
	}
}

func TestPlazaHTTPMethodAllowed(t *testing.T) {
	if !PlazaHTTPMethodAllowed("GET") || !PlazaHTTPMethodAllowed("post") {
		t.Fatal("GET/POST should be allowed")
	}
	if PlazaHTTPMethodAllowed("CONNECT") || PlazaHTTPMethodAllowed("TRACE") || PlazaHTTPMethodAllowed("") {
		t.Fatal("unsafe methods should be rejected")
	}
}
