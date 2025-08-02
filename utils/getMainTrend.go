package utils

import (
	"database/sql"
	"energe/types"
)

func GetBTCTrend(db *sql.DB) string {
	GT_BTC := GetPriceGT_EMA25FromDB(db, "BTCUSDT")

	TrendUP := GT_BTC
	TrendDown := !GT_BTC

	if TrendUP {
		return "up"
	} else if TrendDown {
		return "down"
	}
	return "none"
}

func GetETHTrend(db *sql.DB) string {
	GT_ETH := GetPriceGT_EMA25FromDB(db, "ETHUSDT")

	TrendUP := GT_ETH
	TrendDown := !GT_ETH

	if TrendUP {
		return "up"
	} else if TrendDown {
		return "down"
	}
	return "none"
}

func GetBETrend(db *sql.DB) types.BETrend {
	return types.BETrend{
		BTC: GetBTCTrend(db),
		ETH: GetETHTrend(db),
	}
}

func GetMainTrend(bes types.BETrend) string {
	if bes.BTC == "up" || bes.ETH == "up" {
		return "up"
	}
	if bes.BTC == "down" || bes.ETH == "down" {
		return "down"
	}
	return "none"
}
