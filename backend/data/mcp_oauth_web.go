package data

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/tenant"
)

const MCPOAuthCallbackPath = "/api/mcp/oauth/callback"
const webOAuthTTL = 3 * time.Minute

type webOAuthKey struct{ userID, serverID uint }
type webOAuthFlow struct {
	key             webOAuthKey
	rt              *tenant.Runtime
	state, verifier string
	config          MCPAuthConfig
	expires         time.Time
	timer           *time.Timer
}

var webOAuthMu sync.Mutex
var webOAuthFlows = map[webOAuthKey]*webOAuthFlow{}

func expireWebOAuth(flow *webOAuthFlow) {
	webOAuthMu.Lock()
	// Stop may race an already-fired timer. A callback that consumed state owns
	// finalization; the timer must not launch a second reconciliation loop.
	if webOAuthFlows[flow.key] != flow || flow.state == "" {
		webOAuthMu.Unlock()
		return
	}
	flow.state = ""
	webOAuthMu.Unlock()
	finishWebOAuth(flow, "授权超时未完成，请重试")
}

// finishWebOAuth is synchronous and bounded: no retry workers or live state are
// created. Each write rechecks identity under the replacement mutex, with a DB
// deadline; backoff never holds the mutex. Call only after releasing webOAuthMu.
func finishWebOAuth(flow *webOAuthFlow, failure string) {
	const attempts = 3
	for attempt := 1; attempt <= attempts; attempt++ {
		webOAuthMu.Lock()
		if webOAuthFlows[flow.key] != flow {
			webOAuthMu.Unlock()
			return
		}
		flow.state = ""
		flow.timer.Stop()
		if failure == "" {
			delete(webOAuthFlows, flow.key)
			webOAuthMu.Unlock()
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		result := flow.rt.DB.WithContext(ctx).Model(&models.MCPServer{}).Where("id = ?", flow.key.serverID).Updates(map[string]any{"status": "unauthorized", "test_result": failure})
		cancel()
		if result.Error == nil && result.RowsAffected == 1 {
			delete(webOAuthFlows, flow.key)
			webOAuthMu.Unlock()
			return
		}
		if attempt == attempts {
			delete(webOAuthFlows, flow.key)
			webOAuthMu.Unlock()
			// Do not log provider errors, codes, state, or credentials.
			log.Printf("MCP OAuth terminal status persistence exhausted after %d attempts (user=%d server=%d); consumed flow removed, status may require reconciliation", attempts, flow.key.userID, flow.key.serverID)
			return
		}
		webOAuthMu.Unlock()
		time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
	}
}

// HandleWebMCPOAuthCallback requires a currently authenticated user from the HTTP
// layer. State alone never selects another user's workspace. Claim state before
// exchanging code so concurrent callbacks cannot exchange or persist twice.
func HandleWebMCPOAuthCallback(w http.ResponseWriter, r *http.Request, userID uint) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(q["state"]) != 1 || q.Get("state") == "" || len(q["code"]) > 1 || len(q["error"]) > 1 {
		http.Error(w, "invalid OAuth callback", 400)
		return
	}
	webOAuthMu.Lock()
	var flow *webOAuthFlow
	for _, f := range webOAuthFlows {
		if f.key.userID == userID && userID != 0 && f.state == q.Get("state") {
			flow = f
			break
		}
	}
	if flow == nil || !time.Now().Before(flow.expires) {
		if flow != nil {
			flow.state = ""
			flow.timer.Stop()
		}
		webOAuthMu.Unlock()
		if flow != nil {
			finishWebOAuth(flow, "授权超时未完成，请重试")
		}
		http.Error(w, "invalid or expired OAuth state", 400)
		return
	}
	// Consume even denial/malformed callbacks with a valid state.
	flow.state = ""
	flow.timer.Stop()
	webOAuthMu.Unlock()
	failure := "OAuth authorization denied or code missing; please retry"
	defer func() { finishWebOAuth(flow, failure) }()
	if q.Get("error") != "" || q.Get("code") == "" {
		http.Error(w, failure, 400)
		return
	}
	ctx, cancel := context.WithDeadline(r.Context(), flow.expires)
	defer cancel()
	tok, err := exchangeOAuthCode(ctx, flow.config.TokenURL, flow.config.ClientID, q.Get("code"), flow.config.RedirectURI, flow.verifier)
	if err != nil {
		failure = "OAuth token exchange failed; please retry"
		http.Error(w, failure, 502)
		return
	}
	cfg := flow.config
	cfg.AccessToken = tok.AccessToken
	cfg.RefreshToken = tok.RefreshToken
	cfg.TokenType = tok.TokenType
	if tok.ExpiresIn > 0 {
		cfg.ExpiresAt = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).Unix()
	}
	sealed, err := encryptAuthConfig(&cfg)
	if err != nil {
		failure = "OAuth credential encryption failed"
		http.Error(w, failure, 500)
		return
	}
	webOAuthMu.Lock()
	defer webOAuthMu.Unlock()
	// A newer authorization supersedes in-flight exchanges. Never overwrite it.
	if webOAuthFlows[flow.key] != flow || ctx.Err() != nil {
		failure = "OAuth flow expired or replaced"
		http.Error(w, failure, 400)
		return
	}
	expiry := time.Time{}
	if cfg.ExpiresAt > 0 {
		expiry = time.Unix(cfg.ExpiresAt, 0)
	}
	writeCtx, writeCancel := context.WithTimeout(ctx, 2*time.Second)
	defer writeCancel()
	result := flow.rt.DB.WithContext(writeCtx).Model(&models.MCPServer{}).Where("id = ?", flow.key.serverID).Updates(map[string]any{"auth_config": sealed, "token_expire_at": expiry, "status": "unauthorized", "test_result": "授权成功，请点击「测试」验证连接"})
	if result.Error != nil || result.RowsAffected != 1 {
		failure = "OAuth credential save failed"
		http.Error(w, failure, 500)
		return
	}
	failure = ""
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "授权成功。请关闭此页面，返回 go-stock 测试连接。")
}

