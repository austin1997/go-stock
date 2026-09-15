package webauth

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

var migrateMu sync.Mutex

func UsersRoot() string {
	return filepath.Join("data", "users")
}

func WorkspaceRoot(userID uint) string {
	return filepath.Join(UsersRoot(), strconv.FormatUint(uint64(userID), 10))
}

func StockDBPath(userID uint) string {
	return filepath.Join(WorkspaceRoot(userID), "stock.db")
}

func EnsureWorkspaceDirs(userID uint) error {
	root := WorkspaceRoot(userID)
	for _, dir := range []string{
		root,
		filepath.Join(root, "tmp"),
		filepath.Join(root, "memory"),
		filepath.Join(root, "skills"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func defaultSkillsSource() string {
	if env := os.Getenv("GO_STOCK_ROOT_DIR"); env != "" {
		p := filepath.Join(env, "skills")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		p := filepath.Join(filepath.Dir(exe), "skills")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	if st, err := os.Stat("skills"); err == nil && st.IsDir() {
		return "skills"
	}
	return ""
}

func dirEmpty(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()
	names, err := f.Readdirnames(1)
	return err != nil || len(names) == 0
}

func CopyDefaultSkills(userID uint) error {
	src := defaultSkillsSource()
	if src == "" {
		return nil
	}
	dst := filepath.Join(WorkspaceRoot(userID), "skills")
	if !dirEmpty(dst) {
		return nil
	}
	absSrc, _ := filepath.Abs(src)
	absDst, _ := filepath.Abs(dst)
	if absSrc == absDst {
		return nil
	}
	return copyDir(src, dst)
}

// MigrateLegacyIfNeeded 将单用户 data/stock.db、memory/、skills/ 迁入第一个用户工作空间。
func MigrateLegacyIfNeeded(userID uint) error {
	migrateMu.Lock()
	defer migrateMu.Unlock()

	legacyDB := filepath.Join("data", "stock.db")
	marker := filepath.Join("data", "stock.db.migrated")
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	if !ownsLegacyMigration(userID) {
		return nil
	}
	dstDB := StockDBPath(userID)
	_, legacyErr := os.Stat(legacyDB)
	_, dstErr := os.Stat(dstDB)
	if legacyErr != nil && dstErr != nil {
		return nil
	}
	if err := EnsureWorkspaceDirs(userID); err != nil {
		return err
	}
	if legacyErr == nil && dstErr != nil {
		if err := migrateSQLiteFiles(legacyDB, dstDB); err != nil {
			return fmt.Errorf("migrate stock.db: %w", err)
		}
	}
	if st, err := os.Stat("memory"); err == nil && st.IsDir() {
		dst := filepath.Join(WorkspaceRoot(userID), "memory")
		if dirEmpty(dst) {
			if err := copyDir("memory", dst); err != nil {
				_ = os.RemoveAll(dst)
				return fmt.Errorf("migrate memory: %w", err)
			}
		}
	}
	if st, err := os.Stat("skills"); err == nil && st.IsDir() {
		dst := filepath.Join(WorkspaceRoot(userID), "skills")
		if dirEmpty(dst) {
			if err := copyDir("skills", dst); err != nil {
				_ = os.RemoveAll(dst)
				return fmt.Errorf("migrate skills: %w", err)
			}
		}
	}
	if err := os.WriteFile(marker, []byte("1"), 0o644); err != nil {
		return fmt.Errorf("write migration marker: %w", err)
	}
	return nil
}

func ownsLegacyMigration(userID uint) bool {
	if authDB == nil {
		return true
	}
	var first User
	if err := authDB.Order("id asc").First(&first).Error; err != nil {
		return false
	}
	return first.ID == userID
}

func migrateSQLiteFiles(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		from := src + suffix
		to := dst + suffix
		if _, err := os.Stat(from); err != nil {
			continue
		}
		if err := os.Rename(from, to); err == nil {
			continue
		}
		if err := copyFile(from, to); err != nil {
			return err
		}
		_ = os.Rename(from, from+".bak")
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

const (
	MaxUserTmpBytes int64 = 256 << 20
	MaxUserTmpFiles       = 40
	UserTmpMaxAge         = 24 * time.Hour
)

type tmpFileInfo struct {
	path string
	mod  time.Time
	size int64
}

// EnforceTmpQuota 清理过期上传文件，并在需要时淘汰最旧文件，为 incoming 字节腾出空间。
func EnforceTmpQuota(dir string, incoming int64) error {
	if incoming < 0 {
		incoming = 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var files []tmpFileInfo
	now := time.Now()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		st, err := os.Lstat(p)
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		if UserTmpMaxAge > 0 && now.Sub(st.ModTime()) > UserTmpMaxAge {
			_ = os.Remove(p)
			continue
		}
		files = append(files, tmpFileInfo{path: p, mod: st.ModTime(), size: st.Size()})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].mod.Before(files[j].mod)
	})
	var total int64
	for _, f := range files {
		total += f.size
	}
	i := 0
	count := len(files)
	if incoming > 0 {
		count++
	}
	for i < len(files) && (total+incoming > MaxUserTmpBytes || count-i > MaxUserTmpFiles) {
		_ = os.Remove(files[i].path)
		total -= files[i].size
		i++
	}
	if total+incoming > MaxUserTmpBytes {
		return fmt.Errorf("上传目录已满，请删除旧文件后重试")
	}
	return nil
}
