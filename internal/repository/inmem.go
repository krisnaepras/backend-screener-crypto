package repository

import (
	"screener-backend/internal/domain"
	"sync"
)

type InMemoryScreenerRepository struct {
	coins []domain.CoinData
	mu    sync.RWMutex
}

func NewInMemoryScreenerRepository() *InMemoryScreenerRepository {
	return &InMemoryScreenerRepository{
		coins: []domain.CoinData{},
	}
}

func (r *InMemoryScreenerRepository) SaveCoins(coins []domain.CoinData) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Replace entire list for now as we scan all at once
	r.coins = coins
}

func (r *InMemoryScreenerRepository) GetCoins() []domain.CoinData {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// Return copy to avoid race conditions if caller modifies it (though CoinData contains pointers, so be careful.
	// For this use case, we serialize to JSON immediately usually, so shallow copy of slice is enough).
	result := make([]domain.CoinData, len(r.coins))
	copy(result, r.coins)
	return result
}
