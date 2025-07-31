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

func PushTelegram(results []types.CoinIndicator, botToken, high_profit_srsi_botToken, chatID string, volumeCache *types.VolumeCache, db *sql.DB, betrend types.BETrend) error {
	now := time.Now().Format("2006-01-02 15:04")
	var msgBuilder strings.Builder

	var filteredResults []types.CoinIndicator
	for _, r := range results {
		if r.Status != "Wait" && r.Status != "LongWait" {
			filteredResults = append(filteredResults, r)
		}
	}

	// 判断是否为空
	if len(filteredResults) == 0 {
		msgBuilder.WriteString(fmt.Sprintf("（无）Time：%s\n", now))
	} else {
		msgBuilder.WriteString(fmt.Sprintf("🎁Time：%s\n", now))
	}

	// 操作与前缀符号的映射
	operationIcons := map[string]string{
		"Buy":      "🟢",
		"LongBuy":  "🟢",
		"BuyBE":    "🟢",
		"Sell":     "🔴",
		"LongSell": "🔴",
		"SellBE":   "🔴",
		"ViewBE":   "🟣",
	}

	for _, r := range filteredResults {
		icon, ok := operationIcons[r.Operation]
		fmt.Println(ok)
		if !ok {
			continue // 非指定操作类型跳过
		}

		line := fmt.Sprintf("%s%-4s %-10s (%4s)", icon, r.Operation, r.Symbol, r.Status)
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
