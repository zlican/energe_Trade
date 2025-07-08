package types

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// VolumeCache 通过单条 WS 链接实时维护 24 h QuoteVolume。
type VolumeCache struct {
	M         sync.Map
	Stop      chan struct{}
	ReadyOnce sync.Once     // 首次推送到达的保护
	ReadyCh   chan struct{} // 外部等待用
}

// loop: 建立 / 监听 WS；断线后自动重连。
func (vc *VolumeCache) Loop() {
	for {
		doneC, stopC, err := futures.WsAllMiniMarketTickerServe(
			func(ev futures.WsAllMiniMarketTickerEvent) {
				// 首次收到任何推送，就触发 readyOnce
				vc.ReadyOnce.Do(func() { close(vc.ReadyCh) })

				for _, t := range ev {
					if v, err := strconv.ParseFloat(t.QuoteVolume, 64); err == nil {
						vc.M.Store(t.Symbol, v)
					}
				}
			},
			func(err error) { log.Printf("miniTicker WS err: %v", err) },
		)
		if err != nil {
			log.Printf("miniTicker WS 启动失败 (%v)，5s 后重试", err)
			time.Sleep(5 * time.Second)
			continue
		}
		vc.Stop = stopC
		<-doneC // 等待连接关闭再重建
		log.Print("miniTicker WS 断开，重连中…")
	}
}

// Get 返回最新 QuoteVolume；若还没数据 ok=false。
func (vc *VolumeCache) Get(sym string) (float64, bool) {
	v, ok := vc.M.Load(sym)
	if !ok {
		return 0, false
	}
	return v.(float64), true
}

// Ready 返回一个只会关闭一次的 chan，表示“至少收到过一次推送”。
func (vc *VolumeCache) Ready() <-chan struct{} { return vc.ReadyCh }

// Close 优雅关流。
func (vc *VolumeCache) Close() {
	if vc.Stop != nil {
		close(vc.Stop)
	}
}

// SymbolsAbove 返回所有 QuoteVolume > limit 的交易对 symbol 列表
func (vc *VolumeCache) SymbolsAbove(limit float64) []string {
	var result []string
	vc.M.Range(func(key, value any) bool {
		volume, ok := value.(float64)
		if !ok {
			return true
		}
		if volume > limit {
			result = append(result, key.(string))
		}
		return true
	})
	return result
}
