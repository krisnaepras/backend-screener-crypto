import { CoinData, MarketFeatures } from "../types.d";
import { BinanceService, Ticker24h } from "./binance";
import { Indicators } from "../utils/indicators";
import { join } from "path";
import { appendFile, readFile, mkdir } from "fs/promises";

// Configuration
const WATCHTOWER_INTERVAL = 10 * 1000; // 10 seconds
const DEEP_SCAN_INTERVAL = 2 * 1000; // 2 seconds (process queue faster)
const DEEP_SCAN_BATCH_SIZE = 3; // Process 3 coins per tick
const ACTIVE_SCAN_INTERVAL = 1000; // 1 second (High Frequency for Active Signals)

export class ScreenerService {
    public coins: CoinData[] = [];
    private binance: BinanceService;
    private isRunning = false;

    // Priority Queue for Deep Scan (Set to avoid duplicates)
    private priorityQueue: Set<string> = new Set();

    // Cache for last price to detect sudden moves
    private lastPrices: Map<string, number> = new Map();

    // Cache for 24h stats to avoid re-fetching
    private tickerCache: Map<string, Ticker24h> = new Map();

    constructor() {
        this.binance = new BinanceService();
        this.ensureLogDir();
    }

    private async ensureLogDir() {
        // Robust logging setup with mkdir recursive
        try {
            const logPath = join(process.cwd(), "logs");
            await mkdir(logPath, { recursive: true });
        } catch (e) {
            // Ignore if exists
        }
    }

    async getTradeLogs(): Promise<any[]> {
        try {
            const path = join(process.cwd(), "logs", "trades.jsonl");
            const content = await readFile(path, "utf-8");
            return content.trim().split("\n").map(line => JSON.parse(line)).reverse();
        } catch (e) {
            return [];
        }
    }

    start() {
        if (this.isRunning) return;
        this.isRunning = true;

        console.log("🚀 Screener Service Started (Watchtower Mode)");

        // 1. Loop Watchtower (Filter)
        this.runWatchtower();
        setInterval(() => this.runWatchtower(), WATCHTOWER_INTERVAL);

        // 2. Loop Deep Scan (Processor)
        setInterval(() => this.processQueue(), DEEP_SCAN_INTERVAL);

        // 3. Loop High Frequency Monitor (Active Signals)
        setInterval(() => this.runActiveMonitor(), ACTIVE_SCAN_INTERVAL);
    }

    // --- LEVEL 0: WATCHTOWER (Cheap & Fast) ---
    private async runWatchtower() {
        const tickers = await this.binance.get24hTicker();

        let added = 0;
        tickers.forEach(t => {
            const symbol = t.symbol;
            // Filter: Must be USDT Perpetual
            if (!symbol.endsWith("USDT")) return;

            // Update Cache
            this.tickerCache.set(symbol, t);

            const price = parseFloat(t.lastPrice);
            const change24h = parseFloat(t.priceChangePercent);
            const vol = parseFloat(t.quoteVolume);

            // Logic 1: Significant 24h Pump (> 5%)
            let isMoving = change24h > 5;

            // Logic 2: Sudden 10s Pump (> 1% in 10s)
            const lastPrice = this.lastPrices.get(symbol);
            if (lastPrice) {
                const recentMove = Math.abs((price - lastPrice) / lastPrice) * 100;
                if (recentMove > 0.8) isMoving = true;
            }
            this.lastPrices.set(symbol, price);

            // Logic 3: Minimum Liquidity ($10M 24h Vol) - Less noise
            if (vol < 5_000_000) isMoving = false;

            if (isMoving) {
                if (!this.priorityQueue.has(symbol)) {
                    this.priorityQueue.add(symbol);
                    added++;
                }
            }
        });
    }

    // --- LEVEL 1.5: ACTIVE MONITOR (High Frequency) ---
    private async runActiveMonitor() {
        // Filter for coins that are interesting (Status TRIGGER/SETUP or Active Trade)
        const activeCoins = this.coins.filter(c =>
            c.status === "TRIGGER" ||
            c.status === "SETUP" ||
            c.tradeActive
        );

        if (activeCoins.length === 0) return;

        // Process all active coins immediately (1s interval)
        // Using map without Promise.all to avoid blocking the interval if API slightly lags, 
        // but robust enough for Bun.
        activeCoins.forEach(c => this.analyzeSymbol(c.symbol));
    }

