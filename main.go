package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"energe/model"
	"energe/types"
	"energe/utils"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"golang.org/x/sync/semaphore"
)

/* ====================== 结构体 & 全局 ====================== */

var (
	apiKey      = ""
	secretKey   = ""
	proxyURL    = "http://127.0.0.1:10809"
	klinesCount = 200
	maxWorkers  = 20
	limitVolume = 300000000 // 3 亿 USDT
	botToken    = "8040107823:AAHC_qu5cguJf9BG4NDiUB_nwpgF-bPkJAg"
	chatID      = "6074996357"

	// volumeMap      = map[string]float64{}
	volumeCache *types.VolumeCache
	err         error
	slipCoin    = []string{"XRPUSDT", "DOGEUSDT", "1000PEPEUSDT", "ADAUSDT", "BNBUSDT", "UNIUSDT", "TRUMPUSDT",
		"LINKUSDT", "FARTCOINUSDT", "1000BONKUSDT", "AAVEUSDT", "AVAXUSDT", "SUIUSDT", "LTCUSDT",
		"SEIUSDT", "BCHUSDT", "WIFUSDT", "XLMUSDT", "XRPUSDC", "BNXUSDT", "ETHUSDC", "BTCUSDC", "SOLUSDC"} // 想排除的币放这里
	muVolumeMap    sync.Mutex
	progressLogger = log.New(os.Stdout, "[Screener] ", log.LstdFlags)
	db             *sql.DB
)

/* ====================== 主函数 ====================== */

func main() {
	progressLogger.Println("程序启动...")

	client := binance.NewFuturesClient(apiKey, secretKey)
	setHTTPClient(client)

	// 创建并预热 VolumeCache
	volumeCache, err = utils.NewVolumeCache(client, slipCoin, float64(limitVolume))
	if err != nil {
		log.Fatalf("VolumeCache 启动失败: %v", err)
	}

	<-volumeCache.Ready()
	log.Println("volumeCache 启动成功")
	defer volumeCache.Close()

	fmt.Println(volumeCache.SymbolsAbove(float64(limitVolume)))

	model.InitDB()
	db = model.DB

	// 立即执行一次
	utils.Update1hEMA25ToDB(client, db, float64(limitVolume), klinesCount, volumeCache, slipCoin)
	utils.Update15MEMAToDB(client, db, float64(limitVolume), klinesCount, volumeCache, slipCoin)
	utils.Update5MEMAToDB(client, db, float64(limitVolume), klinesCount, volumeCache, slipCoin)
	if err := runScan(client); err != nil {
		progressLogger.Printf("首次 scan 出错: %v", err)
	}

	for {
		now := time.Now()
		next := now.Truncate(time.Minute).Add(time.Minute) // 下一分钟的开始时间
		time.Sleep(time.Until(next))                       // 每分钟检查一次

		now = time.Now()
		minute := now.Minute()
		hour := now.Hour()

		if minute == 0 {
			time.Sleep(10 * time.Second)
			progressLogger.Printf("整点 %02d:00，执行 Update1hEMA25ToDB", hour)
			go utils.Update1hEMA25ToDB(client, db, float64(limitVolume), klinesCount, volumeCache, slipCoin)
		}

		if minute%15 == 0 {
			time.Sleep(10 * time.Second)
			progressLogger.Printf("每15分钟触发，执行 Update15MEMAToDB", hour)
			go utils.Update15MEMAToDB(client, db, float64(limitVolume), klinesCount, volumeCache, slipCoin)

			//这里进行监控扫描，二级市场
			if err := runScan(client); err != nil {
				progressLogger.Printf("周期 scan 出错: %v", err)
			}
			time.Sleep(1 * time.Minute)
		}

		if minute%5 == 0 {
			time.Sleep(10 * time.Second)
			progressLogger.Printf("每5分钟触发，执行 runScan (%02d:%02d)", hour, minute)
			go utils.Update5MEMAToDB(client, db, float64(limitVolume), klinesCount, volumeCache, slipCoin)

		}
	}
}

/* ====================== 核心扫描 ====================== */

