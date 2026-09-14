package events

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"go-stock/backend/webmode"
)

type Event struct {
	Name string `json:"name"`
	Data any    `json:"data"`
}

type Client struct {
	ID string
	ch chan Event
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

var Default = NewHub()

var (
	callerMu sync.Map // goroutine id -> clientID
)

func NewHub() *Hub {
	return &Hub{clients: make(map[string]*Client)}
}

type ctxKey string

const clientIDKey ctxKey = "gostock-client-id"

func WithClientID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, clientIDKey, id)
}

func ClientIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(clientIDKey).(string)
	return v
}

func PushCaller(clientID string) {
	if clientID == "" {
		return
	}
	callerMu.Store(goroutineID(), clientID)
}

func PopCaller() {
	callerMu.Delete(goroutineID())
}

func Caller() string {
	v, ok := callerMu.Load(goroutineID())
	if !ok {
		return ""
	}
	id, _ := v.(string)
	return id
}

func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	var id uint64
	_, _ = fmt.Sscanf(string(buf[:n]), "goroutine %d ", &id)
	return id
}

func directedEvent(name string) bool {
	switch name {
	case "agent-message", "newChatStream", "kb-qa-message":
		return true
	default:
		return strings.HasPrefix(name, "summaryStockNews")
	}
}

func (h *Hub) Subscribe(id string) *Client {
	c := &Client{
		ID: id,
		ch: make(chan Event, 256),
	}
	h.mu.Lock()
	old := h.clients[id]
	h.clients[id] = c
	if old != nil {
		close(old.ch)
	}
	h.mu.Unlock()
	return c
}

func (h *Hub) Unsubscribe(c *Client) {
	if c == nil {
		return
	}
	h.mu.Lock()
	cur := h.clients[c.ID]
	if cur == c {
		delete(h.clients, c.ID)
		close(c.ch)
	}
	h.mu.Unlock()
}

func (c *Client) Events() <-chan Event {
	return c.ch
}

func (h *Hub) Broadcast(ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.clients {
		select {
		case c.ch <- ev:
		default:
		}
	}
}

func (h *Hub) SendTo(id string, ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c := h.clients[id]
	if c == nil {
		return
	}
	select {
	case c.ch <- ev:
	default:
	}
}

// Emit 同时推送到网页 Hub，并在桌面版转发到 Wails EventsEmit。
func Emit(ctx context.Context, name string, data ...any) {
	var payload any
	switch len(data) {
	case 0:
		payload = nil
	case 1:
		payload = data[0]
	default:
		payload = data
	}
	ev := Event{Name: name, Data: payload}

	clientID := ClientIDFrom(ctx)
	if clientID == "" {
		clientID = Caller()
	}
	if clientID != "" && directedEvent(name) {
		Default.SendTo(clientID, ev)
	} else {
		Default.Broadcast(ev)
	}

	if ctx == nil || webmode.Enabled() {
		return
	}
	defer func() { _ = recover() }()
	wailsruntime.EventsEmit(ctx, name, data...)
}
