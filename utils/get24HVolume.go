package utils

import (
	"context"
	"log"
	"strconv"

	"github.com/adshao/go-binance/v2/futures"
)

func Get24HVolume(client *futures.Client, volumeMap map[string]float64) {
	tickerStats, err := client.NewListPriceChangeStatsService().Do(context.Background())
	if err != nil {
		log.Fatalf("获取 24 小时交易量失败: %v", err)
	}

	for _, stat := range tickerStats {
		volume, err := strconv.ParseFloat(stat.QuoteVolume, 64)
		if err != nil {
			continue
		}
		volumeMap[stat.Symbol] = volume
	}
}
