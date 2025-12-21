const fs = require("fs");

try {
    const raw = fs.readFileSync("power_1m_data.json", "utf8");
    const data = JSON.parse(raw); // [time, open, high, low, close, vol, ...]

    // Convert to objects
    const candles = data.map(k => ({
        time: new Date(k[0]).toISOString().split("T")[1].substring(0, 5),
        open: parseFloat(k[1]),
        high: parseFloat(k[2]),
        low: parseFloat(k[3]),
        close: parseFloat(k[4]),
        vol: parseFloat(k[5])
    }));

    console.log("Time  | Price  | Vol    | Wick% | Change%");
    console.log("-----------------------------------------");

    candles.slice(-20).forEach((c, i, arr) => {
        const bodyTop = Math.max(c.open, c.close);
        const candleRange = c.high - c.low;
        const upperWick = c.high - bodyTop;
        const wickRatio = candleRange > 0 ? (upperWick / candleRange * 100) : 0;

        const prev = arr[i - 1];
        const change = prev ? ((c.close - prev.close) / prev.close * 100) : 0;

        // Simple visual marker for "Rejection"
        const isRejection = wickRatio > 50 && change < 0.5; // Long wick, small body or red

        console.log(`${c.time} | ${c.close.toFixed(4)} | ${(c.vol / 1000).toFixed(1)}k | ${wickRatio.toFixed(0)}%   | ${change.toFixed(2)}% ${isRejection ? "<-- REJECT" : ""}`);
    });

} catch (e) {
    console.error("Error:", e.message);
}
