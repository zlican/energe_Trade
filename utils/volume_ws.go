package utils

import (
	"log"
	"strconv"
	"sync"

	"github.com/adshao/go-binance/v2/futures"
)

// VolumeCache 通过单条 websocket 链接实时维护 24h QuoteVolume。
type VolumeCache struct {
	m    sync.Map      // key = Symbol, value = float64
	stop chan struct{} // 关闭 WS 用
}

// NewVolumeCache 启动 WsAllMiniMarketTickerServe；出错直接返回。
func NewVolumeCache() (*VolumeCache, error) {
	vc := &VolumeCache{}

	// ⭐ 一条连接推送所有合约的 24h mini‑ticker【含 QuoteVolume】。
	// volume_ws.go
	_, stopC, err := futures.WsAllMiniMarketTickerServe(
		func(ev futures.WsAllMiniMarketTickerEvent) {
			// ev 才是真正的一组 mini‑ticker
			for _, t := range ev {
				if vol, err := strconv.ParseFloat(t.QuoteVolume, 64); err == nil {
					vc.m.Store(t.Symbol, vol)
				}
			}
		},
		func(err error) { log.Printf("miniTicker ws err: %v", err) },
	)
	if err != nil {
		log.Fatalf("miniTicker WS 启动失败: %v", err)
	}
	defer close(stopC)

	vc.stop = stopC
	return vc, nil
}

// Get 返回最新 QuoteVolume；若尚未收到推送则 ok=false。
func (vc *VolumeCache) Get(symbol string) (vol float64, ok bool) {
	val, ok := vc.m.Load(symbol)
	if !ok {
		return 0, false
	}
	return val.(float64), true
}

// Close 优雅关闭 websocket。
func (vc *VolumeCache) Close() { close(vc.stop) }
