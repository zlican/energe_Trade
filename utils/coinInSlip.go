package utils

func IsSlipCoin(sym string, slipCoin []string) bool {
	for _, s := range slipCoin {
		if s == sym {
			return true
		}
	}
	return false
}
