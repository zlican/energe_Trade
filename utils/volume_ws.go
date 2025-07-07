package utils

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// VolumeCache 通过单条 WS 链接实时维护 24 h QuoteVolume。
type VolumeCache struct {
	m         sync.Map
	stop      chan struct{}
	readyOnce sync.Once     // 首次推送到达的保护
	readyCh   chan struct{} // 外部等待用
}

// NewVolumeCache：① 先 REST 预热 → ② 启动 WS → ③ 自动重连。
func NewVolumeCache(restCli *futures.Client) (*VolumeCache, error) {
	vc := &VolumeCache{readyCh: make(chan struct{})}

	// -------- ① 预热快照 --------
	stats, err := restCli.NewListPriceChangeStatsService().Do(context.Background())
	if err != nil {
		return nil, err
	}
	for _, s := range stats {
		if v, err := strconv.ParseFloat(s.QuoteVolume, 64); err == nil {
			vc.m.Store(s.Symbol, v)
		}
	}

	// -------- ② 启动 WS (带 ③ 自动重连) --------
	go vc.loop()

	return vc, nil
}

// loop: 建立 / 监听 WS；断线后自动重连。
func (vc *VolumeCache) loop() {
	for {
		doneC, stopC, err := futures.WsAllMiniMarketTickerServe(
			func(ev futures.WsAllMiniMarketTickerEvent) {
				// 首次收到任何推送，就触发 readyOnce
				vc.readyOnce.Do(func() { close(vc.readyCh) })

				for _, t := range ev {
					if v, err := strconv.ParseFloat(t.QuoteVolume, 64); err == nil {
						vc.m.Store(t.Symbol, v)
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
		vc.stop = stopC
		<-doneC // 等待连接关闭再重建
		log.Print("miniTicker WS 断开，重连中…")
	}
}

// Get 返回最新 QuoteVolume；若还没数据 ok=false。
func (vc *VolumeCache) Get(sym string) (float64, bool) {
	v, ok := vc.m.Load(sym)
	if !ok {
		return 0, false
	}
	return v.(float64), true
}

// Ready 返回一个只会关闭一次的 chan，表示“至少收到过一次推送”。
func (vc *VolumeCache) Ready() <-chan struct{} { return vc.readyCh }

// Close 优雅关流。
func (vc *VolumeCache) Close() {
	if vc.stop != nil {
		close(vc.stop)
	}
}
