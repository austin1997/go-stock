package webauth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxAuthJSONBytes int64 = 16 << 10

func decodeAuthJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Body == nil {
		writeAuthJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthJSONBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeAuthJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return false
	}
	return true
}

type loginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type createUserBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"isAdmin"`
}

type disabledBody struct {
	Disabled bool `json:"disabled"`
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func SessionToken(r *http.Request) string {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		return c.Value
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func CurrentUser(r *http.Request) (*User, error) {
	return UserBySession(SessionToken(r))
}

type userCtxKey struct{}

func WithUser(ctx context.Context, u *User) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, userCtxKey{}, u)
}

func UserFromRequest(r *http.Request) *User {
	if r == nil {
		return nil
	}
	u, _ := r.Context().Value(userCtxKey{}).(*User)
	return u
}

func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string, expire time.Time) {
	c := &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expire,
		MaxAge:   int(time.Until(expire).Seconds()),
	}
	if r != nil && r.TLS != nil {
		c.Secure = true
	}
	http.SetCookie(w, c)
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func writeAuthJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAuthError(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	switch {
	case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrInvalidCredentials):
		code = http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		code = http.StatusForbidden
	case errors.Is(err, ErrUserDisabled):
		code = http.StatusForbidden
	case errors.Is(err, ErrUserNotFound):
		code = http.StatusNotFound
	case errors.Is(err, ErrRegisterClosed):
		code = http.StatusForbidden
	case errors.Is(err, ErrUserExists):
		code = http.StatusConflict
	}
	writeAuthJSON(w, code, map[string]any{"error": err.Error()})
}

func HandleStatus(w http.ResponseWriter, _ *http.Request) {
	writeAuthJSON(w, http.StatusOK, map[string]any{
		"allowRegister": AllowRegister(),
		"hasUsers":      HasUsers(),
	})
}

func HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body loginBody
	if !decodeAuthJSON(w, r, &body) {
		return
	}
	u, err := Register(body.Username, body.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	s, err := CreateSession(u.ID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	SetSessionCookie(w, r, s.Token, s.ExpiresAt)
	writeAuthJSON(w, http.StatusOK, map[string]any{"user": Public(u)})
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthJSONBytes)
	if !loginLimiter.Allow(clientIP(r)) {
		_, _ = io.Copy(io.Discard, r.Body)
		writeAuthJSON(w, http.StatusTooManyRequests, map[string]any{"error": "登录尝试过多，请稍后再试"})
		return
	}
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAuthJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	u, err := Authenticate(body.Username, body.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	s, err := CreateSession(u.ID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	SetSessionCookie(w, r, s.Token, s.ExpiresAt)
	writeAuthJSON(w, http.StatusOK, map[string]any{"user": Public(u)})
}

func HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	DeleteSession(SessionToken(r))
	ClearSessionCookie(w)
	writeAuthJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func HandleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u, err := CurrentUser(r)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]any{"user": Public(u)})
}

func requireAdmin(w http.ResponseWriter, r *http.Request) (*User, bool) {
	u, err := CurrentUser(r)
	if err != nil {
		writeAuthError(w, err)
		return nil, false
	}
	if !u.IsAdmin {
		writeAuthError(w, ErrForbidden)
		return nil, false
	}
	return u, true
}

func HandleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := requireAdmin(w, r); !ok {
			return
		}
		users, err := ListUsers()
		if err != nil {
			writeAuthError(w, err)
			return
		}
		out := make([]PublicUser, 0, len(users))
		for i := range users {
			out = append(out, Public(&users[i]))
		}
		writeAuthJSON(w, http.StatusOK, map[string]any{"users": out})
	case http.MethodPost:
		if _, ok := requireAdmin(w, r); !ok {
			return
		}
		var body createUserBody
		if !decodeAuthJSON(w, r, &body) {
			return
		}
		u, err := CreateUser(body.Username, body.Password, body.IsAdmin)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		writeAuthJSON(w, http.StatusOK, map[string]any{"user": Public(u)})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func HandleUserDisabled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	admin, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/auth/users/")
	idStr = strings.TrimSuffix(idStr, "/disabled")
	idStr = strings.Trim(idStr, "/")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id64 == 0 {
		writeAuthJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid user id"})
		return
	}
	if uint(id64) == admin.ID {
		writeAuthJSON(w, http.StatusBadRequest, map[string]any{"error": "不能禁用当前登录的管理员"})
		return
	}
	var body disabledBody
	if !decodeAuthJSON(w, r, &body) {
		return
	}
	if err := SetDisabled(uint(id64), body.Disabled); err != nil {
		writeAuthError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]any{"ok": true})
}
