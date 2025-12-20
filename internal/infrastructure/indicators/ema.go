package indicators

// CalculateEMA computes the Exponential Moving Average.
func CalculateEMA(data []float64, period int) []float64 {
	ema := make([]float64, len(data))
	if len(data) < period {
		return ema
	}

	k := 2.0 / (float64(period) + 1.0)

	// Simple MA for the first EMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += data[i]
	}
	ema[period-1] = sum / float64(period)

	for i := period; i < len(data); i++ {
		prevEma := ema[i-1]
		ema[i] = (data[i] * k) + (prevEma * (1 - k))
	}

	return ema
}
