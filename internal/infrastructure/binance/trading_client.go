package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"screener-backend/internal/domain"
	"strconv"
	"time"
)

// TradingClient handles authenticated Binance API requests
type TradingClient struct {
	apiKey     string
	secretKey  string
	baseURL    string
	httpClient *http.Client
}

// BinanceAPIError captures structured error info returned by Binance.
type BinanceAPIError struct {
	StatusCode int
	Code       int    `json:"code"`
	Message    string `json:"msg"`
	Body       string
}

func (e *BinanceAPIError) Error() string {
	if e == nil {
		return "binance API error"
	}
	if e.Code != 0 || e.Message != "" {
		return fmt.Sprintf("binance API error %d (code=%d): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("binance API error %d: %s", e.StatusCode, e.Body)
}

func parseBinanceAPIError(statusCode int, body []byte) error {
	var parsed struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && (parsed.Code != 0 || parsed.Msg != "") {
		return &BinanceAPIError{StatusCode: statusCode, Code: parsed.Code, Message: parsed.Msg, Body: string(body)}
	}
	return &BinanceAPIError{StatusCode: statusCode, Body: string(body)}
}

// NewTradingClient creates a new authenticated Binance client
func NewTradingClient(apiKey, secretKey string, isTestnet bool) *TradingClient {
	baseURL := FapiBaseURL
	if isTestnet {
		baseURL = "https://testnet.binancefuture.com"
	}

	return &TradingClient{
		apiKey:     apiKey,
		secretKey:  secretKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// TestConnection tests if API credentials are valid
func (c *TradingClient) TestConnection() error {
	// Test with simple account endpoint
	_, err := c.GetAccountInfo()
	return err
}

// GetAccountInfo retrieves account balance and positions
func (c *TradingClient) GetAccountInfo() (*domain.BinanceAccountInfo, error) {
	endpoint := "/fapi/v2/account"
	
	resp, err := c.signedRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance API error %d: %s", resp.StatusCode, string(body))
	}

	var binanceResp struct {
		TotalWalletBalance    string `json:"totalWalletBalance"`
		AvailableBalance      string `json:"availableBalance"`
		MaxWithdrawAmount     string `json:"maxWithdrawAmount"`
		TotalUnrealizedProfit string `json:"totalUnrealizedProfit"`
		Assets                []struct {
			Asset            string `json:"asset"`
			WalletBalance    string `json:"walletBalance"`
			AvailableBalance string `json:"availableBalance"`
		} `json:"assets"`
		Positions []struct {
			Symbol           string `json:"symbol"`
			PositionSide     string `json:"positionSide"`
			PositionAmt      string `json:"positionAmt"`
			EntryPrice       string `json:"entryPrice"`
			MarkPrice        string `json:"markPrice"`
			UnRealizedProfit string `json:"unRealizedProfit"`
			Leverage         string `json:"leverage"`
		} `json:"positions"`
	}

	if err := json.Unmarshal(body, &binanceResp); err != nil {
		return nil, err
	}

	// Convert to domain model
	totalBalance, _ := strconv.ParseFloat(binanceResp.TotalWalletBalance, 64)
	availableBalance, _ := strconv.ParseFloat(binanceResp.AvailableBalance, 64)
	totalUnrealizedPL, _ := strconv.ParseFloat(binanceResp.TotalUnrealizedProfit, 64)

	info := &domain.BinanceAccountInfo{
		TotalBalance:      totalBalance,
		AvailableBalance:  availableBalance,
		TotalUnrealizedPL: totalUnrealizedPL,
		Assets:            []domain.BinanceAsset{},
		Positions:         []domain.BinancePosition{},
	}

	// Find USDT balance
	for _, asset := range binanceResp.Assets {
		balance, _ := strconv.ParseFloat(asset.WalletBalance, 64)
		available, _ := strconv.ParseFloat(asset.AvailableBalance, 64)
		
		if asset.Asset == "USDT" {
			info.UsdtBalance = balance
		}

		info.Assets = append(info.Assets, domain.BinanceAsset{
			Asset:            asset.Asset,
			Balance:          balance,
			AvailableBalance: available,
			UsdValue:         balance, // Simplified
		})
	}

	// Parse positions
	openPositions := 0
	for _, pos := range binanceResp.Positions {
		posAmt, _ := strconv.ParseFloat(pos.PositionAmt, 64)
		if posAmt == 0 {
			continue // Skip empty positions
		}

		openPositions++
		entryPrice, _ := strconv.ParseFloat(pos.EntryPrice, 64)
		markPrice, _ := strconv.ParseFloat(pos.MarkPrice, 64)
		unrealizedProfit, _ := strconv.ParseFloat(pos.UnRealizedProfit, 64)
		leverage, _ := strconv.Atoi(pos.Leverage)

		info.Positions = append(info.Positions, domain.BinancePosition{
			Symbol:           pos.Symbol,
			PositionSide:     pos.PositionSide,
			PositionAmount:   posAmt,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			UnrealizedProfit: unrealizedProfit,
			Leverage:         leverage,
		})
	}

	info.PositionsCount = openPositions

	// Get open orders count
	orders, err := c.GetOpenOrders("")
	if err == nil {
		info.OpenOrdersCount = len(orders)
	}

	return info, nil
}

// GetOpenOrders retrieves all open orders (or for specific symbol)
func (c *TradingClient) GetOpenOrders(symbol string) ([]map[string]interface{}, error) {
	endpoint := "/fapi/v1/openOrders"
	params := url.Values{}
	if symbol != "" {
		params.Set("symbol", symbol)
	}

	resp, err := c.signedRequest("GET", endpoint, params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseBinanceAPIError(resp.StatusCode, body)
	}

	var orders []map[string]interface{}
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, err
	}

	return orders, nil
}

// SetLeverage sets the leverage for a symbol (USDT-margined futures).
func (c *TradingClient) SetLeverage(symbol string, leverage int) error {
	endpoint := "/fapi/v1/leverage"

	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("leverage", strconv.Itoa(leverage))

	resp, err := c.signedRequest("POST", endpoint, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return parseBinanceAPIError(resp.StatusCode, body)
	}

	return nil
}

// PlaceStopLossOrder places a STOP_MARKET order with closePosition=true so the stop lives on Binance.
// positionSide should be "SHORT", "LONG", or "BOTH" depending on the account mode.
func (c *TradingClient) PlaceStopLossOrder(symbol string, _ float64, stopPrice float64, positionSide string) (int64, error) {
	endpoint := "/fapi/v1/order"

	side := "BUY"
	if positionSide == "LONG" {
		side = "SELL"
	}

	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", side)
	params.Set("type", "STOP_MARKET")
	params.Set("stopPrice", fmt.Sprintf("%.8f", stopPrice))
	params.Set("closePosition", "true")
	params.Set("workingType", "MARK_PRICE")
	params.Set("priceProtect", "true")

	if positionSide != "" {
		params.Set("positionSide", positionSide)
	}

	resp, err := c.signedRequest("POST", endpoint, params)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, parseBinanceAPIError(resp.StatusCode, body)
	}

	var binanceResp struct {
		OrderID int64 `json:"orderId"`
	}
	if err := json.Unmarshal(body, &binanceResp); err != nil {
		return 0, err
	}

	return binanceResp.OrderID, nil
}

// PlaceOrder places a new order
func (c *TradingClient) PlaceOrder(req *domain.BinanceOrderRequest) (*domain.BinanceOrderResponse, error) {
	endpoint := "/fapi/v1/order"
	
	params := url.Values{}
	params.Set("symbol", req.Symbol)
	params.Set("side", req.Side)
	params.Set("type", req.OrderType)
	params.Set("quantity", fmt.Sprintf("%.8f", req.Quantity))
	
	if req.PositionSide != "" {
		params.Set("positionSide", req.PositionSide)
	}
	
	if req.OrderType == "LIMIT" && req.Price > 0 {
		params.Set("price", fmt.Sprintf("%.8f", req.Price))
		params.Set("timeInForce", "GTC")
	}

	resp, err := c.signedRequest("POST", endpoint, params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseBinanceAPIError(resp.StatusCode, body)
	}

	var binanceResp struct {
		OrderID       int64  `json:"orderId"`
		Symbol        string `json:"symbol"`
		Status        string `json:"status"`
		ExecutedQty   string `json:"executedQty"`
		AvgPrice      string `json:"avgPrice"`
	}

	if err := json.Unmarshal(body, &binanceResp); err != nil {
		return nil, err
	}

	executedQty, _ := strconv.ParseFloat(binanceResp.ExecutedQty, 64)
	avgPrice, _ := strconv.ParseFloat(binanceResp.AvgPrice, 64)

	return &domain.BinanceOrderResponse{
		OrderID:       binanceResp.OrderID,
		Symbol:        binanceResp.Symbol,
		Status:        binanceResp.Status,
		ExecutedQty:   executedQty,
		ExecutedPrice: avgPrice,
	}, nil
}

// CancelOrder cancels an existing order
func (c *TradingClient) CancelOrder(symbol string, orderID int64) error {
	endpoint := "/fapi/v1/order"
	
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", strconv.FormatInt(orderID, 10))

	resp, err := c.signedRequest("DELETE", endpoint, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return parseBinanceAPIError(resp.StatusCode, body)
	}

	return nil
}

// signedRequest makes a signed API request
func (c *TradingClient) signedRequest(method, endpoint string, params url.Values) (*http.Response, error) {
	if params == nil {
		params = url.Values{}
	}

	// Add timestamp
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	params.Set("timestamp", timestamp)

	// Create signature
	queryString := params.Encode()
	signature := c.sign(queryString)
	params.Set("signature", signature)

	// Build URL
	fullURL := c.baseURL + endpoint + "?" + params.Encode()

	// Create request
	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-MBX-APIKEY", c.apiKey)

	return c.httpClient.Do(req)
}

// sign creates HMAC SHA256 signature
func (c *TradingClient) sign(message string) string {
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}
