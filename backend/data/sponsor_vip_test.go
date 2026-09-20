package data

import (
	"testing"
	"time"
)

func TestLocalVipUnlocked(t *testing.T) {
	t.Setenv(LocalVipUnlockEnv, "")
	if LocalVipUnlocked() {
		t.Fatal("empty env should be locked")
	}

	for _, v := range []string{"1", "true", "TRUE", "Yes", "on", " On "} {
		t.Setenv(LocalVipUnlockEnv, v)
		if !LocalVipUnlocked() {
			t.Fatalf("%q should unlock local VIP", v)
		}
	}
	for _, v := range []string{"0", "false", "no", "off", "random"} {
		t.Setenv(LocalVipUnlockEnv, v)
		if LocalVipUnlocked() {
			t.Fatalf("%q should not unlock local VIP", v)
		}
	}
}

func TestEffectiveSponsorVipLevelWhenUnlocked(t *testing.T) {
	t.Setenv(LocalVipUnlockEnv, "true")
	level, active := EffectiveSponsorVipLevel()
	if level != 2 || !active {
		t.Fatalf("unlocked level=%d active=%v, want 2/true", level, active)
	}
}

func TestFollowCountLimitedWhenUnlocked(t *testing.T) {
	t.Setenv(LocalVipUnlockEnv, "true")
	if followCountLimited(100) {
		t.Fatal("watchlist cap should not apply when local VIP is unlocked")
	}
	if followCountLimited(0) {
		t.Fatal("empty watchlist should not be limited")
	}
}

func TestUnlockedLocalSponsorInfo(t *testing.T) {
	info := UnlockedLocalSponsorInfo()
	lvl, _ := info["vipLevel"].(int)
	if lvl != 2 {
		t.Fatalf("vipLevel=%v, want 2", info["vipLevel"])
	}
	end, err := time.ParseInLocation("2006-01-02 15:04:05", info["vipEndTime"].(string), time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if !time.Now().Before(end) {
		t.Fatalf("vipEndTime %v should be in the future", end)
	}
	if _, ok := info["winDownUrl"]; ok {
		t.Fatal("unlocked info must not include CDN download URLs")
	}
}
