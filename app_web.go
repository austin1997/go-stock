//go:build goweb
// +build goweb

package main

import (
	"context"
	"fmt"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/events"
	"go-stock/backend/logger"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
)

func (a *App) startup(ctx context.Context) {
	defer PanicHandler()

	data.ConfigureFromSettings(data.GetSettingConfig())
	a.ctx = ctx
	data.SetAppCtx(ctx)
	a.initJobContext(ctx)
	a.InitCronTasks()
	preCacheTradingDays()
	logger.SugaredLogger.Infof("Version:%s (web)", Version)
}

func (a *App) HideToTray() {}

func (a *App) ShowFromTray() {}

func MonitorStockPrices(a *App) {
	isAStockOpen := isTradingTime(time.Now())
	isHKStockOpen := IsHKTradingTime(time.Now())
	isUSStockOpen := IsUSTradingTime(time.Now())
	if !isAStockOpen && !isHKStockOpen && !isUSStockOpen {
		return
	}

	dest := &[]data.FollowedStock{}
	db.Dao.Model(&data.FollowedStock{}).Find(dest)
	total := float64(0)

	stockInfos := GetStockInfos(*dest...)
	for _, stockInfo := range *stockInfos {
		if strutil.HasPrefixAny(stockInfo.Code, []string{"SZ", "SH", "sh", "sz"}) && (!isTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(stockInfo.Code, []string{"hk", "HK"}) && (!IsHKTradingTime(time.Now())) {
			continue
		}
		if strutil.HasPrefixAny(stockInfo.Code, []string{"us", "US", "gb_"}) && (!IsUSTradingTime(time.Now())) {
			continue
		}

		total += stockInfo.ProfitAmountToday
		price, _ := convertor.ToFloat(stockInfo.Price)
		if stockInfo.PrePrice != price {
			go events.Emit(a.ctx, "stock_price", stockInfo)
		}
	}
	go events.Emit(a.ctx, "realtime_profit", fmt.Sprintf("  %.2f", total))
}
