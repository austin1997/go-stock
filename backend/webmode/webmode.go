package webmode

import "sync/atomic"

var enabled atomic.Bool

// Enable 标记当前进程为网页版（goweb）。桌面版不要调用。
func Enable() {
	enabled.Store(true)
}

// Disable 仅用于测试，恢复为桌面模式。
func Disable() {
	enabled.Store(false)
}

// Enabled 返回是否运行在网页版进程中。
func Enabled() bool {
	return enabled.Load()
}
