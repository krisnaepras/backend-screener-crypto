package db

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PoolConfig struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxConns:          10,
		MinConns:          2,
		MaxConnLifetime:   30 * time.Minute,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
	}
}

func PoolConfigFromEnv() PoolConfig {
	cfg := DefaultPoolConfig()

	if v := strings.TrimSpace(os.Getenv("DB_MAX_CONNS")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			cfg.MaxConns = int32(n)
		}
	}
	if v := strings.TrimSpace(os.Getenv("DB_MIN_CONNS")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			cfg.MinConns = int32(n)
		}
	}
	if v := strings.TrimSpace(os.Getenv("DB_MAX_CONN_LIFETIME")); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.MaxConnLifetime = d
		}
	}
	if v := strings.TrimSpace(os.Getenv("DB_MAX_CONN_IDLE_TIME")); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.MaxConnIdleTime = d
		}
	}
	if v := strings.TrimSpace(os.Getenv("DB_HEALTHCHECK_PERIOD")); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.HealthCheckPeriod = d
		}
	}

	if cfg.MaxConns < 1 {
		cfg.MaxConns = 1
	}
	if cfg.MinConns < 0 {
		cfg.MinConns = 0
	}
	if cfg.MinConns > cfg.MaxConns {
		cfg.MinConns = cfg.MaxConns
	}

	return cfg
}

func ensureSSLModeRequire(dbURL string) string {
	// Supabase requires SSL in almost all setups.
	u, err := url.Parse(dbURL)
	if err != nil {
		// If parsing fails, return as-is; pgx will surface a connection error.
		return dbURL
	}

	q := u.Query()
	if q.Get("sslmode") == "" {
		q.Set("sslmode", "require")
		u.RawQuery = q.Encode()
	}

	// Preserve original scheme casing etc.
	return strings.TrimSpace(u.String())
}

func NewPool(ctx context.Context, databaseURL string, cfg PoolConfig) (*pgxpool.Pool, error) {
	databaseURL = ensureSSLModeRequire(databaseURL)

	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	return pgxpool.NewWithConfig(ctx, poolCfg)
}
