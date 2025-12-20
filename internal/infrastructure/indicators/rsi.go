package indicators

// CalculateRSI computes the Relative Strength Index.
func CalculateRSI(closes []float64, period int) []float64 {
	rsi := make([]float64, len(closes))
	if len(closes) < period+1 {
		return rsi // return zeros
	}

	gains := make([]float64, 0, len(closes))
	losses := make([]float64, 0, len(closes))

	// Padding for index 0 to match logic?
	// Dart loop: for (int i = 1; i < closes.length; i++)
	// It pushes to gains/losses. gains[0] corresponds to change between close[1] and close[0].
	
	// We need to match indices carefully.
	// Let's create diff arrays first.
	// diffs[i] = closes[i+1] - closes[i]
	
	for i := 1; i < len(closes); i++ {
		change := closes[i] - closes[i-1]
		if change > 0 {
			gains = append(gains, change)
			losses = append(losses, 0)
		} else {
			gains = append(gains, 0)
			losses = append(losses, -change)
		}
	}

	// First average gain/loss
	// gains[0] corresponds to closes[1]
	// period=14. we need gains[0] to gains[13].
	
	if len(gains) < period {
		return rsi
	}

	sumGain := 0.0
	sumLoss := 0.0
	for i := 0; i < period; i++ {
		sumGain += gains[i]
		sumLoss += losses[i]
	}
	
	avgGain := sumGain / float64(period)
	avgLoss := sumLoss / float64(period)

	// First RSI point is at index 'period'.
	// In Dart: rsi[period]
	// closes[period] corresponds to the change at gains[period-1].
	// The Dart code calculates avgGain/Loss using sublist(0, period), which are indices 0..period-1.
	// Then it sets rsi[period].
	
	rs := 0.0
	if avgLoss != 0 {
		rs = avgGain / avgLoss
	} else {
		rs = 0 // handled by logic below usually. If loss is 0, RSI is 100.
	}
	
	if avgLoss == 0 {
		rsi[period] = 100
	} else {
		rsi[period] = 100 - (100 / (1 + rs))
	}

	// Smoothing
	for i := period + 1; i < len(closes); i++ {
		// Index in gains/losses is i-1
		currentGain := gains[i-1]
		currentLoss := losses[i-1]

		avgGain = ((avgGain * float64(period-1)) + currentGain) / float64(period)
		avgLoss = ((avgLoss * float64(period-1)) + currentLoss) / float64(period)

		if avgLoss == 0 {
			rsi[i] = 100
		} else {
			rs = avgGain / avgLoss
			rsi[i] = 100 - (100 / (1 + rs))
		}
	}
	
	return rsi
}

// Helper for formatting/rounding if needed, but standard logic above is fine.
