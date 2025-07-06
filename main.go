package main

import (
	"context"
	"crypto/tls"
	"energe/telegram"
	"energe/utils"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"golang.org/x/sync/semaphore"
)

/* ====================== 结构体 & 全局 ====================== */

type CoinIndicator struct {
	Symbol       string
	Price        float64
	TimeInternal string
	StochRSI     float64 // 只存最后一个值够用了
	Operation    string
}

var (
	apiKey      = ""
	secretKey   = ""
	proxyURL    = "http://127.0.0.1:10809"
	klinesCount = 200
	maxWorkers  = 40
	limitVolume = 3e8 // 3 亿 USDT
	botToken    = "8040107823:AAHC_qu5cguJf9BG4NDiUB_nwpgF-bPkJAg"
	chatID      = "6074996357"

	volumeMap      = map[string]float64{}
	slipCoin       = []string{} // 想排除的币放这里
	muVolumeMap    sync.Mutex
	progressLogger = log.New(os.Stdout, "[Screener] ", log.LstdFlags)
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

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	// 立即跑一次
	if err := runScan(client, exchangeInfo); err != nil {
		progressLogger.Printf("首次 scan 出错: %v", err)
	}

	for range ticker.C {
		if err := runScan(client, exchangeInfo); err != nil {
			progressLogger.Printf("周期 scan 出错: %v", err)
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

	// ---------- 2. 刷新 24h 成交额 ----------
	utils.Get24HVolume(client, volumeMap)
	progressLogger.Println("已拉取 24h 成交额")

	// ---------- 3. 并发处理 ----------
	var (
		results []CoinIndicator
		resMu   sync.Mutex
		wg      sync.WaitGroup
		sem     = semaphore.NewWeighted(int64(maxWorkers))
	)

	for _, symbol := range symbols {
		if volumeMap[symbol] < limitVolume {
			continue
		}
		if inSlip(symbol) {
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

	// ---------- 4. 推送到 Telegram ----------
	return pushTelegram(results)
}

/* ====================== 单币分析 ====================== */

func analyseSymbol(client *futures.Client, symbol, tf string) (CoinIndicator, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	klines, err := client.NewKlinesService().
		Symbol(symbol).Interval(tf).
		Limit(klinesCount).Do(ctx)
	if err != nil || len(klines) < 35 {
		if err != nil {
			progressLogger.Printf("拉取 %s K 线失败: %v", symbol, err) // 👈
		}
		return CoinIndicator{}, false
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
	up := ema25[len(ema25)-1] > ema50[len(ema50)-1] && price > ema50[len(ema50)-1]
	down := ema25[len(ema25)-1] < ema50[len(ema50)-1] && price < ema50[len(ema50)-1]

	buyCond := kLine[len(kLine)-1] < 25 && kLine[len(kLine)-2] < 25
	sellCond := kLine[len(kLine)-1] > 75 && kLine[len(kLine)-2] > 75

	switch {
	case up && buyCond:
		progressLogger.Printf("BUY 触发: %s %.2f", symbol, price) // 👈
		return CoinIndicator{symbol, price, tf, kLine[len(kLine)-1], "Buy"}, true
	case down && sellCond:
		progressLogger.Printf("SELL 触发: %s %.2f", symbol, price) // 👈
		return CoinIndicator{symbol, price, tf, kLine[len(kLine)-1], "Sell"}, true
	default:
		return CoinIndicator{}, false
	}
}

/* ====================== Telegram 推送 ====================== */

func pushTelegram(results []CoinIndicator) error {
	now := time.Now().Format("2006-01-02 15:04")
	header := fmt.Sprintf("----15m 信号（%s）", now)

	if err := telegram.SendMessage(botToken, chatID, header); err != nil {
		return err
	}
	for _, r := range results {
		msg := fmt.Sprintf("%-4s %-10s SRSI:%3.1f",
			r.Operation, r.Symbol, r.StochRSI)
		if err := telegram.SendMessage(botToken, chatID, msg); err != nil {
			return err
		}
	}
	return nil
}

/* ====================== 工具函数 ====================== */

func inSlip(sym string) bool {
	for _, s := range slipCoin {
		if s == sym {
			return true
		}
	}
	return false
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
