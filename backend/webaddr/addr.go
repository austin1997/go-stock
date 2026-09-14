package webaddr

import (
	"net"
	"strings"
)

const Default = "127.0.0.1:8080"

// FromEnv returns WEB_ADDR, defaulting to loopback so host-published Docker
// ports are not bound on 0.0.0.0 unless explicitly configured.
func FromEnv(env string) string {
	addr := strings.TrimSpace(env)
	if addr == "" {
		return Default
	}
	return addr
}

// IsLoopback reports whether addr only accepts connections from the local
// machine. Empty host (":8080") and 0.0.0.0 bind all interfaces.
func IsLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			return false
		}
		return false
	}
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
