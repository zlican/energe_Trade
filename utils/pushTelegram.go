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
	header := fmt.Sprintf("15m 信号（%s）👇👇", now)

	if err := sendWithRetry(botToken, chatID, header); err != nil {
		log.Printf("发送 header 消息失败: %v", err)
	}

	var gt bool
	var Trend1H, Trend15M string
	gt = GetPriceGT_EMA25FromDB(db, "BTCUSDT")
	ema25M15, ema50M15 := Get15MEMAFromDB(db, "BTCUSDT")
	if gt {
		Trend1H = "禁空"
	} else {
		Trend1H = "禁多"
	}
	if ema25M15 > ema50M15 {
		Trend15M = "做多"
	} else if ema25M15 < ema50M15 {
		Trend15M = "做空"
	} else {
		Trend15M = "震荡"
	}

	trend := fmt.Sprintf("趋势：1H（%s）15M（%s）", Trend1H, Trend15M)
	if err := sendWithRetry(botToken, chatID, trend); err != nil {
		log.Printf("发送 trend 消息失败: %v", err)
	}

	for _, r := range results {
		volume, ok := volumeCache.Get(r.Symbol)
		if !ok {
			volume = 0
		}
		operation := r.Operation
		var msg string

		if operation == "Buy" && volume > 300000000 {
			if r.Symbol == "BTCUSDT" {
				msg = fmt.Sprintf("💎%-4s %-10s (%4s)", r.Operation, r.Symbol, r.Status)
			} else if r.Symbol == "ETHUSDT" || r.Symbol == "SOLUSDT" || r.Symbol == "HYPEUSDT" {
				msg = fmt.Sprintf("🌟%-4s %-10s (%4s)", r.Operation, r.Symbol, r.Status)
			} else {
				msg = fmt.Sprintf("🟢%-4s %-10s (%4s)", r.Operation, r.Symbol, r.Status)
			}
		} else if operation == "Sell" && volume > 300000000 {
			if gt {
				continue
			}
			if r.Symbol == "BTCUSDT" {
				msg = fmt.Sprintf("💎%-4s %-10s SRSI:%3.1f", r.Operation, r.Symbol, r.StochRSI)
			} else if r.Symbol == "ETHUSDT" || r.Symbol == "SOLUSDT" || r.Symbol == "HYPEUSDT" {
				msg = fmt.Sprintf("🌟%-4s %-10s SRSI:%3.1f", r.Operation, r.Symbol, r.StochRSI)
			} else {
				msg = fmt.Sprintf("🔴%-4s %-10s SRSI:%3.1f", r.Operation, r.Symbol, r.StochRSI)
			}
		} else {
			continue // 不满足推送条件
		}

		if err := sendWithRetry(botToken, chatID, msg); err != nil {
			log.Printf("发送 %s 消息失败: %v", r.Symbol, err)
			continue
		}
	}

	if err := sendWithRetry(botToken, chatID, "END          "); err != nil {
		log.Printf("发送结束标记失败: %v", err)
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
