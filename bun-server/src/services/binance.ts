const FAPI_BASE_URL = "https://fapi.binance.com";
const TIMEOUT_MS = 5000;

interface Ticker24h {
    symbol: string;
    priceChangePercent: string;
    lastPrice: string;
    quoteVolume: string; // "50000000.00"
}

export class BinanceService {
    constructor(private baseUrl: string = FAPI_BASE_URL) { }

    private async fetchWithTimeout(url: string, retries = 2): Promise<Response> {
        for (let i = 0; i < retries; i++) {
            try {
                const controller = new AbortController();
                const id = setTimeout(() => controller.abort(), TIMEOUT_MS);

                const res = await fetch(url, { signal: controller.signal });
                clearTimeout(id);

                if (res.ok) return res;
                throw new Error(`${res.status} ${res.statusText}`);
            } catch (e: any) {
                if (i === retries - 1) throw e;
                await new Promise(r => setTimeout(r, 500)); // Backoff
            }
        }
        throw new Error("Fetch failed after retries");
    }

    async getActiveSymbols(): Promise<string[]> {
        try {
            const res = await this.fetchWithTimeout(`${this.baseUrl}/fapi/v1/exchangeInfo`);
            const data = await res.json() as any;

            return (data.symbols as any[])
                .filter(s => s.status === "TRADING" && s.contractType === "PERPETUAL" && s.quoteAsset === "USDT")
                .map(s => s.symbol);
        } catch (e: any) {
            console.error(`[Binance] getActiveSymbols failed: ${e.message}`);
            return [];
        }
    }

    // Weight 40 - Efficient for Bulk Check
    async get24hTicker(): Promise<Ticker24h[]> {
        try {
            const res = await this.fetchWithTimeout(`${this.baseUrl}/fapi/v1/ticker/24hr`);
            return await res.json() as Ticker24h[];
        } catch (e: any) {
            console.error(`[Binance] get24hTicker failed: ${e.message}`);
            return [];
        }
    }

    // Weight 1 - Per Symbol
    async getOpenInterest(symbol: string): Promise<number> {
        try {
            const res = await this.fetchWithTimeout(`${this.baseUrl}/fapi/v1/openInterest?symbol=${symbol}`);
            const data = await res.json() as any;
            // data: { symbol: "BTCUSDT", openInterest: "100.50", time: ... }
            return parseFloat(data.openInterest);
        } catch (e) {
            // console.warn(`[Binance] Failed OI for ${symbol}`);
            return 0;
        }
    }

    async getKlines(symbol: string, limit = 300): Promise<{ prices: number[], highs: number[], lows: number[], volumes: number[], opens: number[] }> {
        try {
            const res = await this.fetchWithTimeout(`${this.baseUrl}/fapi/v1/klines?symbol=${symbol}&interval=15m&limit=${limit}`);
            const raw = await res.json() as any[][];

            // raw[i]: [time, open, high, low, close, volume, ...]
            const prices: number[] = [];
            const highs: number[] = [];
            const lows: number[] = [];
            const volumes: number[] = [];
            const opens: number[] = [];

            raw.forEach(k => {
                opens.push(parseFloat(k[1]));
                highs.push(parseFloat(k[2]));
                lows.push(parseFloat(k[3]));
                prices.push(parseFloat(k[4]));
                volumes.push(parseFloat(k[5]));
            });

            return { prices, highs, lows, volumes, opens };
        } catch (e: any) {
            console.error(`[Binance] getKlines failed for ${symbol}: ${e.message}`);
            return { prices: [], highs: [], lows: [], volumes: [], opens: [] };
        }
    }

    async getFundingRate(symbol: string): Promise<number> {
        try {
            const res = await this.fetchWithTimeout(`${this.baseUrl}/fapi/v1/premiumIndex?symbol=${symbol}`);
            const data = await res.json() as any;
            return parseFloat(data.lastFundingRate);
        } catch (e) {
            return 0;
        }
    }
}
