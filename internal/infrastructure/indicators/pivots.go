package indicators

type Pivot struct {
	Index int
	Price float64
}

// FindPivotLows identifies pivot lows in the price data.
func FindPivotLows(lows []float64, leftBars, rightBars int) []Pivot {
	var pivots []Pivot
	length := len(lows)

	for i := leftBars; i < length-rightBars; i++ {
		currentLow := lows[i]
		isPivot := true

		// Check left
		for j := 1; j <= leftBars; j++ {
			if lows[i-j] <= currentLow {
				isPivot = false
				break
			}
		}

		// Check right
		if isPivot {
			for j := 1; j <= rightBars; j++ {
				if lows[i+j] <= currentLow {
					isPivot = false
					break
				}
			}
		}

		if isPivot {
			pivots = append(pivots, Pivot{Index: i, Price: currentLow})
		}
	}

	return pivots
}

// FindPivotHighs identifies pivot highs in the price data (for resistance).
func FindPivotHighs(highs []float64, leftBars, rightBars int) []Pivot {
	var pivots []Pivot
	length := len(highs)

	for i := leftBars; i < length-rightBars; i++ {
		currentHigh := highs[i]
		isPivot := true

		// Check left
		for j := 1; j <= leftBars; j++ {
			if highs[i-j] >= currentHigh {
				isPivot = false
				break
			}
		}

		// Check right
		if isPivot {
			for j := 1; j <= rightBars; j++ {
				if highs[i+j] >= currentHigh {
					isPivot = false
					break
				}
			}
		}

		if isPivot {
			pivots = append(pivots, Pivot{Index: i, Price: currentHigh})
		}
	}

	return pivots
}

// GetNearestSupport finds the nearest support pivot below the current price (conceptually).
// The original logic just returned the last pivot before currentIndex.
func GetNearestSupport(pivots []Pivot, currentIndex int) *Pivot {
	for i := len(pivots) - 1; i >= 0; i-- {
		if pivots[i].Index < currentIndex {
			p := pivots[i] // copy
			return &p
		}
	}
	return nil
}

func IsBreakdown(close, support, atr, thresholdFactor float64) bool {
	return close < support-(thresholdFactor*atr)
}

func IsInRetestZone(high, low, support, atr, rangeFactor float64) bool {
	upperZone := support + rangeFactor*atr
	lowerZone := support - rangeFactor*atr
	return low <= upperZone && high >= lowerZone
}
