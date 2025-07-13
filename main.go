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
	"sync"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"golang.org/x/sync/semaphore"
)

/* ====================== 结构体 & 全局 ====================== */

var (
	apiKey               = ""
	secretKey            = ""
	proxyURL             = "http://127.0.0.1:10809"
	klinesCount          = 200
	maxWorkers           = 20
	limitVolume          = 100000000 // 1亿 USDT
	botToken             = "8040107823:AAHC_qu5cguJf9BG4NDiUB_nwpgF-bPkJAg"
	wait_energe_botToken = "7381664741:AAEmhhEhsq8nBgThtsOfVklNb6q4TjvI_Og"
	chatID               = "6074996357"

	// volumeMap      = map[string]float64{}
	volumeCache *types.VolumeCache
	err         error
	slipCoin    = []string{"XRPUSDT", "DOGEUSDT", "1000PEPEUSDT", "ADAUSDT", "BNBUSDT", "UNIUSDT", "TRUMPUSDT",
		"LINKUSDT", "FARTCOINUSDT", "1000BONKUSDT", "AAVEUSDT", "AVAXUSDT", "SUIUSDT", "LTCUSDT",
		"SEIUSDT", "BCHUSDT", "WIFUSDT", "XLMUSDT", "XRPUSDC", "BNXUSDT", "ETHUSDC", "BTCUSDC", "SOLUSDC",
		"DOTUSDT", "NEARUSDT", "ARBUSDT", "1000SHIBUSDT", "TIAUSDT", "TRXUSDT", "HYPEUSDT", "PNUTUSDT",
		"HBARUSDT", "VIRTUALUSDT", "PUMPUSDT", "1INCHUSDT", "SUIUSDC", "1000FLOKIUSDT", "GALAUSDT",
		"WLDUSDT", "FILUSDT", "APTUSDT", "TAOUSDT", "CRVUSDT", "FETUSDT", "INJUSDT", "1000BONKUSDC",
		"SPXUSDT", "TONUSDT", "ETCUSDT"} // 想排除的币放这里
	muVolumeMap    sync.Mutex
	progressLogger = log.New(os.Stdout, "[Screener] ", log.LstdFlags)
	db             *sql.DB
	waitChan       = make(chan []types.CoinIndicator, 30) //等待区
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

	//开启等待区
	go utils.WaitEnerge(waitChan, db, wait_energe_botToken, chatID, client, klinesCount)

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

		}

		if minute%5 == 0 {
			time.Sleep(10 * time.Second)
			progressLogger.Printf("每5分钟触发，执行 runScan (%02d:%02d)", hour, minute)
			go utils.Update5MEMAToDB(client, db, float64(limitVolume), klinesCount, volumeCache, slipCoin)

			//这里进行监控扫描，二级市场
			if err := runScan(client); err != nil {
				progressLogger.Printf("周期 scan 出错: %v", err)
			}
			time.Sleep(1 * time.Minute)

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

	//取消层级压制
	/* 	symbolSet := make(map[string]types.CoinIndicator)
	   	for _, r := range results {
	   		symbolSet[r.Symbol] = r
	   	}

	   	var filteredResults []types.CoinIndicator

	   	// 优先保留 BTCUSDT
	   	if ind, ok := symbolSet["BTCUSDT"]; ok {
	   		filteredResults = []types.CoinIndicator{ind}
	   	} else {
	   		filteredResults = results
	   	}

		results = filteredResults
	*/
	waitChan <- results //将一次runScan的数据推送给等待区

	progressLogger.Printf("本轮符合条件标的数量: %d", len(results))

	sort.Slice(results, func(i, j int) bool {
		return results[i].StochRSI > results[j].StochRSI // “>” 表示降序
	})

	// ---------- 4. 推送到 Telegram ----------
	return utils.PushTelegram(results, botToken, chatID, volumeCache, db)
}

/* ====================== 单币分析 ====================== */

