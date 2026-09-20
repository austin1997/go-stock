package webdownload

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go-stock/backend/tenant"
)

const Prefix = "WEB_DOWNLOAD:"
const maxDownloadBytes = 32 << 20
const maxStoreBytes = 64 << 20
const maxUserBytes = 32 << 20
const maxStoreItems = 64

type item struct {
	Filename  string
	Content   []byte
	ExpiresAt time.Time
	UserID    uint
}

var (
	mu         sync.Mutex
	store      = map[string]item{}
	totalBytes int
)

func resetStore() {
	mu.Lock()
	defer mu.Unlock()
	store = map[string]item{}
	totalBytes = 0
}

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
	need := len(content)
	uid := tenant.UserID()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		b = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
	}
	id := hex.EncodeToString(b)

	mu.Lock()
	defer mu.Unlock()
	cleanupLocked()
	for overQuotaLocked(uid, need) {
		if evictOldestLocked(uid) {
			continue
		}
		if evictOldestLocked(0) {
			continue
		}
		return ""
	}
	store[id] = item{
		Filename:  filename,
		Content:   content,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		UserID:    uid,
	}
	totalBytes += need
	return id
}

func overQuotaLocked(uid uint, need int) bool {
	if len(store) >= maxStoreItems {
		return true
	}
	if totalBytes+need > maxStoreBytes {
		return true
	}
	return userBytesLocked(uid)+need > maxUserBytes
}

func userBytesLocked(uid uint) int {
	n := 0
	for _, it := range store {
		if it.UserID == uid {
			n += len(it.Content)
		}
	}
	return n
}

func evictOldestLocked(preferUser uint) bool {
	var bestID string
	var bestTime time.Time
	found := false
	for id, it := range store {
		if preferUser != 0 && it.UserID != preferUser {
			continue
		}
		if !found || it.ExpiresAt.Before(bestTime) {
			bestID = id
			bestTime = it.ExpiresAt
			found = true
		}
	}
	if !found {
		return false
	}
	totalBytes -= len(store[bestID].Content)
	delete(store, bestID)
	return true
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
	totalBytes -= len(it.Content)
	if totalBytes < 0 {
		totalBytes = 0
	}
	return it.Filename, it.Content, true
}

func cleanupLocked() {
	now := time.Now()
	for k, v := range store {
		if now.After(v.ExpiresAt) {
			totalBytes -= len(v.Content)
			delete(store, k)
		}
	}
	if totalBytes < 0 {
		totalBytes = 0
	}
}
