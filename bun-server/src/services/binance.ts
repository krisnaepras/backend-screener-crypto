const FAPI_BASE_URL = "https://fapi.binance.com";

interface Ticker24h {
    symbol: string;
    priceChangePercent: string;
    lastPrice: string;
    quoteVolume: string;
}

export class BinanceService {
    constructor(private baseUrl: string = FAPI_BASE_URL) { }

    async getActiveSymbols(): Promise<string[]> {
        try {
            const res = await fetch(`${this.baseUrl}/fapi/v1/exchangeInfo`);
            if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
            const data = await res.json() as any;

            return (data.symbols as any[])
                .filter(s => s.status === "TRADING" && s.contractType === "PERPETUAL" && s.quoteAsset === "USDT")
                .map(s => s.symbol);
        } catch (e: any) {
            console.error(`[Binance] getActiveSymbols failed: ${e.message}`);
            // Log more details if connection failed
            if (e.cause) console.error("Cause:", e.cause);
            return [];
        }
    }

    async get24hTicker(): Promise<Map<string, Ticker24h>> {
        try {
            const res = await fetch(`${this.baseUrl}/fapi/v1/ticker/24hr`);
            if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
            const data = await res.json() as Ticker24h[];

            const map = new Map<string, Ticker24h>();
            data.forEach(t => map.set(t.symbol, t));
            return map;
        } catch (e: any) {
            console.error(`[Binance] get24hTicker failed: ${e.message}`);
            return new Map();
        }
    }

    async getKlines(symbol: string, limit = 100): Promise<{ prices: number[], highs: number[], lows: number[] }> {
        try {
            const res = await fetch(`${this.baseUrl}/fapi/v1/klines?symbol=${symbol}&interval=15m&limit=${limit}`);
            if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
            const raw = await res.json() as any[][];

            const prices: number[] = [];
            const highs: number[] = [];
            const lows: number[] = [];

            raw.forEach(k => {
                highs.push(parseFloat(k[2]));
                lows.push(parseFloat(k[3]));
                prices.push(parseFloat(k[4]));
            });

            return { prices, highs, lows };
        } catch (e: any) {
            console.error(`[Binance] getKlines failed for ${symbol}: ${e.message}`);
            return { prices: [], highs: [], lows: [] };
        }
    }

    async getFundingRate(symbol: string): Promise<number> {
        try {
            const res = await fetch(`${this.baseUrl}/fapi/v1/premiumIndex?symbol=${symbol}`);
            if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
            const data = await res.json() as any;
            return parseFloat(data.lastFundingRate);
        } catch (e) {
            return 0;
        }
    }
}
