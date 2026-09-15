package data

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

var (
	errInvalidPlazaPath   = errors.New("invalid plaza path")
	errPlazaHost          = errors.New("plaza host not allowed")
	errForbiddenPlazaAddr = errors.New("plaza host resolves to a forbidden address")
)

var plazaHTTPClientOnce sync.Once
var plazaHTTPClient *resty.Client

// PlazaHTTPMethodAllowed 广场代理只允许普通 REST 方法，拒绝 CONNECT 等。
func PlazaHTTPMethodAllowed(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func plazaAllowedHost() string {
	u, err := url.Parse(DefaultPromptPlazaApiBase)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func plazaHostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	return host == plazaAllowedHost()
}

func forbiddenIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	// RFC 6598 shared address space (CGNAT), not covered by IP.IsPrivate.
	return ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127
}

// ResolvePlazaProxyURL 将调用方提供的 path 拼到官方广场根地址上。
// 忽略调用方 apiBase，避免网页 RPC 把容器变成任意 URL 代理。
func ResolvePlazaProxyURL(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasPrefix(path, "/") {
		return "", errInvalidPlazaPath
	}
	if strings.ContainsAny(path, " \t\r\n\\?#%") || strings.Contains(path, "://") {
		return "", errInvalidPlazaPath
	}
	u, err := url.Parse(path)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", errInvalidPlazaPath
	}
	decoded, err := url.PathUnescape(u.EscapedPath())
	if err != nil || decoded == "" || strings.Contains(decoded, "..") {
		return "", errInvalidPlazaPath
	}

	joined := strings.TrimRight(DefaultPromptPlazaApiBase, "/") + path
	out, err := url.Parse(joined)
	if err != nil {
		return "", errInvalidPlazaPath
	}
	if out.Scheme != "https" {
		return "", errPlazaHost
	}
	if !plazaHostAllowed(out.Hostname()) {
		return "", errPlazaHost
	}
	return out.String(), nil
}

func plazaDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	d := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	// HTTP 代理时 Dial 的是代理地址（可能是 127.0.0.1），不能按广场主机策略拦截。
	if !plazaHostAllowed(host) {
		return d.DialContext(ctx, network, addr)
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var last error
	for _, ipa := range ips {
		if forbiddenIP(ipa.IP) {
			last = errForbiddenPlazaAddr
			continue
		}
		conn, err := d.DialContext(ctx, network, net.JoinHostPort(ipa.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	if last == nil {
		return nil, errForbiddenPlazaAddr
	}
	return nil, last
}

func PlazaHTTPClient() *resty.Client {
	plazaHTTPClientOnce.Do(func() {
		transport := &http.Transport{
			DialContext:           plazaDialContext,
			MaxIdleConns:          8,
			MaxIdleConnsPerHost:   4,
			IdleConnTimeout:       60 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 20 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ForceAttemptHTTP2:     true,
			Proxy:                 resolveHTTPProxy,
		}
		httpClient := &http.Client{
			Transport: transport,
			Timeout:   20 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return fmt.Errorf("plaza redirects are not followed")
			},
		}
		plazaHTTPClient = resty.NewWithClient(httpClient).
			SetRetryCount(0).
			SetTimeout(20 * time.Second)
	})
	return plazaHTTPClient
}
