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

// sendWaitListBroadcast 用于主动推送等待区列表
func sendWaitListBroadcast(now time.Time, waiting_token, chatID string) {
	waitMu.Lock()
	defer waitMu.Unlock()

	if len(waitList) == 0 {
		log.Println("📤 无需推送等待区列表：等待池为空")
		return
	}

	var msgBuilder strings.Builder
	msgBuilder.WriteString(fmt.Sprintf("等待区更新（%s）👇\n", now.Format("15:04")))

	var emoje string

	for _, token := range waitList {
		if token.Operation == "Buy" || token.Operation == "LongBuy" {
			emoje = "🟢"
		} else if token.Operation == "Sell" || token.Operation == "LongSell" {
			emoje = "🔴"
		} else {
			emoje = "-"
		}
		msgBuilder.WriteString(fmt.Sprintf("%s %-12s	加入: %s\n", emoje, token.Symbol, token.AddedAt.Format("15:04")))
	}
	msg := msgBuilder.String()
	log.Printf("📤 推送等待区更新列表，共 %d 个代币", len(waitList))
	telegram.SendMessage(waiting_token, chatID, msg)
}

func WaitEnerge(resultsChan chan []types.CoinIndicator, db *sql.DB, wait_sucess_token, chatID string, client *futures.Client, klinesCount int, waiting_token string) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case newResults := <-resultsChan:
			var newAdded bool
			now := time.Now()

			waitMu.Lock()
			for _, coin := range newResults {
				if coin.Status == "Wait" {
					if _, exists := waitList[coin.Symbol]; !exists {
						waitList[coin.Symbol] = waitToken{
							Symbol:    coin.Symbol,
							Operation: coin.Operation,
							AddedAt:   now,
						}
						log.Printf("✅ 添加等待代币: %s", coin.Symbol)
						newAdded = true
					}
				}
			}
			waitMu.Unlock()

			// 若有新代币加入等待区，则立即推送一次等待列表
			if newAdded {
				sendWaitListBroadcast(now, waiting_token, chatID)
			}

		case <-ticker.C:
			go func() {
				now := time.Now()
				var changed bool // 是否发生了删除

				waitMu.Lock()
				waitCopy := make(map[string]waitToken)
				for k, v := range waitList {
					waitCopy[k] = v
				}
				waitMu.Unlock()

				for sym, token := range waitCopy {
					_, closes, err := GetKlinesByAPI(client, sym, "1m", klinesCount)
					if err != nil {
						log.Printf("❌ 获取K线失败: %s", sym)
						continue
					}

					price1 := closes[len(closes)-1]
					priceGT := GetPriceGT_EMA25FromDB(db, sym)
					ema25H1, ema50H1 := Get1HEMAFromDB(db, sym)
					ema25M15, ema50M15, _ := Get15MEMAFromDB(db, sym)
					ema25M5, ema50M5 := Get5MEMAFromDB(db, sym)
					EMA25M1 := CalculateEMA(closes, 25)
					EMA50M1 := CalculateEMA(closes, 50)

					switch token.Operation {
					case "Buy":
						condition3 := EMA25M1[len(EMA25M1)-1] > EMA50M1[len(EMA50M1)-1]
						if priceGT && price1 > ema25M15 && ema25M5 > ema50M5 && condition3 {
							//1小时GT，15分钟站上，5分钟金叉，1分钟金叉
							msg := fmt.Sprintf("🟢%s \n价格：%.4f  时间：%s", sym, price1, now.Format("15:04"))
							telegram.SendMessage(wait_sucess_token, chatID, msg)
							log.Printf("🟢 等待成功 Buy : %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							changed = true
						} else if ema25M15 < ema50M15 {
							log.Printf("❌ Wait失败 Buy : %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							changed = true
						}

					case "Sell":
						condition3 := EMA25M1[len(EMA25M1)-1] < EMA50M1[len(EMA50M1)-1]
						if !priceGT && ema25M5 < ema50M5 && price1 < ema25M15 && condition3 {
							//1小时非GT，15分钟站下，5分钟死叉，1分钟死叉
							msg := fmt.Sprintf("🔴%s \n价格：%.4f  时间：%s", sym, price1, now.Format("15:04"))
							telegram.SendMessage(wait_sucess_token, chatID, msg)
							log.Printf("🔴 等待成功 Sell : %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							changed = true
						} else if ema25M15 > ema50M15 {
							log.Printf("❌ Wait失败 Sell : %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							changed = true
						}
					case "LongBuy":
						condition3 := EMA25M1[len(EMA25M1)-1] > EMA50M1[len(EMA50M1)-1]
						if priceGT && price1 > ema25M15 && ema25M5 > ema50M5 && condition3 {
							//1小时GT，15分钟站上，5分钟金叉，1分钟金叉
							msg := fmt.Sprintf("🟢%s \n价格：%.4f  时间：%s", sym, price1, now.Format("15:04"))
							telegram.SendMessage(wait_sucess_token, chatID, msg)
							log.Printf("🟢 等待成功 Buy : %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							changed = true
						} else if ema25H1 < ema50H1 {
							log.Printf("❌ Wait失败 Buy : %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							changed = true
						}
					case "LongSell":
						condition3 := EMA25M1[len(EMA25M1)-1] < EMA50M1[len(EMA50M1)-1]
						if !priceGT && ema25M5 < ema50M5 && price1 < ema25M15 && condition3 {
							//1小时非GT，15分钟站下，5分钟死叉，1分钟死叉
							msg := fmt.Sprintf("🔴%s \n价格：%.4f  时间：%s", sym, price1, now.Format("15:04"))
							telegram.SendMessage(wait_sucess_token, chatID, msg)
							log.Printf("🔴 等待成功 Sell : %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							changed = true
						} else if ema25H1 > ema50H1 {
							log.Printf("❌ Wait失败 Sell : %s", sym)
							waitMu.Lock()
							delete(waitList, sym)
							waitMu.Unlock()
							changed = true
						}
					}

					// 超时（8小时）
					if now.Sub(token.AddedAt) > 8*time.Hour {
						log.Printf("⏰ Wait超时清理 : %s", sym)
						waitMu.Lock()
						delete(waitList, sym)
						waitMu.Unlock()
						changed = true
					}
				}

				// 有代币被移除时，再次推送等待列表
				if changed {
					sendWaitListBroadcast(now, waiting_token, chatID)
				}
			}()
		}
	}
}
