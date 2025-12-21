package repository

import (
"fmt"
"screener-backend/internal/domain"
"sync"
)

// InMemoryTradeRepository stores trade entries in memory
type InMemoryTradeRepository struct {
	mu      sync.RWMutex
	entries map[string]*domain.TradeEntry
	history []*domain.TradeEntry
}

// NewInMemoryTradeRepository creates a new in-memory trade repository
func NewInMemoryTradeRepository() domain.TradeEntryRepository {
	return &InMemoryTradeRepository{
		entries: make(map[string]*domain.TradeEntry),
		history: make([]*domain.TradeEntry, 0),
	}
}

// CreateEntry creates a new trade entry
func (r *InMemoryTradeRepository) CreateEntry(entry *domain.TradeEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[entry.ID]; exists {
		return fmt.Errorf("entry with ID %s already exists", entry.ID)
	}

	r.entries[entry.ID] = entry
	return nil
}

// GetActiveEntries returns all active trade entries
func (r *InMemoryTradeRepository) GetActiveEntries() []*domain.TradeEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	active := make([]*domain.TradeEntry, 0)
	for _, entry := range r.entries {
		if entry.Status == "active" || entry.Status == "tp1_hit" || entry.Status == "tp2_hit" {
			active = append(active, entry)
		}
	}
	return active
}

// GetEntryByID retrieves an entry by ID
func (r *InMemoryTradeRepository) GetEntryByID(id string) (*domain.TradeEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.entries[id]
	if !exists {
		return nil, fmt.Errorf("entry not found")
	}
	return entry, nil
}

// UpdateEntry updates an existing entry
func (r *InMemoryTradeRepository) UpdateEntry(entry *domain.TradeEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[entry.ID]; !exists {
		return fmt.Errorf("entry not found")
	}

	// If status changed to closed or stopped, move to history
	if (entry.Status == "closed" || entry.Status == "stopped") && r.entries[entry.ID].Status != entry.Status {
		r.history = append(r.history, entry)
		delete(r.entries, entry.ID)
		return nil
	}

	r.entries[entry.ID] = entry
	return nil
}

// GetEntryHistory returns all closed/stopped entries
func (r *InMemoryTradeRepository) GetEntryHistory() []*domain.TradeEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domain.TradeEntry, len(r.history))
	copy(result, r.history)
	return result
}

// DeleteEntry removes an entry
func (r *InMemoryTradeRepository) DeleteEntry(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[id]; !exists {
		return fmt.Errorf("entry not found")
	}

	delete(r.entries, id)
	return nil
}
