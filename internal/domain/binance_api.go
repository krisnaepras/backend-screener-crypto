package domain

import "time"

// BinanceAPICredentials represents user's Binance API credentials
type BinanceAPICredentials struct {
	UserID      string    `json:"userId"`
	APIKey      string    `json:"apiKey"`
	SecretKey   string    `json:"secretKey"` // Will be encrypted in storage
	IsTestnet   bool      `json:"isTestnet"`
	IsEnabled   bool      `json:"isEnabled"`
	Permissions []string  `json:"permissions"` // ["SPOT", "FUTURES", "MARGIN"]
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	LastTested  time.Time `json:"lastTested"`
}

// BinanceAccountInfo represents account balance and info
type BinanceAccountInfo struct {
	TotalBalance      float64            `json:"totalBalance"`
	AvailableBalance  float64            `json:"availableBalance"`
	UsdtBalance       float64            `json:"usdtBalance"`
	MarginLevel       float64            `json:"marginLevel,omitempty"`
	OpenOrdersCount   int                `json:"openOrdersCount"`
	PositionsCount    int                `json:"positionsCount"`
	TotalUnrealizedPL float64            `json:"totalUnrealizedPL,omitempty"`
	Assets            []BinanceAsset     `json:"assets"`
	Positions         []BinancePosition  `json:"positions,omitempty"`
}

// BinanceAsset represents a single asset balance
type BinanceAsset struct {
	Asset            string  `json:"asset"`
	Balance          float64 `json:"balance"`
	AvailableBalance float64 `json:"availableBalance"`
	UsdValue         float64 `json:"usdValue"`
}

// BinancePosition represents a futures position
type BinancePosition struct {
	Symbol           string  `json:"symbol"`
	PositionSide     string  `json:"positionSide"` // LONG/SHORT
	PositionAmount   float64 `json:"positionAmount"`
	EntryPrice       float64 `json:"entryPrice"`
	MarkPrice        float64 `json:"markPrice"`
	UnrealizedProfit float64 `json:"unrealizedProfit"`
	Leverage         int     `json:"leverage"`
}

// BinanceTradingConfig represents trading configuration
type BinanceTradingConfig struct {
	UserID              string  `json:"userId"`
	TradeAmountUSDT     float64 `json:"tradeAmountUsdt"`     // Amount per position
	Leverage            int     `json:"leverage"`            // 1-20x
	OrderType           string  `json:"orderType"`           // MARKET or LIMIT
	MaxSlippagePercent  float64 `json:"maxSlippagePercent"`  // For market orders
	MaxDailyLossUSDT    float64 `json:"maxDailyLossUsdt"`    // Daily loss limit
	MaxDailyTrades      int     `json:"maxDailyTrades"`      // Max trades per day
	EnableRealTrading   bool    `json:"enableRealTrading"`   // Safety switch
	UseStopLoss         bool    `json:"useStopLoss"`
	UseTakeProfit       bool    `json:"useTakeProfit"`
	DefaultStopLossPct  float64 `json:"defaultStopLossPct"`  // Default SL %
	DefaultTakeProfitPct float64 `json:"defaultTakeProfitPct"` // Default TP %
}

// BinanceOrderRequest represents a request to place an order
type BinanceOrderRequest struct {
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`         // BUY or SELL
	PositionSide string  `json:"positionSide"` // LONG or SHORT (futures)
	OrderType    string  `json:"orderType"`    // MARKET or LIMIT
	Quantity     float64 `json:"quantity"`
	Price        float64 `json:"price,omitempty"`
	StopLoss     float64 `json:"stopLoss,omitempty"`
	TakeProfit   float64 `json:"takeProfit,omitempty"`
}

// BinanceOrderResponse represents the response from Binance after placing an order
type BinanceOrderResponse struct {
	OrderID       int64   `json:"orderId"`
	Symbol        string  `json:"symbol"`
	Status        string  `json:"status"`
	ExecutedQty   float64 `json:"executedQty"`
	ExecutedPrice float64 `json:"executedPrice"`
	Commission    float64 `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
}

// BinanceAPIStatus represents API connection status
type BinanceAPIStatus struct {
	IsConnected     bool      `json:"isConnected"`
	LastSync        time.Time `json:"lastSync"`
	PermissionsOK   bool      `json:"permissionsOk"`
	ErrorMessage    string    `json:"errorMessage,omitempty"`
	DailyTradeCount int       `json:"dailyTradeCount"`
	DailyPL         float64   `json:"dailyPL"`
}
