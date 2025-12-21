package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"screener-backend/internal/domain"
	"time"
)

// TradeHandler handles trade entry endpoints
type TradeHandler struct {
	repo domain.TradeEntryRepository
}

// NewTradeHandler creates a new trade handler
func NewTradeHandler(repo domain.TradeEntryRepository) *TradeHandler {
	return &TradeHandler{repo: repo}
}

// CreateEntry handles POST /api/trades
func (h *TradeHandler) CreateEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var entry domain.TradeEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Set default values
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if entry.EntryTime.IsZero() {
		entry.EntryTime = time.Now()
	}
	if entry.Status == "" {
		entry.Status = "active"
	}

	if err := h.repo.CreateEntry(&entry); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

// GetActiveEntries handles GET /api/trades/active
func (h *TradeHandler) GetActiveEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	entries := h.repo.GetActiveEntries()
	if entries == nil {
		entries = make([]*domain.TradeEntry, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// GetHistory handles GET /api/trades/history
func (h *TradeHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	entries := h.repo.GetEntryHistory()
	if entries == nil {
		entries = make([]*domain.TradeEntry, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// UpdateEntry handles PUT /api/trades/update?id={id}
func (h *TradeHandler) UpdateEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	// Get existing entry
	existing, err := h.repo.GetEntryByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Partial update payload
	type updatePayload struct {
		Status      *string    `json:"status"`
		ExitPrice   *float64   `json:"exitPrice"`
		ExitTime    *string    `json:"exitTime"`
		ProfitLoss  *float64   `json:"profitLoss"`
		EntryReason *string    `json:"entryReason"`
	}

	var payload updatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Merge updates into existing entry
	updated := *existing
	if payload.Status != nil {
		updated.Status = *payload.Status
	}
	if payload.ExitPrice != nil {
		updated.ExitPrice = payload.ExitPrice
		// Auto-set exit time if not provided
		if payload.ExitTime != nil {
			t, _ := time.Parse(time.RFC3339, *payload.ExitTime)
			updated.ExitTime = &t
		} else {
			now := time.Now()
			updated.ExitTime = &now
		}
	}
	if payload.EntryReason != nil {
		updated.EntryReason = *payload.EntryReason
	}

	// Calculate P/L if closing and not explicitly provided
	if updated.ExitPrice != nil && (updated.Status == "closed" || updated.Status == "stopped") {
		if payload.ProfitLoss != nil {
			updated.ProfitLoss = payload.ProfitLoss
		} else {
			// Simple P/L calculation (assuming position size = 1)
			diff := 0.0
			if updated.IsLong {
				diff = *updated.ExitPrice - updated.EntryPrice
			} else {
				diff = updated.EntryPrice - *updated.ExitPrice
			}
			updated.ProfitLoss = &diff
		}
	}

	if err := h.repo.UpdateEntry(&updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// DeleteEntry handles DELETE /api/trades/delete?id={id}
func (h *TradeHandler) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	if err := h.repo.DeleteEntry(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"deleted"}`))
}
