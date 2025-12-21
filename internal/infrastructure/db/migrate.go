package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate creates the minimal tables needed by this app.
// This keeps setup simple (no external migration tool), but still gives persistence.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	stmts := []string{
		`create table if not exists binance_credentials (
			user_id text primary key,
			api_key text not null,
			secret_key_enc text not null,
			is_testnet boolean not null default false,
			is_enabled boolean not null default true,
			permissions jsonb not null default '[]'::jsonb,
			created_at timestamptz not null default now(),
			updated_at timestamptz not null default now(),
			last_tested timestamptz not null default '1970-01-01'::timestamptz
		);`,
		`create table if not exists binance_trading_config (
			user_id text primary key,
			trade_amount_usdt double precision not null default 10,
			leverage int not null default 1,
			order_type text not null default 'MARKET',
			max_slippage_percent double precision not null default 0.5,
			max_daily_loss_usdt double precision not null default 100,
			max_daily_trades int not null default 10,
			enable_real_trading boolean not null default false,
			use_stop_loss boolean not null default true,
			use_take_profit boolean not null default true,
			default_stop_loss_pct double precision not null default 0.8,
			default_take_profit_pct double precision not null default 1.5,
			updated_at timestamptz not null default now()
		);`,
		`create table if not exists autoscalp_entries (
			id text primary key,
			symbol text not null,
			entry_price double precision not null,
			stop_loss double precision not null,
			entry_time timestamptz not null,
			exit_price double precision null,
			exit_time timestamptz null,
			exit_reason text not null default '',
			profit_loss double precision null,
			profit_loss_pct double precision null,
			duration_seconds int not null default 0,
			status text not null,
			entry_score double precision not null default 0,
			highest_price double precision not null default 0,
			trailing_stop_pct double precision not null default 0,

			is_real_trade boolean not null default false,
			binance_order_id bigint null,
			binance_sl_order_id bigint null,
			quantity double precision not null default 0,
			leverage int not null default 0
		);`,
		`create index if not exists autoscalp_entries_status_idx on autoscalp_entries(status);`,
		`create index if not exists autoscalp_entries_exit_time_idx on autoscalp_entries(exit_time);`,
		`create index if not exists autoscalp_entries_symbol_entry_time_idx on autoscalp_entries(symbol, entry_time desc);`,
		`create table if not exists emergency_stop_events (
			id bigserial primary key,
			user_id text not null,
			occurred_at timestamptz not null,
			reason text not null
		);`,
	}

	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
