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
				price1 := closes[len(closes)-1]
				priceGT := GetPriceGT_EMA25FromDB(db, sym)
				ema25M15, ema50M15 := Get15MEMAFromDB(db, sym)
				//ema25M5, ema50M5 := Get5MEMAFromDB(db, sym)
				EMA25M1 := CalculateEMA(closes, 25)
				EMA50M1 := CalculateEMA(closes, 50)

				if token.Operation == "Buy" {
					condition3 := EMA25M1[len(EMA25M1)-1] > EMA50M1[len(EMA50M1)-1]

					if priceGT && ema25M15 > ema50M15 && price1 > ema25M15 && condition3 {
						msg := fmt.Sprintf("🟢%s \n价格：%.4f  时间：%s", sym, price1, now.Format("15:04"))
						telegram.SendMessage(botToken, chatID, msg)
						log.Printf("🟢 等待成功 Buy : %s", sym)
						waitMu.Lock()
						delete(waitList, sym)
						waitMu.Unlock()
						continue
					} else if ema25M15 < ema50M15 {
						log.Printf("❌ Wait失败 Buy : %s", sym)
						waitMu.Lock()
						delete(waitList, sym)
						waitMu.Unlock()
						continue
					}
				} else if token.Operation == "Sell" {
					condition3 := EMA25M1[len(EMA25M1)-1] < EMA50M1[len(EMA50M1)-1]
					if !priceGT && ema25M15 < ema50M15 && price1 < ema25M15 && condition3 {
						msg := fmt.Sprintf("🔴%s \n价格：%.4f  时间：%s", sym, price1, now.Format("15:04"))
						telegram.SendMessage(botToken, chatID, msg)
						log.Printf("🔴 等待成功 Sell : %s", sym)
						waitMu.Lock()
						delete(waitList, sym)
						waitMu.Unlock()
						continue
					} else if ema25M15 > ema50M15 {
						log.Printf("❌ Wait失败 Sell : %s", sym)
						waitMu.Lock()
						delete(waitList, sym)
						waitMu.Unlock()
						continue
					}
				}
				// 超时（8小时）
				if now.Sub(token.AddedAt) > 8*time.Hour {
					log.Printf("⏰ Wait超时清理 : %s", sym)
					waitMu.Lock()
					delete(waitList, sym)
					waitMu.Unlock()
				}
			}
		}
	}
}
