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
		return nil, fmt.Errorf("binance API error %d: %s", resp.StatusCode, string(body))
	}

	var orders []map[string]interface{}
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, err
	}

	return orders, nil
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
		return nil, fmt.Errorf("binance API error %d: %s", resp.StatusCode, string(body))
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
		return fmt.Errorf("binance API error %d: %s", resp.StatusCode, string(body))
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
