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

// CalculateVolatility computes volatility metrics for a coin
// Returns: atrPercent, bbWidth, volumeRatio, volatilityScore
func CalculateVolatility(prices, highs, lows, volumes []float64, atr, bbUpper, bbLower []float64) (float64, float64, float64, float64) {
	if len(prices) < 20 || len(atr) < 14 {
		return 0, 0, 0, 0
	}

	lastIdx := len(prices) - 1
	currentPrice := prices[lastIdx]
	currentAtr := atr[lastIdx]

	// ATR as percentage of price (higher = more volatile)
	atrPercent := 0.0
	if currentPrice > 0 {
		atrPercent = (currentAtr / currentPrice) * 100
	}

	// Bollinger Band width percentage
	bbWidth := 0.0
	if len(bbUpper) > lastIdx && len(bbLower) > lastIdx && currentPrice > 0 {
		bbWidth = ((bbUpper[lastIdx] - bbLower[lastIdx]) / currentPrice) * 100
	}

	// Volume ratio (current vs 20-period average)
	volumeRatio := 1.0
	if len(volumes) >= 20 {
		avgVolume := 0.0
		for i := lastIdx - 19; i < lastIdx; i++ {
			avgVolume += volumes[i]
		}
		avgVolume /= 20
		if avgVolume > 0 {
			volumeRatio = volumes[lastIdx] / avgVolume
		}
	}

	// Volatility Score (0-100)
	// Focus on ATR% and BB width - higher = more volatile
	volatilityScore := 0.0

	// ATR% scoring (0-40)
	// Crypto typically: <1% = low, 1-3% = medium, >3% = high volatility
	if atrPercent >= 4.0 {
		volatilityScore += 40 // Very high volatility
	} else if atrPercent >= 3.0 {
		volatilityScore += 35
	} else if atrPercent >= 2.0 {
		volatilityScore += 25
	} else if atrPercent >= 1.5 {
		volatilityScore += 15
	} else if atrPercent >= 1.0 {
		volatilityScore += 10
	}

	// BB Width scoring (0-30)
	// Higher width = more volatile
	if bbWidth >= 8.0 {
		volatilityScore += 30
	} else if bbWidth >= 6.0 {
		volatilityScore += 25
	} else if bbWidth >= 4.0 {
		volatilityScore += 20
	} else if bbWidth >= 3.0 {
		volatilityScore += 15
	} else if bbWidth >= 2.0 {
		volatilityScore += 10
	}

	// Volume spike scoring (0-30)
	// Volume > 1.5x avg = active trading = good for pullback
	if volumeRatio >= 3.0 {
		volatilityScore += 30
	} else if volumeRatio >= 2.0 {
		volatilityScore += 25
	} else if volumeRatio >= 1.5 {
		volatilityScore += 20
	} else if volumeRatio >= 1.2 {
		volatilityScore += 10
	}

	if volatilityScore > 100 {
		volatilityScore = 100
	}

	return atrPercent, bbWidth, volumeRatio, volatilityScore
}

// CalculatePullbackScore computes score for Buy the Dip / Pullback Entry setup
// Criteria: Uptrend + Pullback to support/EMA + Bounce signal + HIGH VOLATILITY
func CalculatePullbackScore(prices, highs, lows, volumes, ema20, ema50, rsi, atr, bbUpper, bbLower []float64, features *domain.MarketFeatures) (float64, float64, float64, float64, float64) {
	if len(prices) < 50 || len(ema20) < 50 || len(ema50) < 50 || len(rsi) < 14 {
		return 0, 0, 0, 0, 0
	}

	// Calculate volatility first
	atrPercent, bbWidth, volumeRatio, volatilityScore := CalculateVolatility(prices, highs, lows, volumes, atr, bbUpper, bbLower)

	// VOLATILITY GATE: If volatility too low, reduce attractiveness
	// We want volatile coins for pullback plays
	volatilityMultiplier := 1.0
	if volatilityScore < 30 {
		volatilityMultiplier = 0.5 // Low volatility = less attractive
	} else if volatilityScore < 50 {
		volatilityMultiplier = 0.75
	} else if volatilityScore >= 70 {
		volatilityMultiplier = 1.2 // High volatility bonus
	}

	lastIdx := len(prices) - 1
	currentPrice := prices[lastIdx]
	currentEma20 := ema20[lastIdx]
	currentEma50 := ema50[lastIdx]
	currentRsi := rsi[lastIdx]

	score := 0.0

	// === TREND SCORE (0-25) ===
	// Check if in uptrend: EMA20 > EMA50, price trending up
	trendScore := 0.0

	// EMA alignment (uptrend)
	if currentEma20 > currentEma50 {
		trendScore += 12 // EMAs aligned bullish
	}

	// Price above EMA50 (still in uptrend structure)
	if currentPrice > currentEma50*0.98 { // Allow slight dip below EMA50
		trendScore += 8
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

	if trendScore > 25 {
		trendScore = 25
	}
	score += trendScore

	// === PULLBACK SCORE (0-30) ===
	// Check if price has pulled back but not crashed
	pullbackScore := 0.0

	// RSI in pullback zone (30-45 ideal for dip buying)
	if currentRsi >= 30 && currentRsi <= 40 {
		pullbackScore += 20 // Ideal oversold bounce zone
	} else if currentRsi > 40 && currentRsi <= 50 {
		pullbackScore += 12 // Mild pullback
	} else if currentRsi >= 25 && currentRsi < 30 {
		pullbackScore += 8 // Very oversold, risky but potential
	}

	// Price pulled back to EMA zone (near EMA20 or between EMA20-EMA50)
	distToEma20 := (currentPrice - currentEma20) / currentEma20
	if distToEma20 >= -0.02 && distToEma20 <= 0.01 {
		pullbackScore += 10 // Price touching/near EMA20 (support)
	} else if distToEma20 >= -0.03 && distToEma20 < -0.02 {
		pullbackScore += 5 // Slight dip below EMA20
	}

	if pullbackScore > 25 {
		pullbackScore = 25
	}
	score += pullbackScore

	// === BOUNCE SIGNAL SCORE (0-20) ===
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
			bounceScore += 12
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
			bounceScore += 8
		}
	}

	if bounceScore > 20 {
		bounceScore = 20
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

	// === VOLATILITY BONUS (0-15) ===
	// Add points for high volatility coins
	volBonus := 0.0
	if volatilityScore >= 70 {
		volBonus = 15
	} else if volatilityScore >= 50 {
		volBonus = 10
	} else if volatilityScore >= 35 {
		volBonus = 5
	}
	score += volBonus

	// Apply volatility multiplier to final score
	score = score * volatilityMultiplier

	if score > 100 {
		score = 100
	}

	return score, atrPercent, bbWidth, volumeRatio, volatilityScore
}

