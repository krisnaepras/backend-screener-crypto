package repository

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"screener-backend/internal/domain"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresBinanceAPIRepository stores Binance credentials/config in Postgres.
// SecretKey is encrypted at rest using AES-GCM with a 32-byte key.
type PostgresBinanceAPIRepository struct {
	pool       *pgxpool.Pool
	encryptKey []byte
}

func NewPostgresBinanceAPIRepository(pool *pgxpool.Pool, encryptionKey string) *PostgresBinanceAPIRepository {
	key := []byte(encryptionKey)
	if len(key) < 32 {
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	} else if len(key) > 32 {
		key = key[:32]
	}

	return &PostgresBinanceAPIRepository{pool: pool, encryptKey: key}
}

func (r *PostgresBinanceAPIRepository) SaveCredentials(cred *domain.BinanceAPICredentials) error {
	if cred == nil {
		return errors.New("nil credentials")
	}

	encryptedSecret, err := r.encrypt(cred.SecretKey)
	if err != nil {
		return err
	}

	permissionsJSON, err := json.Marshal(cred.Permissions)
	if err != nil {
		return err
	}

	now := time.Now()
	createdAt := cred.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}

	lastTested := cred.LastTested
	if lastTested.IsZero() {
		lastTested = time.Unix(0, 0).UTC()
	}

	_, err = r.pool.Exec(context.Background(), `
		insert into binance_credentials(
			user_id, api_key, secret_key_enc, is_testnet, is_enabled, permissions,
			created_at, updated_at, last_tested
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		on conflict (user_id) do update set
			api_key = excluded.api_key,
			secret_key_enc = excluded.secret_key_enc,
			is_testnet = excluded.is_testnet,
			is_enabled = excluded.is_enabled,
			permissions = excluded.permissions,
			updated_at = excluded.updated_at
	`,
		cred.UserID,
		cred.APIKey,
		encryptedSecret,
		cred.IsTestnet,
		cred.IsEnabled,
		permissionsJSON,
		createdAt,
		now,
		lastTested,
	)
	return err
}

func (r *PostgresBinanceAPIRepository) GetCredentials(userID string) (*domain.BinanceAPICredentials, error) {
	row := r.pool.QueryRow(context.Background(), `
		select user_id, api_key, secret_key_enc, is_testnet, is_enabled, permissions,
			created_at, updated_at, last_tested
		from binance_credentials
		where user_id = $1
	`, userID)

	var cred domain.BinanceAPICredentials
	var secretEnc string
	var permissionsRaw []byte
	var lastTested time.Time

	if err := row.Scan(
		&cred.UserID,
		&cred.APIKey,
		&secretEnc,
		&cred.IsTestnet,
		&cred.IsEnabled,
		&permissionsRaw,
		&cred.CreatedAt,
		&cred.UpdatedAt,
		&lastTested,
	); err != nil {
		return nil, errors.New("credentials not found")
	}

	secret, err := r.decrypt(secretEnc)
	if err != nil {
		return nil, err
	}
	cred.SecretKey = secret
	cred.LastTested = lastTested

	_ = json.Unmarshal(permissionsRaw, &cred.Permissions)
	return &cred, nil
}

func (r *PostgresBinanceAPIRepository) DeleteCredentials(userID string) error {
	_, err := r.pool.Exec(context.Background(), `delete from binance_credentials where user_id = $1`, userID)
	return err
}

func (r *PostgresBinanceAPIRepository) SaveTradingConfig(config *domain.BinanceTradingConfig) error {
	if config == nil {
		return errors.New("nil config")
	}

	_, err := r.pool.Exec(context.Background(), `
		insert into binance_trading_config(
			user_id, trade_amount_usdt, leverage, order_type, max_slippage_percent,
			max_daily_loss_usdt, max_daily_trades, enable_real_trading,
			use_stop_loss, use_take_profit, default_stop_loss_pct, default_take_profit_pct, updated_at
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12, now())
		on conflict (user_id) do update set
			trade_amount_usdt = excluded.trade_amount_usdt,
			leverage = excluded.leverage,
			order_type = excluded.order_type,
			max_slippage_percent = excluded.max_slippage_percent,
			max_daily_loss_usdt = excluded.max_daily_loss_usdt,
			max_daily_trades = excluded.max_daily_trades,
			enable_real_trading = excluded.enable_real_trading,
			use_stop_loss = excluded.use_stop_loss,
			use_take_profit = excluded.use_take_profit,
			default_stop_loss_pct = excluded.default_stop_loss_pct,
			default_take_profit_pct = excluded.default_take_profit_pct,
			updated_at = now()
	`,
		config.UserID,
		config.TradeAmountUSDT,
		config.Leverage,
		config.OrderType,
		config.MaxSlippagePercent,
		config.MaxDailyLossUSDT,
		config.MaxDailyTrades,
		config.EnableRealTrading,
		config.UseStopLoss,
		config.UseTakeProfit,
		config.DefaultStopLossPct,
		config.DefaultTakeProfitPct,
	)
	return err
}

func (r *PostgresBinanceAPIRepository) GetTradingConfig(userID string) (*domain.BinanceTradingConfig, error) {
	row := r.pool.QueryRow(context.Background(), `
		select user_id, trade_amount_usdt, leverage, order_type, max_slippage_percent,
			max_daily_loss_usdt, max_daily_trades, enable_real_trading,
			use_stop_loss, use_take_profit, default_stop_loss_pct, default_take_profit_pct
		from binance_trading_config
		where user_id = $1
	`, userID)

	cfg := &domain.BinanceTradingConfig{}
	if err := row.Scan(
		&cfg.UserID,
		&cfg.TradeAmountUSDT,
		&cfg.Leverage,
		&cfg.OrderType,
		&cfg.MaxSlippagePercent,
		&cfg.MaxDailyLossUSDT,
		&cfg.MaxDailyTrades,
		&cfg.EnableRealTrading,
		&cfg.UseStopLoss,
		&cfg.UseTakeProfit,
		&cfg.DefaultStopLossPct,
		&cfg.DefaultTakeProfitPct,
	); err != nil {
		// fall back to the same defaults as the in-memory repo
		return (&BinanceAPIRepository{}).GetTradingConfig(userID)
	}

	return cfg, nil
}

func (r *PostgresBinanceAPIRepository) UpdateLastTested(userID string) error {
	_, err := r.pool.Exec(context.Background(), `update binance_credentials set last_tested = now() where user_id = $1`, userID)
	return err
}

func (r *PostgresBinanceAPIRepository) encrypt(plaintext string) (string, error) {
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

func (r *PostgresBinanceAPIRepository) decrypt(encrypted string) (string, error) {
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

// compile-time check
var _ domain.BinanceAPIStore = (*PostgresBinanceAPIRepository)(nil)
