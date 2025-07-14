package utils

import (
	"database/sql"
	"energe/telegram"
	"energe/types"
	"fmt"
	"log"
	"strings"
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

func WaitEnerge(resultsChan chan []types.CoinIndicator, db *sql.DB, wait_sucess_token, chatID string, client *futures.Client, klinesCount int, waiting_token string) {
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

			// === 添加: 每15分钟推送等待区列表 ===
			if now.Minute()%15 == 0 {
				waitMu.Lock()
				defer waitMu.Unlock()

				var msgBuilder strings.Builder
				msgBuilder.WriteString(fmt.Sprintf("等待区播报（%s）👇👇\n", now.Format("15:04")))

				if len(waitList) > 0 {
					for _, token := range waitList {
						msgBuilder.WriteString(fmt.Sprintf("- %s (%s)   加入时间: %s\n", token.Symbol, token.Operation, token.AddedAt.Format("15:04")))
					}
					log.Printf("📤 推送等待区列表，共 %d 个代币", len(waitList))
				} else {
					msgBuilder.WriteString("📭 当前等待区为空\n")
					log.Println("📤 推送等待区列表：无代币")
				}

				finalMsg := msgBuilder.String()
				telegram.SendMessage(waiting_token, chatID, finalMsg)
			}

			for sym, token := range waitList {
				_, closes, err := GetKlinesByAPI(client, sym, "1m", klinesCount)
				if err != nil {
					log.Printf("❌ 获取K线失败: %s", sym)
					continue
				}
				price1 := closes[len(closes)-1]
				priceGT := GetPriceGT_EMA25FromDB(db, sym)
				ema25M15, ema50M15 := Get15MEMAFromDB(db, sym)
				ema25M5, ema50M5 := Get5MEMAFromDB(db, sym)
				EMA25M1 := CalculateEMA(closes, 25)
				EMA50M1 := CalculateEMA(closes, 50)

				if token.Operation == "Buy" {
					condition3 := EMA25M1[len(EMA25M1)-1] > EMA50M1[len(EMA50M1)-1]

					if priceGT && ema25M15 > ema50M15 && ema25M5 > ema50M5 && price1 > ema25M15 && condition3 {
						msg := fmt.Sprintf("🟢%s \n价格：%.4f  时间：%s", sym, price1, now.Format("15:04"))
						telegram.SendMessage(wait_sucess_token, chatID, msg)
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
					if !priceGT && ema25M15 < ema50M15 && ema25M5 < ema50M5 && price1 < ema25M15 && condition3 {
						msg := fmt.Sprintf("🔴%s \n价格：%.4f  时间：%s", sym, price1, now.Format("15:04"))
						telegram.SendMessage(wait_sucess_token, chatID, msg)
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
