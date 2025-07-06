package utils

import (
	"math"
)

// ------------------------ RSI (Wilder) ------------------------

// CalculateRSI 返回 0~100 区间，前 period 值为 NaN。
func CalculateRSI(close []float64, period int) []float64 {
	n := len(close)
	out := make([]float64, n)

	if n == 0 || period <= 0 || n < period+1 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}

	// 标记 NaN
	for i := 0; i < period; i++ {
		out[i] = math.NaN()
	}

	// 首个平均
	var gain, loss float64
	for i := 1; i <= period; i++ {
		delta := close[i] - close[i-1]
		if delta > 0 {
			gain += delta
		} else {
			loss -= delta
		}
	}
	avgGain := gain / float64(period)
	avgLoss := loss / float64(period)
	out[period] = rsiValue(avgGain, avgLoss)

	// 递推
	for i := period + 1; i < n; i++ {
		delta := close[i] - close[i-1]
		var g, l float64
		if delta > 0 {
			g = delta
		} else {
			l = -delta
		}
		avgGain = (avgGain*float64(period-1) + g) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + l) / float64(period)
		out[i] = rsiValue(avgGain, avgLoss)
	}
	return out
}

func rsiValue(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		if avgGain == 0 {
			return 50 // 横盘
		}
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - 100/(1+rs)
}

// ------------------------ StochRSI ------------------------

// StochRSI 返回 (raw, %K, %D)，三条都 0~100；无效期为 NaN。
// stochPeriod: 常用 14；k, d: 常用 3。
func StochRSI(rsi []float64, stochPeriod, k, d int) (raw, kLine, dLine []float64) {
	n := len(rsi)
	raw = make([]float64, n)
	kLine = make([]float64, n)
	dLine = make([]float64, n)

	// 全 NaN 初始化
	for i := 0; i < n; i++ {
		raw[i], kLine[i], dLine[i] = math.NaN(), math.NaN(), math.NaN()
	}

	if n < stochPeriod || stochPeriod <= 0 {
		return
	}

	// 用双端队列 O(n) 维护滑窗最小/最大
	type pair struct {
		idx int
		val float64
	}
	var minQ, maxQ []pair

	pushMin := func(i int, v float64) {
		for len(minQ) > 0 && minQ[len(minQ)-1].val >= v {
			minQ = minQ[:len(minQ)-1]
		}
		minQ = append(minQ, pair{i, v})
	}
	pushMax := func(i int, v float64) {
		for len(maxQ) > 0 && maxQ[len(maxQ)-1].val <= v {
			maxQ = maxQ[:len(maxQ)-1]
		}
		maxQ = append(maxQ, pair{i, v})
	}

	for i := 0; i < n; i++ {
		if math.IsNaN(rsi[i]) {
			continue
		}
		pushMin(i, rsi[i])
		pushMax(i, rsi[i])

		left := i - stochPeriod + 1
		// 滑窗过期弹出
		if minQ[0].idx < left {
			minQ = minQ[1:]
		}
		if maxQ[0].idx < left {
			maxQ = maxQ[1:]
		}
		if left < 0 {
			continue // 窗口未满
		}
		lo, hi := minQ[0].val, maxQ[0].val
		if hi != lo {
			raw[i] = (rsi[i] - lo) / (hi - lo) * 100
		} else {
			raw[i] = 0
		}

		// %K: k‑period SMA
		if i >= stochPeriod+k-1 {
			var sum float64
			cnt := 0
			for j := i; j > i-k && j >= 0; j-- {
				if !math.IsNaN(raw[j]) {
					sum += raw[j]
					cnt++
				}
			}
			if cnt == k {
				kLine[i] = sum / float64(k)
			}
		}

		// %D: d‑period SMA of %K
		if i >= stochPeriod+k+d-2 {
			var sum float64
			cnt := 0
			for j := i; j > i-d && j >= 0; j-- {
				if !math.IsNaN(kLine[j]) {
					sum += kLine[j]
					cnt++
				}
			}
			if cnt == d {
				dLine[i] = sum / float64(d)
			}
		}
	}
	return
}

// StochRSIFromClose 一站式。
func StochRSIFromClose(close []float64, rsiPeriod, stochPeriod, k, d int) (raw, kLine, dLine []float64) {
	rsi := CalculateRSI(close, rsiPeriod)
	return StochRSI(rsi, stochPeriod, k, d)
}
