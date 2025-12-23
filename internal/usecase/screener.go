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
	coreTimeframes := []string{"1m", "5m"}
	intradayTimeframes := []string{"15m", "1h"}

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

			// === INTRADAY STATUS (15m + 1h) ===
			if len(intradayTFScores) >= 2 {
				var intradayTotalScore float64
				intradayConfluence := 0
				var intradayPrimaryFeatures *domain.MarketFeatures

				for _, tf := range intradayTimeframes {
					feat, ok := intradayFeaturesMap[tf]
					if !ok {
						continue
					}

					isOverbought := feat.RSI > 60 || feat.OverExtEma > 0.02 || feat.IsAboveUpperBand
					hasLosingMomentum := feat.IsLosingMomentum || feat.HasRsiDivergence || feat.HasVolumeDivergence
					if isOverbought || hasLosingMomentum {
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
				
				// Intraday confluence multiplier
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

				// Intraday Status: HOT (ready), WARM (preparing), COOL (watching)
				if intradayConfluence >= 2 && coin.IntradayScore >= 45 {
					coin.IntradayStatus = "HOT"
				} else if intradayConfluence >= 1 && coin.IntradayScore >= 35 {
					coin.IntradayStatus = "WARM"
				} else if coin.IntradayScore >= 30 {
					coin.IntradayStatus = "COOL"
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
