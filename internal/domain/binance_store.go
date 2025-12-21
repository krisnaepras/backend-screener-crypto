package domain

// BinanceAPIStore abstracts storage for Binance credentials/config.
// Implementations: in-memory (for dev) and Postgres (for production).
//
// Note: SecretKey is expected to be encrypted at rest by the implementation.
type BinanceAPIStore interface {
	SaveCredentials(cred *BinanceAPICredentials) error
	GetCredentials(userID string) (*BinanceAPICredentials, error)
	DeleteCredentials(userID string) error

	SaveTradingConfig(config *BinanceTradingConfig) error
	GetTradingConfig(userID string) (*BinanceTradingConfig, error)

	UpdateLastTested(userID string) error
}
