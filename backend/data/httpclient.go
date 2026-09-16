package data

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"

	"go-stock/backend/tenant"
	"go-stock/backend/webmode"
)

var (
	sharedTransport     *http.Transport
	sharedHTTPClient    *http.Client
	SharedHTTPClient    *resty.Client
	httpConfigMutex     sync.RWMutex
	currentProxyEnabled bool
	currentProxyURL     string
	currentTimeout      = 300 * time.Second
)

type timeoutRoundTripper struct {
	base http.RoundTripper
}

func requestTimeout() time.Duration {
	if d := tenant.HTTPTimeout(); d > 0 {
		return d
	}
	httpConfigMutex.RLock()
	defer httpConfigMutex.RUnlock()
	if currentTimeout > 0 {
		return currentTimeout
	}
	return 300 * time.Second
}

type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnCloseBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.cancel()
	}
	return n, err
}

func (b *cancelOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func (t *timeoutRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return t.base.RoundTrip(req)
	}
	d := requestTimeout()
	if d <= 0 {
		return t.base.RoundTrip(req)
	}
	parent := req.Context()
	if webmode.Enabled() {
		// 网页版忽略 Client.Timeout / SetTimeout 带来的进程级 deadline，
		// 只按当前租户 CrawlTimeOut 限制，避免租户互相覆盖。
		parent = context.WithoutCancel(parent)
	} else if deadline, ok := parent.Deadline(); ok {
		if rem := time.Until(deadline); rem > 0 && rem < d {
			d = rem
		}
	}
	ctx, cancel := context.WithTimeout(parent, d)
	resp, err := t.base.RoundTrip(req.WithContext(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	if resp.Body == nil || resp.Body == http.NoBody {
		cancel()
		return resp, nil
	}
	resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

func init() {
	sharedTransport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   4,
		MaxConnsPerHost:       10,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 120 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		Proxy:                 resolveHTTPProxy,
	}

	sharedHTTPClient = &http.Client{
		Transport: &timeoutRoundTripper{base: sharedTransport},
		Timeout:   300 * time.Second,
	}

	SharedHTTPClient = resty.NewWithClient(sharedHTTPClient).
		SetRetryCount(0).
		SetTimeout(300 * time.Second)
}

func resolveHTTPProxy(req *http.Request) (*url.URL, error) {
	if rt := tenant.Current(); rt != nil && rt.ProxyOn && rt.ProxyURL != "" {
		return parseProxyURL(rt.ProxyURL), nil
	}
	httpConfigMutex.RLock()
	defer httpConfigMutex.RUnlock()
	if currentProxyEnabled && currentProxyURL != "" {
		return parseProxyURL(currentProxyURL), nil
	}
	return nil, nil
}

func UpdateHTTPClientProxy(proxyURL string) {
	httpConfigMutex.Lock()
	defer httpConfigMutex.Unlock()

	if proxyURL == "" || proxyURL == currentProxyURL {
		return
	}

	currentProxyURL = proxyURL
	currentProxyEnabled = true
}

func DisableHTTPClientProxy() {
	httpConfigMutex.Lock()
	defer httpConfigMutex.Unlock()

	currentProxyEnabled = false
	currentProxyURL = ""
}

func UpdateHTTPClientTimeout(timeout time.Duration) {
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	if webmode.Enabled() {
		tenant.SetHTTPTimeout(timeout)
		return
	}
	httpConfigMutex.Lock()
	currentTimeout = timeout
	httpConfigMutex.Unlock()
	sharedHTTPClient.Timeout = timeout
	SharedHTTPClient.SetTimeout(timeout)
}

func parseProxyURL(proxyURL string) *url.URL {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil
	}
	return u
}

func ConfigureFromSettings(config *SettingConfig) {
	if config == nil {
		return
	}

	if webmode.Enabled() {
		tenant.SetProxy(config.HttpProxy, config.HttpProxyEnabled)
		if config.CrawlTimeOut > 0 {
			tenant.SetHTTPTimeout(time.Duration(config.CrawlTimeOut) * time.Second)
		} else {
			tenant.SetHTTPTimeout(300 * time.Second)
		}
		return
	} else if config.HttpProxyEnabled && config.HttpProxy != "" {
		UpdateHTTPClientProxy(config.HttpProxy)
	} else {
		DisableHTTPClientProxy()
	}

	if config.CrawlTimeOut > 0 {
		UpdateHTTPClientTimeout(time.Duration(config.CrawlTimeOut) * time.Second)
	} else {
		UpdateHTTPClientTimeout(300 * time.Second)
	}
}

func CreateHTTPClientWithTimeout(timeout time.Duration) *resty.Client {
	httpConfigMutex.RLock()
	transport := sharedTransport
	httpConfigMutex.RUnlock()

	httpClient := &http.Client{
		Transport: &timeoutRoundTripper{base: transport},
		Timeout:   timeout,
	}

	return resty.NewWithClient(httpClient).
		SetTimeout(timeout).
		SetRetryCount(0)
}

func CreateDownloadClient() *resty.Client {
	httpConfigMutex.RLock()
	transport := sharedTransport
	httpConfigMutex.RUnlock()

	downloadTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          5,
		MaxIdleConnsPerHost:   2,
		MaxConnsPerHost:       2,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		Proxy:                 transport.Proxy,
	}

	downloadHTTPClient := &http.Client{
		Transport: downloadTransport,
		Timeout:   0,
	}

	return resty.NewWithClient(downloadHTTPClient).
		SetTimeout(0).
		SetRetryCount(2).
		SetRetryWaitTime(5 * time.Second).
		SetRetryMaxWaitTime(30 * time.Second)
}

// GetSharedTransport 返回共享 HTTP Transport（包含用户代理配置），
// 供流式下载等场景复用代理设置。
func GetSharedTransport() *http.Transport {
	httpConfigMutex.RLock()
	defer httpConfigMutex.RUnlock()
	return sharedTransport
}
