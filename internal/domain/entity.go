package domain

// MarketFeatures represents the technical indicators and market conditions for a coin.
type MarketFeatures struct {
	PctChange24h       float64  `json:"pctChange24h"`
	OverExtEma         float64  `json:"overExtEma"`
	OverExtVwap        float64  `json:"overExtVwap"`
	IsAboveUpperBand   bool     `json:"isAboveUpperBand"`
	CandleRangeRatio   float64  `json:"candleRangeRatio"`
	RSI                float64  `json:"rsi"`
	IsRsiBearishDiv    bool     `json:"isRsiBearishDiv"`
	RejectionWickRatio float64  `json:"rejectionWickRatio"`
	FundingRate        float64  `json:"fundingRate"`
	OpenInterestDelta  float64  `json:"openInterestDelta"`
	NearestSupport     *float64 `json:"nearestSupport"`
	DistToSupportATR   *float64 `json:"distToSupportATR"`
	IsBreakdown        bool     `json:"isBreakdown"`
	IsRetest           bool     `json:"isRetest"`
	IsRetestFail       bool     `json:"isRetestFail"`
	// Loss of Momentum indicators
	HasRsiDivergence    bool    `json:"hasRsiDivergence"`    // Price HH, RSI LH
	HasVolumeDivergence bool    `json:"hasVolumeDivergence"` // Price up, volume down
	MomentumSlope       float64 `json:"momentumSlope"`       // Rate of RSI change (negative = slowing)
	RsiSlope            float64 `json:"rsiSlope"`            // RSI trend direction
	VolumeDeclineRatio  float64 `json:"volumeDeclineRatio"`  // Current vs avg volume
	IsLosingMomentum    bool    `json:"isLosingMomentum"`    // Combined momentum loss signal
}

// TimeframeScore stores score for a single timeframe.
type TimeframeScore struct {
	TF    string  `json:"tf"`
	Score float64 `json:"score"`
	RSI   float64 `json:"rsi"`
}

// TimeframeFeatures stores features per timeframe for display.
type TimeframeFeatures struct {
	TF             string  `json:"tf"`
	RSI            float64 `json:"rsi"`
	OverExtEma     float64 `json:"overExtEma"`
	IsAboveUpperBB bool    `json:"isAboveUpperBB"`
	IsBreakdown    bool    `json:"isBreakdown"`
}

// CoinData represents the main data structure for a screened coin.
type CoinData struct {
	Symbol             string              `json:"symbol"`
	Price              float64             `json:"price"`
	Score              float64             `json:"score"`                    // Combined confluence score
	Status             string              `json:"status"`                   // e.g., "SETUP", "TRIGGER", "WATCH"
	TriggerTF          string              `json:"triggerTf,omitempty"`      // Primary TF that triggered
	ConfluenceCount    int                 `json:"confluenceCount"`          // How many TFs aligned (1-3)
	TFScores           []TimeframeScore    `json:"tfScores,omitempty"`       // Scores per TF
	TFFeatures         []TimeframeFeatures `json:"tfFeatures,omitempty"`     // Features per TF
	PriceChangePercent float64             `json:"priceChangePercent"`
	FundingRate        float64             `json:"fundingRate"`
	BasisSpread        float64             `json:"basisSpread"`
	Features           *MarketFeatures     `json:"features"`                 // Primary TF features
	// Intraday Setup (15m + 1h analysis) - SHORT
	IntradayStatus     string              `json:"intradayStatus,omitempty"` // "HOT", "WARM", "COOL"
	IntradayScore      float64             `json:"intradayScore"`            // Score based on 15m + 1h
	IntradayTFScores   []TimeframeScore    `json:"intradayTfScores,omitempty"`
	IntradayFeatures   *MarketFeatures     `json:"intradayFeatures,omitempty"`
	// Pullback Entry (Buy the Dip) - setup 5m/15m, execution 1m/3m
	PullbackStatus     string              `json:"pullbackStatus,omitempty"` // "DIP", "BOUNCE", "WAIT"
	PullbackScore      float64             `json:"pullbackScore"`
	PullbackTFScores   []TimeframeScore    `json:"pullbackTfScores,omitempty"`
	PullbackFeatures   *MarketFeatures     `json:"pullbackFeatures,omitempty"`
}