// CalculateBreakoutScore computes score for Breakout Hunter setup
// Criteria: Near resistance + Volume spike + Momentum + Breakout confirmation
func CalculateBreakoutScore(prices, highs, lows, volumes, bbUpper, bbLower []float64, features *domain.MarketFeatures) (float64, float64, float64) {
	if len(prices) < 50 || len(highs) < 20 || len(volumes) < 20 {
		return 0, 0, 0
	}

	lastIdx := len(prices) - 1
	currentPrice := prices[lastIdx]
	currentHigh := highs[lastIdx]

	// Find resistance level (recent high in last 50 candles)
	resistance := highs[lastIdx-20]
	for i := lastIdx - 20; i <= lastIdx; i++ {
		if highs[i] > resistance {
			resistance = highs[i]
		}
	}

	// Distance to resistance
	distToResistance := 0.0
	if resistance > 0 {
		distToResistance = ((resistance - currentPrice) / currentPrice) * 100
	}

	score := 0.0

	// === PROXIMITY TO RESISTANCE (0-30) ===
	proximityScore := 0.0
	if distToResistance <= 0 {
		// Already broken resistance!
		proximityScore = 30
	} else if distToResistance <= 0.5 {
		// Very close to resistance
		proximityScore = 25
	} else if distToResistance <= 1.0 {
		// Close to resistance
		proximityScore = 20
	} else if distToResistance <= 2.0 {
		// Approaching resistance
		proximityScore = 15
	} else if distToResistance <= 3.0 {
		// Within reach
		proximityScore = 10
	}
	score += proximityScore

	// === VOLUME SPIKE (0-35) - Critical for breakout confirmation ===
	volumeScore := 0.0
	avgVolume := 0.0
	for i := lastIdx - 19; i < lastIdx; i++ {
		avgVolume += volumes[i]
	}
	avgVolume /= 20

	currentVolume := volumes[lastIdx]
	volumeRatio := 1.0
	if avgVolume > 0 {
		volumeRatio = currentVolume / avgVolume
	}

	// Volume spike is KEY for breakout
	if volumeRatio >= 3.0 {
		volumeScore = 35 // Huge volume spike!
	} else if volumeRatio >= 2.5 {
		volumeScore = 30
	} else if volumeRatio >= 2.0 {
		volumeScore = 25
	} else if volumeRatio >= 1.5 {
		volumeScore = 20
	} else if volumeRatio >= 1.2 {
		volumeScore = 10
	}
	score += volumeScore

	// === MOMENTUM (0-20) ===
	momentumScore := 0.0
	// RSI > 50 = bullish momentum
	if features.RSI >= 60 {
		momentumScore += 15
	} else if features.RSI >= 50 {
		momentumScore += 10
	} else if features.RSI >= 45 {
		momentumScore += 5
	}

	// Price trending up
	if currentPrice > prices[lastIdx-5] {
		momentumScore += 5
	}

	if momentumScore > 20 {
		momentumScore = 20
	}
	score += momentumScore

	// === BREAKOUT CONFIRMATION (0-15) ===
	breakoutScore := 0.0
	// Price above resistance
	if currentPrice > resistance {
		breakoutScore += 10
	}

	// High above resistance (strong breakout)
	if currentHigh > resistance*1.01 {
		breakoutScore += 5
	}

	// Above upper Bollinger Band (breakout strength)
	if len(bbUpper) > lastIdx && currentPrice > bbUpper[lastIdx] {
		// Already counted in proximity
	}

	if breakoutScore > 15 {
		breakoutScore = 15
	}
	score += breakoutScore

	if score > 100 {
		score = 100
	}

	return score, resistance, distToResistance
}
