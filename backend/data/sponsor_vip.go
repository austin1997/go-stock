package data

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/cryptor"
)

// LocalVipUnlockEnv 为 true/1/yes/on 时解除本地 VIP2 功能门禁（不影响广场远端校验）。
const LocalVipUnlockEnv = "GO_STOCK_UNLOCK_LOCAL_VIP"

// DefaultSponsorAESKeyHex 与 main.checkDir 在 BuildKey 为空时的回退值一致，
// 供 ai-assistant-web 等独立进程解密本地配置中的赞助码。
const DefaultSponsorAESKeyHex = ""

// SponsorDecryptKeyHex 由主程序在启动时同步为 ldflags 注入的 BuildKey；为空则使用 DefaultSponsorAESKeyHex。
var SponsorDecryptKeyHex string

// SafeDecryptSponsorCode 解密赞助码（hex 解码 + AES-ECB）。
// 预校验密钥长度（16/24/32 字节）与密文块长度（16 字节整数倍），并 defer recover 兜底
// lancet AesEcbDecrypt 对非法输入（如密钥不匹配导致 PKCS#7 填充非法）的直接 panic；
// 任何失败以 error 返回，绝不使调用方进程崩溃。
func SafeDecryptSponsorCode(sponsorCode, keyHex string) (raw []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			raw, err = nil, fmt.Errorf("赞助码解密异常: %v", r)
		}
	}()
	sponsorCode = strings.TrimSpace(sponsorCode)
	if sponsorCode == "" {
		return nil, fmt.Errorf("赞助码为空")
	}
	encrypted, err := hex.DecodeString(sponsorCode)
	if err != nil {
		return nil, fmt.Errorf("赞助码 hex 解码失败: %w", err)
	}
	key, err := hex.DecodeString(strings.TrimSpace(keyHex))
	if err != nil {
		return nil, fmt.Errorf("赞助码密钥 hex 解码失败: %w", err)
	}
	if l := len(key); l != 16 && l != 24 && l != 32 {
		return nil, fmt.Errorf("赞助码密钥长度非法: %d 字节", l)
	}
	if len(encrypted) == 0 || len(encrypted)%16 != 0 {
		return nil, fmt.Errorf("赞助码密文长度非法: %d 字节（须为 16 字节整数倍）", len(encrypted))
	}
	return cryptor.AesEcbDecrypt(encrypted, key), nil
}

// LocalVipUnlocked 是否通过环境变量解除本地 VIP 功能限制。
func LocalVipUnlocked() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(LocalVipUnlockEnv))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// UnlockedLocalSponsorInfo 无有效赞助码时供界面展示的本地解锁信息（不写入赞助码，不含 CDN 下载地址）。
func UnlockedLocalSponsorInfo() map[string]any {
	return map[string]any{
		"vipLevel":     2,
		"vipStartTime": "2000-01-01 00:00:00",
		"vipEndTime":   "2099-12-31 23:59:59",
		"vipAuthTime":  "2000-01-01 00:00:00",
	}
}

// EffectiveSponsorVipLevel 根据设置中的 sponsorCode 解析 VIP 等级，并按 vipAuthTime / vipStartTime / vipEndTime 判断是否当前有效。
// 与 app.isVip 时间判断逻辑保持一致。
func EffectiveSponsorVipLevel() (level int, active bool) {
	if LocalVipUnlocked() {
		return 2, true
	}
	keyHex := strings.TrimSpace(SponsorDecryptKeyHex)
	if keyHex == "" {
		keyHex = DefaultSponsorAESKeyHex
	}
	raw, err := SafeDecryptSponsorCode(GetSettingConfig().SponsorCode, keyHex)
	if err != nil || len(raw) == 0 {
		return 0, false
	}
	var info map[string]any
	if err := json.Unmarshal(raw, &info); err != nil {
		return 0, false
	}
	lvl64, _ := convertor.ToInt(info["vipLevel"])
	lvl := int(lvl64)
	vipStartTime, err1 := time.ParseInLocation("2006-01-02 15:04:05", convertor.ToString(info["vipStartTime"]), time.Local)
	vipEndTime, err2 := time.ParseInLocation("2006-01-02 15:04:05", convertor.ToString(info["vipEndTime"]), time.Local)
	vipAuthTime, err3 := time.ParseInLocation("2006-01-02 15:04:05", convertor.ToString(info["vipAuthTime"]), time.Local)
	if err1 != nil || err2 != nil || err3 != nil {
		return lvl, false
	}
	now := time.Now()
	if now.After(vipAuthTime) && now.After(vipStartTime) && now.Before(vipEndTime) {
		return lvl, true
	}
	return lvl, false
}

const maxNonVipFollowCount = 63

// followCountLimited 非有效 VIP 时自选数量达到上限则不可再加。
func followCountLimited(count int64) bool {
	_, active := EffectiveSponsorVipLevel()
	return !active && count >= maxNonVipFollowCount
}
