package utils

import (
	"energe/telegram"
	"fmt"
	"time"

	"energe/types"
)

func PushTelegram(results []types.CoinIndicator, botToken, chatID string, volumeCache *types.VolumeCache) error {
	now := time.Now().Format("2006-01-02 15:04")
	header := fmt.Sprintf("----15m 信号（%s）", now)

	if err := telegram.SendMessage(botToken, chatID, header); err != nil {
		return err
	}
	for _, r := range results {
		volume, ok := volumeCache.Get(r.Symbol)
		if !ok {
			volume = 0
		}
		operation := r.Operation

		if operation == "Buy" && volume > 300000000 {
			msg := fmt.Sprintf("🟢%-4s %-10s SRSI:%3.1f",
				r.Operation, r.Symbol, r.StochRSI)
			if err := telegram.SendMessage(botToken, chatID, msg); err != nil {
				return err
			}
		} else if operation == "Sell" && volume > 50000000 {
			msg := fmt.Sprintf("🔴%-4s %-10s SRSI:%3.1f",
				r.Operation, r.Symbol, r.StochRSI)
			if err := telegram.SendMessage(botToken, chatID, msg); err != nil {
				return err
			}
		}
	}
	return nil
}
