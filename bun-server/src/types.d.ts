export interface MarketFeatures {
    pctChange24h: number;
    overExtEma: number;
    overExtVwap: number;
    isAboveUpperBand: boolean;
    candleRangeRatio: number;
    rsi: number;
    isRsiBearishDiv: boolean;
    rejectionWickRatio: number;
    fundingRate: number;
    openInterestDelta: number;
    nearestSupport: number | null;
    distToSupportATR: number | null;
    isBreakdown: boolean;
    isRetest: boolean;
    isRetestFail: boolean;
}

export interface CoinData {
    symbol: string;
    price: number;
    score: number;
    status: string; // "SETUP" | "TRIGGER" | "AVOID" | "WATCH"
    priceChangePercent: number;
    fundingRate: number;
    basisSpread: number;
    features: MarketFeatures | null;
}
