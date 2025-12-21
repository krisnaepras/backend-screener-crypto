# Bun.js Crypto Screener Server

This is a **High-Performance Crypto Screener** ported to [Bun.js](https://bun.sh/). It is designed for Web Applications, providing real-time updates via WebSocket.

## Prerequisites
- **Bun** must be installed: `curl -fsSL https://bun.sh/install | bash`

## Structure
- **Core Logic**: Replicates the Go backend's EMA, RSI, Bollinger Bands, and Scoring logic 100%.
- **Architecture**: Service-based (Idiomatic JS/TS).
- **Transport**: Native `Bun.serve` with WebSocket support.

### Directory Layout
- `src/services/` - Business logic (Screener) and API clients (Binance).
- `src/utils/` - Pure mathematical indicator functions.
- `src/index.ts` - Entry point and server configuration.
- `src/types.d.ts` - TypeScript definitions.

## Running the Server

1. **Install Dependencies** (if any new ones are added, currently zero-dep):
   ```bash
   bun install
   ```

2. **Start Development Mode** (Hot Reload):
   ```bash
   bun run dev
   ```

3. **Start Production Mode**:
   ```bash
   bun start
   ```

The server will start on port **8080** (or `PORT` env var).

## API Endpoints

- **WebSocket**: `ws://localhost:8080/ws`
    - Connect to receive real-time updates.
    - Sends initial data on connect.
    - Broadcasts updates every 5 seconds (configurable).

- **Health Check**: `GET /health`
    - Returns `{"status": "ok"}`

- **Get All Coins (HTTP Snapshot)**: `GET /api/coins`
    - Returns JSON array of all screened coins.
