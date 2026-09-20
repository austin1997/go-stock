package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/events"
	log "go-stock/backend/logger"
	"go-stock/backend/models"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/slice"
)

//go:embed build/appicon.png
var icon []byte

//go:embed build/app.ico
var icon2 []byte

//go:embed build/screenshot/alipay.jpg
var alipay []byte

//go:embed build/screenshot/wxpay.jpg
var wxpay []byte

//go:embed build/screenshot/扫码_搜索联合传播样式-白色版.png
var wxgzh []byte

//go:embed build/stock_basic.json
var stocksBin []byte

//go:embed build/stock_base_info_hk.json
var stocksBinHK []byte

//go:embed build/stock_base_info_us.json
var stocksBinUS []byte

//go:embed docs/go-stock使用手册.md
var userManual []byte

//go:generate cp -R ./data ./build/bin

var Version string
var VersionCommit string
var OFFICIAL_STATEMENT string
var BuildKey string

func cacheCookies(url string) {
	log.SugaredLogger.Info("预缓存东财 Cookie...")
	_, err := data.FetchEastMoneyCookiesViaChromedp("", 3*time.Minute, url)
	if err != nil {
		log.SugaredLogger.Warnf("预缓存东财 Cookie 失败：%v", err)
	} else {
		log.SugaredLogger.Info("东财 Cookie 预缓存完成")
	}
}

func updateMultipleModel() {
	oldSettings := &models.OldSettings{}
	db.Dao.Model(oldSettings).First(oldSettings)
	aiConfig := &data.AIConfig{}
	db.Dao.Model(aiConfig).First(aiConfig)
	if oldSettings.OpenAiEnable && oldSettings.OpenAiApiKey != "" && aiConfig.ID == 0 {
		aiConfig.Name = oldSettings.OpenAiModelName
		aiConfig.ApiKey = oldSettings.OpenAiApiKey
		aiConfig.BaseUrl = oldSettings.OpenAiBaseUrl
		aiConfig.ModelName = oldSettings.OpenAiModelName
		aiConfig.Temperature = oldSettings.OpenAiTemperature
		aiConfig.MaxTokens = oldSettings.OpenAiMaxTokens
		aiConfig.TimeOut = oldSettings.OpenAiApiTimeOut
		err := db.Dao.Model(aiConfig).Create(aiConfig).Error
		if err != nil {
			log.SugaredLogger.Error(err.Error())
		}
	}
}

func AutoMigrate() {
	db.Dao.AutoMigrate(&data.StockInfo{})
	db.Dao.AutoMigrate(&data.StockBasic{})
	db.Dao.AutoMigrate(&data.FollowedStock{})
	db.Dao.AutoMigrate(&data.IndexBasic{})
	db.Dao.AutoMigrate(&data.Settings{})
	db.Dao.AutoMigrate(&models.AIResponseResult{})
	db.Dao.AutoMigrate(&models.StockInfoHK{})
	db.Dao.AutoMigrate(&models.StockInfoUS{})
	db.Dao.AutoMigrate(&data.FollowedFund{})
	db.Dao.AutoMigrate(&data.FollowedStock{})
	db.Dao.AutoMigrate(&data.FundBasic{})
	db.Dao.AutoMigrate(&models.PromptTemplate{})
	db.Dao.AutoMigrate(&data.Group{})
	db.Dao.AutoMigrate(&data.GroupStock{})
	db.Dao.AutoMigrate(&data.Concept{})
	db.Dao.AutoMigrate(&data.ConceptStock{})
	db.Dao.AutoMigrate(&models.Tags{})
	db.Dao.AutoMigrate(&models.Telegraph{})
	db.Dao.AutoMigrate(&models.TelegraphTags{})
	db.Dao.AutoMigrate(&models.LongTigerRankData{})
	db.Dao.AutoMigrate(&data.AIConfig{})
	db.Dao.AutoMigrate(&models.BKDict{})
	db.Dao.AutoMigrate(&models.WordAnalyze{})
	db.Dao.AutoMigrate(&models.SentimentResultAnalyze{})
	db.Dao.AutoMigrate(&models.AiRecommendStocks{})
	db.Dao.AutoMigrate(&models.AllStockInfo{})
	db.Dao.AutoMigrate(&models.CronTask{})
	db.Dao.AutoMigrate(&models.AiAssistantSession{})
	db.Dao.AutoMigrate(&models.GlobalStockIndex{})
	db.Dao.AutoMigrate(&data.TradingRecord{})
	db.Dao.AutoMigrate(&models.MCPServer{})
	db.Dao.AutoMigrate(&models.MCPServerTool{})
	db.Dao.AutoMigrate(&models.Skill{})
	db.Dao.AutoMigrate(&models.CustomStrategy{})
	db.Dao.AutoMigrate(&models.BKFundFlow{})
	db.Dao.AutoMigrate(&models.ConceptFundFlow{})
	db.Dao.AutoMigrate(&models.DailyOperationPlan{})
	db.Dao.AutoMigrate(&models.DailyReview{})
	db.Dao.AutoMigrate(&models.MorningStrategy{})

	//updateMultipleModel()

	// 初始化 global_stock_index_cache 定时任务
	initGlobalStockIndexCacheTask()
}

