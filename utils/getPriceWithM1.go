package utils

func IsM1Up(opens, closes []float64) bool {
	if len(opens) < 2 || len(closes) < 2 {
		return false
	}

	preOpen := opens[len(opens)-2]
	preClose := closes[len(closes)-2]

	ema25 := CalculateEMA(closes, 25)
	ema50 := CalculateEMA(closes, 50)
	if len(ema25) == 0 || len(ema50) == 0 {
		return false
	}

	return preOpen > ema25[len(ema25)-1] &&
		preClose > ema25[len(ema25)-1] &&
		preOpen > ema50[len(ema50)-1] &&
		preClose > ema50[len(ema50)-1]
}

func IsM1Down(opens, closes []float64) bool {
	if len(opens) < 2 || len(closes) < 2 {
		return false
	}

	preOpen := opens[len(opens)-2]
	preClose := closes[len(closes)-2]

	ema25 := CalculateEMA(closes, 25)
	ema50 := CalculateEMA(closes, 50)
	if len(ema25) == 0 || len(ema50) == 0 {
		return false
	}

	return preOpen < ema25[len(ema25)-1] &&
		preClose < ema25[len(ema25)-1] &&
		preOpen < ema50[len(ema50)-1] &&
		preClose < ema50[len(ema50)-1]
}
