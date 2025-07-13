package utils

import (
	"database/sql"
	"energe/telegram"
	"fmt"
	"log"
	"time"

	"energe/types"
)

func PushTelegram(results []types.CoinIndicator, botToken, chatID string, volumeCache *types.VolumeCache, db *sql.DB) error {
	now := time.Now().Format("2006-01-02 15:04")
	header := fmt.Sprintf("15m 播报（%s）👇👇", now)

	if err := sendWithRetry(botToken, chatID, header); err != nil {
		log.Printf("发送 header 消息失败: %v", err)
	}

	for _, r := range results {
		operation := r.Operation
		if r.Status == "Wait" {
			continue
		}
		volume, ok := volumeCache.Get(r.Symbol)
		if !ok || volume < 300000000 {
			continue
		}
		var msg string

		if operation == "Buy" {
			if r.Symbol == "BTCUSDT" || r.Symbol == "ETHUSDT" {
				msg = fmt.Sprintf("💎%-4s %-10s (%4s)", r.Operation, r.Symbol, r.Status)
			} else {
				msg = fmt.Sprintf("🟢%-4s %-10s (%4s)", r.Operation, r.Symbol, r.Status)
			}
		} else if operation == "Sell" {
			if r.Symbol == "BTCUSDT" || r.Symbol == "ETHUSDT" {
				msg = fmt.Sprintf("💎%-4s %-10s (%4s)", r.Operation, r.Symbol, r.Status)
			} else {
				msg = fmt.Sprintf("🔴%-4s %-10s (%4s)", r.Operation, r.Symbol, r.Status)
			}
		} else {
			continue // 不满足推送条件
		}

		if err := sendWithRetry(botToken, chatID, msg); err != nil {
			log.Printf("发送 %s 消息失败: %v", r.Symbol, err)
			continue
		}
	}
	return nil
}

// sendWithRetry 封装了带一次重试的 Telegram 发送逻辑
func sendWithRetry(botToken, chatID, msg string) error {
	err := telegram.SendMessage(botToken, chatID, msg)
	if err != nil {
		time.Sleep(2 * time.Second) // 可根据需求调节重试等待时间
		err = telegram.SendMessage(botToken, chatID, msg)
	}
	return err
}
