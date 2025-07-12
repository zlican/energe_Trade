package utils

import (
	"database/sql"
	"energe/telegram"
	"energe/types"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

type waitToken struct {
	Symbol    string
	Operation string
	AddedAt   time.Time
}

var waitMu sync.Mutex
var waitList = make(map[string]waitToken)

func WaitEnerge(resultsChan chan []types.CoinIndicator, db *sql.DB, botToken, chatID string, client *futures.Client, klinesCount int) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	isCoreCoin := func(sym string) bool {
		return sym == "BTCUSDT" || sym == "ETHUSDT" || sym == "SOLUSDT" || sym == "HYPEUSDT"
	}

	for {
		select {
		case newResults := <-resultsChan:
			for _, coin := range newResults {
				if coin.Status == "Wait" {
					waitMu.Lock()
					if _, exists := waitList[coin.Symbol]; !exists {
						waitList[coin.Symbol] = waitToken{
							Symbol:    coin.Symbol,
							Operation: coin.Operation,
							AddedAt:   time.Now(),
						}
						log.Printf("✅ 添加等待代币: %s", coin.Symbol)
					}
					waitMu.Unlock()
				}
			}

		case <-ticker.C:
			now := time.Now()
			for sym, token := range waitList {
				_, closes, err := GetKlinesByAPI(client, sym, "1m", klinesCount)
				if err != nil {
					log.Printf("❌ 获取K线失败: %s", sym)
					continue
				}
				price := closes[len(closes)-1]
				priceGT := GetPriceGT_EMA25FromDB(db, sym)
				ema25M15, ema50M15 := Get15MEMAFromDB(db, sym)
				ema25M5, ema50M5 := Get5MEMAFromDB(db, sym)

				if isCoreCoin(sym) {
					// 四大币判断逻辑
					if token.Operation == "Buy" {
						if priceGT && ema25M15 > ema50M15 && price > ema25M5 {
							msg := fmt.Sprintf("✅ [等待成功-BUY] %s 出现右侧信号\n价格：%.4f\n时间：%s", sym, price, now.Format("15:04"))
							telegram.SendMessage(botToken, chatID, msg)
							log.Printf("✅ Wait→Soon Buy (四大): %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							continue
						} else if ema25M15 < ema50M15 {
							log.Printf("❌ Wait失败 Buy (四大): %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							continue
						}
					} else if token.Operation == "Sell" {
						if !priceGT && ema25M15 < ema50M15 && price < ema25M5 {
							msg := fmt.Sprintf("❌ [等待成功-SELL] %s 出现右侧信号（做空）\n价格：%.4f\n时间：%s", sym, price, now.Format("15:04"))
							telegram.SendMessage(botToken, chatID, msg)
							log.Printf("❌ Wait→Soon Sell (四大): %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							continue
						} else if ema25M15 > ema50M15 {
							log.Printf("❌ Wait失败 Sell (四大): %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							continue
						}
					}
					// 超时（8小时）
					if now.Sub(token.AddedAt) > 8*time.Hour {
						log.Printf("⏰ Wait超时清理 (四大): %s", sym)
						waitMu.Lock()
						delete(waitList, sym)
						waitMu.Unlock()
					}
				} else {
					// 动能币判断逻辑
					_, closes, err := GetKlinesByAPI(client, sym, "1m", klinesCount)
					if err != nil {
						log.Printf("❌ 获取K线失败: %s", sym)
						continue
					}
					price := closes[len(closes)-1]
					ema25M1List := CalculateEMA(closes, 25)
					ema25M1 := ema25M1List[len(ema25M1List)-1]

					if token.Operation == "Buy" {
						if ema25M15 > ema50M15 && ema25M5 > ema50M5 && price > ema25M1 {
							msg := fmt.Sprintf("✅ [等待成功-BUY] %s 出现右侧信号\n价格：%.4f\n时间：%s", sym, price, now.Format("15:04"))
							telegram.SendMessage(botToken, chatID, msg)
							log.Printf("✅ Wait→Soon Buy (动能): %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							continue
						} else if ema25M5 < ema50M5 {
							log.Printf("❌ Wait失败 Buy (动能): %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							continue
						}
					} else if token.Operation == "Sell" {
						if ema25M15 < ema50M15 && ema25M5 > ema50M5 && price < ema25M1 {
							msg := fmt.Sprintf("❌ [等待成功-SELL] %s 出现右侧信号（做空）\n价格：%.4f\n时间：%s", sym, price, now.Format("15:04"))
							telegram.SendMessage(botToken, chatID, msg)
							log.Printf("❌ Wait→Soon Sell (动能): %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							continue
						} else if ema25M5 > ema50M5 {
							log.Printf("❌ Wait失败 Sell (动能): %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							continue
						}
					}

					// 超时（4小时）
					if now.Sub(token.AddedAt) > 4*time.Hour {
						log.Printf("⏰ Wait超时清理 (动能): %s", sym)
						waitMu.Lock()
						delete(waitList, sym)
						waitMu.Unlock()
					}
				}
			}
		}
	}
}
