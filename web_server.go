//go:build goweb
// +build goweb

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/google/uuid"

	"go-stock/backend/events"
	"go-stock/backend/logger"
	"go-stock/backend/tenant"
	"go-stock/backend/webauth"
	"go-stock/backend/webcors"
	"go-stock/backend/webdownload"
)

const (
	maxRPCBodyBytes    = 32 << 20
	maxUploadBodyBytes = 64 << 20
)

type webServer struct {
	http      *http.Server
	staticDir string
	methods   map[string]reflect.Method
	runtimes  *runtimeManager
}

type rpcRequest struct {
	Method string            `json:"method"`
	Args   []json.RawMessage `json:"args"`
}

type rpcResponse struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type wsInbound struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Data any    `json:"data"`
}

func newWebServer(staticDir string) *webServer {
	s := &webServer{
		staticDir: staticDir,
		methods:   map[string]reflect.Method{},
		runtimes:  newRuntimeManager(),
	}
	t := reflect.TypeOf((*App)(nil))
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		s.methods[m.Name] = m
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/auth/status", webauth.HandleStatus)
	mux.HandleFunc("/api/auth/register", webauth.HandleRegister)
	mux.HandleFunc("/api/auth/login", webauth.HandleLogin)
	mux.HandleFunc("/api/auth/logout", webauth.HandleLogout)
	mux.HandleFunc("/api/auth/me", webauth.HandleMe)
	mux.HandleFunc("/api/auth/users", webauth.HandleUsers)
	mux.HandleFunc("/api/auth/users/", webauth.HandleUserDisabled)
	mux.HandleFunc("/api/rpc", s.withAuth(s.handleRPC))
	mux.HandleFunc("/api/ws", s.withAuth(s.handleWS))
	mux.HandleFunc("/api/upload", s.withAuth(s.handleUpload))
	mux.HandleFunc("/api/download/", s.withAuth(s.handleDownload))
	mux.Handle("/", s.staticHandler())

	s.http = &http.Server{
		Handler:           webcors.Middleware(mux),
		ReadHeaderTimeout: 15 * time.Second,
	}
	return s
}

func (s *webServer) ListenAndServe(addr string) error {
	s.http.Addr = addr
	return s.http.ListenAndServe()
}

func (s *webServer) Shutdown(ctx context.Context) error {
	s.runtimes.shutdown(ctx)
	return s.http.Shutdown(ctx)
}

func (s *webServer) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := webauth.CurrentUser(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "未登录"})
			return
		}
		next(w, r.WithContext(webauth.WithUser(r.Context(), u)))
	}
}

func (s *webServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"version": Version,
		"time":    time.Now().Format("2006-01-02 15:04:05"),
	})
}

func (s *webServer) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := webauth.UserFromRequest(r)
	item, err := s.runtimes.get(u.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, rpcResponse{Error: "workspace: " + err.Error()})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRPCBodyBytes)
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, rpcResponse{Error: "invalid json: " + err.Error()})
		return
	}
	clientID := r.Header.Get("X-Client-Id")
	result, err := s.invoke(item, clientID, req.Method, req.Args)
	if err != nil {
		writeJSON(w, http.StatusOK, rpcResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rpcResponse{Result: result})
}

func (s *webServer) invoke(item *userRuntime, clientID, method string, rawArgs []json.RawMessage) (any, error) {
	m, ok := s.methods[method]
	if !ok {
		return nil, fmt.Errorf("unknown method: %s", method)
	}
	tenant.Bind(item.rt)
	defer tenant.Unbind()
	events.PushCaller(clientID)
	defer events.PopCaller()

	in := make([]reflect.Value, 0, m.Type.NumIn())
	in = append(in, reflect.ValueOf(item.app))
	need := m.Type.NumIn() - 1
	if len(rawArgs) < need {
		padded := make([]json.RawMessage, need)
		copy(padded, rawArgs)
		for i := len(rawArgs); i < need; i++ {
			padded[i] = []byte("null")
		}
		rawArgs = padded
	}
	for i := 0; i < need; i++ {
		argType := m.Type.In(i + 1)
		raw := json.RawMessage("null")
		if i < len(rawArgs) && len(rawArgs[i]) > 0 {
			raw = rawArgs[i]
		}
		val, err := convertJSONArg(raw, argType)
		if err != nil {
			return nil, fmt.Errorf("arg %d: %w", i, err)
		}
		in = append(in, val)
	}

	out := m.Func.Call(in)
	return packResults(out)
}

