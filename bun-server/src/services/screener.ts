import { CoinData, MarketFeatures } from "../types.d";
import { BinanceService } from "./binance";
import { Indicators } from "../utils/indicators";

export class ScreenerService {
    // Simple in-memory storage
    public coins: CoinData[] = [];
    private binance: BinanceService;
    private isRunning = false;

    constructor() {
        this.binance = new BinanceService();
    }

    start() {
        if (this.isRunning) return;
        this.isRunning = true;
        this.process(); // Run immediately
        setInterval(() => this.process(), 60 * 1000); // And every minute
    }

    private async process() {
        console.log("[Screener] Starting cycle...");
        const start = performance.now();

        const symbols = await this.binance.getActiveSymbols();
        const tickerMap = await this.binance.get24hTicker();

        const activeSymbols = symbols.filter(s => tickerMap.has(s));
        console.log(`[Screener] Analyzing ${activeSymbols.length} active symbols`);

        const results: CoinData[] = [];
        const MAX_CONCURRENT = 10;

        // Process in chunks
        for (let i = 0; i < activeSymbols.length; i += MAX_CONCURRENT) {
            const chunk = activeSymbols.slice(i, i + MAX_CONCURRENT);
            await Promise.all(chunk.map(s => this.analyze(s, tickerMap.get(s)! as any, results)));
        }

        results.sort((a, b) => b.score - a.score);
        this.coins = results; // Atomic update of reference

        const duration = ((performance.now() - start) / 1000).toFixed(2);
        console.log(`[Screener] Cycle done in ${duration}s. Processed: ${results.length}`);
    }

    private async analyze(symbol: string, ticker: any, results: CoinData[]) {
        const { prices, highs, lows } = await this.binance.getKlines(symbol);
        if (prices.length === 0) return;

        const funding = await this.binance.getFundingRate(symbol);

        // Calculate Indicators
        const ema50 = Indicators.calculateEMA(prices, 50);
        const rsi = Indicators.calculateRSI(prices, 14);
        const atr = Indicators.calculateATR(highs, lows, prices, 14);
        const bb = Indicators.calculateBollingerBands(prices, 20, 2.0);
        const pivots = Indicators.findPivotLows(lows, 5, 2);

        // Scoring Logic
        const currentClose = prices[prices.length - 1];
        const features = this.extractFeatures(
            currentClose, highs[highs.length - 1], lows[lows.length - 1],
            ticker, ema50, rsi, bb, atr, pivots, funding
        );

        const score = this.calculateScore(features);

        let status = "AVOID";
        if (score >= 70) status = "TRIGGER";
        else if (score >= 50) status = "SETUP";
        else if (score >= 20) status = "WATCH";

        if (status !== "AVOID" || score > 10) { // Keep some weak ones just for debug/list
            results.push({
                symbol,
                price: currentClose,
                score,
                status,
                priceChangePercent: features.pctChange24h,
                fundingRate: funding,
                basisSpread: 0,
                features
            });
        }
    }

    private extractFeatures(
        close: number, high: number, low: number,
        ticker: any,
        ema50: number[], rsi: number[],
        bb: any, atr: number[], pivots: any[],
        funding: number
    ): MarketFeatures {
        const idx = ema50.length - 1;
        const currentEma = ema50[idx] || 0;
        const currentRsi = rsi[idx] || 50;
        const currentAtr = atr[idx] || 0;
        const upperBand = bb.upper[idx] || 0;

        let overExtEma = 0;
        if (currentEma !== 0) overExtEma = (close - currentEma) / currentEma;

        const nearestSup = Indicators.getNearestSupport(pivots, idx);
        const supportPrice = nearestSup ? nearestSup.price : null;

        let isBrk = false;
        let isRetest = false;
        if (supportPrice && currentAtr > 0) {
            isBrk = close < supportPrice - (0.1 * currentAtr);
            const upper = supportPrice + (0.2 * currentAtr);
            const lower = supportPrice - (0.2 * currentAtr);
            isRetest = low <= upper && high >= lower;
        }

        return {
            pctChange24h: parseFloat(ticker.priceChangePercent),
            overExtEma,
            overExtVwap: 0, // Simplified
            isAboveUpperBand: upperBand !== 0 && close > upperBand,
            candleRangeRatio: 0,
            rsi: currentRsi,
            isRsiBearishDiv: false,
            rejectionWickRatio: 0,
            fundingRate: funding,
            openInterestDelta: 0,
            nearestSupport: supportPrice,
            distToSupportATR: null,
            isBreakdown: isBrk,
            isRetest,
            isRetestFail: false
        };
    }

    private calculateScore(f: MarketFeatures): number {
        let s = 0;
        if (f.pctChange24h >= 40) s += 15;
        else if (f.pctChange24h >= 20) s += 10;

        if (f.overExtEma >= 0.05) s += 10;
        else if (f.overExtEma >= 0.03) s += 5;

        if (f.fundingRate > 0.0001) s += 5;
        if (f.fundingRate > 0.0005) s += 10;

        if (f.rsi > 70) s += 15;
        else if (f.rsi > 60) s += 5;
        if (f.isAboveUpperBand) s += 5;

        if (f.isBreakdown) s += 15;
        if (f.isRetest) s += 10;

        return Math.min(s, 100);
    }
}
