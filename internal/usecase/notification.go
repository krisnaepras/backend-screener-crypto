package usecase

import (
	"fmt"
	"log"
	"time"

	"screener-backend/internal/domain"
)

// sendNotificationsForTriggers sends FCM notifications for coins with TRIGGER status
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
		// Only notify for TRIGGER status with high score
		if coin.Status != "TRIGGER" || coin.Score < 70 {
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
		
		title := fmt.Sprintf("🚀 %s TRIGGER", displaySymbol)
		body := fmt.Sprintf("Score: %.0f | Price: $%.5f | Change: %.2f%%", 
			coin.Score, coin.Price, coin.PriceChangePercent)

		data := map[string]string{
			"symbol": coin.Symbol,
			"score":  fmt.Sprintf("%.2f", coin.Score),
			"price":  fmt.Sprintf("%.5f", coin.Price),
			"type":   "trigger",
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