func convertJSONArg(raw json.RawMessage, t reflect.Type) (reflect.Value, error) {
	if string(raw) == "null" || len(raw) == 0 {
		return reflect.Zero(t), nil
	}
	ptr := reflect.New(t)
	if err := json.Unmarshal(raw, ptr.Interface()); err != nil {
		return reflect.Value{}, err
	}
	return ptr.Elem(), nil
}

func packResults(out []reflect.Value) (any, error) {
	if len(out) == 0 {
		return nil, nil
	}
	last := out[len(out)-1]
	if last.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		if !last.IsNil() {
			return nil, last.Interface().(error)
		}
		if len(out) == 1 {
			return nil, nil
		}
		if len(out) == 2 {
			return exportValue(out[0]), nil
		}
		vals := make([]any, 0, len(out)-1)
		for i := 0; i < len(out)-1; i++ {
			vals = append(vals, exportValue(out[i]))
		}
		return vals, nil
	}
	if len(out) == 1 {
		return exportValue(out[0]), nil
	}
	vals := make([]any, 0, len(out))
	for _, v := range out {
		vals = append(vals, exportValue(v))
	}
	return vals, nil
}

func exportValue(v reflect.Value) any {
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Pointer && v.IsNil() {
		return nil
	}
	return v.Interface()
}

func (s *webServer) handleWS(w http.ResponseWriter, r *http.Request) {
	u := webauth.UserFromRequest(r)
	item, err := s.runtimes.get(u.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	clientID := r.URL.Query().Get("clientId")
	if clientID == "" {
		clientID = uuid.NewString()
	}
	session := webauth.SessionToken(r)
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		logger.SugaredLogger.Errorf("ws upgrade: %v", err)
		return
	}

	c := events.Default.SubscribeUserSession(clientID, u.ID, session)
	var writeMu sync.Mutex
	write := func(v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		return wsutil.WriteServerText(conn, b)
	}

	_ = write(map[string]any{"name": "connected", "data": map[string]string{"clientId": clientID}})

	go func() {
		defer conn.Close()
		for ev := range c.Events() {
			if err := write(ev); err != nil {
				return
			}
		}
	}()

	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if _, err := webauth.UserBySession(session); err != nil {
					_ = conn.Close()
					return
				}
			}
		}
	}()

	defer events.Default.Unsubscribe(c)
	_ = item
	for {
		data, _, err := wsutil.ReadClientData(conn)
		if err != nil {
			return
		}
		if len(data) == 0 {
			continue
		}
		var msg wsInbound
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.Type == "emit" && msg.Name != "" {
			if msg.Name == "frontendError" {
				logger.SugaredLogger.Errorf("Frontend error: %v", msg.Data)
			}
		}
		if msg.Type == "ping" {
			_ = write(map[string]any{"name": "pong", "data": time.Now().Unix()})
		}
	}
}

func (s *webServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := webauth.UserFromRequest(r)
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBodyBytes)
	if err := r.ParseMultipartForm(maxUploadBodyBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing file"})
		return
	}
	defer file.Close()
	dir := userUploadDir(u.ID)
	name := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(hdr.Filename))
	name = strings.ReplaceAll(name, "..", "_")
	dstPath := filepath.Join(dir, name)
	dst, err := os.Create(dstPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		_ = os.Remove(dstPath)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	abs, _ := filepath.Abs(dstPath)
	writeJSON(w, http.StatusOK, map[string]any{"path": abs, "name": hdr.Filename})
}

func (s *webServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/download/")
	id = strings.Trim(id, "/")
	filename, content, ok := webdownload.Take(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
	_, _ = w.Write(content)
}

func (s *webServer) staticHandler() http.Handler {
	dir := s.staticDir
	fs := http.Dir(dir)
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
