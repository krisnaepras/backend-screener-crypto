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

// CalculateBreakoutScore calculates score for breakout hunter strategy (LONG or SHORT)
// Scoring: Breakout (0-30) + Volume (0-30) + Momentum (0-25) + Structure (0-15) = max 100
func CalculateBreakoutScore(prices []float64, extremes []float64, volumes []float64, ema20 []float64, ema50 []float64, rsi []float64, features *domain.MarketFeatures, direction string) float64 {
	if len(prices) < 20 || len(extremes) < 20 || len(volumes) < 20 {
		return 0
	}

	score := 0.0
	currentPrice := prices[len(prices)-1]
	currentVolume := volumes[len(volumes)-1]
	currentRSI := rsi[len(rsi)-1]

	// === BREAKOUT SCORE (0-30) ===
	// For LONG: Price breaking recent resistance (highs)
	// For SHORT: Price breaking recent support (lows)
	breakoutScore := 0.0

	// Find recent extreme (resistance for LONG, support for SHORT)
	recentExtreme := 0.0
	lookback := 50
	if len(extremes) < 50 {
		lookback = len(extremes)
	}

	if direction == "LONG" {
		// Find highest high (resistance)
		for i := len(extremes) - lookback; i < len(extremes)-1; i++ { // Exclude current candle
			if extremes[i] > recentExtreme {
				recentExtreme = extremes[i]
			}
		}

		// Check if price is breaking above recent high
		if recentExtreme > 0 {
			breakoutPct := ((currentPrice - recentExtreme) / recentExtreme) * 100
			
			if breakoutPct > 1.5 {
				breakoutScore += 30 // Strong breakout (>1.5% above resistance)
			} else if breakoutPct > 0.5 {
				breakoutScore += 25 // Clear breakout
			} else if breakoutPct > 0 {
				breakoutScore += 20 // Just broke through
			} else if breakoutPct > -0.5 {
				breakoutScore += 10 // Testing resistance
			}
		}

		// Price above EMA20 and EMA50 (uptrend structure)
		if len(ema20) > 0 && len(ema50) > 0 {
			ema20Val := ema20[len(ema20)-1]
			ema50Val := ema50[len(ema50)-1]
			
			if currentPrice > ema20Val && currentPrice > ema50Val && ema20Val > ema50Val {
				breakoutScore += 5 // Bullish EMA alignment
			}
		}
	} else if direction == "SHORT" {
		// Find lowest low (support)
		recentExtreme = extremes[len(extremes) - lookback] // Initialize with first value
		for i := len(extremes) - lookback; i < len(extremes)-1; i++ { // Exclude current candle
			if extremes[i] < recentExtreme {
				recentExtreme = extremes[i]
			}
		}

		// Check if price is breaking below recent low
		if recentExtreme > 0 {
			breakdownPct := ((recentExtreme - currentPrice) / recentExtreme) * 100
			
			if breakdownPct > 1.5 {
				breakoutScore += 30 // Strong breakdown (>1.5% below support)
			} else if breakdownPct > 0.5 {
				breakoutScore += 25 // Clear breakdown
			} else if breakdownPct > 0 {
				breakoutScore += 20 // Just broke through
			} else if breakdownPct > -0.5 {
				breakoutScore += 10 // Testing support
			}
		}

		// Price below EMA20 and EMA50 (downtrend structure)
		if len(ema20) > 0 && len(ema50) > 0 {
			ema20Val := ema20[len(ema20)-1]
			ema50Val := ema50[len(ema50)-1]
			
			if currentPrice < ema20Val && currentPrice < ema50Val && ema20Val < ema50Val {
				breakoutScore += 5 // Bearish EMA alignment
			}
		}
	}

	if breakoutScore > 30 {
		breakoutScore = 30
	}
	score += breakoutScore

	// === VOLUME SPIKE SCORE (0-30) ===
	// Strong volume confirms breakout/breakdown validity
	volumeScore := 0.0

	// Calculate average volume (last 20 candles, excluding current)
	avgVolume := 0.0
	volCount := 0
	for i := len(volumes) - 21; i < len(volumes)-1; i++ {
		if i >= 0 {
			avgVolume += volumes[i]
			volCount++
		}
	}
	if volCount > 0 {
		avgVolume /= float64(volCount)
	}

	// Check volume spike
	if avgVolume > 0 {
		volumeRatio := currentVolume / avgVolume
		
		if volumeRatio > 3.0 {
			volumeScore += 30 // Massive volume spike (>3x)
		} else if volumeRatio > 2.5 {
			volumeScore += 25 // Very high volume
		} else if volumeRatio > 2.0 {
			volumeScore += 20 // High volume
		} else if volumeRatio > 1.5 {
			volumeScore += 15 // Good volume
		} else if volumeRatio > 1.2 {
			volumeScore += 10 // Decent volume
		}
	}

	if volumeScore > 30 {
		volumeScore = 30
	}
	score += volumeScore

	// === MOMENTUM SCORE (0-25) ===
	// For LONG: RSI and price momentum confirm bullish breakout
	// For SHORT: RSI and price momentum confirm bearish breakdown
	momentumScore := 0.0

	if direction == "LONG" {
		// RSI in bullish zone (50-75 = ideal, not overbought yet)
		if currentRSI > 65 && currentRSI < 75 {
			momentumScore += 15 // Strong momentum, not overbought
		} else if currentRSI > 55 && currentRSI < 70 {
			momentumScore += 12 // Good momentum
		} else if currentRSI > 50 && currentRSI < 65 {
			momentumScore += 8 // Building momentum
		} else if currentRSI < 50 {
			momentumScore += 0 // Weak momentum (bearish RSI)
		} else if currentRSI > 75 {
			momentumScore += 5 // Too overbought, risky
		}

		// Price action momentum (recent price increase)
		if features.PctChange24h > 5 {
			momentumScore += 10 // Strong upward move
		} else if features.PctChange24h > 2 {
			momentumScore += 7 // Good move
		} else if features.PctChange24h > 0 {
			momentumScore += 3 // Positive move
		}
	} else if direction == "SHORT" {
		// RSI in bearish zone (25-50 = ideal, not oversold yet)
		if currentRSI < 35 && currentRSI > 25 {
			momentumScore += 15 // Strong bearish momentum, not oversold
		} else if currentRSI < 45 && currentRSI > 30 {
			momentumScore += 12 // Good bearish momentum
		} else if currentRSI < 50 && currentRSI > 35 {
			momentumScore += 8 // Building bearish momentum
		} else if currentRSI > 50 {
			momentumScore += 0 // Weak momentum (bullish RSI)
		} else if currentRSI < 25 {
			momentumScore += 5 // Too oversold, risky
		}

		// Price action momentum (recent price decrease)
		if features.PctChange24h < -5 {
			momentumScore += 10 // Strong downward move
		} else if features.PctChange24h < -2 {
			momentumScore += 7 // Good move
		} else if features.PctChange24h < 0 {
			momentumScore += 3 // Negative move
		}
	}

	if momentumScore > 25 {
		momentumScore = 25
	}
	score += momentumScore

	// === STRUCTURE SCORE (0-15) ===
	// Clean breakout/breakdown structure
	structureScore := 0.0

	// Low rejection wick = clean breakout (bulls in control)
	if features.RejectionWickRatio < 0.2 {
		structureScore += 8 // Very clean breakout
	} else if features.RejectionWickRatio < 0.4 {
		structureScore += 5 // Decent breakout
	}

	// Not breaking down (no support break)
	if !features.IsBreakdown {
		structureScore += 4
	}

	// Not overextended yet (room to run)
	if !features.IsAboveUpperBand {
		structureScore += 3
	}

	if structureScore > 15 {
		structureScore = 15
	}
	score += structureScore

	return score
}

