package webaddr

import "testing"

func TestFromEnvDefaultLoopback(t *testing.T) {
	if got := FromEnv(""); got != Default {
		t.Fatalf("FromEnv empty = %q, want %q", got, Default)
	}
	if got := FromEnv("  :9090 "); got != ":9090" {
		t.Fatalf("FromEnv override = %q", got)
	}
}

func TestIsLoopback(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8080", true},
		{"localhost:8080", true},
		{"[::1]:8080", true},
		{":8080", false},
		{"0.0.0.0:8080", false},
		{"192.168.1.10:8080", false},
	}
	for _, tc := range cases {
		if got := IsLoopback(tc.addr); got != tc.want {
			t.Fatalf("IsLoopback(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}
