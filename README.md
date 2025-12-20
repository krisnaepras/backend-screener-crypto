# Screener Backend (Golang WebSocket Server)

Backend service untuk aplikasi Screener Micin - menyediakan data cryptocurrency real-time melalui WebSocket.

## Features

- **WebSocket Stream**: Real-time cryptocurrency data streaming
- **Binance Integration**: Fetch data dari Binance API (klines, ticker 24h)
- **Technical Indicators**: Perhitungan RSI, EMA, Bollinger Bands, ATR, VWAP
- **Multi-Strategy Screening**: 
  - Short 2x Leverage
  - RSI Reversal Timing
  - Scalping Signals

## Architecture

screener-backend/
├── cmd/server/main.go          # Entry point
├── internal/
│   ├── binance/client.go       # Binance API client
│   ├── indicators/             # Technical indicators
│   ├── models/coin.go          # CoinData struct
│   ├── screener/logic.go       # Screening logic
│   └── websocket/hub.go        # WebSocket hub
├── go.mod
├── go.sum
└── server                      # Binary executable

## Endpoints

### WebSocket
- **URL**: ws://localhost:8080/ws
- **Protocol**: WebSocket
- **Data Format**: JSON array of CoinData objects
- **Update Frequency**: ~2 seconds

### Health Check
- **URL**: GET http://localhost:8080/health
- **Response**: {"status":"ok"}

## Data Model (CoinData)

{
  "symbol": "BTCUSDT",
  "price": 98765.43,
  "change": 2.5,
  "volume": 12345678.90,
  "rsi": 65.2,
  "ema20": 98500.0,
  "ema50": 97800.0,
  "atr": 1234.56,
  "vwap": 98600.0,
  "bollingerUpper": 99000.0,
  "bollingerLower": 98000.0,
  "score": 75,
  "status": "TRIGGER",
  "scalpSignal": "LONG",
  "scalpScore": 80,
  "reasons": ["RSI di zona optimal", "EMA bullish cross"]
}

### Status Values
- TRIGGER: Sinyal kuat (score 70-100)
- SETUP: Potensial setup (score 50-69)
- WAIT: Tunggu (score < 50)

### Scalp Signal Values
- LONG: Sinyal scalping long
- SHORT: Sinyal scalping short
- NONE: Tidak ada sinyal

## Build & Run

### Prerequisites
- Go 1.21 or higher
- Internet connection (untuk Binance API)

### Build
go build -o server cmd/server/main.go

### Run
./server

Server akan listen di port 8080.

### Development Mode
go run cmd/server/main.go

## Integration with Flutter App

Flutter app terhubung ke backend melalui WebSocket:

### Android Emulator
// ApiService menggunakan 10.0.2.2 (emulator address untuk localhost)
static const String wsUrl = 'ws://10.0.2.2:8080/ws';

### iOS Simulator
static const String wsUrl = 'ws://localhost:8080/ws';

### Real Device (Same Network)
// Ganti dengan IP lokal komputer backend
static const String wsUrl = 'ws://192.168.1.x:8080/ws';

## Technical Indicators

### RSI (Relative Strength Index)
- Period: 14
- Overbought: > 70
- Oversold: < 30

### EMA (Exponential Moving Average)
- EMA 20: Short-term trend
- EMA 50: Medium-term trend

### Bollinger Bands
- Period: 20
- Standard Deviation: 2

### ATR (Average True Range)
- Period: 14
- Volatility measurement

### VWAP (Volume Weighted Average Price)
- Intraday price benchmark

## Screening Logic

### Short 2x Strategy
- Target: Coins dengan momentum bearish kuat
- Criteria:
  - RSI < 50 (preferensi < 40)
  - Price < EMA20
  - High volatility (ATR)
  - Negative price change

### RSI Reversal
- Overbought: RSI 60-70+ (potential short)
- Oversold: RSI 30-40 (potential long)

### Scalping
- Quick entries (15-30min timeframe)
- Combined indicators (RSI, EMA, Bollinger, VWAP)
- High frequency signals

## Performance

- **Symbols Tracked**: ~200 USDT pairs
- **Update Interval**: 2 seconds
- **API Rate Limit**: Managed with delays
- **Memory**: ~50MB typical usage

## Troubleshooting

### Port Already in Use
# Check process
lsof -i :8080

# Kill process
kill -9 <PID>

### Connection Refused (Flutter)
- Pastikan backend running (lsof -i :8080)
- Check firewall settings
- Verify WebSocket URL (10.0.2.2 untuk Android emulator)

### No Data Streaming
- Check Binance API accessibility
- Verify internet connection
- Check backend logs

## Future Improvements

- Redis caching untuk mengurangi Binance API calls
- PostgreSQL untuk historical data
- Authentication/API keys
- Multiple timeframe support (1m, 5m, 15m, 1h)
- Custom indicator configuration
- REST API endpoints (selain WebSocket)
- Prometheus metrics
- Docker containerization

## License

Proprietary - Internal use only

## Author

Krisna Epras - December 2025
