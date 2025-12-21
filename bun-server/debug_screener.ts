
import { BinanceService } from "./src/services/binance";
import { Indicators } from "./src/utils/indicators";

async function debugAnalysis(symbol: string) {
    const binance = new BinanceService();
    console.log(`Fetching data for ${symbol}...`);

    const klines = await binance.getKlines(symbol, "1m", 90);
    const prices = klines.prices;
    const volumes = klines.volumes;
    const highs = klines.highs;
    const lows = klines.lows;
    const opens = klines.opens;

    if (prices.length < 50) {
        console.log("Not enough data");
        return;
    }

    const currentIdx = prices.length - 1;
    const curPrice = prices[currentIdx];

    // Indicators
    const ema21 = Indicators.calculateEMA(prices, 21);
    const curEma21 = ema21[currentIdx];
    const realDistFromEma21 = ((curPrice - curEma21) / curEma21) * 100;

    const rsi = Indicators.calculateRSI(prices, 14);
    const curRsi = rsi[currentIdx];

    const stoch = Indicators.calculateStochRSI(prices, 14, 3, 3);
    const curStochK = stoch.k[currentIdx];
    const curStochD = stoch.d[currentIdx];

    const volExhaustion = Indicators.detectVolumeExhaustion(volumes, highs, lows, prices, opens);
    const volSpike = volExhaustion.spikeRatio;

    // L/S Ratio (Mocked or Real if possible)
    let longShortRatio = 0;
    try {
        longShortRatio = await binance.getTopLongShortAccountRatio(symbol);
    } catch (e) {
        console.log("LS Ratio failed, using mock 1.0");
        longShortRatio = 1.0;
    }

    // Parabolic Check
    const price15mAgo = prices[currentIdx - 15] || prices[0];
    const pump15m = ((prices[currentIdx] - price15mAgo) / price15mAgo) * 100;
    const isParabolic = pump15m > 2.5 || realDistFromEma21 > 2.0;

    console.log(`\n--- ANALYSIS FOR ${symbol} ---`);
    console.log(`Price: ${curPrice}`);
    console.log(`EMA21 Ext: ${realDistFromEma21.toFixed(2)}%`);
    console.log(`RSI: ${curRsi.toFixed(1)}`);
    console.log(`Stoch K/D: ${curStochK.toFixed(1)} / ${curStochD.toFixed(1)}`);
    console.log(`Vol Spike: ${volSpike.toFixed(1)}x`);
    console.log(`L/S Ratio: ${longShortRatio.toFixed(2)}`);
    console.log(`Parabolic: ${isParabolic}`);

    // Scoring Simulation
    let score = 0;
    if (isParabolic) {
        if (curRsi > 70) score += 10;
        if (curRsi > 80) score += 10;

        const isConfluence = curRsi > 85 && volSpike > 2.5 && (longShortRatio < 0.8 && longShortRatio > 0);

        if (curRsi > 85) {
            if (isConfluence) {
                score += 40;
                console.log("Triggered: RSI 85 Confluence (+40)");
            } else {
                score -= 10;
                console.log("Triggered: RSI 85 Penalty (-10)");
            }
        }

        if (curStochK > 80 && curStochD > 80) {
            score += 15;
            console.log("Triggered: Stoch OB (+15)");
        }

        if (longShortRatio < 0.8 && longShortRatio > 0) {
            score += 15;
            console.log("Triggered: Smart Money Short (+15)");
        }

        const isExtended = realDistFromEma21 > 2.0;
        const isSuperExtended = realDistFromEma21 > 4.0;

        if (isSuperExtended) {
            score += 35;
            console.log("Triggered: Super Extended (+35)");
        } else if (isExtended) {
            score += 15;
            console.log("Triggered: Extended (+15)");
        }

        if (volSpike > 2.5) {
            score += 15;
            console.log("Triggered: Vol Spike (+15)");
        }
    }

    console.log(`\nTOTAL SCORE: ${score}`);
}

const symbol = process.argv[2] || "XPINUSDT";
debugAnalysis(symbol);