// initGlobalStockIndexCacheTask 检查并创建 global_stock_index_cache 定时任务
func initGlobalStockIndexCacheTask() {
	var count int64
	db.Dao.Model(&models.CronTask{}).Where("task_type = ?", "global_stock_index_cache").Count(&count)
	if count == 0 {
		task := &models.CronTask{
			Name:        "全球指数缓存",
			CronExpr:    "0 0/5 * * * *", // 每分钟执行一次
			TaskType:    "global_stock_index_cache",
			Target:      "",
			Params:      `{"crawlTimeOut": 30}`,
			Enable:      true,
			Status:      "active",
			Description: "自动缓存全球股票指数数据",
		}
		err := db.Dao.Create(task).Error
		if err != nil {
			log.SugaredLogger.Errorf("创建 global_stock_index_cache 定时任务失败：%v", err)
		} else {
			log.SugaredLogger.Info("创建 global_stock_index_cache 定时任务成功")
		}
	}

}

func initStockDataUS(ctx context.Context) {
	defer func() {
		go events.Emit(ctx, "loadingMsg", "done")
	}()
	var v []models.StockInfoUS
	err := json.Unmarshal(stocksBinUS, &v)
	if err != nil {
		log.SugaredLogger.Error(err.Error())
		return
	}
	log.SugaredLogger.Infof("init stock data us %d", len(v))
	var total int64
	db.Dao.Model(&models.StockInfoUS{}).Count(&total)
	if total != int64(len(v)) {
		for _, item := range v {
			var count int64
			db.Dao.Model(&models.StockInfoUS{}).Where("code = ?", item.Code).Count(&count)
			if count > 0 {
				//log.SugaredLogger.Infof("stock data us %s exist", item.Code)
				continue
			}
			db.Dao.Model(&models.StockInfoUS{}).Create(&item)
		}
	}
}

func initStockDataHK(ctx context.Context) {
	defer func() {
		go events.Emit(ctx, "loadingMsg", "done")
	}()
	var v []models.StockInfoHK
	err := json.Unmarshal(stocksBinHK, &v)
	if err != nil {
		log.SugaredLogger.Error(err.Error())
		return
	}
	log.SugaredLogger.Infof("init stock data hk %d", len(v))
	var total int64
	db.Dao.Model(&models.StockInfoHK{}).Count(&total)
	if total != int64(len(v)) {
		for _, item := range v {
			var count int64
			db.Dao.Model(&models.StockInfoHK{}).Where("code = ?", item.Code).Count(&count)
			if count > 0 {
				//log.SugaredLogger.Infof("stock data hk %s exist", item.Code)
				continue
			}
			db.Dao.Model(&models.StockInfoHK{}).Create(&item)
		}
	}

}