func runScan(client *futures.Client) error {
	progressLogger.Println("开始新一轮扫描...")

	// ---------- 1. 过滤 USDT 交易对 ----------
	var symbols []string
	symbols = volumeCache.SymbolsAbove(float64(limitVolume))
	progressLogger.Printf("USDT 交易对数量: %d", len(symbols))
	// ---------- 3. 并发处理 ----------
	var (
		results []types.CoinIndicator
		resMu   sync.Mutex
		wg      sync.WaitGroup
		sem     = semaphore.NewWeighted(int64(maxWorkers))
	)

	for _, symbol := range symbols {
		if err := sem.Acquire(context.Background(), 1); err != nil {
			progressLogger.Printf("semaphore acquire 失败: %v", err)
			continue
		}

		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			defer sem.Release(1)

			ind, ok := analyseSymbol(client, sym, "15m", db)
			if ok {
				resMu.Lock()
				results = append(results, ind)
				resMu.Unlock()
			}
		}(symbol)
	}
	wg.Wait()

	// --------- 过滤逻辑：优先级保留 ---------
	prioritySymbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "HYPEUSDT"}
	symbolSet := make(map[string]types.CoinIndicator)
	for _, r := range results {
		symbolSet[r.Symbol] = r
	}

	var filteredResults []types.CoinIndicator

	// 优先保留 BTCUSDT
	if ind, ok := symbolSet["BTCUSDT"]; ok {
		filteredResults = []types.CoinIndicator{ind}
	} else {
		// 否则保留其他主流币
		for _, sym := range prioritySymbols[1:] {
			if ind, ok := symbolSet[sym]; ok {
				filteredResults = append(filteredResults, ind)
			}
		}
		// 如果四大主流币都没有，才保留全部动能币
		if len(filteredResults) == 0 {
			filteredResults = results
		}
	}

	progressLogger.Printf("本轮符合条件标的数量: %d", len(filteredResults))

	sort.Slice(results, func(i, j int) bool {
		return results[i].StochRSI > results[j].StochRSI // “>” 表示降序
	})

	// ---------- 4. 推送到 Telegram ----------
	return utils.PushTelegram(results, botToken, chatID, volumeCache, db)
}

/* ====================== 单币分析 ====================== */

func analyseSymbol(client *futures.Client, symbol, tf string, db *sql.DB) (types.CoinIndicator, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()

	const maxRetries = 2

	var (
		klines []*futures.Kline
		err    error
	)

	// 最多尝试 3 次
	for attempt := 1; attempt <= maxRetries; attempt++ {
		klines, err = client.NewKlinesService().
			Symbol(symbol).Interval(tf).
			Limit(klinesCount).Do(ctx)

		// 拉取成功且数量够用，直接跳出循环
		if err == nil && len(klines) >= 35 {
			break
		}

		// 记录本次失败
		progressLogger.Printf("第 %d 次拉取 %s K 线失败: %v", attempt, symbol, err)

		// 如果还没到最后一次，可以选择短暂等待再试（可按需调整或使用指数退避）
		if attempt < maxRetries {
			time.Sleep(time.Second)
		}
	}

	// 若三次仍失败或数量不足，返回失败标记
	if err != nil || len(klines) < 35 {
		return types.CoinIndicator{}, false
	}

	closes := make([]float64, len(klines))
	for i, k := range klines {
		c, _ := strconv.ParseFloat(k.Close, 64)
		closes[i] = c
	}

	price := closes[len(closes)-1]
	ema25M15, ema50M15 := utils.Get15MEMAFromDB(db, symbol)
	ema25M5, ema50M5 := utils.Get5MEMAFromDB(db, symbol)
	_, kLine, _ := utils.StochRSIFromClose(closes, 14, 14, 3, 3)
	priceGT_EMA25 := utils.GetPriceGT_EMA25FromDB(db, symbol)

	var up, down bool
	if symbol == "BTCUSDT" {
		up = ema25M15 > ema50M15 && priceGT_EMA25
		down = ema25M15 < ema50M15 && !priceGT_EMA25
	} else {
		up = ema25M5 > ema50M5 && priceGT_EMA25 && ema25M5 > ema50M5
		down = ema25M5 < ema50M5 && !priceGT_EMA25 && ema25M5 < ema50M5
	}

	buyCond := kLine[len(kLine)-1] < 25 || kLine[len(kLine)-2] < 20
	sellCond := kLine[len(kLine)-1] > 75 || kLine[len(kLine)-2] > 80

	switch {
	case up && buyCond:
		progressLogger.Printf("BUY 触发: %s %.2f", symbol, price) // 👈
		return types.CoinIndicator{Symbol: symbol, Price: price, TimeInternal: tf, StochRSI: kLine[len(kLine)-1], Operation: "Buy"}, true
	case down && sellCond:
		progressLogger.Printf("SELL 触发: %s %.2f", symbol, price) // 👈
		return types.CoinIndicator{Symbol: symbol, Price: price, TimeInternal: tf, StochRSI: kLine[len(kLine)-1], Operation: "Sell"}, true
	default:
		return types.CoinIndicator{}, false
	}
}

func setHTTPClient(c *futures.Client) {
	proxy, _ := url.Parse(proxyURL)
	tr := &http.Transport{
		Proxy:           http.ProxyURL(proxy),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	c.HTTPClient = &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}
}
