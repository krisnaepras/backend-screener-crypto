package domain

import "time"

// TradeEntry represents a trade position
type TradeEntry struct {
	ID            string    `json:"id"`
	Symbol        string    `json:"symbol"`
	IsLong        bool      `json:"isLong"`
	EntryPrice    float64   `json:"entryPrice"`
	StopLoss      float64   `json:"stopLoss"`
	TakeProfit1   float64   `json:"takeProfit1"`
	TakeProfit2   float64   `json:"takeProfit2"`
	TakeProfit3   float64   `json:"takeProfit3"`
	EntryTime     time.Time `json:"entryTime"`
	Status        string    `json:"status"` // active, tp1_hit, tp2_hit, tp3_hit, stopped, closed
	ExitPrice     *float64  `json:"exitPrice,omitempty"`
	ExitTime      *time.Time `json:"exitTime,omitempty"`
	ProfitLoss    *float64  `json:"profitLoss,omitempty"`
	EntryReason   string    `json:"entryReason"`
}

// TradeEntryRepository defines the interface for trade entry operations
type TradeEntryRepository interface {
	CreateEntry(entry *TradeEntry) error
	GetActiveEntries() []*TradeEntry
	GetEntryByID(id string) (*TradeEntry, error)
	UpdateEntry(entry *TradeEntry) error
	GetEntryHistory() []*TradeEntry
	DeleteEntry(id string) error
}
