package usecase

import (
	"fmt"
	"log"
	"time"

	"screener-backend/internal/domain"
)

// sendNotificationsForTriggers sends FCM notifications for coins with TRIGGER status only
func (uc *ScreenerUsecase) sendNotificationsForTriggers(coins []domain.CoinData) {
	if uc.fcmClient == nil || !uc.fcmClient.IsEnabled() {
		return // FCM not configured
	}

	tokens := uc.tokenRepo.GetAllTokens()
	if len(tokens) == 0 {
		return // No registered devices
	}

	now := time.Now()
	cooldownDuration := 5 * time.Minute

	for _, coin := range coins {
		// Notify only for TRIGGER (entry ready!)
		if coin.Status != "TRIGGER" {
			continue
		}

		// Check cooldown
		uc.mu.RLock()
		lastNotified, exists := uc.notifiedCoins[coin.Symbol]
		uc.mu.RUnlock()

		if exists && now.Sub(lastNotified) < cooldownDuration {
			continue // Skip, still in cooldown
		}

		// Prepare notification
		symbol := coin.Symbol
		displaySymbol := symbol[:len(symbol)-4] // Remove "USDT"
		
		title := fmt.Sprintf("🔥 %s TRIGGER - Entry Now!", displaySymbol)
		
		body := fmt.Sprintf("Score: %.0f | %dTF Aligned | $%.4f | +%.1f%%", 
			coin.Score, coin.ConfluenceCount, coin.Price, coin.PriceChangePercent)

		data := map[string]string{
			"symbol": coin.Symbol,
			"score":  fmt.Sprintf("%.2f", coin.Score),
			"price":  fmt.Sprintf("%.5f", coin.Price),
			"status": coin.Status,
			"type":   "TRIGGER",
		}

		// Send to all registered tokens
		err := uc.fcmClient.SendMulticast(tokens, title, body, data)
		if err != nil {
			log.Printf("Error sending notification for %s: %v", coin.Symbol, err)
		} else {
			log.Printf("Sent notification for %s to %d devices", coin.Symbol, len(tokens))
			
			// Update notified timestamp
			uc.mu.Lock()
			uc.notifiedCoins[coin.Symbol] = now
			uc.mu.Unlock()
		}
	}

	// Cleanup old entries (older than cooldown period)
	uc.mu.Lock()
	for symbol, timestamp := range uc.notifiedCoins {
		if now.Sub(timestamp) > cooldownDuration*2 {
			delete(uc.notifiedCoins, symbol)
		}
	}
	uc.mu.Unlock()
}

// sendNotificationsForBreakouts sends FCM notifications for coins with BREAKOUT status
func (uc *ScreenerUsecase) sendNotificationsForBreakouts(coins []domain.CoinData) {
	if uc.fcmClient == nil || !uc.fcmClient.IsEnabled() {
		return // FCM not configured
	}

	tokens := uc.tokenRepo.GetAllTokens()
	if len(tokens) == 0 {
		return // No registered devices
	}

	now := time.Now()
	cooldownDuration := 5 * time.Minute

	for _, coin := range coins {
		// Notify only for BREAKOUT_LONG or BREAKOUT_SHORT (confirmed breakout!)
		if coin.BreakoutStatus != "BREAKOUT_LONG" && coin.BreakoutStatus != "BREAKOUT_SHORT" {
			continue
		}

		// Check cooldown
		uc.mu.RLock()
		lastNotified, exists := uc.notifiedCoins[coin.Symbol+"_BREAKOUT"]
		uc.mu.RUnlock()

		if exists && now.Sub(lastNotified) < cooldownDuration {
			continue // Skip, still in cooldown
		}

		// Prepare notification
		symbol := coin.Symbol
		displaySymbol := symbol[:len(symbol)-4] // Remove "USDT"
		
		var title, emoji string
		if coin.BreakoutDirection == "LONG" {
			emoji = "🚀"
			title = fmt.Sprintf("%s %s BREAKOUT - Buy Signal!", emoji, displaySymbol)
		} else {
			emoji = "📉"
			title = fmt.Sprintf("%s %s BREAKDOWN - Sell Signal!", emoji, displaySymbol)
		}
		
		body := fmt.Sprintf("Score: %.0f | Direction: %s | $%.4f | +%.1f%%", 
			coin.BreakoutScore, coin.BreakoutDirection, coin.Price, coin.PriceChangePercent)

		data := map[string]string{
			"symbol":    coin.Symbol,
			"score":     fmt.Sprintf("%.2f", coin.BreakoutScore),
			"price":     fmt.Sprintf("%.5f", coin.Price),
			"status":    coin.BreakoutStatus,
			"direction": coin.BreakoutDirection,
			"type":      "BREAKOUT",
		}

		// Send to all registered tokens
		err := uc.fcmClient.SendMulticast(tokens, title, body, data)
		if err != nil {
			log.Printf("Error sending breakout notification for %s: %v", coin.Symbol, err)
		} else {
			log.Printf("Sent breakout notification for %s (%s) to %d devices", 
				coin.Symbol, coin.BreakoutDirection, len(tokens))
			
			// Update notified timestamp
			uc.mu.Lock()
			uc.notifiedCoins[coin.Symbol+"_BREAKOUT"] = now
			uc.mu.Unlock()
		}
	}

	// Cleanup old entries (older than cooldown period)
	uc.mu.Lock()
	for symbol, timestamp := range uc.notifiedCoins {
		if now.Sub(timestamp) > cooldownDuration*2 {
			delete(uc.notifiedCoins, symbol)
		}
	}
	uc.mu.Unlock()
}
