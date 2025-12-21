package usecase

import (
	"fmt"
	"log"
	"math"
	"screener-backend/internal/domain"
	"time"
)

// AutoScalpingService manages automatic scalping trades
type AutoScalpingService struct {
	repo          domain.AutoScalpRepository
	screeningRepo domain.ScreenerRepository
	settings      *domain.AutoScalpSettings
	priceCache    map[string]float64 // symbol -> current price
}

// NewAutoScalpingService creates a new auto scalping service
func NewAutoScalpingService(
	repo domain.AutoScalpRepository,
	screeningRepo domain.ScreenerRepository,
) *AutoScalpingService {
	return &AutoScalpingService{
		repo:          repo,
		screeningRepo: screeningRepo,
		priceCache:    make(map[string]float64),
		settings: &domain.AutoScalpSettings{
			Enabled:              false, // Start disabled
			MaxConcurrentTrades:  3,
			MinEntryScore:        75,    // Only TRIGGER level
			StopLossPercent:      0.4,   // 0.4% SL - tight but reasonable
			MinProfitPercent:     0.3,   // Start trailing at 0.3% profit
			TrailingStopPercent:  0.15,  // Trail by 0.15% from peak
			MaxPositionTime:      1800,  // 30 minutes max
		},
	}
}

// GetSettings returns current settings
func (s *AutoScalpingService) GetSettings() *domain.AutoScalpSettings {
	return s.settings
}

// UpdateSettings updates auto scalping settings
func (s *AutoScalpingService) UpdateSettings(settings *domain.AutoScalpSettings) {
	s.settings = settings
}

// MonitorAndExecute checks for entry/exit opportunities (called periodically)
func (s *AutoScalpingService) MonitorAndExecute() {
	if !s.settings.Enabled {
		return
	}

	// Update price cache
	s.updatePriceCache()

	// Check for exits on active trades
	s.checkExits()

	// Check for new entries
	s.checkEntries()
}

func (s *AutoScalpingService) updatePriceCache() {
	coins := s.screeningRepo.GetCoins()
	for _, coin := range coins {
		s.priceCache[coin.Symbol] = coin.Price
	}
}

func (s *AutoScalpingService) checkExits() {
	activeEntries := s.repo.GetActiveEntries()
	
	for _, entry := range activeEntries {
		currentPrice, exists := s.priceCache[entry.Symbol]
		if !exists {
			continue
		}

		// Update highest price since entry
		if currentPrice < entry.HighestPrice {
			// For SHORT, lower price = higher profit, so we track LOWEST as "highest profit"
			entry.HighestPrice = currentPrice
		}

		shouldExit, reason := s.shouldExit(entry, currentPrice)
		if shouldExit {
			s.closePosition(entry, currentPrice, reason)
		} else {
			// Update entry with new highest price
			s.repo.UpdateEntry(entry)
		}
	}
}

func (s *AutoScalpingService) shouldExit(entry *domain.AutoScalpEntry, currentPrice float64) (bool, string) {
	// 1. Check Stop Loss
	if currentPrice >= entry.StopLoss {
		return true, "SL_HIT"
	}

	// 2. Check max position time
	duration := time.Since(entry.EntryTime).Seconds()
	if int(duration) >= s.settings.MaxPositionTime {
		return true, "MAX_TIME"
	}

	// 3. Calculate profit
	profitPct := ((entry.EntryPrice - currentPrice) / entry.EntryPrice) * 100

	// 4. Dynamic trailing stop logic
	// Once we hit minimum profit, activate trailing stop
	if profitPct >= s.settings.MinProfitPercent {
		// Calculate peak profit
		peakProfitPct := ((entry.EntryPrice - entry.HighestPrice) / entry.EntryPrice) * 100
		
		// If price retraces from peak by trailing stop %, exit
		retraceFromPeak := peakProfitPct - profitPct
		if retraceFromPeak >= s.settings.TrailingStopPercent {
			return true, "TRAILING_STOP"
		}
	}

	// 5. Emergency exit if profit turns negative (price went up beyond entry)
	if profitPct < -s.settings.StopLossPercent {
		return true, "EMERGENCY_EXIT"
	}

	return false, ""
}

func (s *AutoScalpingService) closePosition(entry *domain.AutoScalpEntry, exitPrice float64, reason string) {
	now := time.Now()
	pl := (entry.EntryPrice - exitPrice) * 100 // Assuming position size 100 USDT
	plPct := ((entry.EntryPrice - exitPrice) / entry.EntryPrice) * 100
	duration := int(now.Sub(entry.EntryTime).Seconds())

	entry.ExitPrice = &exitPrice
	entry.ExitTime = &now
	entry.ExitReason = reason
	entry.ProfitLoss = &pl
	entry.ProfitLossPct = &plPct
	entry.DurationSeconds = duration
	entry.Status = "CLOSED"

	if err := s.repo.UpdateEntry(entry); err != nil {
		log.Printf("Error closing position %s: %v", entry.ID, err)
	} else {
		log.Printf("✓ Auto scalp closed: %s | %s | P/L: %.2f%% | Duration: %ds | Reason: %s",
			entry.Symbol, entry.ID, plPct, duration, reason)
	}
}

