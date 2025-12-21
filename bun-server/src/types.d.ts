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
    isRsiBearishDiv: boolean; // Lower High RSI + Higher High Price
    isBollingerRejection: boolean; // Touched Upper + Close Inside
    isRejectionWick: boolean; // Top Wick > 50% body

    // Volume & Sentiment
    volumeSpike: number; // Ratio vs Avg (e.g. 3.5x)
    isVolumeExhaustion: boolean; // High Vol + Small Body
    openInterest: number;
    isOIDivergence: boolean; // Price Up + OI Down
    fundingRate: number;
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
