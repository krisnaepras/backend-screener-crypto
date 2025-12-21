package repository

import (
	"fmt"
	"screener-backend/internal/domain"
	"sync"
	"time"
)

// InMemoryAutoScalpRepository implements domain.AutoScalpRepository
type InMemoryAutoScalpRepository struct {
	mu      sync.RWMutex
	entries map[string]*domain.AutoScalpEntry // Active entries
	history []*domain.AutoScalpEntry          // Closed entries

	lastEmergencyStopUser   string
	lastEmergencyStopAt     time.Time
	lastEmergencyStopReason string
}

// NewInMemoryAutoScalpRepository creates a new repository
func NewInMemoryAutoScalpRepository() *InMemoryAutoScalpRepository {
	return &InMemoryAutoScalpRepository{
		entries: make(map[string]*domain.AutoScalpEntry),
		history: make([]*domain.AutoScalpEntry, 0),
	}
}

func (r *InMemoryAutoScalpRepository) CreateEntry(entry *domain.AutoScalpEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[entry.ID]; exists {
		return fmt.Errorf("entry with ID %s already exists", entry.ID)
	}

	r.entries[entry.ID] = entry
	return nil
}

func (r *InMemoryAutoScalpRepository) GetActiveEntries() []*domain.AutoScalpEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]*domain.AutoScalpEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	return entries
}

func (r *InMemoryAutoScalpRepository) GetEntryByID(id string) (*domain.AutoScalpEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.entries[id]
	if !exists {
		return nil, fmt.Errorf("entry with ID %s not found", id)
	}
	return entry, nil
}

func (r *InMemoryAutoScalpRepository) UpdateEntry(entry *domain.AutoScalpEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[entry.ID]; !exists {
		return fmt.Errorf("entry with ID %s not found", entry.ID)
	}

	// If closing, move to history
	if entry.Status == "CLOSED" {
		r.history = append(r.history, entry)
		delete(r.entries, entry.ID)
	} else {
		r.entries[entry.ID] = entry
	}

	return nil
}

func (r *InMemoryAutoScalpRepository) GetHistory(fromTime time.Time) []*domain.AutoScalpEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	filtered := make([]*domain.AutoScalpEntry, 0)
	for _, entry := range r.history {
		if entry.ExitTime != nil && entry.ExitTime.After(fromTime) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func (r *InMemoryAutoScalpRepository) DeleteEntry(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[id]; !exists {
		return fmt.Errorf("entry with ID %s not found", id)
	}

	delete(r.entries, id)
	return nil
}

// UpdateOrAttachBinanceOrders best-effort attaches Binance order metadata to the most recent active entry for a symbol.
func (r *InMemoryAutoScalpRepository) UpdateOrAttachBinanceOrders(symbol string, entryOrderID int64, slOrderID int64, qty float64, leverage int, filledPrice float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var selected *domain.AutoScalpEntry
	for _, entry := range r.entries {
		if entry.Symbol != symbol {
			continue
		}
		if entry.Status != "ACTIVE" {
			continue
		}
		if selected == nil || entry.EntryTime.After(selected.EntryTime) {
			selected = entry
		}
	}

	if selected == nil {
		return fmt.Errorf("no active entry found for symbol %s", symbol)
	}

	selected.IsRealTrade = true
	selected.Quantity = qty
	selected.Leverage = leverage
	if filledPrice > 0 {
		selected.EntryPrice = filledPrice
	}
	selected.BinanceOrderID = &entryOrderID
	selected.BinanceSLOrderID = &slOrderID

	// Map holds pointers; entry updated in-place.
	return nil
}

func (r *InMemoryAutoScalpRepository) RecordEmergencyStop(userID string, at time.Time, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastEmergencyStopUser = userID
	r.lastEmergencyStopAt = at
	r.lastEmergencyStopReason = reason
	return nil
}
