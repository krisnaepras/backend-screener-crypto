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

	// Momentum Loss Score: 0-15
	sMomentum := 0.0
	if features.IsLosingMomentum {
		sMomentum += 8
	}
	if features.HasRsiDivergence {
		sMomentum += 5
	}
	if features.HasVolumeDivergence {
		sMomentum += 3
	}
	if features.RsiSlope < -3 {
		sMomentum += 4
	} else if features.RsiSlope < -1 {
		sMomentum += 2
	}

	if sMomentum > 15 {
		sMomentum = 15
	}

	return sOver + sCrowd + sExhaust + sStruct + sMomentum
}

// ExtractFeatures computes indicators and extracts features for a coin.
func ExtractFeatures(
	prices, highs, lows, volumes []float64,
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

	// Momentum Loss Detection
	momentumSignals := indicators.DetectMomentumLoss(prices, highs, volumes, rsi)

	return &domain.MarketFeatures{
		PctChange24h:        pctChange,
		OverExtEma:          overExtEma,
		OverExtVwap:         overExtVwap,
		IsAboveUpperBand:    isAboveUpperBand,
		CandleRangeRatio:    0, // Placeholder
		RSI:                 currentRsi,
		IsRsiBearishDiv:     momentumSignals.HasRsiDivergence,
		RejectionWickRatio:  rejectionWickRatio,
		FundingRate:         fundingRate,
		OpenInterestDelta:   oiDelta,
		NearestSupport:      supportPrice,
		DistToSupportATR:    distToSupportATR,
		IsBreakdown:         isBrk,
		IsRetest:            isRetestZone,
		IsRetestFail:        false,
		HasRsiDivergence:    momentumSignals.HasRsiDivergence,
		HasVolumeDivergence: momentumSignals.HasVolumeDivergence,
		MomentumSlope:       momentumSignals.MomentumSlope,
		RsiSlope:            momentumSignals.RsiSlope,
		VolumeDeclineRatio:  momentumSignals.VolumeDeclineRatio,
		IsLosingMomentum:    momentumSignals.IsLosingMomentum,
	}
}

// Helper
func strconvToFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// CalculatePullbackScore computes score for Buy the Dip / Pullback Entry setup
// Criteria: Uptrend + Pullback to support/EMA + Bounce signal
func CalculatePullbackScore(prices, ema20, ema50, rsi []float64, features *domain.MarketFeatures) float64 {
	if len(prices) < 50 || len(ema20) < 50 || len(ema50) < 50 || len(rsi) < 14 {
		return 0
	}

	lastIdx := len(prices) - 1
	currentPrice := prices[lastIdx]
	currentEma20 := ema20[lastIdx]
	currentEma50 := ema50[lastIdx]
	currentRsi := rsi[lastIdx]

	score := 0.0

	// === TREND SCORE (0-30) ===
	// Check if in uptrend: EMA20 > EMA50, price trending up
	trendScore := 0.0

	// EMA alignment (uptrend)
	if currentEma20 > currentEma50 {
		trendScore += 15 // EMAs aligned bullish
	}

	// Price above EMA50 (still in uptrend structure)
	if currentPrice > currentEma50*0.98 { // Allow slight dip below EMA50
		trendScore += 10
	}

	// Higher highs check (last 20 candles)
	if lastIdx >= 20 {
		recentHigh := prices[lastIdx-10]
		olderHigh := prices[lastIdx-20]
		for i := lastIdx - 10; i <= lastIdx; i++ {
			if prices[i] > recentHigh {
				recentHigh = prices[i]
			}
		}
		for i := lastIdx - 20; i < lastIdx-10; i++ {
			if prices[i] > olderHigh {
				olderHigh = prices[i]
			}
		}
		if recentHigh >= olderHigh*0.98 { // Recent high is near or above older high
			trendScore += 5
		}
	}

	if trendScore > 30 {
		trendScore = 30
	}
	score += trendScore

	// === PULLBACK SCORE (0-30) ===
	// Check if price has pulled back but not crashed
	pullbackScore := 0.0

	// RSI in pullback zone (30-45 ideal for dip buying)
	if currentRsi >= 30 && currentRsi <= 40 {
		pullbackScore += 20 // Ideal oversold bounce zone
	} else if currentRsi > 40 && currentRsi <= 50 {
		pullbackScore += 15 // Mild pullback
	} else if currentRsi >= 25 && currentRsi < 30 {
		pullbackScore += 10 // Very oversold, risky but potential
	}

	// Price pulled back to EMA zone (near EMA20 or between EMA20-EMA50)
	distToEma20 := (currentPrice - currentEma20) / currentEma20
	if distToEma20 >= -0.02 && distToEma20 <= 0.01 {
		pullbackScore += 10 // Price touching/near EMA20 (support)
	} else if distToEma20 >= -0.03 && distToEma20 < -0.02 {
		pullbackScore += 5 // Slight dip below EMA20
	}

	if pullbackScore > 30 {
		pullbackScore = 30
	}
	score += pullbackScore

	// === BOUNCE SIGNAL SCORE (0-25) ===
	bounceScore := 0.0

	// RSI bouncing (current RSI higher than recent low)
	if lastIdx >= 5 {
		minRsi := rsi[lastIdx-5]
		for i := lastIdx - 5; i < lastIdx; i++ {
			if rsi[i] < minRsi {
				minRsi = rsi[i]
			}
		}
		if currentRsi > minRsi+3 { // RSI bouncing up
			bounceScore += 15
		}
	}

	// Price bounce (current price higher than recent low)
	if lastIdx >= 5 {
		minPrice := prices[lastIdx-5]
		for i := lastIdx - 5; i < lastIdx; i++ {
			if prices[i] < minPrice {
				minPrice = prices[i]
			}
		}
		if currentPrice > minPrice*1.005 { // Price bounced at least 0.5%
			bounceScore += 10
		}
	}

	if bounceScore > 25 {
		bounceScore = 25
	}
	score += bounceScore

	// === RISK SCORE (0-15) ===
	riskScore := 0.0

	// Near support (good risk/reward)
	if features.DistToSupportATR != nil && *features.DistToSupportATR < 2.0 {
		riskScore += 10 // Close to support = tight stop loss
	}

	// Low rejection wick (not being sold off)
	if features.RejectionWickRatio < 0.3 {
		riskScore += 5
	}

	if riskScore > 15 {
		riskScore = 15
	}
	score += riskScore

	return score
}
