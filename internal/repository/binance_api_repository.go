package repository

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"screener-backend/internal/domain"
	"sync"
	"time"
)

// BinanceAPIRepository manages Binance API credentials
type BinanceAPIRepository struct {
	credentials map[string]*domain.BinanceAPICredentials // key: userID
	configs     map[string]*domain.BinanceTradingConfig  // key: userID
	mu          sync.RWMutex
	encryptKey  []byte // 32 bytes for AES-256
}

// NewBinanceAPIRepository creates a new repository
func NewBinanceAPIRepository(encryptionKey string) *BinanceAPIRepository {
	// Use provided key or generate default (in production, use env var)
	key := []byte(encryptionKey)
	if len(key) < 32 {
		// Pad to 32 bytes
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	} else if len(key) > 32 {
		key = key[:32]
	}

	return &BinanceAPIRepository{
		credentials: make(map[string]*domain.BinanceAPICredentials),
		configs:     make(map[string]*domain.BinanceTradingConfig),
		encryptKey:  key,
	}
}

// SaveCredentials saves or updates API credentials
func (r *BinanceAPIRepository) SaveCredentials(cred *domain.BinanceAPICredentials) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Encrypt the secret key
	encryptedSecret, err := r.encrypt(cred.SecretKey)
	if err != nil {
		return err
	}

	// Create a copy to store
	stored := *cred
	stored.SecretKey = encryptedSecret
	stored.UpdatedAt = time.Now()

	if stored.CreatedAt.IsZero() {
		stored.CreatedAt = time.Now()
	}

	r.credentials[cred.UserID] = &stored
	return nil
}

// GetCredentials retrieves credentials with decrypted secret
func (r *BinanceAPIRepository) GetCredentials(userID string) (*domain.BinanceAPICredentials, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cred, exists := r.credentials[userID]
	if !exists {
		return nil, errors.New("credentials not found")
	}

	// Decrypt the secret key
	decryptedSecret, err := r.decrypt(cred.SecretKey)
	if err != nil {
		return nil, err
	}

	// Return a copy with decrypted secret
	result := *cred
	result.SecretKey = decryptedSecret
	return &result, nil
}

// DeleteCredentials removes credentials
func (r *BinanceAPIRepository) DeleteCredentials(userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.credentials, userID)
	return nil
}

// SaveTradingConfig saves trading configuration
func (r *BinanceAPIRepository) SaveTradingConfig(config *domain.BinanceTradingConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.configs[config.UserID] = config
	return nil
}

// GetTradingConfig retrieves trading configuration
func (r *BinanceAPIRepository) GetTradingConfig(userID string) (*domain.BinanceTradingConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	config, exists := r.configs[userID]
	if !exists {
		// Return default config
		return &domain.BinanceTradingConfig{
			UserID:              userID,
			TradeAmountUSDT:     10.0,
			Leverage:            1,
			OrderType:           "MARKET",
			MaxSlippagePercent:  0.5,
			MaxDailyLossUSDT:    100.0,
			MaxDailyTrades:      10,
			EnableRealTrading:   false,
			UseStopLoss:         true,
			UseTakeProfit:       true,
			DefaultStopLossPct:  0.8,
			DefaultTakeProfitPct: 1.5,
		}, nil
	}

	return config, nil
}

// UpdateLastTested updates the last tested timestamp
func (r *BinanceAPIRepository) UpdateLastTested(userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cred, exists := r.credentials[userID]
	if !exists {
		return errors.New("credentials not found")
	}

	cred.LastTested = time.Now()
	return nil
}

// encrypt encrypts a string using AES-GCM
func (r *BinanceAPIRepository) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(r.encryptKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts an encrypted string
func (r *BinanceAPIRepository) decrypt(encrypted string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(r.encryptKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
