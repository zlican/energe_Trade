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
		if token.Operation == "Buy" || token.Operation == "LongBuy" || token.Operation == "BuyBE" {
			emoje = "🟢"
		} else if token.Operation == "Sell" || token.Operation == "LongSell" {
			emoje = "🔴"
		} else {
			emoje = "-"
		}
		// 如果是 BTC 或 ETH，加上 💎
		if token.Symbol == "BTCUSDT" || token.Symbol == "ETHUSDT" {
			emoje = "💎" + emoje
		}
		msgBuilder.WriteString(fmt.Sprintf("%s %-12s	加入: %s\n", emoje, token.Symbol, token.AddedAt.Format("15:04")))
	}
	msg := msgBuilder.String()
	log.Printf("📤 推送等待区更新列表，共 %d 个代币", len(waitList))
	telegram.SendMessage(waiting_token, chatID, msg)
}

func waitUntilNext5Min() time.Duration {
	now := time.Now()
	next := now.Truncate(time.Minute).Add(time.Duration(5-now.Minute()%5) * time.Minute)
	if next.Before(now) || next.Equal(now) {
		next = next.Add(5 * time.Minute)
	}
	return time.Until(next)
}

func WaitEnerge(resultsChan chan []types.CoinIndicator, db *sql.DB, wait_sucess_token, chatID string, client *futures.Client, klinesCount int, waiting_token string) {
	go func() {
		// 首次对齐等待，直到下一个 5 分钟整点
		time.Sleep(waitUntilNext5Min())
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for now := range ticker.C {
			go func(now time.Time) {
				var changed bool // 是否发生了删除

				waitMu.Lock()
				waitCopy := make(map[string]waitToken)
				for k, v := range waitList {
					waitCopy[k] = v
				}
				waitMu.Unlock()

				for sym, token := range waitCopy {
					_, opens, closes, err := GetKlinesByAPI(client, sym, "15m", klinesCount)
					if err != nil {
						log.Printf("❌ 获取K线失败: %s", sym)
						continue
					}

					price1 := closes[len(closes)-1]
					priceGT := GetPriceGT_EMA25FromDB(db, sym)
					ema25H1, ema50H1 := Get1HEMAFromDB(db, sym)
					ema25M15, ema50M15, _ := Get15MEMAFromDB(db, sym)
					ema25M5, ema50M5 := Get5MEMAFromDB(db, sym)

					UpMACD := IsAboutToGoldenCross(closes, 6, 13, 5)
					DownMACD := IsAboutToDeadCross(closes, 6, 13, 5)

					//1分钟金死叉（金死叉传递理论）
					_, _, closesM1, err := GetKlinesByAPI(client, sym, "1m", klinesCount)
					if err != nil || len(opens) < 2 || len(closes) < 2 {
						continue
					}
					EMA25M1 := CalculateEMA(closesM1, 25)
					EMA50M1 := CalculateEMA(closesM1, 50)

					switch token.Operation {
					case "Buy":
						if priceGT && ema25M15 > ema50M15 && ema25M5 > ema50M5 && UpMACD && EMA25M1[len(EMA25M1)-1] > EMA50M1[len(EMA50M1)-1] {
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
						if !priceGT && ema25M15 < ema50M15 && ema25M5 < ema50M5 && DownMACD && EMA25M1[len(EMA25M1)-1] < EMA50M1[len(EMA50M1)-1] {
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
						if ema25M5 > ema50M5 && UpMACD && EMA25M1[len(EMA25M1)-1] > EMA50M1[len(EMA50M1)-1] {
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
						if ema25M5 < ema50M5 && DownMACD && EMA25M1[len(EMA25M1)-1] < EMA50M1[len(EMA50M1)-1] {
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
					case "BuyBE":
						if ema25M5 > ema50M5 && UpMACD && EMA25M1[len(EMA25M1)-1] > EMA50M1[len(EMA50M1)-1] {
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
					}

					if now.Sub(token.AddedAt) > 8*time.Hour {
						log.Printf("⏰ Wait超时清理 : %s", sym)
						waitMu.Lock()
						delete(waitList, sym)
						waitMu.Unlock()
						changed = true
					}
				}

				if changed {
					sendWaitListBroadcast(now, waiting_token, chatID)
				}
			}(now)
		}
	}()

	// 接收新 results 并更新 waitList（逻辑不变）
	for newResults := range resultsChan {
		var newAdded bool
		now := time.Now()

		waitMu.Lock()
		for _, coin := range newResults {
			if coin.Status == "Wait" {
				existing, exists := waitList[coin.Symbol]
				if !exists || existing.Operation != coin.Operation {
					waitList[coin.Symbol] = waitToken{
						Symbol:    coin.Symbol,
						Operation: coin.Operation,
						AddedAt:   now,
					}
					log.Printf("✅ 添加或替换等待代币: %s", coin.Symbol)
					newAdded = true
				}
			}
		}
		waitMu.Unlock()

		if newAdded {
			sendWaitListBroadcast(now, waiting_token, chatID)
		}
	}
}
