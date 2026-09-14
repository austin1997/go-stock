package webauth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	CookieName    = "go_stock_web_session"
	sessionTTL    = 7 * 24 * time.Hour
	loginWindow   = time.Minute
	loginMaxHits  = 5
	tokenFilePerm = 0o600
)

// Auth is a single-user shared-secret gate for the web RPC/API.
type Auth struct {
	tokenHash [32]byte

	mu       sync.Mutex
	sessions map[string]time.Time
	logins   map[string][]time.Time
}

type loginRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// ResolveToken returns WEB_AUTH_TOKEN, or a persisted random token in tokenFile.
// generated is true when a new token was written to disk.
func ResolveToken(tokenFile string) (token string, generated bool, err error) {
	if t := strings.TrimSpace(os.Getenv("WEB_AUTH_TOKEN")); t != "" {
		return t, false, nil
	}
	if tokenFile != "" {
		if b, readErr := os.ReadFile(tokenFile); readErr == nil {
			t := strings.TrimSpace(string(b))
			if t != "" {
				return t, false, nil
			}
		}
	}
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", false, err
	}
	token = hex.EncodeToString(buf)
	if tokenFile == "" {
		return token, true, nil
	}
	if err = os.MkdirAll(filepath.Dir(tokenFile), 0o700); err != nil {
		return "", false, err
	}
	if err = os.WriteFile(tokenFile, []byte(token+"\n"), tokenFilePerm); err != nil {
		return "", false, err
	}
	return token, true, nil
}

func New(token string) *Auth {
	return &Auth{
		tokenHash: sha256.Sum256([]byte(token)),
		sessions:  map[string]time.Time{},
		logins:    map[string][]time.Time{},
	}
}

func (a *Auth) tokenOK(got string) bool {
	sum := sha256.Sum256([]byte(got))
	return subtle.ConstantTimeCompare(sum[:], a.tokenHash[:]) == 1
}

func (a *Auth) Authorized(r *http.Request) bool {
	if a == nil {
		return false
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		raw := strings.TrimSpace(auth[7:])
		if a.tokenOK(raw) || a.validSession(raw) {
			return true
		}
	}
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return false
	}
	return a.validSession(c.Value)
}

func (a *Auth) validSession(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	exp, ok := a.sessions[id]
	if !ok {
		return false
	}
	now := time.Now()
	if now.After(exp) {
		delete(a.sessions, id)
		return false
	}
	a.sessions[id] = now.Add(sessionTTL)
	return true
}

func (a *Auth) createSession() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf)
	a.mu.Lock()
	a.sessions[id] = time.Now().Add(sessionTTL)
	a.mu.Unlock()
	return id, nil
}

func (a *Auth) revokeSession(id string) {
	if id == "" {
		return
	}
	a.mu.Lock()
	delete(a.sessions, id)
	a.mu.Unlock()
}

func (a *Auth) allowLogin(ip string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-loginWindow)
	hits := a.logins[ip]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= loginMaxHits {
		a.logins[ip] = kept
		return false
	}
	a.logins[ip] = append(kept, now)
	return true
}

// Middleware protects /api/* except health and auth endpoints.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requiresAuth(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if a.Authorized(r) {
			next.ServeHTTP(w, r)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
	})
}

func requiresAuth(path string) bool {
	switch path {
	case "/api/health", "/api/auth/login", "/api/auth/logout", "/api/auth/status":
		return false
	}
	return strings.HasPrefix(path, "/api/")
}

func (a *Auth) HandleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"required":      true,
		"authenticated": a.Authorized(r),
	})
}

func (a *Auth) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ip := clientIP(r)
	if !a.allowLogin(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many login attempts"})
		return
	}
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	secret := strings.TrimSpace(req.Token)
	if secret == "" {
		secret = strings.TrimSpace(req.Password)
	}
	if !a.tokenOK(secret) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid token"})
		return
	}
	id, err := a.createSession()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "session"})
		return
	}
	setSessionCookie(w, r, id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *Auth) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		a.revokeSession(c.Value)
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, id string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
		MaxAge:   -1,
	})
}

func cookieSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
