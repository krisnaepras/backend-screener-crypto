package repository

import (
	"context"
	"errors"
	"fmt"
	"screener-backend/internal/domain"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresAutoScalpRepository stores autoscalp entries in Postgres.
// Active entries: status='ACTIVE'. History: status='CLOSED'.
type PostgresAutoScalpRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresAutoScalpRepository(pool *pgxpool.Pool) *PostgresAutoScalpRepository {
	return &PostgresAutoScalpRepository{pool: pool}
}

func (r *PostgresAutoScalpRepository) CreateEntry(entry *domain.AutoScalpEntry) error {
	if entry == nil {
		return errors.New("nil entry")
	}

	_, err := r.pool.Exec(context.Background(), `
		insert into autoscalp_entries(
			id, symbol, entry_price, stop_loss, entry_time,
			exit_price, exit_time, exit_reason,
			profit_loss, profit_loss_pct, duration_seconds,
			status, entry_score, highest_price, trailing_stop_pct,
			is_real_trade, binance_order_id, binance_sl_order_id, quantity, leverage
		) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
	`,
		entry.ID,
		entry.Symbol,
		entry.EntryPrice,
		entry.StopLoss,
		entry.EntryTime,
		nullableFloat(entry.ExitPrice),
		nullableTime(entry.ExitTime),
		entry.ExitReason,
		nullableFloat(entry.ProfitLoss),
		nullableFloat(entry.ProfitLossPct),
		entry.DurationSeconds,
		entry.Status,
		entry.EntryScore,
		entry.HighestPrice,
		entry.TrailingStopPct,
		entry.IsRealTrade,
		nullableInt64(entry.BinanceOrderID),
		nullableInt64(entry.BinanceSLOrderID),
		entry.Quantity,
		entry.Leverage,
	)
	return err
}

func (r *PostgresAutoScalpRepository) GetActiveEntries() []*domain.AutoScalpEntry {
	rows, err := r.pool.Query(context.Background(), `
		select id, symbol, entry_price, stop_loss, entry_time,
			exit_price, exit_time, exit_reason,
			profit_loss, profit_loss_pct, duration_seconds,
			status, entry_score, highest_price, trailing_stop_pct,
			is_real_trade, binance_order_id, binance_sl_order_id, quantity, leverage
		from autoscalp_entries
		where status = 'ACTIVE'
		order by entry_time desc
	`)
	if err != nil {
		return []*domain.AutoScalpEntry{}
	}
	defer rows.Close()

	entries := make([]*domain.AutoScalpEntry, 0)
	for rows.Next() {
		entry, scanErr := scanAutoScalpEntry(rows)
		if scanErr != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

func (r *PostgresAutoScalpRepository) GetEntryByID(id string) (*domain.AutoScalpEntry, error) {
	row := r.pool.QueryRow(context.Background(), `
		select id, symbol, entry_price, stop_loss, entry_time,
			exit_price, exit_time, exit_reason,
			profit_loss, profit_loss_pct, duration_seconds,
			status, entry_score, highest_price, trailing_stop_pct,
			is_real_trade, binance_order_id, binance_sl_order_id, quantity, leverage
		from autoscalp_entries
		where id = $1
	`, id)

	e, err := scanAutoScalpEntry(row)
	if err != nil {
		return nil, fmt.Errorf("entry with ID %s not found", id)
	}
	return e, nil
}

func (r *PostgresAutoScalpRepository) UpdateEntry(entry *domain.AutoScalpEntry) error {
	if entry == nil {
		return errors.New("nil entry")
	}

	_, err := r.pool.Exec(context.Background(), `
		update autoscalp_entries set
			symbol=$2,
			entry_price=$3,
			stop_loss=$4,
			entry_time=$5,
			exit_price=$6,
			exit_time=$7,
			exit_reason=$8,
			profit_loss=$9,
			profit_loss_pct=$10,
			duration_seconds=$11,
			status=$12,
			entry_score=$13,
			highest_price=$14,
			trailing_stop_pct=$15,
			is_real_trade=$16,
			binance_order_id=$17,
			binance_sl_order_id=$18,
			quantity=$19,
			leverage=$20
		where id=$1
	`,
		entry.ID,
		entry.Symbol,
		entry.EntryPrice,
		entry.StopLoss,
		entry.EntryTime,
		nullableFloat(entry.ExitPrice),
		nullableTime(entry.ExitTime),
		entry.ExitReason,
		nullableFloat(entry.ProfitLoss),
		nullableFloat(entry.ProfitLossPct),
		entry.DurationSeconds,
		entry.Status,
		entry.EntryScore,
		entry.HighestPrice,
		entry.TrailingStopPct,
		entry.IsRealTrade,
		nullableInt64(entry.BinanceOrderID),
		nullableInt64(entry.BinanceSLOrderID),
		entry.Quantity,
		entry.Leverage,
	)
	return err
}

func (r *PostgresAutoScalpRepository) GetHistory(fromTime time.Time) []*domain.AutoScalpEntry {
	rows, err := r.pool.Query(context.Background(), `
		select id, symbol, entry_price, stop_loss, entry_time,
			exit_price, exit_time, exit_reason,
			profit_loss, profit_loss_pct, duration_seconds,
			status, entry_score, highest_price, trailing_stop_pct,
			is_real_trade, binance_order_id, binance_sl_order_id, quantity, leverage
		from autoscalp_entries
		where status = 'CLOSED' and exit_time is not null and exit_time >= $1
		order by exit_time desc
	`, fromTime)
	if err != nil {
		return []*domain.AutoScalpEntry{}
	}
	defer rows.Close()

	entries := make([]*domain.AutoScalpEntry, 0)
	for rows.Next() {
		entry, scanErr := scanAutoScalpEntry(rows)
		if scanErr != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

func (r *PostgresAutoScalpRepository) DeleteEntry(id string) error {
	_, err := r.pool.Exec(context.Background(), `delete from autoscalp_entries where id=$1`, id)
	return err
}

func (r *PostgresAutoScalpRepository) UpdateOrAttachBinanceOrders(symbol string, entryOrderID int64, slOrderID int64, qty float64, leverage int, filledPrice float64) error {
	// Attach to most recent ACTIVE entry for symbol.
	_, err := r.pool.Exec(context.Background(), `
		update autoscalp_entries set
			is_real_trade = true,
			quantity = $2,
			leverage = $3,
			entry_price = case when $4 > 0 then $4 else entry_price end,
			binance_order_id = $5,
			binance_sl_order_id = $6
		where id = (
			select id from autoscalp_entries
			where status='ACTIVE' and symbol=$1
			order by entry_time desc
			limit 1
		)
	`, symbol, qty, leverage, filledPrice, entryOrderID, slOrderID)
	return err
}

func (r *PostgresAutoScalpRepository) RecordEmergencyStop(userID string, at time.Time, reason string) error {
	_, err := r.pool.Exec(context.Background(), `
		insert into emergency_stop_events(user_id, occurred_at, reason)
		values ($1,$2,$3)
	`, userID, at, reason)
	return err
}

// Helpers

type scanner interface {
	Scan(dest ...any) error
}

func scanAutoScalpEntry(s scanner) (*domain.AutoScalpEntry, error) {
	var e domain.AutoScalpEntry
	var exitPrice pgtype.Float8
	var exitTime pgtype.Timestamptz
	var profitLoss pgtype.Float8
	var profitLossPct pgtype.Float8
	var orderID pgtype.Int8
	var slOrderID pgtype.Int8

	if err := s.Scan(
		&e.ID,
		&e.Symbol,
		&e.EntryPrice,
		&e.StopLoss,
		&e.EntryTime,
		&exitPrice,
		&exitTime,
		&e.ExitReason,
		&profitLoss,
		&profitLossPct,
		&e.DurationSeconds,
		&e.Status,
		&e.EntryScore,
		&e.HighestPrice,
		&e.TrailingStopPct,
		&e.IsRealTrade,
		&orderID,
		&slOrderID,
		&e.Quantity,
		&e.Leverage,
	); err != nil {
		return nil, err
	}

	if exitPrice.Valid {
		v := exitPrice.Float64
		e.ExitPrice = &v
	}
	if exitTime.Valid {
		v := exitTime.Time
		e.ExitTime = &v
	}
	if profitLoss.Valid {
		v := profitLoss.Float64
		e.ProfitLoss = &v
	}
	if profitLossPct.Valid {
		v := profitLossPct.Float64
		e.ProfitLossPct = &v
	}
	if orderID.Valid {
		v := orderID.Int64
		e.BinanceOrderID = &v
	}
	if slOrderID.Valid {
		v := slOrderID.Int64
		e.BinanceSLOrderID = &v
	}

	return &e, nil
}

func nullableFloat(v *float64) any {
	if v == nil {
		return pgtype.Float8{Valid: false}
	}
	return pgtype.Float8{Valid: true, Float64: *v}
}

func nullableTime(v *time.Time) any {
	if v == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Valid: true, Time: *v}
}

func nullableInt64(v *int64) any {
	if v == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Valid: true, Int64: *v}
}

// compile-time check
var _ domain.AutoScalpRepository = (*PostgresAutoScalpRepository)(nil)
