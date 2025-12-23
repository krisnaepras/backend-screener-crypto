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
	coreTimeframes := []string{"1m", "5m"}
	intradayTimeframes := []string{"15m", "1h"}
	pullbackSetupTFs := []string{"5m", "15m"}
	pullbackExecTFs := []string{"1m", "3m"}

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
				// Initialize volatility fields with defaults
				VolatilityScore:    0,
				AtrPercent:         0,
				BbWidth:            0,
				VolumeRatio:        0,
			}

			// === INTRADAY STATUS (15m + 1h) - SHORT ONLY ===
			// Fokus mencari setup SHORT/SELL berdasarkan kondisi overbought
			if len(intradayTFScores) >= 2 {
				var intradayTotalScore float64
				intradayConfluence := 0
				var intradayPrimaryFeatures *domain.MarketFeatures

				for _, tf := range intradayTimeframes {
					feat, ok := intradayFeaturesMap[tf]
					if !ok {
						continue
					}

					// SHORT CRITERIA: Overbought conditions for selling
					// RSI > 65 (overbought), Price extended above EMA, Above upper BB
					isOverbought := feat.RSI > 65 || feat.OverExtEma > 0.025 || feat.IsAboveUpperBand
					hasLosingMomentum := feat.IsLosingMomentum || feat.HasRsiDivergence || feat.HasVolumeDivergence
					hasBearishSignal := feat.IsRsiBearishDiv || feat.RejectionWickRatio > 0.5
					
					// Count as aligned if showing bearish/overbought signals
					if isOverbought || hasLosingMomentum || hasBearishSignal {
						intradayConfluence++
					}

					for _, ts := range intradayTFScores {
						if ts.TF == tf {
							intradayTotalScore += ts.Score
							if intradayPrimaryFeatures == nil || ts.Score > CalculateScore(intradayPrimaryFeatures) {
								intradayPrimaryFeatures = feat
							}
						}
					}
				}

				intradayAvgScore := intradayTotalScore / float64(len(intradayTFScores))
				
				// Intraday confluence multiplier for SHORT
				var intradayMultiplier float64
				switch intradayConfluence {
				case 2:
					intradayMultiplier = 1.3
				case 1:
					intradayMultiplier = 1.15
				default:
					intradayMultiplier = 1.0
				}

				coin.IntradayScore = intradayAvgScore * intradayMultiplier
				if coin.IntradayScore > 100 {
					coin.IntradayScore = 100
				}
				coin.IntradayFeatures = intradayPrimaryFeatures

				// Intraday Status for SHORT: HOT (ready to short), WARM (preparing), COOL (watching)
				// Only assign status if there's overbought signal (good for shorting)
				if intradayPrimaryFeatures != nil {
					hasShortSignal := intradayPrimaryFeatures.RSI > 60 || 
						intradayPrimaryFeatures.IsAboveUpperBand || 
						intradayPrimaryFeatures.IsLosingMomentum ||
						intradayPrimaryFeatures.HasRsiDivergence
					
					if hasShortSignal {
						if intradayConfluence >= 2 && coin.IntradayScore >= 45 {
							coin.IntradayStatus = "HOT"
						} else if intradayConfluence >= 1 && coin.IntradayScore >= 35 {
							coin.IntradayStatus = "WARM"
						} else if coin.IntradayScore >= 30 {
							coin.IntradayStatus = "COOL"
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

				// Calculate pullback score with volatility (different criteria)
				pullbackScore, atrPct, bbW, volR, volScore := CalculatePullbackScore(prices, highs, lows, volumes, ema20, ema50, rsi, atr, bb.Upper, bb.Lower, features)
				
				// Store volatility metrics (use first TF as reference)
				if coin.AtrPercent == 0 {
					coin.AtrPercent = atrPct
					coin.BbWidth = bbW
					coin.VolumeRatio = volR
					coin.VolatilityScore = volScore
				}
				
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

				pullbackScore, atrPct, bbW, volR, volScore := CalculatePullbackScore(prices, highs, lows, volumes, ema20, ema50, rsi, atr, bb.Upper, bb.Lower, features)
				
				// Update volatility if higher than setup TF
				if volScore > coin.VolatilityScore {
					coin.AtrPercent = atrPct
					coin.BbWidth = bbW
					coin.VolumeRatio = volR
					coin.VolatilityScore = volScore
				}
				
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
