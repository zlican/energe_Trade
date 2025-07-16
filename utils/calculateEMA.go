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

// CalculateEMADerivative 返回 ema 曲线的一阶导数（离散斜率）。
// derivative[i] = ema[i] - ema[i-1]，长度与 ema 相同，
// 其中 derivative[0] 约定为 0（或可改为 NaN / math.NaN()）。
func CalculateEMADerivative(ema []float64) []float64 {
	if len(ema) == 0 {
		return nil
	}
	derivative := make([]float64, len(ema))
	derivative[0] = 0 // 或 math.NaN()

	for i := 1; i < len(ema); i++ {
		derivative[i] = ema[i] - ema[i-1]
	}
	return derivative
}
