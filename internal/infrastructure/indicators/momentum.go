package indicators

// MomentumSignals contains all momentum loss detection signals
type MomentumSignals struct {
	HasRsiDivergence    bool
	HasVolumeDivergence bool
	MomentumSlope       float64
	RsiSlope            float64
	VolumeDeclineRatio  float64
	IsLosingMomentum    bool
}

// DetectMomentumLoss analyzes candle data to detect loss of momentum signals
// closes, highs, volumes should be ordered oldest to newest
func DetectMomentumLoss(closes, highs, volumes []float64, rsiValues []float64) MomentumSignals {
	signals := MomentumSignals{}

	if len(closes) < 20 || len(rsiValues) < 20 {
		return signals
	}

	n := len(closes)

	// 1. RSI Divergence Detection (Price Higher High, RSI Lower High)
	signals.HasRsiDivergence = detectRsiDivergence(highs, rsiValues, 10)

	// 2. Volume Divergence (Price going up but volume declining)
	signals.HasVolumeDivergence, signals.VolumeDeclineRatio = detectVolumeDivergence(closes, volumes, 10)

	// 3. RSI Slope (Rate of change of RSI over last 5 candles)
	signals.RsiSlope = calculateSlope(rsiValues[n-5:])

	// 4. Momentum Slope (Price ROC trend)
	signals.MomentumSlope = calculateMomentumSlope(closes, 10)

	// 5. Combined signal: Is Losing Momentum
	momentumLossCount := 0
	if signals.HasRsiDivergence {
		momentumLossCount++
	}
	if signals.HasVolumeDivergence {
		momentumLossCount++
	}
	if signals.RsiSlope < -2 { // RSI declining
		momentumLossCount++
	}
	if signals.MomentumSlope < 0 { // Price momentum slowing
		momentumLossCount++
	}

	signals.IsLosingMomentum = momentumLossCount >= 2

	return signals
}

// detectRsiDivergence checks for bearish RSI divergence
// Price makes higher high but RSI makes lower high
func detectRsiDivergence(highs, rsiValues []float64, lookback int) bool {
	n := len(highs)
	if n < lookback || len(rsiValues) < n {
		return false
	}

	// Find local peaks in the last 'lookback' candles
	peaks := findLocalPeaks(highs, n-lookback, n)
	if len(peaks) < 2 {
		return false
	}

	// Check if latest 2 peaks show divergence
	lastPeak := peaks[len(peaks)-1]
	prevPeak := peaks[len(peaks)-2]

	priceHH := highs[lastPeak] > highs[prevPeak]
	rsiLH := rsiValues[lastPeak] < rsiValues[prevPeak]

	return priceHH && rsiLH
}

// findLocalPeaks finds indices of local maxima in a range
func findLocalPeaks(data []float64, start, end int) []int {
	peaks := []int{}
	for i := start + 1; i < end-1; i++ {
		if data[i] > data[i-1] && data[i] > data[i+1] {
			peaks = append(peaks, i)
		}
	}
	return peaks
}

// detectVolumeDivergence checks if price is rising but volume is declining
func detectVolumeDivergence(closes, volumes []float64, lookback int) (bool, float64) {
	n := len(closes)
	if n < lookback || len(volumes) < n {
		return false, 1.0
	}

	// Calculate price change over lookback
	priceChange := (closes[n-1] - closes[n-lookback]) / closes[n-lookback]

	// Calculate volume trend
	recentVolAvg := average(volumes[n-3:]) // Last 3 candles
	olderVolAvg := average(volumes[n-lookback : n-3])

	if olderVolAvg == 0 {
		return false, 1.0
	}

	volumeRatio := recentVolAvg / olderVolAvg

	// Divergence: price up 2%+, but volume down 30%+
	hasDivergence := priceChange > 0.02 && volumeRatio < 0.7

	return hasDivergence, volumeRatio
}

// calculateSlope calculates linear regression slope of data
func calculateSlope(data []float64) float64 {
	n := len(data)
	if n < 2 {
		return 0
	}

	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i := 0; i < n; i++ {
		x := float64(i)
		y := data[i]
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := float64(n)*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	slope := (float64(n)*sumXY - sumX*sumY) / denominator
	return slope
}

// calculateMomentumSlope measures rate of change trend
func calculateMomentumSlope(closes []float64, lookback int) float64 {
	n := len(closes)
	if n < lookback {
		return 0
	}

	// Calculate ROC for each period
	rocs := make([]float64, lookback-1)
	for i := n - lookback + 1; i < n; i++ {
		if closes[i-1] != 0 {
			rocs[i-(n-lookback+1)] = (closes[i] - closes[i-1]) / closes[i-1] * 100
		}
	}

	// Calculate slope of ROC (accelerating vs decelerating)
	return calculateSlope(rocs)
}

// average calculates the mean of a slice
func average(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}
