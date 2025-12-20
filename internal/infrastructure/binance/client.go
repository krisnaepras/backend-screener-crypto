package binance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

const (
	FapiBaseURL = "https://fapi.binance.com"
	SpotBaseURL = "https://api.binance.com"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{},
	}
}

type Ticker24h struct {
	Symbol             string `json:"symbol"`
	PriceChangePercent string `json:"priceChangePercent"`
	LastPrice          string `json:"lastPrice"`
	QuoteVolume        string `json:"quoteVolume"`
}

type ExchangeInfo struct {
	Symbols []SymbolInfo `json:"symbols"`
}

type SymbolInfo struct {
	Symbol string `json:"symbol"`
	Status string `json:"status"`
}

type PremiumIndex struct {
	LastFundingRate string `json:"lastFundingRate"`
}

// GetActiveTradingSymbols returns symbols with status "TRADING" from Futures API.
func (c *Client) GetActiveTradingSymbols() ([]string, error) {
	resp, err := c.httpClient.Get(FapiBaseURL + "/fapi/v1/exchangeInfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance API error: %d", resp.StatusCode)
	}

	var info ExchangeInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	var active []string
	for _, s := range info.Symbols {
		if s.Status == "TRADING" {
			active = append(active, s.Symbol)
		}
	}
	return active, nil
}

// GetFutures24hrTicker returns 24hr statistics for all markets.
func (c *Client) GetFutures24hrTicker() ([]Ticker24h, error) {
	resp, err := c.httpClient.Get(FapiBaseURL + "/fapi/v1/ticker/24hr")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance API error: %d", resp.StatusCode)
	}

	var tickers []Ticker24h
	if err := json.NewDecoder(resp.Body).Decode(&tickers); err != nil {
		return nil, err
	}
	return tickers, nil
}

// GetKlines returns candlestick data.
// Returns raw interface slice to keep it simple or we can parse to float per field.
// Binance returns: [ [open_time, open, high, low, close, volume, ...], ... ]
// All are nums or strings representing nums.
func (c *Client) GetKlines(symbol, interval string, limit int) ([][]interface{}, error) {
	url := fmt.Sprintf("%s/fapi/v1/klines?symbol=%s&interval=%s&limit=%d", FapiBaseURL, symbol, interval, limit)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance API error: %d", resp.StatusCode)
	}

	var klines [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&klines); err != nil {
		return nil, err
	}
	return klines, nil
}

// GetFundingRate returns the last funding rate for a symbol.
func (c *Client) GetFundingRate(symbol string) (float64, error) {
	url := fmt.Sprintf("%s/fapi/v1/premiumIndex?symbol=%s", FapiBaseURL, symbol)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("binance API error: %d", resp.StatusCode)
	}

	var data PremiumIndex
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}
	
	rate, _ := strconv.ParseFloat(data.LastFundingRate, 64)
	return rate, nil
}
