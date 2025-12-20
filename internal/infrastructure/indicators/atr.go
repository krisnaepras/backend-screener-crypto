package indicators

import "math"

// CalculateATR computes the Average True Range.
func CalculateATR(highs, lows, closes []float64, period int) []float64 {
	length := len(closes)
	atr := make([]float64, length)
	if length < period+1 {
		return atr
	}

	trs := make([]float64, length) // Map TR to index of the candle it belongs to.
	
	// TR calculation
	// trs[0] is just H-L
	trs[0] = highs[0] - lows[0]
	
	for i := 1; i < length; i++ {
		hl := highs[i] - lows[i]
		hc := math.Abs(highs[i] - closes[i-1])
		lc := math.Abs(lows[i] - closes[i-1])
		
		maxVal := hl
		if hc > maxVal {
			maxVal = hc
		}
		if lc > maxVal {
			maxVal = lc
		}
		trs[i] = maxVal
	}

	// First ATR
	sumTR := 0.0
	for i := 0; i < period; i++ {
		sumTR += trs[i]
	}
	atr[period-1] = sumTR / float64(period)

	// Smoothing
	for i := period; i < length; i++ {
		prevAtr := atr[i-1]
		atr[i] = (prevAtr*float64(period-1) + trs[i]) / float64(period)
	}

	return atr
}
