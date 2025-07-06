package utils

// 计算简单移动平均线：输入数据和数量，返回当下MA数值
func CalculateMA(data []float64, period int) float64 {
	if len(data) < period || period <= 0 {
		return 0
	}

	sum := 0.0
	for i := len(data) - period; i < len(data); i++ {
		sum += data[i]
	}
	return sum / float64(period)
}
