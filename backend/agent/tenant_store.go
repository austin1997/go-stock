package agent

import (
	"fmt"
	"path/filepath"
	"strings"

	"go-stock/backend/tenant"
	"go-stock/backend/webmode"
)

func memoryTenantKey() string {
	return deepAgentRootDir()
}

func scopedName(name string) string {
	return memoryTenantKey() + "\x1f" + name
}

func graphKey(name string) string {
	return scopedName(name)
}

func restrictWebUploadPath(filePath string) (string, error) {
	if !webmode.Enabled() {
		return filePath, nil
	}
	root := tenant.Root()
	if root == "" {
		return "", fmt.Errorf("未绑定用户工作空间")
	}
	uploadDir := filepath.Join(root, "tmp")
	absDir, err := filepath.Abs(uploadDir)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absDir); err == nil {
		absDir = resolved
	}
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	if !pathUnderDir(absFile, absDir) {
		return "", fmt.Errorf("文件路径不在允许的上传目录内")
	}
	if resolved, err := filepath.EvalSymlinks(absFile); err == nil {
		absFile = resolved
		if !pathUnderDir(absFile, absDir) {
			return "", fmt.Errorf("文件路径不在允许的上传目录内")
		}
	}
	return absFile, nil
}

func pathUnderDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
