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

	// Timeframes to check - from fastest to slowest
	timeframes := []string{"1m", "5m", "15m", "1h"}

	for _, sym := range targetSymbols {
		wg.Add(1)
		go func(symbol string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Funding Rate (same for all TFs)
			funding, _ := uc.binanceClient.GetFundingRate(symbol)

			var bestScore float64
			var bestTF string
			var bestFeatures *domain.MarketFeatures
			var bestPrice float64
			var tfScores []domain.TimeframeScore

			// Check each timeframe
			for _, tf := range timeframes {
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

				for i, k := range rawKlines {
					h, _ := parseValue(k[2])
					l, _ := parseValue(k[3])
					c, _ := parseValue(k[4])
					prices[i] = c
					highs[i] = h
					lows[i] = l
				}

				// Calculate Indicators
				ema50 := indicators.CalculateEMA(prices, 50)
				vwap := make([]float64, len(prices)) // placeholder
				rsi := indicators.CalculateRSI(prices, 14)
				atr := indicators.CalculateATR(highs, lows, prices, 14)
				bb := indicators.CalculateBollingerBands(prices, 20, 2.0)
				pivots := indicators.FindPivotLows(lows, 5, 2)

				features := ExtractFeatures(
					prices, highs, lows,
					tickerMap[symbol],
					ema50, vwap, rsi,
					bb, atr, pivots,
					funding, 0,
				)

				if features == nil {
					continue
				}

				scoreResult := CalculateScore(features)
				tfScores = append(tfScores, domain.TimeframeScore{TF: tf, Score: scoreResult})

				// Track the best (highest) score across TFs
				if scoreResult > bestScore {
					bestScore = scoreResult
					bestTF = tf
					bestFeatures = features
					bestPrice = prices[len(prices)-1]
				}
			}

			if bestFeatures == nil {
				return
			}

			coin := domain.CoinData{
				Symbol:             symbol,
				Price:              bestPrice,
				Score:              bestScore,
				Status:             "AVOID",
				TriggerTF:          bestTF,
				TFScores:           tfScores,
				PriceChangePercent: bestFeatures.PctChange24h,
				FundingRate:        funding,
				Features:           bestFeatures,
			}

			// Determine Status based on Score (4 levels)
			// TRIGGER: 70+ (lowered from 75 for earlier detection)
			// SETUP: 50+ (lowered from 55)
			// WATCH: 25+
			// AVOID: <25
			if bestScore >= 70 {
				coin.Status = "TRIGGER"
			} else if bestScore >= 50 {
				coin.Status = "SETUP"
			} else if bestScore >= 25 {
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
