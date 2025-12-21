export interface MarketFeatures {
    // Momentum
    pctChange24h: number; // 24h change
    currentPrice: number;

    // Trend (EMA Ribbon)
    ema21: number;
    ema50: number;
    ema100: number;
    ema200: number;
    isUptrend: boolean; // 21 > 50 > 100
    distFromEma21: number; // % distance

    // Volatility & Structure
    rsi: number;
    stochK: number; // New
    stochD: number; // New
    isRsiBearishDiv: boolean; // Lower High RSI + Higher High Price
    isBollingerRejection: boolean; // Touched Upper + Close Inside
    isRejectionWick: boolean; // Top Wick > 50% body

    // Volume & Sentiment
    volumeSpike: number; // Ratio vs Avg (e.g. 3.5x)
    isVolumeExhaustion: boolean; // High Vol + Small Body
    openInterest: number;
    isOIDivergence: boolean; // Price Up + OI Down
    fundingRate: number;
    longShortRatio: number; // Top Traders Long/Short Ratio
    isBreakingStructure: boolean; // Setup: Structure Break
}

export interface CoinData {
    symbol: string;
    price: number;
    score: number;
    status: string; // "TRIGGER" | "SETUP" | "WATCH" | "NORMAL"
    priceChangePercent: number;
    fundingRate: number;
    features: MarketFeatures | null;
    lastUpdated: number;

    // Trade Tracking
    tradeActive: boolean;
    tradeEntryPrice?: number;
    tradeStartTime?: number;
    currentPnL?: number; // Live PnL (50x)
}

export interface TradeLog {
    id: string;
    symbol: string;
    entryPrice: number;
    exitPrice: number;
    pnl: number; // 50x leverage
    startTime: number;
    endTime: number;
    exitReason: string;
}
