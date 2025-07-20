package utils

import (
	"database/sql"
	"energe/telegram"
	"fmt"
	"log"
	"strings"
	"time"

	"energe/types"
)

func PushTelegram(results []types.CoinIndicator, botToken, high_profit_srsi_botToken, chatID string, volumeCache *types.VolumeCache, db *sql.DB, bestrend types.BESTrend) error {
	now := time.Now().Format("2006-01-02 15:04")
	var msgBuilder strings.Builder

	// ---------- 添加主趋势播报 ----------
	var upCoins []string
	var downCoins []string

	if bestrend.BTC == "up" {
		upCoins = append(upCoins, "BTC")
	} else if bestrend.BTC == "down" {
		downCoins = append(downCoins, "BTC")
	}
	if bestrend.ETH == "up" {
		upCoins = append(upCoins, "ETH")
	} else if bestrend.ETH == "down" {
		downCoins = append(downCoins, "ETH")
	}
	if bestrend.SOL == "up" {
		upCoins = append(upCoins, "SOL")
	} else if bestrend.SOL == "down" {
		downCoins = append(downCoins, "SOL")
	}

	var trendLine string
	switch {
	case len(upCoins) > 0:
		trendLine = fmt.Sprintf("🟢 BES趋势：上涨（%s）", strings.Join(upCoins, ", "))
	case len(downCoins) > 0:
		trendLine = fmt.Sprintf("🔴 BES趋势：下跌（%s）", strings.Join(downCoins, ", "))
	default:
		trendLine = "⚪️ BES趋势：随机漫步"
	}

	msgBuilder.WriteString(fmt.Sprintf("%s（%s）\n", trendLine, now))
	msgBuilder.WriteString("\n")

	for _, r := range results {
		operation := r.Operation
		if r.Status == "Wait" {
			continue
		}
		volume, ok := volumeCache.Get(r.Symbol)
		if !ok || volume < 150000000 {
			continue
		}

		var line string
		if operation == "Buy" {
			if r.Symbol == "BTCUSDT" || r.Symbol == "ETHUSDT" {
				line = fmt.Sprintf("💎%-4s %-10s (%4s)", r.Operation, r.Symbol, r.Status)
			} else {
				line = fmt.Sprintf("🟢%-4s %-10s (%4s)", r.Operation, r.Symbol, r.Status)
			}
		} else if operation == "Sell" {
			if r.Symbol == "BTCUSDT" || r.Symbol == "ETHUSDT" {
				line = fmt.Sprintf("💎%-4s %-10s (%4s)", r.Operation, r.Symbol, r.Status)
			} else {
				line = fmt.Sprintf("🔴%-4s %-10s (%4s)", r.Operation, r.Symbol, r.Status)
			}
		} else if operation == "LongBuy" {
			line = fmt.Sprintf("🟢%-4s %-10s (%4s)", r.Operation, r.Symbol, r.Status)
		} else if operation == "LongSell" {
			line = fmt.Sprintf("🔴%-4s %-10s (%4s)", r.Operation, r.Symbol, r.Status)
		} else {
			continue // 忽略非 Buy/Sell 操作
		}

		msgBuilder.WriteString(line + "\n")
	}

	msg := msgBuilder.String()
	if strings.TrimSpace(msg) == "" {
		log.Println("📭 无需推送 Telegram 消息")
		return nil
	}

	if err := sendWithRetry(botToken, chatID, msg); err != nil {
		log.Printf("发送合并消息失败: %v", err)
	}
	return nil
}

// sendWithRetry 封装了带一次重试的 Telegram 发送逻辑
func sendWithRetry(botToken, chatID, msg string) error {
	err := telegram.SendMessage(botToken, chatID, msg)
	if err != nil {
		time.Sleep(2 * time.Second)
		err = telegram.SendMessage(botToken, chatID, msg)
	}
	return err
}
