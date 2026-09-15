package tenant

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"gorm.io/gorm"
)

// Runtime 网页版当前请求/任务绑定的用户工作空间。
// 桌面版不会 Bind，所有读写继续走全局 data/stock.db。
type Runtime struct {
	UserID      uint
	DB          *gorm.DB
	Root        string
	ProxyOn     bool
	ProxyURL    string
	HTTPTimeout time.Duration
}

type ctxKey struct{}

type slot struct {
	rt    *Runtime
	depth int
}

var store sync.Map // goroutine id -> *slot

// Bind 将当前 goroutine 绑定到用户工作空间。可嵌套，须与 Unbind 成对。
func Bind(rt *Runtime) {
	if rt == nil {
		return
	}
	id := goroutineID()
	if v, ok := store.Load(id); ok {
		s := v.(*slot)
		s.rt = rt
		s.depth++
		return
	}
	store.Store(id, &slot{rt: rt, depth: 1})
}

// Unbind 解除当前 goroutine 的租户绑定。
func Unbind() {
	id := goroutineID()
	v, ok := store.Load(id)
	if !ok {
		return
	}
	s := v.(*slot)
	s.depth--
	if s.depth <= 0 {
		store.Delete(id)
	}
}

// Capture 返回当前绑定的 Runtime（同一指针，可供定时任务在其它 goroutine 上 Bind）。
func Capture() *Runtime {
	return Current()
}

// Current 返回当前 goroutine 绑定的工作空间。
func Current() *Runtime {
	v, ok := store.Load(goroutineID())
	if !ok {
		return nil
	}
	s := v.(*slot)
	if s == nil {
		return nil
	}
	return s.rt
}

// UserID 返回当前租户用户 ID，未绑定为 0。
func UserID() uint {
	rt := Current()
	if rt == nil {
		return 0
	}
	return rt.UserID
}

// Root 返回当前用户工作空间根目录，未绑定为空。
func Root() string {
	rt := Current()
	if rt == nil {
		return ""
	}
	return rt.Root
}

// SetProxy 更新当前租户的 HTTP 代理（网页版按请求隔离，不改全局 Transport）。
func SetProxy(proxyURL string, enabled bool) {
	rt := Current()
	if rt == nil {
		return
	}
	rt.ProxyOn = enabled && proxyURL != ""
	if rt.ProxyOn {
		rt.ProxyURL = proxyURL
	} else {
		rt.ProxyURL = ""
	}
}

// SetHTTPTimeout 更新当前租户的出站 HTTP 超时，避免改到全局共享 Client。
func SetHTTPTimeout(d time.Duration) {
	rt := Current()
	if rt == nil {
		return
	}
	rt.HTTPTimeout = d
}

// HTTPTimeout 返回当前租户超时；未绑定为 0。
func HTTPTimeout() time.Duration {
	rt := Current()
	if rt == nil {
		return 0
	}
	return rt.HTTPTimeout
}

// WithRuntime 将 Runtime 写入 context，供异步 Emit 在丢失 goid 绑定时仍能隔离事件。
func WithRuntime(ctx context.Context, rt *Runtime) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if rt == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, rt)
}

// FromContext 读取 WithRuntime 写入的工作空间。
func FromContext(ctx context.Context) *Runtime {
	if ctx == nil {
		return nil
	}
	rt, _ := ctx.Value(ctxKey{}).(*Runtime)
	return rt
}

// Go 在子 goroutine 中继承当前租户绑定。
func Go(fn func()) {
	GoContext(nil, fn)
}

// GoContext 在子 goroutine 中继承当前绑定，若当前未绑定则回退到 context 中的 Runtime。
func GoContext(ctx context.Context, fn func()) {
	if fn == nil {
		return
	}
	rt := Capture()
	if rt == nil {
		rt = FromContext(ctx)
	}
	go func() {
		if rt != nil {
			Bind(rt)
			defer Unbind()
		}
		fn()
	}()
}

func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	var id uint64
	_, _ = fmt.Sscanf(string(buf[:n]), "goroutine %d ", &id)
	return id
}
