package indicators

import "math"

type BollingerBands struct {
	Upper  []float64
	Middle []float64
	Lower  []float64
}

// CalculateBollingerBands computes the Bollinger Bands.
func CalculateBollingerBands(closes []float64, period int, multiplier float64) BollingerBands {
	length := len(closes)
	upper := make([]float64, length)
	middle := make([]float64, length)
	lower := make([]float64, length)

	if length < period {
		return BollingerBands{upper, middle, lower}
	}

	for i := period - 1; i < length; i++ {
		// Simple MA
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += closes[i-j]
		}
		ma := sum / float64(period)
		middle[i] = ma

		// Standard Deviation
		sumSqDiff := 0.0
		for j := 0; j < period; j++ {
			diff := closes[i-j] - ma
			sumSqDiff += diff * diff
		}
		
		stdDev := 0.0
		if period > 1 {
			stdDev = math.Sqrt(sumSqDiff / float64(period))
		}

		upper[i] = ma + (multiplier * stdDev)
		lower[i] = ma - (multiplier * stdDev)
	}

	return BollingerBands{Upper: upper, Middle: middle, Lower: lower}
}