// Explicit deployment configuration only; never derive this from request headers.
func webOAuthRedirectURI() (string, error) {
	raw := os.Getenv("WEB_PUBLIC_ORIGIN")
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" && u.Scheme != "http" || u.Host == "" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || u.Opaque != "" || u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("WEB_PUBLIC_ORIGIN must be an explicit http(s) origin, e.g. https://stocks.example.com")
	}
	if strings.ContainsAny(raw, "#?\\") || strings.HasSuffix(u.Host, ":") {
		return "", fmt.Errorf("WEB_PUBLIC_ORIGIN must contain only an origin")
	}
	if port := u.Port(); port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("WEB_PUBLIC_ORIGIN has an invalid port")
		}
	}
	if u.Scheme == "http" {
		ip := net.ParseIP(u.Hostname())
		if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return "", fmt.Errorf("WEB_PUBLIC_ORIGIN requires HTTPS except for loopback development")
		}
	}
	return u.Scheme + "://" + u.Host + MCPOAuthCallbackPath, nil
}

func (a *MCPServerApi) startWebOAuth(ctx context.Context, id uint) (string, error) {
	redirect, err := webOAuthRedirectURI()
	if err != nil {
		return "", err
	}
	rt := tenant.Capture()
	if rt == nil || rt.UserID == 0 || rt.DB == nil {
		return "", fmt.Errorf("MCP OAuth requires an authenticated tenant")
	}
	var server models.MCPServer
	if err := rt.DB.First(&server, id).Error; err != nil {
		return "", err
	}
	as, err := DiscoverOAuthMetadata(ctx, server.URL)
	if err != nil {
		return "", err
	}
	var clientID string
	if cfg, e := decryptAuthConfig(server.AuthConfig); e == nil && cfg.ClientID != "" && cfg.RedirectURI == redirect && cfg.TokenURL == as.TokenEndpoint && cfg.AuthorizationURL == as.AuthorizationEndpoint {
		clientID = cfg.ClientID
	} else {
		reg, e := registerOAuthClient(ctx, as, redirect)
		if e != nil {
			return "", e
		}
		clientID = reg.ClientID
	}
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return "", err
	}
	state, err := randomState()
	if err != nil {
		return "", err
	}
	au, err := url.Parse(as.AuthorizationEndpoint)
	if err != nil {
		return "", err
	}
	cfg := MCPAuthConfig{ClientID: clientID, RedirectURI: redirect, AuthorizationURL: as.AuthorizationEndpoint, TokenURL: as.TokenEndpoint, RegisterURL: as.RegistrationEndpoint, Scopes: strings.Join(as.ScopesSupported, " ")}
	q := au.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirect)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	if cfg.Scopes != "" {
		q.Set("scope", cfg.Scopes)
	}
	au.RawQuery = q.Encode()
	flow := &webOAuthFlow{key: webOAuthKey{rt.UserID, id}, rt: rt, state: state, verifier: verifier, config: cfg, expires: time.Now().Add(webOAuthTTL)}
	webOAuthMu.Lock()
	defer webOAuthMu.Unlock()
	// Persist credentials and pending status together before publishing a flow.
	// A failed start must not replace an existing authorization or leave a timer.
	sealed, err := encryptAuthConfig(&cfg)
	if err != nil {
		return "", err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	result := rt.DB.WithContext(writeCtx).Model(&models.MCPServer{}).Where("id = ?", id).Updates(map[string]any{
		"auth_config": sealed, "token_expire_at": time.Time{},
		"status": "testing", "test_result": "等待浏览器完成授权...",
	})
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected != 1 {
		return "", fmt.Errorf("OAuth initial status save affected %d rows", result.RowsAffected)
	}
	if old := webOAuthFlows[flow.key]; old != nil {
		old.timer.Stop()
	}
	webOAuthFlows[flow.key] = flow
	flow.timer = time.AfterFunc(webOAuthTTL, func() { expireWebOAuth(flow) })
	return au.String(), nil
}
