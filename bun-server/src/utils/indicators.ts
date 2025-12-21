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

    calculateATR(highs: number[], lows: number[], closes: number[], period: number): number[] {
        const length = closes.length;
        const atr = new Array(length).fill(0);
        if (length < period + 1) return atr;

        const trs = new Array(length).fill(0);
        trs[0] = highs[0] - lows[0];

        for (let i = 1; i < length; i++) {
            const hl = highs[i] - lows[i];
            const hc = Math.abs(highs[i] - closes[i - 1]);
            const lc = Math.abs(lows[i] - closes[i - 1]);
            trs[i] = Math.max(hl, hc, lc);
        }

        let sumTR = 0;
        for (let i = 0; i < period; i++) sumTR += trs[i];
        atr[period - 1] = sumTR / period;

        for (let i = period; i < length; i++) {
            atr[i] = (atr[i - 1] * (period - 1) + trs[i]) / period;
        }
        return atr;
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

    findPivotLows(lows: number[], left: number, right: number): Pivot[] {
        const pivots: Pivot[] = [];
        for (let i = left; i < lows.length - right; i++) {
            const current = lows[i];
            let isPivot = true;

            for (let j = 1; j <= left; j++) if (lows[i - j] <= current) isPivot = false;
            if (isPivot) {
                for (let j = 1; j <= right; j++) if (lows[i + j] <= current) isPivot = false;
            }

            if (isPivot) pivots.push({ index: i, price: current });
        }
        return pivots;
    },

    getNearestSupport(pivots: Pivot[], currentIdx: number): Pivot | null {
        for (let i = pivots.length - 1; i >= 0; i--) {
            if (pivots[i].index < currentIdx) return { ...pivots[i] };
        }
        return null;
    }
};
