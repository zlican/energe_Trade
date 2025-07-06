package utils

// 计算指数移动平均线
func CalculateEMA(data []float64, period int) []float64 {
	ema := make([]float64, len(data))
	multiplier := 2.0 / float64(period+1)
	ema[0] = data[0]
	for i := 1; i < len(data); i++ {
		ema[i] = (data[i]-ema[i-1])*multiplier + ema[i-1]
	}
	return ema
}
