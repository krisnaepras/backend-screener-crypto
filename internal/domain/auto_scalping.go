package domain

import "time"

// AutoScalpEntry represents an auto scalping trade
type AutoScalpEntry struct {
	ID               string     `json:"id"`
	Symbol           string     `json:"symbol"`
	EntryPrice       float64    `json:"entryPrice"`
	StopLoss         float64    `json:"stopLoss"`
	EntryTime        time.Time  `json:"entryTime"`
	ExitPrice        *float64   `json:"exitPrice,omitempty"`
	ExitTime         *time.Time `json:"exitTime,omitempty"`
	ExitReason       string     `json:"exitReason"` // "TP_HIT", "SL_HIT", "TRAILING_STOP", "MANUAL"
	ProfitLoss       *float64   `json:"profitLoss,omitempty"`
	ProfitLossPct    *float64   `json:"profitLossPct,omitempty"`
	DurationSeconds  int        `json:"durationSeconds"`
	Status           string     `json:"status"` // "ACTIVE", "CLOSED"
	EntryScore       float64    `json:"entryScore"`
	HighestPrice     float64    `json:"highestPrice"`     // Track highest since entry
	TrailingStopPct  float64    `json:"trailingStopPct"`  // Dynamic trailing stop %
}

// AutoScalpSettings represents user settings for auto scalping
type AutoScalpSettings struct {
	Enabled              bool    `json:"enabled"`
	MaxConcurrentTrades  int     `json:"maxConcurrentTrades"`
	MinEntryScore        float64 `json:"minEntryScore"`      // Min score to enter (e.g., 70)
	StopLossPercent      float64 `json:"stopLossPercent"`    // e.g., 0.5 (0.5%)
	MinProfitPercent     float64 `json:"minProfitPercent"`   // Min profit to start trailing (e.g., 0.3%)
	TrailingStopPercent  float64 `json:"trailingStopPercent"` // Trailing from peak (e.g., 0.2%)
	MaxPositionTime      int     `json:"maxPositionTime"`    // Max seconds in position (e.g., 1800 = 30min)
}

// AutoScalpRepository defines auto scalp operations
type AutoScalpRepository interface {
	CreateEntry(entry *AutoScalpEntry) error
	GetActiveEntries() []*AutoScalpEntry
	GetEntryByID(id string) (*AutoScalpEntry, error)
	UpdateEntry(entry *AutoScalpEntry) error
	GetHistory(fromTime time.Time) []*AutoScalpEntry
	DeleteEntry(id string) error
}
