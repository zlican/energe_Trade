package utils

import (
	"time"
)

// AlignedTicker 每 interval 对齐一次，例如每 1 秒、5 分钟、1 小时等
func AlignedTicker(interval time.Duration) <-chan time.Time {
	ch := make(chan time.Time)
	go func() {
		// 等到下一个完整周期
		time.Sleep(time.Until(time.Now().Truncate(interval).Add(interval)))
		ticker := time.NewTicker(interval)
		ch <- time.Now()
		for t := range ticker.C {
			ch <- t
		}
	}()
	return ch
}
