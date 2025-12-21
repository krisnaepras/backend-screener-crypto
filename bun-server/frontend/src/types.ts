export interface MarketFeatures {
    pctChange24h: number;
    currentPrice: number;

    ema21: number;
    ema50: number;
    ema100: number;
    ema200: number;
    isUptrend: boolean;
    distFromEma21: number;

    rsi: number;
    isRsiBearishDiv: boolean;
    isBollingerRejection: boolean;
    isRejectionWick: boolean;

    volumeSpike: number;
    isVolumeExhaustion: boolean;
    openInterest: number;
    isOIDivergence: boolean;
    fundingRate: number;
}

export interface CoinData {
    symbol: string;
    price: number;
    score: number;
    status: string;
    fundingRate: number;
    priceChangePercent: number;
    features: MarketFeatures | null;
    lastUpdated: number;
    tradeActive?: boolean;
    tradeEntryPrice?: number;
    tradeStartTime?: number;
}
