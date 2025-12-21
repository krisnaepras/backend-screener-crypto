package http

import (
	"encoding/json"
	"net/http"
	"screener-backend/internal/domain"
	"screener-backend/internal/usecase"
	"time"
)

// AutoScalpHandler handles auto scalping endpoints
type AutoScalpHandler struct {
	service *usecase.AutoScalpingService
}

// NewAutoScalpHandler creates a new handler
func NewAutoScalpHandler(service *usecase.AutoScalpingService) *AutoScalpHandler {
	return &AutoScalpHandler{service: service}
}

// GetSettings handles GET /api/autoscalp/settings
func (h *AutoScalpHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settings := h.service.GetSettings()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// UpdateSettings handles POST /api/autoscalp/settings
func (h *AutoScalpHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var settings domain.AutoScalpSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.service.UpdateSettings(&settings)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Settings updated successfully",
	})
}

// GetActivePositions handles GET /api/autoscalp/active
func (h *AutoScalpHandler) GetActivePositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Implementation will be in repository
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]interface{}{}) // Placeholder
}

// GetHistory handles GET /api/autoscalp/history?period=1d|7d|30d
func (h *AutoScalpHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	period := r.URL.Query().Get("period")
	var fromTime time.Time
	
	switch period {
	case "1d":
		fromTime = time.Now().Add(-24 * time.Hour)
	case "7d":
		fromTime = time.Now().Add(-7 * 24 * time.Hour)
	case "30d":
		fromTime = time.Now().Add(-30 * 24 * time.Hour)
	default:
		fromTime = time.Now().Add(-24 * time.Hour) // Default 1 day
	}

	// Get history and stats
	// Implementation will be in repository
	response := map[string]interface{}{
		"history": []interface{}{}, // Placeholder
		"stats":   h.service.GetStatistics(fromTime),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
