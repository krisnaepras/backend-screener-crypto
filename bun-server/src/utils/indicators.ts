export interface BollingerBands {
    upper: number[];
    middle: number[];
    lower: number[];
}

export interface Pivot {
    index: number;
    price: number;
}

export const Indicators = {
    calculateEMA(data: number[], period: number): number[] {
        const ema = new Array(data.length).fill(0);
        if (data.length < period) return ema;

        const k = 2.0 / (period + 1.0);
        let sum = 0.0;
        for (let i = 0; i < period; i++) sum += data[i];
        ema[period - 1] = sum / period;

        for (let i = period; i < data.length; i++) {
            ema[i] = data[i] * k + ema[i - 1] * (1 - k);
        }
        return ema;
    },

    calculateRSI(closes: number[], period: number): number[] {
        const rsi = new Array(closes.length).fill(0);
        if (closes.length < period + 1) return rsi;

        const gains: number[] = [];
        const losses: number[] = [];

        for (let i = 1; i < closes.length; i++) {
            const change = closes[i] - closes[i - 1];
            if (change > 0) {
                gains.push(change);
                losses.push(0);
            } else {
                gains.push(0);
                losses.push(-change);
            }
        }

        if (gains.length < period) return rsi;

        let sumGain = 0;
        let sumLoss = 0;
        for (let i = 0; i < period; i++) {
            sumGain += gains[i];
            sumLoss += losses[i];
        }

        let avgGain = sumGain / period;
        let avgLoss = sumLoss / period;

        const calculate = (g: number, l: number) => {
            if (l === 0) return 100;
            const rs = g / l;
            return 100 - 100 / (1 + rs);
        };

        rsi[period] = calculate(avgGain, avgLoss);

        for (let i = period + 1; i < closes.length; i++) {
            avgGain = (avgGain * (period - 1) + gains[i - 1]) / period;
            avgLoss = (avgLoss * (period - 1) + losses[i - 1]) / period;
            rsi[i] = calculate(avgGain, avgLoss);
        }

        return rsi;
    },

    calculateBollingerBands(closes: number[], period: number, multiplier: number): BollingerBands {
        const length = closes.length;
        const res = {
            upper: new Array(length).fill(0),
            middle: new Array(length).fill(0),
            lower: new Array(length).fill(0)
        };

        if (length < period) return res;

        for (let i = period - 1; i < length; i++) {
            let sum = 0;
            for (let j = 0; j < period; j++) sum += closes[i - j];
            const ma = sum / period;
            res.middle[i] = ma;

            let sumSqDiff = 0;
            for (let j = 0; j < period; j++) {
                const diff = closes[i - j] - ma;
                sumSqDiff += diff * diff;
            }

            const stdDev = period > 1 ? Math.sqrt(sumSqDiff / period) : 0;
            res.upper[i] = ma + multiplier * stdDev;
            res.lower[i] = ma - multiplier * stdDev;
        }
        return res;
    },

    // --- NEW ADVANCED LOGIC ---

    detectBearishDivergence(prices: number[], rsi: number[], lookback: number = 10): boolean {
        // Simple Pivot Logic:
        // Look for Price Higher High AND RSI Lower High in the last 'lookback' candles
        // Focusing on the most recent peak vs previous peak

        if (prices.length < lookback || rsi.length < lookback) return false;

        const curIdx = prices.length - 1;
        const curPrice = prices[curIdx];
        const curRSI = rsi[curIdx];

        // Only check if current RSI is somewhat elevated (e.g. > 60)
        if (curRSI < 60) return false;

        // Find previous peak in price
        let prevPricePeak = -1;
        let prevRSIPeak = -1;

        for (let i = curIdx - 2; i > curIdx - lookback; i--) {
            if (prices[i] > prices[i - 1] && prices[i] > prices[i + 1]) {
                // Found a local high
                prevPricePeak = prices[i];
                prevRSIPeak = rsi[i];
                break; // Just compare with the most recent significant high
            }
        }

        if (prevPricePeak !== -1) {
            // Price Higher High: Current > Previous
            // RSI Lower High: Current < Previous
            if (curPrice > prevPricePeak && curRSI < prevRSIPeak) {
                return true;
            }
        }

        return false;
    },

    detectVolumeExhaustion(volumes: number[], highs: number[], lows: number[], closes: number[], opens: number[]): { isExhaustion: boolean, spikeRatio: number } {
        // Logic: Volume is HUGE (> 2x Avg) BUT Candle Body is Small (Doji/Spinning Top)
        const len = volumes.length;
        if (len < 20) return { isExhaustion: false, spikeRatio: 0 };

        const currentVol = volumes[len - 1];

        // Calculate Avg Vol (last 20)
        let sumVol = 0;
        for (let i = len - 21; i < len - 1; i++) sumVol += volumes[i];
        const avgVol = sumVol / 20;

        const spikeRatio = currentVol / (avgVol || 1);

        if (spikeRatio < 2.0) return { isExhaustion: false, spikeRatio };

        // Check Candle Body
        const high = highs[len - 1];
        const low = lows[len - 1];
        const open = opens[len - 1];
        const close = closes[len - 1];

        const range = high - low;
        const body = Math.abs(close - open);

        // If body is less than 30% of total range => Indecision/Stalling
        if (range > 0 && (body / range) < 0.3) {
            return { isExhaustion: true, spikeRatio };
        }

        return { isExhaustion: false, spikeRatio };
    },

    detectRejectionWick(high: number, low: number, open: number, close: number): boolean {
        // Long Upper Wick (> 50% of total range) at the top
        const range = high - low;
        if (range === 0) return false;

        const upperWick = high - Math.max(open, close);
        return (upperWick / range) > 0.5;
    },

    calculateVWAP(highs: number[], lows: number[], closes: number[], volumes: number[]): number[] {
        const vwap: number[] = [];
        let cumVol = 0;
        let cumVolPrice = 0;

        for (let i = 0; i < closes.length; i++) {
            const typPrice = (highs[i] + lows[i] + closes[i]) / 3;
            const vol = volumes[i];

            // For a rolling VWAP (e.g., session based), we should typically reset.
            // But for this simple screener, we'll use a rolling window or continuous accumulation.
            // A common approximation for indicators without session data is an anchored VWAP or just rolling.
            // Let's use a rolling window of recent history to simulate "recent session" relevance, or just simple accumulation if assuming start of array is start of session.
            // Better: Just pure accumulation from start of array (assuming array is reasonably sized, e.g. 100-300 candles).

            cumVol += vol;
            cumVolPrice += typPrice * vol;

            vwap.push(cumVolPrice / (cumVol || 1));
        }
        return vwap;
    },

    calculateStochRSI(closes: number[], period: number = 14, smoothK: number = 3, smoothD: number = 3): { k: number[], d: number[] } {
        const rsi = this.calculateRSI(closes, period);
        const stochRsi: number[] = [];

        // Calculate StochRSI
        for (let i = 0; i < rsi.length; i++) {
            if (i < period) {
                stochRsi.push(0);
                continue;
            }

            const window = rsi.slice(i - period + 1, i + 1);
            const min = Math.min(...window);
            const max = Math.max(...window);

            if (max === min) {
                stochRsi.push(0);
            } else {
                stochRsi.push((rsi[i] - min) / (max - min)); // 0-1 range
            }
        }

        // Smooth K
        const k = this.calculateSMA(stochRsi, smoothK).map(v => v * 100); // 0-100
        const d = this.calculateSMA(k, smoothD);

        return { k, d };
    },

    calculateSMA(data: number[], period: number): number[] {
        const sma = new Array(data.length).fill(0);
        for (let i = period - 1; i < data.length; i++) {
            let sum = 0;
            for (let j = 0; j < period; j++) sum += data[i - j];
            sma[i] = sum / period;
        }
        return sma;
    }
};
