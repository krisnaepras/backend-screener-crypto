package usecase

import (
	"log"
	"sort"
	"strconv"
	"sync"
	"time"

	"screener-backend/internal/domain"
	"screener-backend/internal/infrastructure/binance"
	"screener-backend/internal/infrastructure/fcm"
	"screener-backend/internal/infrastructure/indicators"
	"screener-backend/internal/repository"
)

type ScreenerUsecase struct {
	repo          domain.ScreenerRepository
	binanceClient *binance.Client
	fcmClient     *fcm.Client
	tokenRepo     *repository.TokenRepository
	notifiedCoins map[string]time.Time // Track notified coins with timestamp
	mu            sync.RWMutex
}

func NewScreenerUsecase(repo domain.ScreenerRepository, tokenRepo *repository.TokenRepository, fcmClient *fcm.Client, binanceBaseURL string) *ScreenerUsecase {
	return &ScreenerUsecase{
		repo:          repo,
		binanceClient: binance.NewClient(binanceBaseURL),
		fcmClient:     fcmClient,
		tokenRepo:     tokenRepo,
		notifiedCoins: make(map[string]time.Time),
	}
}

// Run starts the screening loop.
func (uc *ScreenerUsecase) Run() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Initial run
	go uc.process()

	for range ticker.C {
		go uc.process()
	}
}

