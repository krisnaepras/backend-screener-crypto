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
