package indicators

import (
	"strconv"
)

// KLine represents a single candlestick data point from Binance.
// [Open time, Open, High, Low, Close, Volume, Close time, Quote asset volume, Number of trades, Taker buy base asset volume, Taker buy quote asset volume, Ignore]
// We assume incoming klines are []interface{} or similar, but for strict typing let's define a struct or assume parsed floats.
// The Dart code took List<List<dynamic>>.
// In Go, let's assume we pass struct or separate slices.
// To keep it simple and generic, let's take slices of High, Low, Close, Volume.

// CalculateVWAP computes Volume Weighted Average Price.
// Unlike the Dart code which parsed strings from JSON, we assume pre-parsed floats here.
func CalculateVWAP(highs, lows, closes, volumes []float64) []float64 {
	length := len(closes)
	vwap := make([]float64, length)
	
	cumulativeTPV := 0.0
	cumulativeVol := 0.0

	for i := 0; i < length; i++ {
		h := highs[i]
		l := lows[i]
		c := closes[i]
		v := volumes[i]

		typicalPrice := (h + l + c) / 3.0

		cumulativeTPV += (typicalPrice * v)
		cumulativeVol += v

		if cumulativeVol > 0 {
			vwap[i] = cumulativeTPV / cumulativeVol
		}
	}

	return vwap
}

// Helper to parse string to float if needed later in infrastructure
func parseToFloat(v interface{}) float64 {
	switch val := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case float64:
		return val
	}
	return 0
}
