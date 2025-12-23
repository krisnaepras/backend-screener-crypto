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
}

// TimeframeScore stores score for a single timeframe.
type TimeframeScore struct {
	TF    string  `json:"tf"`
	Score float64 `json:"score"`
}

// CoinData represents the main data structure for a screened coin.
type CoinData struct {
	Symbol             string           `json:"symbol"`
	Price              float64          `json:"price"`
	Score              float64          `json:"score"`               // Max score across all TFs
	Status             string           `json:"status"`              // e.g., "SETUP", "TRIGGER", "AVOID"
	TriggerTF          string           `json:"triggerTf,omitempty"` // Which TF triggered (e.g. "1m", "15m")
	TFScores           []TimeframeScore `json:"tfScores,omitempty"`  // Scores per TF
	PriceChangePercent float64          `json:"priceChangePercent"`
	FundingRate        float64          `json:"fundingRate"`
	BasisSpread        float64          `json:"basisSpread"`
	Features           *MarketFeatures  `json:"features"`
}