func updateBasicInfo() {
	config := data.GetSettingConfig()
	if config.UpdateBasicInfoOnStart {
		//更新基本信息
		go data.NewStockDataApi().GetStockBaseInfo()
		go data.NewStockDataApi().GetIndexBasic()
	}
}

func initStockData(ctx context.Context) {
	defer func() {
		go events.Emit(ctx, "loadingMsg", "done")
	}()
	fields := "ts_code,symbol,name,area,industry,cnspell,market,list_date,act_name,act_ent_type,fullname,exchange,list_status,curr_type,enname,delist_date,is_hs"
	log.SugaredLogger.Info("init stock data")
	res := &data.TushareStockBasicResponse{}
	err := json.Unmarshal(stocksBin, res)
	if err != nil {
		log.SugaredLogger.Error(err.Error())
		return
	}

	for _, item := range res.Data.Items {
		stock := &data.StockBasic{}
		stockData := map[string]any{}
		for _, field := range strings.Split(fields, ",") {
			//logger.SugaredLogger.Infof("field: %s", field)
			idx := slice.IndexOf(res.Data.Fields, field)
			if idx == -1 {
				continue
			}
			stockData[field] = item[idx]
		}
		jsonData, _ := json.Marshal(stockData)
		err := json.Unmarshal(jsonData, stock)
		if err != nil {
			continue
		}
		stock.ID = 0
		var count int64
		db.Dao.Model(&data.StockBasic{}).Where("ts_code = ?", stock.TsCode).Count(&count)
		if count > 0 {
			continue
		} else {
			db.Dao.Create(stock)
		}

		//db.Dao.Model(&data.StockBasic{}).FirstOrCreate(stock, &data.StockBasic{TsCode: stock.TsCode}).Where("ts_code = ?", stock.TsCode).Updates(stock)
	}

	//for _, item := range res.Data.Items {
	//	stock := &data.StockBasic{}
	//	stock.Exchange = convertor.ToString(item[0])
	//	stock.IsHs = convertor.ToString(item[1])
	//	stock.Name = convertor.ToString(item[2])
	//	stock.Industry = convertor.ToString(item[3])
	//	stock.ListStatus = convertor.ToString(item[4])
	//	stock.ActName = convertor.ToString(item[5])
	//	stock.ID = uint(item[6].(float64))
	//	stock.CurrType = convertor.ToString(item[7])
	//	stock.Area = convertor.ToString(item[8])
	//	stock.ListDate = convertor.ToString(item[9])
	//	stock.DelistDate = convertor.ToString(item[10])
	//	stock.ActEntType = convertor.ToString(item[11])
	//	stock.TsCode = convertor.ToString(item[12])
	//	stock.Symbol = convertor.ToString(item[13])
	//	stock.Cnspell = convertor.ToString(item[14])
	//	stock.Fullname = convertor.ToString(item[20])
	//	stock.Ename = convertor.ToString(item[21])
	//
	//	var count int64
	//	db.Dao.Model(&data.StockBasic{}).Where("ts_code = ?", stock.TsCode).Count(&count)
	//	if count > 0 {
	//		continue
	//	} else {
	//		db.Dao.Create(stock)
	//	}
	//}
}

func checkDir(dir string) {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		os.Mkdir(dir, os.ModePerm)
		log.SugaredLogger.Info("create dir: " + dir)
	}
	if BuildKey == "" {
		BuildKey = "cc1e0d684e32f176c56ff1fcf384dcd9"
	}
}

// PanicHandler 捕获 panic 的包装函数
func PanicHandler() {
	if r := recover(); r != nil {
		fmt.Printf("Recovered from panic: %v\n", r)
		debug.PrintStack()
	}
}
