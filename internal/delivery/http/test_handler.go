package http

import (
	"encoding/json"
	"net/http"

	"screener-backend/internal/infrastructure/fcm"
	"screener-backend/internal/repository"
)

type TestHandler struct {
	fcmClient *fcm.Client
	tokenRepo *repository.TokenRepository
}

func NewTestHandler(fcmClient *fcm.Client, tokenRepo *repository.TokenRepository) *TestHandler {
	return &TestHandler{
		fcmClient: fcmClient,
		tokenRepo: tokenRepo,
	}
}

func (h *TestHandler) SendTestNotification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.fcmClient == nil || !h.fcmClient.IsEnabled() {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "FCM not configured",
		})
		return
	}

	tokens := h.tokenRepo.GetAllTokens()
	if len(tokens) == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "No registered devices",
			"count":   0,
		})
		return
	}

	// Send test notification
	title := "🧪 Test Notification"
	body := "This is a test notification from Screener Backend. If you see this, notifications are working! ✅"
	data := map[string]string{
		"type": "test",
		"timestamp": "now",
	}

	err := h.fcmClient.SendMulticast(tokens, title, body, data)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to send notification: " + err.Error(),
			"count":   len(tokens),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Test notification sent successfully",
		"count":   len(tokens),
	})
}
