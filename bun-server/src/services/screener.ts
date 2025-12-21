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

    async deleteTradeLogs(ids: string[]): Promise<boolean> {
        try {
            const path = join(process.cwd(), "logs", "trades.jsonl");
            let content = "";
            try {
                content = await readFile(path, "utf-8");
            } catch {
                return true; // File doesn't exist, technically deleted
            }

            const logs = content.trim().split("\n").map(line => JSON.parse(line));
            const newLogs = logs.filter(log => !ids.includes(log.id));

            // Rewrite file
            // Note: Bun's write allows overwriting
            await Bun.write(path, newLogs.map(l => JSON.stringify(l)).join("\n") + "\n");
            return true;
        } catch (e) {
            console.error("Failed to delete logs", e);
            return false;
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

    async manualCloseTrade(symbol: string): Promise<boolean> {
        const coin = this.coins.find(c => c.symbol === symbol);
        if (!coin || !coin.tradeActive) return false;

        // Force close logic by analyzing with special flag / just rewriting state?
        // Simpler: Just log it and clear state directly.
        const exitPrice = coin.price;
        const entryPrice = coin.tradeEntryPrice!;
        const pnlRaw = (entryPrice - exitPrice) / entryPrice;
        const pnlLeverage = pnlRaw * 100 * 50;

        await this.logTrade({
            id: crypto.randomUUID(),
            symbol,
            entryPrice,
            exitPrice,
            pnl: pnlLeverage,
            startTime: coin.tradeStartTime!,
            endTime: Date.now(),
            exitReason: "Manual Close (User)"
        });

        coin.tradeActive = false;
        coin.tradeEntryPrice = undefined;
        coin.tradeStartTime = undefined;
        coin.currentPnL = undefined;
        coin.status = "NORMAL";

        return true;
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

    async analyzeOnDemand(symbol: string): Promise<CoinData | null> {
        // Check cache first
        const cached = this.coins.find(c => c.symbol === symbol);
        if (cached) return cached;

        // Otherwise generate fresh analysis
        // Note: We don't save this to this.coins to avoid polluting the "Screener" list
        // We just return the data for the UI
        try {
            return await this.performAnalysis(symbol, false); // false = read-only (don't update state)
        } catch (e) {
            console.error(`Failed to analyze ${symbol} on demand`, e);
            return null;
        }
    }

    private async analyzeSymbol(symbol: string) {
        await this.performAnalysis(symbol, true); // true = update state
    }

    private async performAnalysis(symbol: string, updateState: boolean): Promise<CoinData | null> {
        // 1. Fetch Heavy Data (Reversal Timeframe: 1m for Sniping)
        const klines = await this.binance.getKlines(symbol, "1m", 90); // 90 mins context
        if (klines.prices.length < 50) return null;

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

        // 3. New Strategy: Flash Pump Reversal (Parabolic + Structure Break)

        // Calculate Indicators FIRST (Required for logic)
        const curPrice = prices[currentIdx];
        const ema21 = Indicators.calculateEMA(prices, 21);
        const curEma21 = ema21[currentIdx];
        const realDistFromEma21 = ((curPrice - curEma21) / curEma21) * 100;

        // A. Parabolic Check: Must be vertial pump. 
        // 4% in 60m is just a strong trend. We want PUMPS.
        const price15mAgo = prices[currentIdx - 15] || prices[0];
        const pump15m = ((prices[currentIdx] - price15mAgo) / price15mAgo) * 100;

        const price60mAgo = prices[currentIdx - 60] || prices[0];
        const pump60m = ((prices[currentIdx] - price60mAgo) / price60mAgo) * 100;

        // Thresholds: 4% in 15m OR 10% in 60m OR 2.5% Extended from EMA
        const isParabolic = pump15m > 4.0 || pump60m > 10.0 || realDistFromEma21 > 2.5;

        // B. Structure Break (Micro): Look for a Lower High after a High
        // Simplified: Price is now BELOW the EMA21 after being ABOVE it,
        // AND the recent high was a "spike" (high > bb upper).
        const bb = Indicators.calculateBollingerBands(prices, 20, 2.5); // Wider BB (2.5 SD)

        // Did we spike above BB recently? (Last 5 candles)
        let spikedAboveBB = false;
        for (let i = 0; i < 5; i++) {
            if (highs[currentIdx - i] > bb.upper[currentIdx - i]) {
                spikedAboveBB = true;
                break;
            }
        }

        // Are we breaking structure? (Price closing below EMA21 after spike)
        const isBreakingStructure = spikedAboveBB && curPrice < curEma21;

        // C. Indicators
        const rsi = Indicators.calculateRSI(prices, 14);
        const curRsi = rsi[currentIdx];

        const stoch = Indicators.calculateStochRSI(prices, 14, 3, 3);
        const curStochK = stoch.k[currentIdx];
        const curStochD = stoch.d[currentIdx];

        const volExhaustion = Indicators.detectVolumeExhaustion(volumes, highs, lows, prices, opens);
        const volSpike = volExhaustion.spikeRatio;

        // Wick Rejection Logic (still useful)
        const cOpen = opens[currentIdx];
        const cClose = prices[currentIdx];
        const cHigh = highs[currentIdx];
        const cLow = lows[currentIdx];
        const bodyTop = Math.max(cOpen, cClose);
        const range = cHigh - cLow;
        const upperWick = cHigh - bodyTop;
        const wickRatio = range > 0 ? (upperWick / range) : 0;
        const isWickRejection = wickRatio > 0.5; // Stricter: 50% wick

        // Bearish Engulfing
        const prevOpen = opens[currentIdx - 1];
        const prevClose = prices[currentIdx - 1];
        const isBearishEngulfing = (prevClose > prevOpen) && (cClose < cOpen) && (cOpen >= prevClose) && (cClose <= prevOpen);

        // 5. Construct Features Object
        const features: MarketFeatures = {
            pctChange24h: change24h,
            currentPrice: curPrice,
            ema21: curEma21,
            ema50: 0, // Not vital for this logic
            ema100: 0,
            ema200: 0,
            isUptrend: isParabolic,
            distFromEma21: realDistFromEma21, // Now storing actual distance
            rsi: curRsi,
            stochK: curStochK,
            stochD: curStochD,
            isRsiBearishDiv: false,
            isBollingerRejection: spikedAboveBB,
            isRejectionWick: isWickRejection,
            volumeSpike: volSpike,
            isVolumeExhaustion: volExhaustion.isExhaustion,
            openInterest,
            isOIDivergence: false,
            fundingRate,
            longShortRatio,
            isBreakingStructure
        };

        // 6. Scoring Logic (Sniper Edition)
        let score = 0;

        // MUST BE PARABOLIC to even consider (The "Filter")
        if (!isParabolic) {
            score = 0; // Kill score if not pumping
        } else {
            // Base Score for Context
            if (curRsi > 70) score += 10;
            if (curRsi > 80) score += 10;

            // USER REQUEST CHECK: "RSI 85 + Vol Sell + Ratio Padu"
            // Only award the massive +40 points if we have CONFLUENCE.
            // Avoid blind shorting into a parabolic run.
            const isConfluence = curRsi > 85 && volSpike > 2.5 && (longShortRatio < 0.8 && longShortRatio > 0);

            if (curRsi > 85) {
                if (isConfluence) {
                    score += 40; // Jackpot Setup
                } else {
                    score -= 10; // PENALTY! RSI 85 without Volume/Smart Money is likely a "Runner". DANGEROUS.
                }
            }

            // Stoch RSI Overbought (New Real Logic)
            if (curStochK > 80 && curStochD > 80) score += 15;

            // Smart Money Shorting (Ratio < 0.8)
            if (longShortRatio < 0.8 && longShortRatio > 0) score += 15;

            // Trigger A: Structure Break (Trend Reversal)
            // Price broke below EMA21 after a spike
            if (isBreakingStructure) score += 30;

            // Trigger B: Mean Reversion (Extreme Extension)
            // Price is WAY above EMA21 (>2%) 
            const isExtended = realDistFromEma21 > 2.0;
            const isSuperExtended = realDistFromEma21 > 4.0;

            // CONFIRMATION: Don't short a green rocket.
            // Require at least a red candle OR a rejection wick.
            const isRedCandle = cClose < cOpen;
            const hasResistance = isWickRejection || isBearishEngulfing || isRedCandle;

            if (isSuperExtended && hasResistance) {
                score += 35; // Huge confidence only if we see resistance
            } else if (isExtended && hasResistance) {
                score += 15; // Moderate confidence
            }
            // If it's extended but still a massive green candle with no wick, SCORE 0 for extension.
            // Let it pump until it hits resistance.

            // Candle Confirmation (Synced with UI)
            if (isWickRejection) score += 20; // UI says +20
            if (isBearishEngulfing) score += 25; // UI says +25
            if (volSpike > 2.5) score += 15; // Adjusted to > 2.5
        }

        score = Math.min(score, 100);

        // Stricter Thresholds
        let status = score >= 75 ? "TRIGGER" : score >= 50 ? "SETUP" : "WATCH";

        // 7. Persistence & State Machine (Only if updateState is true)
        const existingIdx = this.coins.findIndex(c => c.symbol === symbol);
        const existingCoin = existingIdx !== -1 ? this.coins[existingIdx] : null;

        let tradeActive = existingCoin?.tradeActive || false;
        let tradeEntryPrice = existingCoin?.tradeEntryPrice;
        let tradeStartTime = existingCoin?.tradeStartTime;

        if (updateState) {
            // STATE MACHINE
            if (tradeActive) {
                // Check Exit Conditions
                const pnlRaw = (tradeEntryPrice! - curPrice) / tradeEntryPrice!;
                const pnlPct = pnlRaw * 100;

                // 1. Dynamic Take Profit (Runners vs Rejection)
                let shouldTakeProfit = false;
                let reason = "";

                // Scenario A: Jackpot Territory (> 3%)
                if (pnlPct > 3.0) {
                    // Check if we should HOLD for more (Greed Mode)
                    const isDumping = volSpike > 2.0 && curPrice < opens[currentIdx];
                    if (!isDumping) {
                        shouldTakeProfit = true;
                        reason = `Take Profit (Target Hit > 3% & Momentum Slowed)`;
                    }
                }
                // Scenario B: Moderate Profit (> 1%) but hitting Support (EMA21)
                else if (pnlPct > 1.0) {
                    const distRaw = Math.abs((curPrice - curEma21) / curEma21) * 100;
                    const isAtSupport = distRaw < 0.5; // Within 0.5% of EMA
                    const isBouncing = curPrice > opens[currentIdx]; // Green Candle

                    if (isAtSupport && isBouncing) {
                        shouldTakeProfit = true;
                        reason = `Scalp Exit (Support at EMA21 + Bounce)`;
                    }
                }

                // 3. Stop Loss
                const stopLossHit = pnlPct < -0.8;

                // 4. Trend Reversal Exit
                const trendReversalFail = curPrice > curEma21 && curPrice > tradeEntryPrice! && pnlPct > 0.1;

                if (shouldTakeProfit || stopLossHit || trendReversalFail) {
                    // EXIT TRADE
                    const exitPrice = curPrice;
                    const pnlLeverage = pnlPct * 50; // 50x

                    if (!reason) { // Fallback reasons
                        if (stopLossHit) reason = "Stop Loss";
                        else if (trendReversalFail) reason = "Trend Reversal";
                    }

                    this.logTrade({
                        id: crypto.randomUUID(),
                        symbol,
                        entryPrice: tradeEntryPrice!,
                        exitPrice,
                        pnl: pnlLeverage,
                        startTime: tradeStartTime!,
                        endTime: Date.now(),
                        exitReason: reason
                    });

                    tradeActive = false;
                    tradeEntryPrice = undefined;
                    tradeStartTime = undefined;
                    status = "NORMAL";
                } else {
                    status = "TRIGGER"; // Maintain lock
                }
            } else {
                if (status === "TRIGGER") {
                    tradeActive = true;
                    tradeEntryPrice = curPrice;
                    tradeStartTime = Date.now();
                }
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

        if (updateState) {
            if (existingIdx !== -1) {
                this.coins[existingIdx] = coinData;
            } else {
                this.coins.push(coinData);
            }

            this.coins.sort((a, b) => b.score - a.score);
        }

        return coinData;
    }

    private async logTrade(log: any) {
        try {
            const path = join(process.cwd(), "logs", "trades.jsonl");
            console.log("Saving trade to:", path); // Debug
            await appendFile(path, JSON.stringify(log) + "\n");
            console.log(`[TRADE CLOSED] ${log.symbol} PnL: ${log.pnl.toFixed(2)}%`);
        } catch (e) {
            console.error("Failed to log trade:", e);
        }
    }
}
