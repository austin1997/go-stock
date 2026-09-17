package data

import (
	"fmt"
	"path/filepath"

	"go-stock/backend/tenant"
)

// tenantDataFile 网页版把可写 JSON 放到当前用户工作空间，桌面版仍用进程相对路径。
func tenantDataFile(name string) string {
	if root := tenant.Root(); root != "" {
		return filepath.Join(root, "data", name)
	}
	return filepath.Join("data", name)
}

func tenantCacheKey(key string) string {
	return fmt.Sprintf("%d:%s", tenant.UserID(), key)
}