func analyseSymbol(client *futures.Client, symbol, tf string, db *sql.DB) (types.CoinIndicator, bool) {

	_, closes, err := utils.GetKlinesByAPI(client, symbol, tf, klinesCount)
	if err != nil {
		return types.CoinIndicator{}, false
	}

	price := closes[len(closes)-1]
	ema25M15, ema50M15 := utils.Get15MEMAFromDB(db, symbol)
	ema25M5, ema50M5 := utils.Get5MEMAFromDB(db, symbol)
	//_, kLine, _ := utils.StochRSIFromClose(closes, 14, 14, 3, 3)
	priceGT_EMA25 := utils.GetPriceGT_EMA25FromDB(db, symbol) //1H 价格在25EMA上方

	var up, down bool
	if symbol == "BTCUSDT" || symbol == "ETHUSDT" || symbol == "SOLUSDT" {
		up = priceGT_EMA25 && ema25M15 > ema50M15    //1H GT +15分钟在上
		down = !priceGT_EMA25 && ema25M15 < ema50M15 //1H !GT + 15分钟在下
	} else {
		up = ema25M15 > ema50M15 && ema25M5 > ema50M5   //15分钟在上+5分钟在上
		down = ema25M15 < ema50M15 && ema25M5 < ema50M5 //15分钟在下+5分钟在下
	}

	var srsi float64
	if symbol == "BTCUSDT" || symbol == "ETHUSDT" || symbol == "SOLUSDT" {
		srsi = utils.Get15SRSIFromDB(db, symbol)
	} else {
		srsi = utils.Get5SRSIFromDB(db, symbol)
	}

	buyCond := srsi < 25 || srsi < 20
	sellCond := srsi > 75 || srsi > 80

	var status string
	var SmallEMA25, SmallEMA50 float64
	switch {
	case up && buyCond:
		progressLogger.Printf("BUY 触发: %s %.2f", symbol, price) // 👈
		if symbol == "BTCUSDT" || symbol == "ETHUSDT" || symbol == "SOLUSDT" {
			SmallEMA25, SmallEMA50 = utils.Get5MEMAFromDB(db, symbol) //三大对5分钟进行判断
			if SmallEMA25 > SmallEMA50 && price > ema25M15 {          //(这里价格破中时黄)
				status = "Soon"
			} else {
				status = "Wait"
			}
		} else {
			SmallEMA25, SmallEMA50 = utils.Get1MEMA(client, klinesCount, symbol) //动能币对1分钟进行判断
			if SmallEMA25 > SmallEMA50 && price > ema25M5 {                      //(这里价格破中时黄)
				status = "Soon"
			} else {
				status = "Wait"
			}
		}
		return types.CoinIndicator{
			Symbol:       symbol,
			Price:        price,
			TimeInternal: tf,
			StochRSI:     srsi,
			Status:       status,
			Operation:    "Buy"}, true
	case down && sellCond:
		progressLogger.Printf("SELL 触发: %s %.2f", symbol, price) // 👈
		if symbol == "BTCUSDT" || symbol == "ETHUSDT" || symbol == "SOLUSDT" {
			SmallEMA25, SmallEMA50 = utils.Get5MEMAFromDB(db, symbol) //三大对5分钟进行判断
			if SmallEMA25 < SmallEMA50 && price < ema25M15 {          //(这里价格破中时黄)
				status = "Soon"
			} else {
				status = "Wait"
			}
		} else {
			SmallEMA25, SmallEMA50 = utils.Get1MEMA(client, klinesCount, symbol) //动能币对1分钟进行判断
			if SmallEMA25 < SmallEMA50 && price < ema25M5 {                      //(这里价格破中时黄)
				status = "Soon"
			} else {
				status = "Wait"
			}
		}
		return types.CoinIndicator{
			Symbol:       symbol,
			Price:        price,
			TimeInternal: tf,
			StochRSI:     srsi,
			Status:       status,
			Operation:    "Sell"}, true
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
