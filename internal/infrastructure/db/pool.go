package db

import (
	"context"
	"errors"
	"net"
	"net/url"
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

	// Preserve the original host from the DSN so we can resolve A records even if the
	// underlying driver passes an already-resolved IPv6 literal into DialFunc.
	originalHost := ""
	originalPort := ""
	if u, err := url.Parse(databaseURL); err == nil {
		originalHost = u.Hostname()
		originalPort = u.Port()
	}

	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	// Heroku dynos commonly don't have IPv6 connectivity. Supabase may resolve to IPv6.
	// Prefer IPv4 to avoid: dial tcp [ipv6]: connect: network is unreachable.
	poolCfg.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, portStr, splitErr := net.SplitHostPort(addr)
		if splitErr != nil {
			// Best-effort fallback.
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		}

		// If we were given an IPv6 literal address, switch to resolving via the original
		// DSN host so we can still choose IPv4.
		lookupHost := host
		lookupPort := portStr
		if ip := net.ParseIP(host); ip != nil && ip.To4() == nil && originalHost != "" {
			lookupHost = originalHost
			if originalPort != "" {
				lookupPort = originalPort
			}
		}

		port, err := strconv.Atoi(lookupPort)
		if err != nil {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		}

		ips, err := net.DefaultResolver.LookupIPAddr(ctx, lookupHost)
		if err != nil {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		}

		hasIPv6 := false
		var lastErr error
		for _, ip := range ips {
			if ip.IP == nil || ip.IP.To4() == nil {
				if ip.IP != nil && ip.IP.To16() != nil {
					hasIPv6 = true
				}
				continue
			}
			candidate := net.JoinHostPort(ip.IP.String(), strconv.Itoa(port))
			conn, err := (&net.Dialer{}).DialContext(ctx, network, candidate)
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}

		if lastErr != nil {
			return nil, lastErr
		}
		if hasIPv6 {
			return nil, errors.New("database host resolves to IPv6 only; Heroku typically has no IPv6 egress. Use Supabase 'Connection Pooling' host (pooler) that provides IPv4")
		}

		return nil, errors.New("no IPv4 addresses resolved for database host")
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	return pgxpool.NewWithConfig(ctx, poolCfg)
}