func (s *AutoScalpingService) checkEntries() {
	// Check if we can add more positions
	activeCount := len(s.repo.GetActiveEntries())
	if activeCount >= s.settings.MaxConcurrentTrades {
		return
	}

	// Get high-score coins
	coins := s.screeningRepo.GetCoins()
	for _, coin := range coins {
		if activeCount >= s.settings.MaxConcurrentTrades {
			break
		}

		// Check if already have position in this symbol
		if s.hasActivePosition(coin.Symbol) {
			continue
		}

		// Check entry criteria
		if s.shouldEnter(&coin) {
			s.openPosition(&coin)
			activeCount++
		}
	}
}

func (s *AutoScalpingService) hasActivePosition(symbol string) bool {
	activeEntries := s.repo.GetActiveEntries()
	for _, entry := range activeEntries {
		if entry.Symbol == symbol {
			return true
		}
	}
	return false
}

func (s *AutoScalpingService) shouldEnter(coin *domain.CoinData) bool {
	// Must have features
	if coin.Features == nil {
		return false
	}

	features := coin.Features

	// PRIMARY FILTER: RSI must be overbought (75+) for SHORT
	if features.RSI < 75 {
		return false
	}

	// REVERSAL VALIDATION: Need at least 2 reversal signs
	reversalSigns := 0

	// 1. Rejection wick (upper wick > 50% of candle range)
	if features.RejectionWick > 0.5 {
		reversalSigns++
	}

	// 2. Above upper Bollinger Band (overextension)
	if features.IsAboveUpperBand {
		reversalSigns++
	}

	// 3. EMA overextension (3%+)
	if features.OverExtEma >= 0.03 {
		reversalSigns++
	}

	// 4. Breakdown signal (price rejecting higher level)
	if features.IsBreakdown {
		reversalSigns++
	}

	// 5. High funding rate (longs getting squeezed)
	if features.FundingRate > 0.0003 {
		reversalSigns++
	}

	// 6. Significant pump (15%+ in 24h suggests overheating)
	if features.PctChange24h >= 15 {
		reversalSigns++
	}

	// Need at least 2 reversal confirmation signs
	if reversalSigns >= 2 {
		log.Printf("🎯 Auto scalp entry candidate: %s | RSI: %.1f | Reversal signs: %d | Price: %.6f",
			coin.Symbol, features.RSI, reversalSigns, coin.Price)
		return true
	}

	return false
}

func (s *AutoScalpingService) openPosition(coin *domain.CoinData) {
	entryPrice := coin.Price
	stopLoss := entryPrice * (1 + s.settings.StopLossPercent/100)
	
	entry := &domain.AutoScalpEntry{
		ID:              fmt.Sprintf("%d", time.Now().UnixNano()),
		Symbol:          coin.Symbol,
		EntryPrice:      entryPrice,
		StopLoss:        stopLoss,
		EntryTime:       time.Now(),
		Status:          "ACTIVE",
		EntryScore:      coin.Score,
		HighestPrice:    entryPrice, // Initialize with entry price
		TrailingStopPct: s.settings.TrailingStopPercent,
	}

	if err := s.repo.CreateEntry(entry); err != nil {
		log.Printf("Error creating auto scalp entry: %v", err)
		return
	}

	log.Printf("🎯 Auto scalp opened: %s | Score: %.0f | Entry: $%.4f | SL: $%.4f",
		coin.Symbol, coin.Score, entryPrice, stopLoss)
}

// GetStatistics calculates performance stats for a time period
func (s *AutoScalpingService) GetStatistics(fromTime time.Time) map[string]interface{} {
	history := s.repo.GetHistory(fromTime)
	
	if len(history) == 0 {
		return map[string]interface{}{
			"totalTrades":    0,
			"winRate":        0.0,
			"totalProfitPct": 0.0,
			"avgDuration":    0,
		}
	}

	wins := 0
	totalProfitPct := 0.0
	totalDuration := 0

	for _, entry := range history {
		if entry.ProfitLossPct != nil {
			if *entry.ProfitLossPct > 0 {
				wins++
			}
			totalProfitPct += *entry.ProfitLossPct
		}
		totalDuration += entry.DurationSeconds
	}

	winRate := float64(wins) / float64(len(history)) * 100
	avgDuration := totalDuration / len(history)

	return map[string]interface{}{
		"totalTrades":    len(history),
		"winRate":        math.Round(winRate*100) / 100,
		"totalProfitPct": math.Round(totalProfitPct*100) / 100,
		"avgDuration":    avgDuration,
	}
}