// CalculateShortReadinessScore computes 0-100 score for SHORT readiness on top gainers
// Based on exhaustion signals: overextension, volume climax, wick rejection, OI crowding, structure break
// Higher score = more ready for short reversal
func CalculateShortReadinessScore(
	prices, highs, lows, volumes []float64,
	ema20, ema50, rsi []float64,
	features *domain.MarketFeatures,
	ticker binance.Ticker24h,
) float64 {
	if features == nil || len(prices) < 30 {
		return 0
	}

	lastIdx := len(prices) - 1
	currentPrice := prices[lastIdx]
	currentHigh := highs[lastIdx]
	currentLow := lows[lastIdx]
	currentVolume := volumes[lastIdx]
	currentRSI := features.RSI

	score := 0.0

	// === 1. OVEREXTENSION SCORE (0-20) ===
	// Parabolic + too far from EMA/VWAP = exhaustion
	overextScore := 0.0

	// Distance from EMA50 (z-score approach)
	if len(ema50) > lastIdx && ema50[lastIdx] > 0 {
		distFromEma := (currentPrice - ema50[lastIdx]) / ema50[lastIdx]
		if distFromEma > 0.15 { // >15% above EMA50
			overextScore += 10
		} else if distFromEma > 0.10 {
			overextScore += 7
		} else if distFromEma > 0.05 {
			overextScore += 4
		}
	}

	// 24h pump % (parabolic move)
	pctChange, _ := strconvToFloat(ticker.PriceChangePercent)
	if pctChange > 50 {
		overextScore += 10 // Extreme pump
	} else if pctChange > 30 {
		overextScore += 7
	} else if pctChange > 20 {
		overextScore += 4
	}

	if overextScore > 20 {
		overextScore = 20
	}
	score += overextScore

	// === 2. VOLUME CLIMAX SCORE (0-20) ===
	// Volume spike that fails to follow through = distribution
	volumeScore := 0.0

	// Calculate average volume (last 20 candles excluding current)
	if len(volumes) >= 21 {
		sumVol := 0.0
		for i := lastIdx - 20; i < lastIdx; i++ {
			sumVol += volumes[i]
		}
		avgVol := sumVol / 20.0

		// Volume spike detection
		volRatio := currentVolume / avgVol
		if volRatio > 3.0 {
			volumeScore += 12 // Massive volume spike
		} else if volRatio > 2.0 {
			volumeScore += 8
		} else if volRatio > 1.5 {
			volumeScore += 4
		}

		// Check follow-through failure: volume spike but price didn't break previous high convincingly
		if lastIdx >= 1 && volRatio > 1.5 {
			prevHigh := highs[lastIdx-1]
			// Price made new high but couldn't hold it (weak close relative to high)
			closeToHighRatio := (currentHigh - currentPrice) / (currentHigh - currentLow + 0.0001)
			if currentPrice > prevHigh && closeToHighRatio > 0.5 {
				volumeScore += 8 // Failed follow-through after volume spike
			}
		}
	}

	if volumeScore > 20 {
		volumeScore = 20
	}
	score += volumeScore

	// === 3. WICK REJECTION SCORE (0-20) ===
	// Long upper wick at high = rejection = exhaustion
	wickScore := 0.0

	candleRange := currentHigh - currentLow
	if candleRange > 0 {
		upperWick := currentHigh - currentPrice
		wickRatio := upperWick / candleRange

		if wickRatio > 0.6 {
			wickScore += 15 // Very strong rejection
		} else if wickRatio > 0.5 {
			wickScore += 12
		} else if wickRatio > 0.4 {
			wickScore += 8
		} else if wickRatio > 0.3 {
			wickScore += 4
		}

		// Additional: if it's a "shooting star" pattern (small body, long upper wick)
		bodySize := abs(currentPrice - prices[max(0, lastIdx-1)])
		if wickRatio > 0.5 && bodySize/candleRange < 0.3 {
			wickScore += 5 // Shooting star bonus
		}
	}

	if wickScore > 20 {
		wickScore = 20
	}
	score += wickScore

	// === 4. DERIVATIF CROWDING SCORE (0-20) ===
	// High funding + OI increase = crowded long positions = rawan flush
	crowdScore := 0.0

	// Funding rate (relative approach - looking for extreme values)
	// Normal funding: 0.0001-0.0003, High: >0.0005, Extreme: >0.001
	if features.FundingRate > 0.0015 {
		crowdScore += 12 // Extremely high funding
	} else if features.FundingRate > 0.001 {
		crowdScore += 10
	} else if features.FundingRate > 0.0007 {
		crowdScore += 7
	} else if features.FundingRate > 0.0005 {
		crowdScore += 4
	}

	// OI delta (rapid increase = position building)
	if features.OpenInterestDelta > 15 {
		crowdScore += 8 // Rapid OI increase
	} else if features.OpenInterestDelta > 10 {
		crowdScore += 5
	} else if features.OpenInterestDelta > 5 {
		crowdScore += 2
	}

	if crowdScore > 20 {
		crowdScore = 20
	}
	score += crowdScore

	// === 5. STRUCTURE BREAK SCORE (0-20) ===
	// BOS (break of structure) = trigger ready
	structureScore := 0.0

	// RSI divergence (price higher high, RSI lower high)
	if features.HasRsiDivergence {
		structureScore += 8 // Divergence warning
	}

	// Volume divergence
	if features.HasVolumeDivergence {
		structureScore += 4
	}

	// Check for BOS: if price broke below recent higher low
	// Simple check: compare current low with lows from last 10-20 candles
	if lastIdx >= 20 {
		// Find recent swing low
		recentLow := lows[lastIdx-1]
		for i := lastIdx - 10; i < lastIdx; i++ {
			if lows[i] < recentLow {
				recentLow = lows[i]
			}
		}

		// BOS if current price closed below recent swing low
		if currentPrice < recentLow {
			structureScore += 8 // BOS confirmed
		}
	}

	// RSI overbought (not a trigger, but adds to readiness)
	if currentRSI > 75 {
		structureScore += 0 // Already counted in momentum, don't double count
	}

	if structureScore > 20 {
		structureScore = 20
	}
	score += structureScore

	// Final sanity checks
	if score > 100 {
		score = 100
	}

	return score
}

// Helper functions
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
