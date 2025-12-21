import { CoinData, MarketFeatures } from "../types.d";
import { BinanceService } from "./binance";
import { Indicators } from "../utils/indicators";
import { join } from "path";
import { appendFile, readFile } from "fs/promises";

// Configuration
const WATCHTOWER_INTERVAL = 10 * 1000; // 10 seconds
const DEEP_SCAN_INTERVAL = 2 * 1000; // 2 seconds (process queue faster)
const DEEP_SCAN_BATCH_SIZE = 3; // Process 3 coins per tick

export class ScreenerService {
    public coins: CoinData[] = [];
    private binance: BinanceService;
    private isRunning = false;

    // Priority Queue for Deep Scan (Set to avoid duplicates)
    private priorityQueue: Set<string> = new Set();

    // Cache for last price to detect sudden moves
    private lastPrices: Map<string, number> = new Map();

    constructor() {
        this.binance = new BinanceService();
        this.ensureLogDir();
    }

    private async ensureLogDir() {
        // Ensure logs directory exists (optional, Bun usually handles write well, but good practice)
        // Ignoring for now, assuming write will work or throw.
    }

    async getTradeLogs(): Promise<any[]> {
        try {
            const path = join(process.cwd(), "trades.jsonl");
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
    }

    // --- LEVEL 0: WATCHTOWER (Cheap & Fast) ---
    private async runWatchtower() {
        // console.log("[Watchtower] Scanning market...");
        const tickers = await this.binance.get24hTicker();

        let added = 0;
        tickers.forEach(t => {
            const symbol = t.symbol;
            // Filter: Must be USDT Perpetual
            if (!symbol.endsWith("USDT")) return;

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

        // console.log(`[Watchtower] Added ${added} coins to Deep Scan Queue. Total in Queue: ${this.priorityQueue.size}`);
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

        // console.log(`[DeepScan] Processing: ${batch.join(", ")}`);

        await Promise.all(batch.map(s => this.analyzeSymbol(s)));
    }

    private async analyzeSymbol(symbol: string) {
        // 1. Fetch Heavy Data (Reversal Timeframe: 1m for Sniping)
        const klines = await this.binance.getKlines(symbol, "1m", 60); // 1 hour of data context
        if (klines.prices.length < 50) return;

        const openInterest = await this.binance.getOpenInterest(symbol);
        const fundingRate = await this.binance.getFundingRate(symbol);

        // 2. Calculate Indicators
        const prices = klines.prices;
        const closes = prices;

        const ema21 = Indicators.calculateEMA(closes, 21);
        const ema50 = Indicators.calculateEMA(closes, 50);
        const ema100 = Indicators.calculateEMA(closes, 100);
        const ema200 = Indicators.calculateEMA(closes, 200);
        const rsi = Indicators.calculateRSI(closes, 14);
        const bb = Indicators.calculateBollingerBands(closes, 20, 2.0);

        // 3. Advanced Detection
        const currentIdx = prices.length - 1;
        const curPrice = prices[currentIdx];
        const curEma21 = ema21[currentIdx];

        // Reversal Logic
        const isBearishDiv = Indicators.detectBearishDivergence(prices, rsi, 15);
        const volExhaustion = Indicators.detectVolumeExhaustion(klines.volumes, klines.highs, klines.lows, klines.prices, klines.opens);
        const isRejectionWick = Indicators.detectRejectionWick(klines.highs[currentIdx], klines.lows[currentIdx], klines.opens[currentIdx], klines.prices[currentIdx]);
        const isBollingerRejection = klines.highs[currentIdx] > bb.upper[currentIdx] && curPrice < bb.upper[currentIdx]; // Touched upper but below now

        // 4. Construct Features
        const features: MarketFeatures = {
            pctChange24h: 0, // Placeholder, usually comes from ticker. But ok.
            currentPrice: curPrice,
            ema21: curEma21,
            ema50: ema50[currentIdx],
            ema100: ema100[currentIdx],
            ema200: ema200[currentIdx],
            isUptrend: curEma21 > ema50[currentIdx] && ema50[currentIdx] > ema100[currentIdx],
            distFromEma21: ((curPrice - curEma21) / curEma21) * 100,
            rsi: rsi[currentIdx],
            isRsiBearishDiv: isBearishDiv,
            isBollingerRejection,
            isRejectionWick,
            volumeSpike: volExhaustion.spikeRatio,
            isVolumeExhaustion: volExhaustion.isExhaustion,
            openInterest,
            isOIDivergence: false, // Need history for this, skipping for now to safe complexity.
            fundingRate
        };

        // 5. Scoring
        const score = this.calculateReversalScore(features);
        let status = score >= 70 ? "TRIGGER" : score >= 50 ? "SETUP" : "WATCH";

        // 6. Persistence & State Machine
        const existingIdx = this.coins.findIndex(c => c.symbol === symbol);
        const existingCoin = existingIdx !== -1 ? this.coins[existingIdx] : null;

        let tradeActive = existingCoin?.tradeActive || false;
        let tradeEntryPrice = existingCoin?.tradeEntryPrice;
        let tradeStartTime = existingCoin?.tradeStartTime;

        // STATE MACHINE
        if (tradeActive) {
            // Check Exit Conditions (Post-drop Exhaustion)
            const isPriceRecovering = curPrice > ema21[currentIdx]; // Price broke above EMA21
            const isOversold = rsi[currentIdx] < 30; // RSI oversold
            const isCooldwn = score < 50; // Use score drop as backup

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
            priceChangePercent: 0,
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

        // Clean up old coins (> 5 min no update)
        // this.coins = this.coins.filter(c => Date.now() - c.lastUpdated < 5 * 60 * 1000);
        this.coins.sort((a, b) => b.score - a.score);
    }

    private async logTrade(log: any) {
        const path = join(process.cwd(), "trades.jsonl");
        await appendFile(path, JSON.stringify(log) + "\n");
        console.log(`[TRADE CLOSED] ${log.symbol} PnL: ${log.pnl.toFixed(2)}%`);
    }

    private calculateReversalScore(f: MarketFeatures): number {
        let s = 0;

        // 1. Overextension (30 pts)
        if (f.distFromEma21 > 3) s += 15; // Price flying away from EMA21
        if (f.distFromEma21 > 5) s += 15;

        // 2. Exhaustion (30 pts)
        if (f.rsi > 70) s += 10;
        if (f.isRsiBearishDiv) s += 20; // GOLD SIGNAL
        if (f.isBollingerRejection) s += 10;

        // 3. Volume/Stress (20 pts)
        if (f.isVolumeExhaustion) s += 20; // GOLD SIGNAL
        else if (f.volumeSpike > 3) s += 10;

        // 4. Funding (20 pts)
        if (f.fundingRate > 0.05) s += 10; // High
        if (f.fundingRate > 0.1) s += 10; // Extreme

        return Math.min(s, 100);
    }
}