    // --- LEVEL 1: DEEP SCAN (Robust Logic) ---
    private async processQueue() {
        if (this.priorityQueue.size === 0) return;

        // Take batch
        const iterator = this.priorityQueue.values();
        const batch: string[] = [];
        for (let i = 0; i < DEEP_SCAN_BATCH_SIZE; i++) {
            const next = iterator.next();
            if (next.done) break;
            batch.push(next.value);
            this.priorityQueue.delete(next.value); // Remove from queue
        }

        await Promise.all(batch.map(s => this.analyzeSymbol(s)));
    }

    private async analyzeSymbol(symbol: string) {
        // 1. Fetch Heavy Data (Reversal Timeframe: 1m for Sniping)
        const klines = await this.binance.getKlines(symbol, "1m", 60); // 1 hour of data context
        if (klines.prices.length < 50) return;

        const openInterest = await this.binance.getOpenInterest(symbol);
        const fundingRate = await this.binance.getFundingRate(symbol);
        const longShortRatio = await this.binance.getTopLongShortAccountRatio(symbol);

        // Fetch 24h Stats from Cache
        const ticker = this.tickerCache.get(symbol);
        const change24h = ticker ? parseFloat(ticker.priceChangePercent) : 0;

        // 2. Prepare Data
        const prices = klines.prices;
        const opens = klines.opens;
        const highs = klines.highs;
        const lows = klines.lows;
        const volumes = klines.volumes;
        const currentIdx = prices.length - 1;

        // 3. Calculate Indicators
        const ema21 = Indicators.calculateEMA(prices, 21);
        const ema50 = Indicators.calculateEMA(prices, 50);
        const ema100 = Indicators.calculateEMA(prices, 100);
        const ema200 = Indicators.calculateEMA(prices, 200);

        const rsi = Indicators.calculateRSI(prices, 14);
        const stoch = Indicators.calculateStochRSI(prices, 14, 3, 3);
        const vwap = Indicators.calculateVWAP(highs, lows, prices, volumes);
        const bb = Indicators.calculateBollingerBands(prices, 20, 2.0);

        // 4. Pattern Recognition & Features
        const curPrice = prices[currentIdx];
        const curEma21 = ema21[currentIdx];
        const curRsi = rsi[currentIdx];

        // A. Volume Analysis
        const volExhaustion = Indicators.detectVolumeExhaustion(volumes, highs, lows, prices, opens);
        const volSpike = volExhaustion.spikeRatio;

        // B. Wick Rejection (> 40% upper wick)
        const cOpen = opens[currentIdx];
        const cClose = prices[currentIdx];
        const cHigh = highs[currentIdx];
        const cLow = lows[currentIdx];

        const bodyTop = Math.max(cOpen, cClose);
        const range = cHigh - cLow;
        const upperWick = cHigh - bodyTop;
        const wickRatio = range > 0 ? (upperWick / range) : 0;
        const isWickRejection = wickRatio > 0.4;

        // C. Bearish Engulfing
        const prevOpen = opens[currentIdx - 1];
        const prevClose = prices[currentIdx - 1];
        const isBearishEngulfing = (prevClose > prevOpen) && // Prev Green
            (cClose < cOpen) &&       // Curr Red
            (cOpen >= prevClose) &&   // Open at/above prev close
            (cClose <= prevOpen);     // Close at/below prev open

        // D. Context Checks
        const isOverboughtStoch = stoch.k[currentIdx] > 80 && stoch.d[currentIdx] > 80;
        const isVWAPExtended = curPrice > vwap[currentIdx] * 1.01; // 1% extension
        const isBollingerRejection = highs[currentIdx] > bb.upper[currentIdx] && curPrice < bb.upper[currentIdx];

        // 5. Construct Features Object
        const isBearishDiv = Indicators.detectBearishDivergence(prices, rsi, 15);

        const features: MarketFeatures = {
            pctChange24h: change24h,
            currentPrice: curPrice,
            ema21: curEma21,
            ema50: ema50[currentIdx],
            ema100: ema100[currentIdx],
            ema200: ema200[currentIdx],
            isUptrend: curEma21 > ema50[currentIdx] && ema50[currentIdx] > ema100[currentIdx],
            distFromEma21: ((curPrice - curEma21) / curEma21) * 100,
            rsi: curRsi,
            isRsiBearishDiv: isBearishDiv,
            isBollingerRejection,
            isRejectionWick: isWickRejection,
            volumeSpike: volSpike,
            isVolumeExhaustion: volExhaustion.isExhaustion,
            openInterest,
            isOIDivergence: false,
            fundingRate,
            longShortRatio
        };

        // 6. Scoring Logic (The "Gold" Logic)
        let score = 0;

        // Context (30pts)
        if (curRsi > 70) score += 10;
        if (isOverboughtStoch) score += 15;
        if (isVWAPExtended) score += 15;

        // Trigger (Price Action) (50pts)
        if (isWickRejection) score += 20;
        if (isBearishEngulfing) score += 25;
        if (isBollingerRejection) score += 10;

        // Volume (20pts)
        if (volSpike > 3) score += 15;
        else if (volSpike > 2) score += 10;

        // Funding & Sentiment
        if (fundingRate > 0.05) score += 5;
        if (longShortRatio > 2.5) score += 10;
        else if (longShortRatio > 1.5) score += 5;

        score = Math.min(score, 100);
        let status = score >= 70 ? "TRIGGER" : score >= 50 ? "SETUP" : "WATCH";

        // 7. Persistence & State Machine
        const existingIdx = this.coins.findIndex(c => c.symbol === symbol);
        const existingCoin = existingIdx !== -1 ? this.coins[existingIdx] : null;

        let tradeActive = existingCoin?.tradeActive || false;
        let tradeEntryPrice = existingCoin?.tradeEntryPrice;
        let tradeStartTime = existingCoin?.tradeStartTime;

        // STATE MACHINE
        if (tradeActive) {
            // Check Exit Conditions (Post-drop Exhaustion)
            const isPriceRecovering = curPrice > curEma21; // Price broke above EMA21
            const isOversold = curRsi < 30; // RSI oversold

            if (isPriceRecovering || isOversold) {
                // EXIT TRADE
                const exitPrice = curPrice;
                const pnlRaw = (tradeEntryPrice! - exitPrice) / tradeEntryPrice!; // Short PnL: (Entry - Exit) / Entry
                const pnlLeverage = pnlRaw * 100 * 50; // 50x

                this.logTrade({
                    id: crypto.randomUUID(),
                    symbol,
                    entryPrice: tradeEntryPrice!,
                    exitPrice,
                    pnl: pnlLeverage,
                    startTime: tradeStartTime!,
                    endTime: Date.now(),
                    exitReason: isPriceRecovering ? "Price > EMA21" : "RSI < 30"
                });

                tradeActive = false;
                tradeEntryPrice = undefined;
                tradeStartTime = undefined;
                status = "NORMAL"; // Cooldown
            } else {
                // MAINTAIN TRADE
                status = "TRIGGER"; // Force lock status
            }
        } else {
            // No active trade, check for Entry
            if (status === "TRIGGER") {
                tradeActive = true;
                tradeEntryPrice = curPrice;
                tradeStartTime = Date.now();
            }
        }

        const coinData: CoinData = {
            symbol,
            price: curPrice,
            score,
            status,
            priceChangePercent: change24h,
            fundingRate,
            features,
            lastUpdated: Date.now(),
            tradeActive,
            tradeEntryPrice,
            tradeStartTime
        };

        if (existingIdx !== -1) {
            this.coins[existingIdx] = coinData;
        } else {
            this.coins.push(coinData);
        }

        this.coins.sort((a, b) => b.score - a.score);
    }

    private async logTrade(log: any) {
        try {
            const path = join(process.cwd(), "logs", "trades.jsonl");
            await appendFile(path, JSON.stringify(log) + "\n");
            console.log(`[TRADE CLOSED] ${log.symbol} PnL: ${log.pnl.toFixed(2)}%`);
        } catch (e) {
            console.error("Failed to log trade:", e);
        }
    }
}
