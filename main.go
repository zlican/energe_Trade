package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"energe/model"
	"energe/types"
	"energe/utils"
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
	limitVolume = 50000000 // 3 亿 USDT
	botToken    = "8040107823:AAHC_qu5cguJf9BG4NDiUB_nwpgF-bPkJAg"
	chatID      = "6074996357"

	// volumeMap      = map[string]float64{}
	volumeCache    *types.VolumeCache
	slipCoin       = []string{"XRPUSDT", "DOGEUSDT", "1000PEPEUSDT", "ADAUSDT", "BNBUSDT"} // 想排除的币放这里
	muVolumeMap    sync.Mutex
	progressLogger = log.New(os.Stdout, "[Screener] ", log.LstdFlags)
	db             *sql.DB
)

/* ====================== 主函数 ====================== */

func main() {
	progressLogger.Println("程序启动...")

	client := binance.NewFuturesClient(apiKey, secretKey)
	setHTTPClient(client)

	exchangeInfo, err := client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		log.Fatalf("获取交易所信息失败: %v", err)
	}

	// 创建并预热 VolumeCache
	volumeCache, err = utils.NewVolumeCache(client)
	if err != nil {
		log.Fatalf("VolumeCache 启动失败: %v", err)
	}

	<-volumeCache.Ready()
	log.Println("volumeCache 启动成功")
	defer volumeCache.Close()

	model.InitDB()
	db = model.DB

	// 立即执行一次
	utils.Update1hEMA50ToDB(client, db, float64(limitVolume), klinesCount, volumeCache, slipCoin)
	if err := runScan(client, exchangeInfo); err != nil {
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
			progressLogger.Printf("整点 %02d:00，执行 Update1hEMA50ToDB", hour)
			go utils.Update1hEMA50ToDB(client, db, float64(limitVolume), klinesCount, volumeCache, slipCoin)
		}
		if minute%15 == 0 {
			progressLogger.Printf("每15分钟触发，执行 runScan (%02d:%02d)", hour, minute)
			if err := runScan(client, exchangeInfo); err != nil {
				progressLogger.Printf("周期 scan 出错: %v", err)
			}
			time.Sleep(1 * time.Minute)
		}
	}
}

/* ====================== 核心扫描 ====================== */

func runScan(client *futures.Client, exchangeInfo *futures.ExchangeInfo) error {
	progressLogger.Println("开始新一轮扫描...")

	// ---------- 1. 过滤 USDT 交易对 ----------
	var symbols []string
	for _, s := range exchangeInfo.Symbols {
		if s.QuoteAsset == "USDT" && s.Status == "TRADING" {
			symbols = append(symbols, s.Symbol)
		}
	}
	progressLogger.Printf("USDT 交易对数量: %d", len(symbols))

	// ---------- 3. 并发处理 ----------
	var (
		results []types.CoinIndicator
		resMu   sync.Mutex
		wg      sync.WaitGroup
		sem     = semaphore.NewWeighted(int64(maxWorkers))
	)

	for _, symbol := range symbols {
		if vol, ok := volumeCache.Get(symbol); !ok || vol < float64(limitVolume) {
			continue
		}
		if utils.IsSlipCoin(symbol, slipCoin) {
			continue
		}

		if err := sem.Acquire(context.Background(), 1); err != nil {
			progressLogger.Printf("semaphore acquire 失败: %v", err)
			continue
		}

		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			defer sem.Release(1)

			ind, ok := analyseSymbol(client, sym, "15m")
			if ok {
				resMu.Lock()
				results = append(results, ind)
				resMu.Unlock()
			}
		}(symbol)
	}
	wg.Wait()
	progressLogger.Printf("本轮符合条件标的数量: %d", len(results))

	sort.Slice(results, func(i, j int) bool {
		return results[i].StochRSI > results[j].StochRSI // “>” 表示降序
	})

	// ---------- 4. 推送到 Telegram ----------
	return utils.PushTelegram(results, botToken, chatID, volumeCache)
}

/* ====================== 单币分析 ====================== */

func analyseSymbol(client *futures.Client, symbol, tf string) (types.CoinIndicator, bool) {

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

	ema25 := utils.CalculateEMA(closes, 25)
	ema50 := utils.CalculateEMA(closes, 50)
	_, kLine, _ := utils.StochRSIFromClose(closes, 14, 14, 3, 3)

	// --- 如果是 BTCUSDT，打印最近 20 个 StochRSI ---
	if symbol == "BTCUSDT" && len(kLine) >= 20 {
		last20 := kLine[len(kLine)-20:]
		progressLogger.Printf("BTCUSDT 最近20个 StochRSI: %v", last20) // 👈
	}

	price := closes[len(closes)-1]
	priceGT_EMA25 := utils.GetPriceGT_EMA25FromDB(db, symbol)
	up := ema25[len(ema25)-1] > ema50[len(ema50)-1] && priceGT_EMA25
	down := ema25[len(ema25)-1] < ema50[len(ema50)-1] && !priceGT_EMA25

	buyCond := kLine[len(kLine)-1] < 25 || kLine[len(kLine)-2] < 20
	sellCond := kLine[len(kLine)-1] > 85 || kLine[len(kLine)-2] > 90

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
