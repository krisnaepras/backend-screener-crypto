package repository

import (
	"sync"
)

// DeviceToken represents a registered device token
type DeviceToken struct {
	Token     string
	Platform  string // "android" or "ios"
	CreatedAt int64
}

// TokenRepository manages device tokens for push notifications
type TokenRepository struct {
	tokens map[string]*DeviceToken // token -> DeviceToken
	mu     sync.RWMutex
}

func NewTokenRepository() *TokenRepository {
	return &TokenRepository{
		tokens: make(map[string]*DeviceToken),
	}
}

// RegisterToken adds or updates a device token
func (r *TokenRepository) RegisterToken(token, platform string, timestamp int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tokens[token] = &DeviceToken{
		Token:     token,
		Platform:  platform,
		CreatedAt: timestamp,
	}
}

// UnregisterToken removes a device token
func (r *TokenRepository) UnregisterToken(token string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.tokens, token)
}

// GetAllTokens returns all registered tokens
func (r *TokenRepository) GetAllTokens() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tokens := make([]string, 0, len(r.tokens))
	for token := range r.tokens {
		tokens = append(tokens, token)
	}
	return tokens
}

// GetTokenCount returns the number of registered tokens
func (r *TokenRepository) GetTokenCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tokens)
}
