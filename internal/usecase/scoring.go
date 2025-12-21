package usecase

import (
	"screener-backend/internal/domain"
	"screener-backend/internal/infrastructure/binance"
	"screener-backend/internal/infrastructure/indicators"
	"strconv"
)

// CalculateScore computes the score based on market features.
func CalculateScore(features *domain.MarketFeatures) float64 {
	// Weights (Stricter for reversal accuracy)
	// Overextension: 0-30
	// Crowding: 0-20
	// Exhaustion: 0-30 (increased from 25)
	// Structure: 0-20 (decreased from 25)

	sOver := 0.0
	// More aggressive pump = higher score
	if features.PctChange24h >= 50 {
		sOver += 20 // Extreme pump
	} else if features.PctChange24h >= 40 {
		sOver += 15
	} else if features.PctChange24h >= 25 {
		sOver += 10
	} else if features.PctChange24h >= 15 {
		sOver += 5
	}

	// EMA Overext - stricter threshold
	if features.OverExtEma >= 0.08 {
		sOver += 15 // Very overextended
	} else if features.OverExtEma >= 0.05 {
		sOver += 10
	} else if features.OverExtEma >= 0.03 {
		sOver += 5
	}

	// VWAP Overext
	if features.OverExtVwap >= 0.05 {
		sOver += 5
	} else if features.OverExtVwap >= 0.03 {
		sOver += 3
	}

	if sOver > 30 {
		sOver = 30
	}

	sCrowd := 0.0
	// Funding - stricter for high leverage crowding
	if features.FundingRate > 0.001 {
		sCrowd += 10 // Very high funding
	} else if features.FundingRate > 0.0005 {
		sCrowd += 7
	} else if features.FundingRate > 0.0001 {
		sCrowd += 3
	}
	
	// OI delta - confirms one-sided position building
	if features.OpenInterestDelta > 10 {
		sCrowd += 7
	} else if features.OpenInterestDelta > 5 {
		sCrowd += 3
	}

	if sCrowd > 20 {
		sCrowd = 20
	}

	sExhaust := 0.0
	// RSI - stricter thresholds for reversal
	if features.RSI > 80 {
		sExhaust += 20 // Extreme overbought
	} else if features.RSI > 75 {
		sExhaust += 15 // Very overbought
	} else if features.RSI > 70 {
		sExhaust += 10
	} else if features.RSI > 65 {
		sExhaust += 5
	}

	if features.IsAboveUpperBand {
		sExhaust += 5
	}
	
	// Rejection wick ratio - shows selling pressure at highs
	if features.RejectionWickRatio > 0.5 {
		sExhaust += 5 // Strong rejection
	}

	if sExhaust > 30 {
		sExhaust = 30
	}

	sStruct := 0.0
	if features.IsBreakdown {
		sStruct += 12
	}
	if features.IsRetest {
		sStruct += 8
	}

	if sStruct > 20 {
		sStruct = 20
	}

	return sOver + sCrowd + sExhaust + sStruct
}

// ExtractFeatures computes indicators and extracts features for a coin.
func ExtractFeatures(
	prices, highs, lows []float64,
	ticker binance.Ticker24h,
	ema50, vwap, rsi []float64,
	bb indicators.BollingerBands,
	atr []float64,
	pivots []indicators.Pivot,
	fundingRate, oiDelta float64,
) *domain.MarketFeatures {
	lastIdx := len(prices) - 1
	if lastIdx < 0 {
		return nil
	}

	currentClose := prices[lastIdx]
	currentHigh := highs[lastIdx]
	currentLow := lows[lastIdx]
	
	// Calculate rejection wick ratio (upper wick / total candle range)
	// High rejection wick = selling pressure at highs = reversal signal
	rejectionWickRatio := 0.0
	candleRange := currentHigh - currentLow
	if candleRange > 0 {
		upperWick := currentHigh - currentClose
		rejectionWickRatio = upperWick / candleRange
	}

	// Indicators
	currentEma := 0.0
	if lastIdx < len(ema50) {
		currentEma = ema50[lastIdx]
	}

	currentVwap := 0.0
	if lastIdx < len(vwap) {
		currentVwap = vwap[lastIdx]
	}
	
	currentRsi := 50.0
	if lastIdx < len(rsi) {
		currentRsi = rsi[lastIdx]
	}
	
	currentAtr := 0.0
	if lastIdx < len(atr) {
		currentAtr = atr[lastIdx]
	}

	currentUpperBand := 0.0
	if lastIdx < len(bb.Upper) {
		currentUpperBand = bb.Upper[lastIdx]
	}

	// Overextension
	overExtEma := 0.0
	if currentEma != 0 {
		overExtEma = (currentClose - currentEma) / currentEma
	}

	overExtVwap := 0.0
	if currentVwap != 0 {
		overExtVwap = (currentClose - currentVwap) / currentVwap
	}

	isAboveUpperBand := false
	if currentUpperBand != 0 {
		isAboveUpperBand = currentClose > currentUpperBand
	}

	// Structure
	nearestSupPivot := indicators.GetNearestSupport(pivots, lastIdx)
	var supportPrice *float64
	if nearestSupPivot != nil {
		supportPrice = &nearestSupPivot.Price
	}

	isBrk := false
	isRetestZone := false

	if supportPrice != nil && currentAtr > 0 {
		isBrk = indicators.IsBreakdown(currentClose, *supportPrice, currentAtr, 0.1)
		isRetestZone = indicators.IsInRetestZone(currentHigh, currentLow, *supportPrice, currentAtr, 0.2)
	}

	var distToSupportATR *float64
	if supportPrice != nil && currentAtr > 0 {
		val := (currentClose - *supportPrice) / currentAtr
		distToSupportATR = &val
	}

	// Ticker pct change
	pctChange, _ := strconvToFloat(ticker.PriceChangePercent)

	return &domain.MarketFeatures{
		PctChange24h:       pctChange,
		OverExtEma:         overExtEma,
		OverExtVwap:        overExtVwap,
		IsAboveUpperBand:   isAboveUpperBand,
		CandleRangeRatio:   0, // Placeholder
		RSI:                currentRsi,
		IsRsiBearishDiv:    false,
		RejectionWickRatio: rejectionWickRatio,
		FundingRate:        fundingRate,
		OpenInterestDelta:  oiDelta,
		NearestSupport:     supportPrice,
		DistToSupportATR:   distToSupportATR,
		IsBreakdown:        isBrk,
		IsRetest:           isRetestZone,
		IsRetestFail:       false,
	}
}

// Helper
func strconvToFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
