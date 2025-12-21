package usecase

import (
	"errors"
	"log"
	"math"
	"screener-backend/internal/domain"
	"screener-backend/internal/infrastructure/binance"
	"time"
)

var (
	ErrRealTradingDisabled = errors.New("real trading is disabled")
	ErrMissingCredentials  = errors.New("binance credentials not configured")
)

type BinanceTradingService struct {
	apiRepo  domain.BinanceAPIStore
	autoRepo domain.AutoScalpRepository
}

func NewBinanceTradingService(
	apiRepo domain.BinanceAPIStore,
	autoRepo domain.AutoScalpRepository,
) *BinanceTradingService {
	return &BinanceTradingService{
		apiRepo:  apiRepo,
		autoRepo: autoRepo,
	}
}

// PlaceShortWithStopLoss places a SHORT market order and immediately places a STOP_MARKET reduce-only stop loss.
// This is the safest baseline because the SL lives on Binance.
func (s *BinanceTradingService) PlaceShortWithStopLoss(
	userID string,
	symbol string,
	entryPrice float64,
	stopLossPrice float64,
	tradeAmountUSDT float64,
	leverage int,
) (entryOrderID int64, slOrderID int64, qty float64, err error) {
	cfg, cfgErr := s.apiRepo.GetTradingConfig(userID)
	if cfgErr != nil {
		// If no config exists, default is returned by repo.
		cfg = &domain.BinanceTradingConfig{UserID: userID, EnableRealTrading: false}
	}

	if !cfg.EnableRealTrading {
		return 0, 0, 0, ErrRealTradingDisabled
	}

	cred, err := s.apiRepo.GetCredentials(userID)
	if err != nil {
		return 0, 0, 0, ErrMissingCredentials
	}

	client := binance.NewTradingClient(cred.APIKey, cred.SecretKey, cred.IsTestnet)
	if err := client.SetLeverage(symbol, leverage); err != nil {
		return 0, 0, 0, err
	}

	// Risk checks: basic
	acct, err := client.GetAccountInfo()
	if err != nil {
		return 0, 0, 0, err
	}

	if tradeAmountUSDT <= 0 {
		tradeAmountUSDT = cfg.TradeAmountUSDT
	}
	if leverage <= 0 {
		leverage = cfg.Leverage
	}
	if leverage < 1 {
		leverage = 1
	}
	if leverage > 20 {
		leverage = 20
	}

	// Daily loss/trade checks are handled elsewhere (emergency stop).
	if acct.AvailableBalance <= 0 {
		return 0, 0, 0, errors.New("insufficient balance")
	}

	// Quantity approximation for USDT-margined futures: qty = (tradeAmountUSDT * leverage) / entryPrice
	// Round down to a reasonable precision.
	rawQty := (tradeAmountUSDT * float64(leverage)) / entryPrice
	qty = floorTo(rawQty, 3) // 0.001 steps baseline; real step size differs per symbol.
	if qty <= 0 {
		return 0, 0, 0, errors.New("calculated quantity too small")
	}

	// 1) Place entry order: SELL MARKET (SHORT)
	positionSide := "SHORT"
	entryResp, err := client.PlaceOrder(&domain.BinanceOrderRequest{
		Symbol:       symbol,
		Side:         "SELL",
		PositionSide: positionSide,
		OrderType:    "MARKET",
		Quantity:     qty,
	})
	if err != nil {
		// Fallback for One-way mode where positionSide must be BOTH.
		if apiErr, ok := err.(*binance.BinanceAPIError); ok && apiErr.Code == -4061 {
			positionSide = "BOTH"
			entryResp, err = client.PlaceOrder(&domain.BinanceOrderRequest{
				Symbol:       symbol,
				Side:         "SELL",
				PositionSide: positionSide,
				OrderType:    "MARKET",
				Quantity:     qty,
			})
		}
	}
	if err != nil {
		return 0, 0, 0, err
	}

	entryOrderID = entryResp.OrderID
	// If avg price returned is 0, fall back to provided entryPrice.
	filledPrice := entryResp.ExecutedPrice
	if filledPrice <= 0 {
		filledPrice = entryPrice
	}

	// 2) Place STOP_MARKET closePosition stop loss
	slID, err := client.PlaceStopLossOrder(symbol, qty, stopLossPrice, positionSide)
	if err != nil {
		// Best effort: if SL placement fails, we should alert loudly.
		log.Printf("CRITICAL: SL order placement failed for %s entryOrder=%d: %v", symbol, entryOrderID, err)
		return entryOrderID, 0, qty, err
	}

	slOrderID = slID

	// Persist in auto scalping repo if exists
	_ = s.autoRepo.UpdateOrAttachBinanceOrders(symbol, entryOrderID, slOrderID, qty, leverage, filledPrice)

	return entryOrderID, slOrderID, qty, nil
}

func floorTo(v float64, decimals int) float64 {
	p := math.Pow10(decimals)
	return math.Floor(v*p) / p
}

// EmergencyStopAll closes all active positions if needed.
func (s *BinanceTradingService) EmergencyStopAll(userID string, reason string) error {
	cred, err := s.apiRepo.GetCredentials(userID)
	if err != nil {
		return ErrMissingCredentials
	}
	cfg, _ := s.apiRepo.GetTradingConfig(userID)
	if cfg != nil && !cfg.EnableRealTrading {
		return ErrRealTradingDisabled
	}

	client := binance.NewTradingClient(cred.APIKey, cred.SecretKey, cred.IsTestnet)
	acct, err := client.GetAccountInfo()
	if err != nil {
		return err
	}

	log.Printf("EMERGENCY STOP user=%s positions=%d reason=%s", userID, acct.PositionsCount, reason)

	// Close each open position at market
	for _, pos := range acct.Positions {
		if pos.PositionAmount == 0 {
			continue
		}

		side := "BUY"
		positionSide := pos.PositionSide
		// If SHORT positionAmt is negative sometimes. Use abs.
		qty := math.Abs(pos.PositionAmount)
		if positionSide == "LONG" {
			side = "SELL"
		}

		_, err := client.PlaceOrder(&domain.BinanceOrderRequest{
			Symbol:       pos.Symbol,
			Side:         side,
			PositionSide: positionSide,
			OrderType:    "MARKET",
			Quantity:     qty,
		})
		if err != nil {
			log.Printf("Failed to close position %s: %v", pos.Symbol, err)
		}
	}

	// Record emergency stop timestamp
	_ = s.autoRepo.RecordEmergencyStop(userID, time.Now(), reason)
	return nil
}
