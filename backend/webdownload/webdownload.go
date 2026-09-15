package webdownload

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const Prefix = "WEB_DOWNLOAD:"
const maxDownloadBytes = 32 << 20

type item struct {
	Filename  string
	Content   []byte
	ExpiresAt time.Time
}

var (
	mu    sync.Mutex
	store = map[string]item{}
)

// PutAndFormat 缓存文件并返回前端可识别的下载令牌：WEB_DOWNLOAD:<id>:<filename>
func PutAndFormat(filename string, content []byte) string {
	id := Put(filename, content)
	if id == "" {
		return "文件过大,无法保存。"
	}
	return fmt.Sprintf("%s%s:%s", Prefix, id, filename)
}

// Put 缓存待下载内容，返回 id。
func Put(filename string, content []byte) string {
	if len(content) > maxDownloadBytes {
		return ""
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		b = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
	}
	id := hex.EncodeToString(b)
	mu.Lock()
	defer mu.Unlock()
	cleanupLocked()
	store[id] = item{
		Filename:  filename,
		Content:   content,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	return id
}

// Take 取出并删除缓存。
func Take(id string) (filename string, content []byte, ok bool) {
	mu.Lock()
	defer mu.Unlock()
	cleanupLocked()
	it, ok := store[id]
	if !ok {
		return "", nil, false
	}
	delete(store, id)
	return it.Filename, it.Content, true
}

func cleanupLocked() {
	now := time.Now()
	for k, v := range store {
		if now.After(v.ExpiresAt) {
			delete(store, k)
		}
	}
}
