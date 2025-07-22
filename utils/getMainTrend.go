package utils

import (
	"database/sql"
	"energe/types"
)

func GetBTCTrend(db *sql.DB) string {
	ema25MH1, ema50MH1 := Get1HEMAFromDB(db, "BTCUSDT")
	ema25M15, ema50M15, _ := Get15MEMAFromDB(db, "BTCUSDT")
	ema25M5, ema50M5 := Get5MEMAFromDB(db, "BTCUSDT")

	TrendUP := ema25MH1 > ema50MH1 && ema25M15 > ema50M15 && ema25M5 > ema50M5
	TrendDown := ema25MH1 < ema50MH1 && ema25M15 < ema50M15 && ema25M5 < ema50M5

	if TrendUP {
		return "up"
	} else if TrendDown {
		return "down"
	}
	return "none"
}

func GetETHTrend(db *sql.DB) string {
	ema25MH1, ema50MH1 := Get1HEMAFromDB(db, "ETHUSDT")
	ema25M15, ema50M15, _ := Get15MEMAFromDB(db, "ETHUSDT")
	ema25M5, ema50M5 := Get5MEMAFromDB(db, "ETHUSDT")

	TrendUP := ema25MH1 > ema50MH1 && ema25M15 > ema50M15 && ema25M5 > ema50M5
	TrendDown := ema25MH1 < ema50MH1 && ema25M15 < ema50M15 && ema25M5 < ema50M5

	if TrendUP {
		return "up"
	} else if TrendDown {
		return "down"
	}
	return "none"
}

/*
	 func GetSOLTrend(db *sql.DB) string {
		priceGT_EMA25 := GetPriceGT_EMA25FromDB(db, "SOLUSDT")
		ema25M15, ema50M15, _ := Get15MEMAFromDB(db, "SOLUSDT")
		ema25M5, ema50M5 := Get5MEMAFromDB(db, "SOLUSDT")

		TrendUP := priceGT_EMA25 && ema25M15 > ema50M15 && ema25M5 > ema50M5
		TrendDown := !priceGT_EMA25 && ema25M15 < ema50M15 && ema25M5 < ema50M5

		if TrendUP {
			return "up"
		} else if TrendDown {
			return "down"
		}
		return "none"
	}
*/
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
