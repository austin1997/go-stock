//go:build !windows && !darwin
// +build !windows,!darwin

package data

import (
	"go-stock/backend/logger"

	"github.com/gen2brain/beeep"
)

type AlertWindowsApi struct {
	AppID   string
	Title   string
	Content string
	Icon    string
}

func NewAlertWindowsApi(AppID string, Title string, Content string, Icon string) *AlertWindowsApi {
	return &AlertWindowsApi{
		AppID:   AppID,
		Title:   Title,
		Content: Content,
		Icon:    Icon,
	}
}

func (a AlertWindowsApi) SendNotification() bool {
	if GetSettingConfig().LocalPushEnable == false {
		logger.SugaredLogger.Error("本地推送未开启")
		return false
	}
	if err := beeep.Notify(a.Title, a.Content, a.Icon); err != nil {
		logger.SugaredLogger.Errorf("本地通知失败: %v", err)
		return false
	}
	return true
}