func (uc *ScreenerUsecase) process() {
	start := time.Now()
	log.Println("Starting screening cycle...")

	// 1. Get Active Symbols
	symbols, err := uc.binanceClient.GetActiveTradingSymbols()
	if err != nil {
		log.Printf("Error getting symbols: %v", err)
		return
	}

	// Limit to top volume or something if too many?
	// For now let's take first 50 to avoid rate limits during dev/testing?
	// User asked for "Golang backend", assume performance is goal.
	// But rate limits are real.
	// Let's process in batches or just wait.
	// Dart app processes ALL.
	// Let's try to process a subset for safety in this initial version, or use `GetFutures24hrTicker` to filter top volume first.

	tickers, err := uc.binanceClient.GetFutures24hrTicker()
	if err != nil {
		log.Printf("Error getting tickers: %v", err)
		return
	}

	// Create map for easy access
	tickerMap := make(map[string]binance.Ticker24h)
	for _, t := range tickers {
		tickerMap[t.Symbol] = t
	}

	var computedCoins []domain.CoinData
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	sem := make(chan struct{}, 10) // Semaphore to limit concurrency

	// Filter symbols to those present in tickerMap (Futures)
	var targetSymbols []string
	for _, s := range symbols {
		if _, ok := tickerMap[s]; ok {
			targetSymbols = append(targetSymbols, s)
		}
	}

	// Let's pick top 20 by volume for the demo/safety first? 
	// Or just do all but slowly?
	// 10 concurrent requests * 100+ symbols might trigger limit. 2400 req/min is weight.
	// Klines weight is 2 usually.
	
	log.Printf("Found %d active symbols", len(targetSymbols))

	// Core timeframes for scalping: 1m + 5m
	// Intraday timeframes: 15m + 1h
	// Pullback setup: 5m + 15m (trend), 1m + 3m (execution)
	// Breakout: 15m + 1h (for solid breakouts)
	coreTimeframes := []string{"1m", "5m"}
	intradayTimeframes := []string{"15m", "1h"}
	pullbackSetupTFs := []string{"5m", "15m"}
	pullbackExecTFs := []string{"1m", "3m"}
	breakoutTimeframes := []string{"15m", "1h"}

	for _, sym := range targetSymbols {
		wg.Add(1)
		go func(symbol string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Funding Rate (same for all TFs)
			funding, _ := uc.binanceClient.GetFundingRate(symbol)

			var tfScores []domain.TimeframeScore
			var tfFeatures []domain.TimeframeFeatures
			var featuresMap = make(map[string]*domain.MarketFeatures)
			var pricesMap = make(map[string]float64)

			// === SCALPING ANALYSIS (1m + 5m) ===
			for _, tf := range coreTimeframes {
				rawKlines, err := uc.binanceClient.GetKlines(symbol, tf, 100)
				if err != nil {
					continue
				}
				if len(rawKlines) < 50 {
					continue
				}

				// Parse Klines to float slices
				prices := make([]float64, len(rawKlines))
				highs := make([]float64, len(rawKlines))
				lows := make([]float64, len(rawKlines))
				volumes := make([]float64, len(rawKlines))

				for i, k := range rawKlines {
					h, _ := parseValue(k[2])
					l, _ := parseValue(k[3])
					c, _ := parseValue(k[4])
					v, _ := parseValue(k[5]) // Volume is at index 5
					prices[i] = c
					highs[i] = h
					lows[i] = l
					volumes[i] = v
				}

				// Calculate Indicators
				ema50 := indicators.CalculateEMA(prices, 50)
				vwap := make([]float64, len(prices)) // placeholder
				rsi := indicators.CalculateRSI(prices, 14)
				atr := indicators.CalculateATR(highs, lows, prices, 14)
				bb := indicators.CalculateBollingerBands(prices, 20, 2.0)
				pivots := indicators.FindPivotLows(lows, 5, 2)

				features := ExtractFeatures(
					prices, highs, lows, volumes,
					tickerMap[symbol],
					ema50, vwap, rsi,
					bb, atr, pivots,
					funding, 0,
				)

				if features == nil {
					continue
				}

				scoreResult := CalculateScore(features)
				tfScores = append(tfScores, domain.TimeframeScore{
					TF:    tf,
					Score: scoreResult,
					RSI:   features.RSI,
				})
				tfFeatures = append(tfFeatures, domain.TimeframeFeatures{
					TF:             tf,
					RSI:            features.RSI,
					OverExtEma:     features.OverExtEma,
					IsAboveUpperBB: features.IsAboveUpperBand,
					IsBreakdown:    features.IsBreakdown,
				})
				featuresMap[tf] = features
				pricesMap[tf] = prices[len(prices)-1]
			}

			// === INTRADAY ANALYSIS (15m + 1h) ===
			var intradayTFScores []domain.TimeframeScore
			var intradayFeaturesMap = make(map[string]*domain.MarketFeatures)

			for _, tf := range intradayTimeframes {
				rawKlines, err := uc.binanceClient.GetKlines(symbol, tf, 100)
				if err != nil {
					continue
				}
				if len(rawKlines) < 50 {
					continue
				}

				prices := make([]float64, len(rawKlines))
				highs := make([]float64, len(rawKlines))
				lows := make([]float64, len(rawKlines))
				volumes := make([]float64, len(rawKlines))

				for i, k := range rawKlines {
					h, _ := parseValue(k[2])
					l, _ := parseValue(k[3])
					c, _ := parseValue(k[4])
					v, _ := parseValue(k[5])
					prices[i] = c
					highs[i] = h
					lows[i] = l
					volumes[i] = v
				}

				ema50 := indicators.CalculateEMA(prices, 50)
				vwap := make([]float64, len(prices))
				rsi := indicators.CalculateRSI(prices, 14)
				atr := indicators.CalculateATR(highs, lows, prices, 14)
				bb := indicators.CalculateBollingerBands(prices, 20, 2.0)
				pivots := indicators.FindPivotLows(lows, 5, 2)

				features := ExtractFeatures(
					prices, highs, lows, volumes,
					tickerMap[symbol],
					ema50, vwap, rsi,
					bb, atr, pivots,
					funding, 0,
				)

				if features == nil {
					continue
				}

				scoreResult := CalculateScore(features)
				intradayTFScores = append(intradayTFScores, domain.TimeframeScore{
					TF:    tf,
					Score: scoreResult,
					RSI:   features.RSI,
				})
				intradayFeaturesMap[tf] = features
			}

			// Need at least 2 TFs to evaluate scalping
			if len(tfScores) < 2 {
				return
			}

			// === MULTI-TF CONFLUENCE SCORING ===
			// Count how many TFs are showing overbought signals
			confluenceCount := 0
			var totalScore float64
			var primaryTF string
			var primaryFeatures *domain.MarketFeatures
			var currentPrice float64

			for _, tf := range coreTimeframes {
				feat, ok := featuresMap[tf]
				if !ok {
					continue
				}

				// A TF is "aligned" if it shows overbought signals OR losing momentum
				// Added momentum loss signals for better reversal detection
				isOverbought := feat.RSI > 60 || feat.OverExtEma > 0.02 || feat.IsAboveUpperBand
				hasLosingMomentum := feat.IsLosingMomentum || feat.HasRsiDivergence || feat.HasVolumeDivergence
				isAligned := isOverbought || hasLosingMomentum
				if isAligned {
					confluenceCount++
				}

				// Find highest scoring TF as primary
				for _, ts := range tfScores {
					if ts.TF == tf {
						totalScore += ts.Score
						if primaryFeatures == nil || ts.Score > CalculateScore(primaryFeatures) {
							primaryTF = tf
							primaryFeatures = feat
							currentPrice = pricesMap[tf]
						}
					}
				}
			}

			if primaryFeatures == nil {
				return
			}

			// === CONFLUENCE BONUS ===
			// Base score is average of all TFs
			avgScore := totalScore / float64(len(tfScores))

			// Confluence multiplier for 1m + 5m:
			// 2 TFs aligned: x1.3 (TRIGGERED - ready for entry!)
			// 1 TF aligned: x1.1 (WATCH)
			// 0 TFs aligned: x1.0 (AVOID)
			var confluenceMultiplier float64
			switch confluenceCount {
			case 2:
				confluenceMultiplier = 1.3
			case 1:
				confluenceMultiplier = 1.1
			default:
				confluenceMultiplier = 1.0
			}

			finalScore := avgScore * confluenceMultiplier
			if finalScore > 100 {
				finalScore = 100
			}

			coin := domain.CoinData{
				Symbol:             symbol,
				Price:              currentPrice,
				Score:              finalScore,
				Status:             "",
				TriggerTF:          primaryTF,
				ConfluenceCount:    confluenceCount,
				TFScores:           tfScores,
				TFFeatures:         tfFeatures,
				PriceChangePercent: primaryFeatures.PctChange24h,
				FundingRate:        funding,
				Features:           primaryFeatures,
				IntradayTFScores:   intradayTFScores,
			}

			// === INTRADAY STATUS (15m + 1h) - SHORT ONLY ===
			// Fokus mencari setup SHORT/SELL berdasarkan exhaustion signals
			// Score 0-100: <50 = strong buy (jangan short), 50-70 = waspada, >70 = ready to short
			if len(intradayTFScores) >= 2 {
				var intradayPrimaryFeatures *domain.MarketFeatures
				var primary15mFeatures *domain.MarketFeatures
				var primary1hFeatures *domain.MarketFeatures

				// Get features for both timeframes
				feat15m, ok15m := intradayFeaturesMap["15m"]
				feat1h, ok1h := intradayFeaturesMap["1h"]

				if ok15m {
					primary15mFeatures = feat15m
				}
				if ok1h {
					primary1hFeatures = feat1h
				}

				// Use 15m as primary (faster reaction)
				if primary15mFeatures != nil {
					intradayPrimaryFeatures = primary15mFeatures
				} else if primary1hFeatures != nil {
					intradayPrimaryFeatures = primary1hFeatures
				}

				if intradayPrimaryFeatures != nil {
					// Get klines for detailed analysis
					rawKlines15m, err15m := uc.binanceClient.GetKlines(symbol, "15m", 100)
					
					if err15m == nil && len(rawKlines15m) >= 30 {
						// Parse klines
						prices := make([]float64, len(rawKlines15m))
						highs := make([]float64, len(rawKlines15m))
						lows := make([]float64, len(rawKlines15m))
						volumes := make([]float64, len(rawKlines15m))

						for i, k := range rawKlines15m {
							h, _ := parseValue(k[2])
							l, _ := parseValue(k[3])
							c, _ := parseValue(k[4])
							v, _ := parseValue(k[5])
							prices[i] = c
							highs[i] = h
							lows[i] = l
							volumes[i] = v
						}

						// Calculate EMAs and RSI for short readiness
						ema20 := indicators.CalculateEMA(prices, 20)
						ema50 := indicators.CalculateEMA(prices, 50)
						rsi := indicators.CalculateRSI(prices, 14)

						// Calculate SHORT READINESS SCORE (0-100)
						shortReadinessScore := CalculateShortReadinessScore(
							prices, highs, lows, volumes,
							ema20, ema50, rsi,
							intradayPrimaryFeatures,
							tickerMap[symbol],
						)

						coin.IntradayScore = shortReadinessScore

						// Determine status based on score and conditions
						// <50 = STRONG_BUY (jangan short!)
						// 50-70 = WATCH (waspada, cari trigger)
						// >70 + BOS = READY (candidate short dengan trigger)
						// >70 + BOS + volume spike = HOT (execute short!)

						if shortReadinessScore < 50 {
							// Strong buy territory - DON'T SHORT
							// Check if truly strong or just no exhaustion yet
							hasStrongBuySignal := false
							
							// Struktur masih sehat: higher highs, EMA alignment
							if len(prices) >= 20 && len(ema20) >= 20 && len(ema50) >= 20 {
								lastIdx := len(prices) - 1
								// Check higher high pattern
								recentHigh := highs[lastIdx]
								prevHigh := highs[lastIdx-10]
								if recentHigh > prevHigh && prices[lastIdx] > ema20[lastIdx] && ema20[lastIdx] > ema50[lastIdx] {
									hasStrongBuySignal = true
								}
							}

							if hasStrongBuySignal {
								coin.IntradayStatus = "STRONG_BUY" // Roket masih punya bahan bakar
							} else {
								coin.IntradayStatus = "" // Neutral, belum ada sinyal
							}

						} else if shortReadinessScore >= 70 {
							// Exhaustion zone - look for trigger
							
							// Check for BOS (Break of Structure)
							hasBOS := false
							if len(prices) >= 20 && len(lows) >= 20 {
								lastIdx := len(prices) - 1
								currentPrice := prices[lastIdx]
								
								// Find recent swing low (last 10-15 candles)
								recentLow := lows[lastIdx-1]
								for i := lastIdx - 15; i < lastIdx; i++ {
									if i >= 0 && lows[i] < recentLow {
										recentLow = lows[i]
									}
								}
								
								// BOS if price broke below recent higher low
								if currentPrice < recentLow {
									hasBOS = true
								}
							}

							// Check for volume spike (climax)
							hasVolumeSpike := false
							if len(volumes) >= 21 {
								lastIdx := len(volumes) - 1
								currentVolume := volumes[lastIdx]
								
								sumVol := 0.0
								for i := lastIdx - 20; i < lastIdx; i++ {
									sumVol += volumes[i]
								}
								avgVol := sumVol / 20.0
								
								if currentVolume / avgVol > 2.0 {
									hasVolumeSpike = true
								}
							}

							// Assign status
							if hasBOS && hasVolumeSpike {
								coin.IntradayStatus = "HOT" // Execute short now!
							} else if hasBOS {
								coin.IntradayStatus = "READY" // BOS confirmed, watch for entry
							} else {
								coin.IntradayStatus = "WATCH" // Exhausted but no trigger yet
							}

						} else {
							// 50-70 range: Waspada zone
							coin.IntradayStatus = "WATCH" // Monitor closely
						}
					}
				}
			}

			// === PULLBACK ENTRY (Buy the Dip) ===
			// Setup di 5m/15m (trend confirmation), eksekusi di 1m/3m (entry timing)
			// Criteria: Uptrend + Pullback to support/EMA + Bounce signal
			var pullbackTFScores []domain.TimeframeScore
			var pullbackFeaturesMap = make(map[string]*domain.MarketFeatures)

			// Analyze setup timeframes (5m, 15m) for trend
			for _, tf := range pullbackSetupTFs {
				rawKlines, err := uc.binanceClient.GetKlines(symbol, tf, 100)
				if err != nil || len(rawKlines) < 50 {
					continue
				}

				prices := make([]float64, len(rawKlines))
				highs := make([]float64, len(rawKlines))
				lows := make([]float64, len(rawKlines))
				volumes := make([]float64, len(rawKlines))

				for i, k := range rawKlines {
					h, _ := parseValue(k[2])
					l, _ := parseValue(k[3])
					c, _ := parseValue(k[4])
					v, _ := parseValue(k[5])
					prices[i] = c
					highs[i] = h
					lows[i] = l
					volumes[i] = v
				}

				ema20 := indicators.CalculateEMA(prices, 20)
				ema50 := indicators.CalculateEMA(prices, 50)
				rsi := indicators.CalculateRSI(prices, 14)
				atr := indicators.CalculateATR(highs, lows, prices, 14)
				bb := indicators.CalculateBollingerBands(prices, 20, 2.0)
				pivots := indicators.FindPivotLows(lows, 5, 2)

				features := ExtractFeatures(
					prices, highs, lows, volumes,
					tickerMap[symbol],
					ema50, make([]float64, len(prices)), rsi,
					bb, atr, pivots,
					funding, 0,
				)

				if features == nil {
					continue
				}

				// Calculate pullback score (different criteria)
				pullbackScore := CalculatePullbackScore(prices, ema20, ema50, rsi, features)
				pullbackTFScores = append(pullbackTFScores, domain.TimeframeScore{
					TF:    tf,
					Score: pullbackScore,
					RSI:   features.RSI,
				})
				pullbackFeaturesMap[tf] = features
			}

			// Analyze execution timeframes (1m, 3m) for entry timing
			for _, tf := range pullbackExecTFs {
				rawKlines, err := uc.binanceClient.GetKlines(symbol, tf, 100)
				if err != nil || len(rawKlines) < 50 {
					continue
				}

				prices := make([]float64, len(rawKlines))
				highs := make([]float64, len(rawKlines))
				lows := make([]float64, len(rawKlines))
				volumes := make([]float64, len(rawKlines))

				for i, k := range rawKlines {
					h, _ := parseValue(k[2])
					l, _ := parseValue(k[3])
					c, _ := parseValue(k[4])
					v, _ := parseValue(k[5])
					prices[i] = c
					highs[i] = h
					lows[i] = l
					volumes[i] = v
				}

				ema20 := indicators.CalculateEMA(prices, 20)
				ema50 := indicators.CalculateEMA(prices, 50)
				rsi := indicators.CalculateRSI(prices, 14)
				atr := indicators.CalculateATR(highs, lows, prices, 14)
				bb := indicators.CalculateBollingerBands(prices, 20, 2.0)
				pivots := indicators.FindPivotLows(lows, 5, 2)

				features := ExtractFeatures(
					prices, highs, lows, volumes,
					tickerMap[symbol],
					ema50, make([]float64, len(prices)), rsi,
					bb, atr, pivots,
					funding, 0,
				)

				if features == nil {
					continue
				}

				pullbackScore := CalculatePullbackScore(prices, ema20, ema50, rsi, features)
				pullbackTFScores = append(pullbackTFScores, domain.TimeframeScore{
					TF:    tf,
					Score: pullbackScore,
					RSI:   features.RSI,
				})
				pullbackFeaturesMap[tf] = features
			}

			// Evaluate Pullback Setup
			if len(pullbackTFScores) >= 2 {
				var pullbackTotalScore float64
				pullbackConfluence := 0
				var pullbackPrimaryFeatures *domain.MarketFeatures

				// Check setup TFs (5m, 15m) for uptrend confirmation
				setupInUptrend := 0
				for _, tf := range pullbackSetupTFs {
					feat, ok := pullbackFeaturesMap[tf]
					if !ok {
						continue
					}
					// Uptrend: price above EMA, positive 24h change, RSI not extremely low
					isUptrend := feat.OverExtEma > -0.02 && feat.PctChange24h > -2
					isPullback := feat.RSI < 45 && feat.RSI > 20 // RSI pulled back but not crashed
					
					if isUptrend && isPullback {
						setupInUptrend++
					}
				}

				// Check execution TFs (1m, 3m) for bounce/reversal signal
				hasEntrySignal := false
				for _, tf := range pullbackExecTFs {
					feat, ok := pullbackFeaturesMap[tf]
					if !ok {
						continue
					}
					// Entry signal: RSI bouncing from oversold, near support
					isBouncing := feat.RSI > 30 && feat.RSI < 50 // Coming out of oversold
					nearSupport := feat.DistToSupportATR != nil && *feat.DistToSupportATR < 1.5
					hasReversal := !feat.IsBreakdown && feat.RejectionWickRatio < 0.3 // No strong rejection

					if isBouncing || nearSupport || hasReversal {
						hasEntrySignal = true
						pullbackConfluence++
					}
					
					if pullbackPrimaryFeatures == nil {
						pullbackPrimaryFeatures = feat
					}
				}

				for _, ts := range pullbackTFScores {
					pullbackTotalScore += ts.Score
				}

				pullbackAvgScore := pullbackTotalScore / float64(len(pullbackTFScores))

				// Multiplier based on setup quality
				var pullbackMultiplier float64
				if setupInUptrend >= 2 && hasEntrySignal {
					pullbackMultiplier = 1.4 // Strong setup
					pullbackConfluence = 2
				} else if setupInUptrend >= 1 && hasEntrySignal {
					pullbackMultiplier = 1.2 // Decent setup
					pullbackConfluence = 1
				} else {
					pullbackMultiplier = 1.0
				}

				coin.PullbackScore = pullbackAvgScore * pullbackMultiplier
				if coin.PullbackScore > 100 {
					coin.PullbackScore = 100
				}
				coin.PullbackTFScores = pullbackTFScores
				coin.PullbackFeatures = pullbackPrimaryFeatures

				// Pullback Status: DIP (ready to buy), BOUNCE (confirming), WAIT (watching)
				if pullbackPrimaryFeatures != nil && setupInUptrend >= 1 {
					if pullbackConfluence >= 2 && coin.PullbackScore >= 45 {
						coin.PullbackStatus = "DIP" // Ready to buy the dip!
					} else if pullbackConfluence >= 1 && coin.PullbackScore >= 35 {
						coin.PullbackStatus = "BOUNCE" // Bounce starting
					} else if coin.PullbackScore >= 30 {
						coin.PullbackStatus = "WAIT" // Waiting for confirmation
					}
				}
			}

			// === BREAKOUT HUNTER (15m + 1h) with Volume Spike ===
			// Detect both LONG (resistance breakout) and SHORT (support breakdown)
			var breakoutTFScores []domain.TimeframeScore
			var breakoutFeaturesMap = make(map[string]*domain.MarketFeatures)
			var breakoutLowsMap = make(map[string][]float64) // For support levels

			for _, tf := range breakoutTimeframes {
				rawKlines, err := uc.binanceClient.GetKlines(symbol, tf, 100)
				if err != nil || len(rawKlines) < 50 {
					continue
				}

				prices := make([]float64, len(rawKlines))
				highs := make([]float64, len(rawKlines))
				lows := make([]float64, len(rawKlines))
				volumes := make([]float64, len(rawKlines))

				for i, k := range rawKlines {
					h, _ := parseValue(k[2])
					l, _ := parseValue(k[3])
					c, _ := parseValue(k[4])
					v, _ := parseValue(k[5])
					prices[i] = c
					highs[i] = h
					lows[i] = l
					volumes[i] = v
				}

				ema20 := indicators.CalculateEMA(prices, 20)
				ema50 := indicators.CalculateEMA(prices, 50)
				rsi := indicators.CalculateRSI(prices, 14)
				atr := indicators.CalculateATR(highs, lows, prices, 14)
				bb := indicators.CalculateBollingerBands(prices, 20, 2.0)
				pivotsLow := indicators.FindPivotLows(lows, 5, 2) // For support

				features := ExtractFeatures(
					prices, highs, lows, volumes,
					tickerMap[symbol],
					ema50, make([]float64, len(prices)), rsi,
					bb, atr, pivotsLow,
					funding, 0,
				)

				if features == nil {
					continue
				}

				// Calculate breakout/breakdown score
				breakoutScoreLong := CalculateBreakoutScore(prices, highs, volumes, ema20, ema50, rsi, features, "LONG")
				breakoutScoreShort := CalculateBreakoutScore(prices, lows, volumes, ema20, ema50, rsi, features, "SHORT")
				
				// Use the higher score
				breakoutScore := breakoutScoreLong
				if breakoutScoreShort > breakoutScoreLong {
					breakoutScore = breakoutScoreShort
				}

				breakoutTFScores = append(breakoutTFScores, domain.TimeframeScore{
					TF:    tf,
					Score: breakoutScore,
					RSI:   features.RSI,
				})
				breakoutFeaturesMap[tf] = features
				breakoutLowsMap[tf] = lows
			}

			// Evaluate Breakout Setup (LONG and SHORT)
			if len(breakoutTFScores) >= 2 {
				var breakoutTotalScore float64
				breakoutConfluence := 0
				var breakoutPrimaryFeatures *domain.MarketFeatures

				// Check both TFs for LONG breakout signals
				confirmedBreakoutsLong := 0
				testingBreakoutsLong := 0

				// Check both TFs for SHORT breakdown signals  
				confirmedBreakoutsShort := 0
				testingBreakoutsShort := 0

				for _, tf := range breakoutTimeframes {
					feat, ok := breakoutFeaturesMap[tf]
					if !ok {
						continue
					}

					// === LONG Breakout Criteria ===
					// 1. Price breaking recent highs
					// 2. Volume spike (>1.5x average)
					// 3. RSI > 50 (bullish momentum)
					// 4. Price above EMA20
					
					isBreakingOutLong := feat.OverExtEma > 0.01 && !feat.IsAboveUpperBand // Above EMA but not overextended
					hasVolumeLong := feat.VolumeDeclineRatio < -0.3                        // Volume increasing
					hasMomentumLong := feat.RSI > 50 && feat.RSI < 75                      // Strong but not overbought
					
					if isBreakingOutLong && hasVolumeLong && hasMomentumLong {
						confirmedBreakoutsLong++
					} else if (isBreakingOutLong && hasVolumeLong) || (isBreakingOutLong && hasMomentumLong) {
						testingBreakoutsLong++
					}

					// === SHORT Breakdown Criteria ===
					// 1. Price breaking recent lows (support)
					// 2. Volume spike (>1.5x average)
					// 3. RSI < 50 (bearish momentum)
					// 4. Price below EMA20
					
					isBreakingDownShort := feat.OverExtEma < -0.01 // Below EMA
					hasVolumeShort := feat.VolumeDeclineRatio < -0.3 // Volume increasing
					hasMomentumShort := feat.RSI < 50 && feat.RSI > 25 // Bearish but not oversold yet
					
					if isBreakingDownShort && hasVolumeShort && hasMomentumShort {
						confirmedBreakoutsShort++
					} else if (isBreakingDownShort && hasVolumeShort) || (isBreakingDownShort && hasMomentumShort) {
						testingBreakoutsShort++
					}

					if breakoutPrimaryFeatures == nil {
						breakoutPrimaryFeatures = feat
					}
				}

				// Determine direction and status
				var breakoutDirection string
				var confirmedBreakouts int
				var testingBreakouts int

				// Prioritize the stronger signal
				if confirmedBreakoutsLong >= confirmedBreakoutsShort && (confirmedBreakoutsLong > 0 || testingBreakoutsLong > testingBreakoutsShort) {
					breakoutDirection = "LONG"
					confirmedBreakouts = confirmedBreakoutsLong
					testingBreakouts = testingBreakoutsLong
					breakoutConfluence = confirmedBreakoutsLong
					if testingBreakoutsLong > 0 && breakoutConfluence == 0 {
						breakoutConfluence = 1
					}
				} else if confirmedBreakoutsShort > 0 || testingBreakoutsShort > 0 {
					breakoutDirection = "SHORT"
					confirmedBreakouts = confirmedBreakoutsShort
					testingBreakouts = testingBreakoutsShort
					breakoutConfluence = confirmedBreakoutsShort
					if testingBreakoutsShort > 0 && breakoutConfluence == 0 {
						breakoutConfluence = 1
					}
				}

				for _, ts := range breakoutTFScores {
					breakoutTotalScore += ts.Score
				}

				breakoutAvgScore := breakoutTotalScore / float64(len(breakoutTFScores))

				// Multiplier based on confirmation
				var breakoutMultiplier float64
				if confirmedBreakouts >= 2 {
					breakoutMultiplier = 1.5 // Strong breakout confirmed on both TFs
				} else if confirmedBreakouts >= 1 || testingBreakouts >= 2 {
					breakoutMultiplier = 1.2 // Decent breakout
				} else {
					breakoutMultiplier = 1.0
				}

				coin.BreakoutScore = breakoutAvgScore * breakoutMultiplier
				if coin.BreakoutScore > 100 {
					coin.BreakoutScore = 100
				}
				coin.BreakoutTFScores = breakoutTFScores
				coin.BreakoutFeatures = breakoutPrimaryFeatures
				coin.BreakoutDirection = breakoutDirection

				// Breakout Status with direction
				if breakoutPrimaryFeatures != nil && breakoutDirection != "" {
					if confirmedBreakouts >= 2 && coin.BreakoutScore >= 50 {
						coin.BreakoutStatus = "BREAKOUT_" + breakoutDirection // "BREAKOUT_LONG" or "BREAKOUT_SHORT"
					} else if confirmedBreakouts >= 1 && coin.BreakoutScore >= 40 {
						coin.BreakoutStatus = "TESTING_" + breakoutDirection // "TESTING_LONG" or "TESTING_SHORT"
					} else if testingBreakouts >= 1 && coin.BreakoutScore >= 30 {
						coin.BreakoutStatus = "WAIT_" + breakoutDirection // "WAIT_LONG" or "WAIT_SHORT"
					}
				}
			}

			// Determine Status based on 1m + 5m confluence
			// TRIGGER: both 1m and 5m aligned (confluence = 2) - ready for entry!
			// SETUP: 1 TF aligned with decent score - preparing
			// WATCH: decent score but weak alignment
			// (no status = not displayed)
			if confluenceCount >= 2 && finalScore >= 40 {
				coin.Status = "TRIGGER"
			} else if confluenceCount >= 1 && finalScore >= 35 {
				coin.Status = "SETUP"
			} else if finalScore >= 30 {
				coin.Status = "WATCH"
			}

			mu.Lock()
			computedCoins = append(computedCoins, coin)
			mu.Unlock()

		}(sym)
	}

	wg.Wait()
	
	// Sort coins by score (highest first)
	sort.Slice(computedCoins, func(i, j int) bool {
		return computedCoins[i].Score > computedCoins[j].Score
	})
	
	uc.repo.SaveCoins(computedCoins)
	
	// Send FCM notifications for TRIGGER coins
	uc.sendNotificationsForTriggers(computedCoins)
	
	// Send FCM notifications for BREAKOUT coins
	uc.sendNotificationsForBreakouts(computedCoins)
	
	log.Printf("Cycle completed in %v. Processed %d coins.", time.Since(start), len(computedCoins))
}

func parseValue(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	}
	return 0, nil
}
