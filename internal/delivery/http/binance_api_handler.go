package http

import (
	"encoding/json"
	"log"
	"net/http"
	"screener-backend/internal/domain"
	"screener-backend/internal/infrastructure/binance"
)

// BinanceAPIHandler handles Binance API management endpoints
type BinanceAPIHandler struct {
	repo domain.BinanceAPIStore
}

// NewBinanceAPIHandler creates a new handler
func NewBinanceAPIHandler(repo domain.BinanceAPIStore) *BinanceAPIHandler {
	return &BinanceAPIHandler{repo: repo}
}

// SaveCredentials handles POST /api/binance/credentials
func (h *BinanceAPIHandler) SaveCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID    string   `json:"userId"`
		APIKey    string   `json:"apiKey"`
		SecretKey string   `json:"secretKey"`
		IsTestnet bool     `json:"isTestnet"`
		IsEnabled bool     `json:"isEnabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.UserID == "" || req.APIKey == "" || req.SecretKey == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Test connection before saving
	client := binance.NewTradingClient(req.APIKey, req.SecretKey, req.IsTestnet)
	if err := client.TestConnection(); err != nil {
		log.Printf("Binance API test failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid API credentials or connection failed",
			"details": err.Error(),
		})
		return
	}

	// Get account info to check permissions
	accountInfo, err := client.GetAccountInfo()
	if err != nil {
		http.Error(w, "Failed to get account info", http.StatusBadRequest)
		return
	}

	// Save credentials
	cred := &domain.BinanceAPICredentials{
		UserID:      req.UserID,
		APIKey:      req.APIKey,
		SecretKey:   req.SecretKey,
		IsTestnet:   req.IsTestnet,
		IsEnabled:   req.IsEnabled,
		Permissions: []string{"FUTURES"}, // Default for futures trading
	}

	if err := h.repo.SaveCredentials(cred); err != nil {
		http.Error(w, "Failed to save credentials", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Credentials saved successfully",
		"balance": accountInfo.UsdtBalance,
		"connected": true,
	})
}

// GetCredentials handles GET /api/binance/credentials?userId=xxx
func (h *BinanceAPIHandler) GetCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "Missing userId", http.StatusBadRequest)
		return
	}

	cred, err := h.repo.GetCredentials(userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"exists": false,
		})
		return
	}

	// Don't return the secret key
	response := map[string]interface{}{
		"exists":     true,
		"userId":     cred.UserID,
		"apiKey":     cred.APIKey,
		"isTestnet":  cred.IsTestnet,
		"isEnabled":  cred.IsEnabled,
		"lastTested": cred.LastTested,
		"createdAt":  cred.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteCredentials handles DELETE /api/binance/credentials?userId=xxx
func (h *BinanceAPIHandler) DeleteCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "Missing userId", http.StatusBadRequest)
		return
	}

	if err := h.repo.DeleteCredentials(userID); err != nil {
		http.Error(w, "Failed to delete credentials", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Credentials deleted successfully",
	})
}

// GetAccountInfo handles GET /api/binance/account?userId=xxx
func (h *BinanceAPIHandler) GetAccountInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "Missing userId", http.StatusBadRequest)
		return
	}

	cred, err := h.repo.GetCredentials(userID)
	if err != nil {
		http.Error(w, "Credentials not found", http.StatusNotFound)
		return
	}

	client := binance.NewTradingClient(cred.APIKey, cred.SecretKey, cred.IsTestnet)
	accountInfo, err := client.GetAccountInfo()
	if err != nil {
		log.Printf("Failed to get account info: %v", err)
		http.Error(w, "Failed to get account info", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accountInfo)
}

// SaveTradingConfig handles POST /api/binance/trading-config
func (h *BinanceAPIHandler) SaveTradingConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var config domain.BinanceTradingConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if config.UserID == "" {
		http.Error(w, "Missing userId", http.StatusBadRequest)
		return
	}

	if err := h.repo.SaveTradingConfig(&config); err != nil {
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Trading config saved successfully",
	})
}

// GetTradingConfig handles GET /api/binance/trading-config?userId=xxx
func (h *BinanceAPIHandler) GetTradingConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "Missing userId", http.StatusBadRequest)
		return
	}

	config, err := h.repo.GetTradingConfig(userID)
	if err != nil {
		http.Error(w, "Failed to get config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// TestConnection handles POST /api/binance/test-connection
func (h *BinanceAPIHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "Missing userId", http.StatusBadRequest)
		return
	}

	cred, err := h.repo.GetCredentials(userID)
	if err != nil {
		http.Error(w, "Credentials not found", http.StatusNotFound)
		return
	}

	client := binance.NewTradingClient(cred.APIKey, cred.SecretKey, cred.IsTestnet)
	if err := client.TestConnection(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"connected": false,
			"error":     err.Error(),
		})
		return
	}

	// Update last tested
	h.repo.UpdateLastTested(userID)

	// Get account info
	accountInfo, _ := client.GetAccountInfo()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"connected": true,
		"balance":   accountInfo.UsdtBalance,
		"positions": accountInfo.PositionsCount,
	})
}
