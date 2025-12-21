import { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { Header } from "../components/Header";
import { ArrowLeft, ArrowUp, ArrowDown, Loader2 } from "lucide-react";
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid, BarChart, Bar, Cell } from 'recharts';
import clsx from "clsx";

export function CoinDetail() {
    const { symbol } = useParams();
    const navigate = useNavigate();
    const [data, setData] = useState<any[]>([]);
    const [timeframe, setTimeframe] = useState("1h");
    const [price, setPrice] = useState("0.00");
    const [change, setChange] = useState(0);

    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const [stats, setStats] = useState<any>(null);

    // New Toggle State
    const [chartType, setChartType] = useState<'area' | 'candle'>('area');

    // Fetch Kline Data from Binance Public API
    useEffect(() => {
        if (!symbol) return;
        setLoading(true);
        setError(null);

        const fetchData = async () => {
            try {
                const interval = timeframe;
                const limit = 200;
                // Ensure we have a valid pair format. Most alts trigger this.
                // If symbol is just "BTC", make it "BTCUSDT". 
                // If it already ends in USDT, keep it.
                // Robustness: Upper case safely.
                const s = symbol.toUpperCase();
                const pair = s.endsWith("USDT") ? s : `${s}USDT`;

                // Fetch Klines (Futures)
                const klineRes = await fetch(`https://fapi.binance.com/fapi/v1/klines?symbol=${pair}&interval=${interval}&limit=${limit}`);

                if (!klineRes.ok) {
                    // Check if it's a symbol error
                    if (klineRes.status === 400) throw new Error("Symbol not found");
                    const errBody = await klineRes.text(); // Debug info
                    throw new Error(`API Error: ${klineRes.status} ${errBody}`);
                }

                const rawKlines = await klineRes.json();

                if (!Array.isArray(rawKlines) || rawKlines.length === 0) {
                    throw new Error("No data returned from Exchange");
                }

                const formatted = rawKlines.map((k: any) => {
                    const open = parseFloat(k[1]);
                    const close = parseFloat(k[4]);
                    return {
                        time: k[0],
                        open,
                        high: parseFloat(k[2]),
                        low: parseFloat(k[3]),
                        close,
                        volume: parseFloat(k[5]),
                        // Prepare body range for Recharts Bar
                        // Note: Recharts <Bar> with [min, max] requires the dataKey to point to an array
                        candleBody: [Math.min(open, close), Math.max(open, close)],
                        isUp: close >= open
                    };
                });

                setData(formatted);
                const last = formatted[formatted.length - 1];
                setPrice(last.close.toFixed(last.close < 1 ? 4 : 2));

                // Fetch 24h Ticker Stats (Futures)
                const tickerRes = await fetch(`https://fapi.binance.com/fapi/v1/ticker/24hr?symbol=${pair}`);
                if (tickerRes.ok) {
                    const ticker = await tickerRes.json();
                    setStats({
                        volume: parseFloat(ticker.quoteVolume), // USDT Volume
                        count: parseInt(ticker.count),
                        priceChangePercent: parseFloat(ticker.priceChangePercent),
                        high: parseFloat(ticker.highPrice),
                        low: parseFloat(ticker.lowPrice)
                    });
                    setChange(parseFloat(ticker.priceChangePercent));
                } else {
                    // Fallback calculation if ticker fails but klines work
                    const first = formatted[0];
                    const pct = ((last.close - first.close) / first.close) * 100;
                    setChange(pct);
                }

                setLoading(false);

            } catch (e: any) {
                console.error("Failed to fetch data", e);
                let msg = e.message || "Failed to load market data";
                // Common issue: Binance 400 doesn't always have CORS, causing "Failed to fetch"
                if (msg === "Failed to fetch") {
                    msg = "Symbol not found (or Network Error)";
                }
                setError(msg);
                setLoading(false);
            }
        };

        fetchData();
        const intervalId = setInterval(fetchData, 10000); // Poll every 10s
        return () => clearInterval(intervalId);
    }, [symbol, timeframe]);

    const timeframes = ["1m", "5m", "15m", "1h", "4h", "1d", "1w"];
    const fmtMoney = (n: number) => new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 0 }).format(n);

    // Helper to show the actual pair we are trying
    const getDisplayPair = () => {
        if (!symbol) return "";
        const s = symbol.toUpperCase();
        return s.endsWith("USDT") ? s : `${s}USDT`;
    };

    return (
        <div className="space-y-6">
            <div className="flex items-center gap-4">
                <button onClick={() => navigate(-1)} className="p-2 rounded-lg bg-surfaceHighlight hover:text-white text-text-secondary transition-colors">
                    <ArrowLeft size={20} />
                </button>
                <Header title={`${symbol?.toUpperCase()} Analysis`} />
            </div>

            {/* Main Chart Card */}
            <div className="bg-surface rounded-2xl p-6 shadow-card h-[600px] flex flex-col">
                <div className="flex justify-between items-start mb-6">
                    <div>
                        <div className="flex items-baseline gap-4">
                            <h2 className="text-4xl font-bold text-white">{!error ? `$${price}` : '---'}</h2>
                            {!loading && !error && (
                                <span className={clsx("flex items-center text-lg font-medium", change >= 0 ? "text-secondary" : "text-danger")}>
                                    {change >= 0 ? <ArrowUp size={20} /> : <ArrowDown size={20} />}
                                    {Math.abs(change).toFixed(2)}%
                                </span>
                            )}
                        </div>
                        <p className="text-text-secondary mt-1">Real-time market composition</p>
                    </div>

                    <div className="flex items-center gap-4">
                        {/* Chart Type Toggle */}
                        <div className="flex bg-surfaceHighlight/30 rounded-lg p-1">
                            <button
                                onClick={() => setChartType('area')}
                                className={clsx(
                                    "px-3 py-1.5 rounded-md text-sm font-medium transition-all",
                                    chartType === 'area' ? "bg-white/10 text-white shadow-sm" : "text-text-secondary hover:text-white"
                                )}
                            >
                                Line
                            </button>
                            <button
                                onClick={() => setChartType('candle')}
                                className={clsx(
                                    "px-3 py-1.5 rounded-md text-sm font-medium transition-all",
                                    chartType === 'candle' ? "bg-white/10 text-white shadow-sm" : "text-text-secondary hover:text-white"
                                )}
                            >
                                Candle
                            </button>
                        </div>

                        {/* Timeframes */}
                        <div className="flex bg-surfaceHighlight/30 rounded-lg p-1">
                            {timeframes.map((tf) => (
                                <button
                                    key={tf}
                                    onClick={() => setTimeframe(tf)}
                                    className={clsx(
                                        "px-4 py-1.5 rounded-md text-sm font-medium transition-all",
                                        timeframe === tf ? "bg-primary text-white shadow-sm" : "text-text-secondary hover:text-white"
                                    )}
                                >
                                    {tf.toUpperCase()}
                                </button>
                            ))}
                        </div>
                    </div>
                </div>

                <div className="flex-1 w-full min-h-0 relative">
                    {loading && (
                        <div className="absolute inset-0 flex flex-col items-center justify-center bg-surface/50 z-10">
                            <Loader2 className="animate-spin text-primary mb-2" size={32} />
                            <p className="text-sm text-text-secondary">Loading market data...</p>
                        </div>
                    )}

                    {error && (
                        <div className="absolute inset-0 flex flex-col items-center justify-center bg-surface/50 z-10">
                            <p className="text-danger font-medium mb-1">Unable to Load Chart</p>
                            <p className="text-xs text-text-muted">{error}</p>
                            <p className="text-xs text-text-muted mt-2 font-mono bg-white/5 px-2 py-1 rounded">PAIR: {getDisplayPair()}</p>
                        </div>
                    )}

                    {!loading && !error && (
                        <ResponsiveContainer width="100%" height="100%">
                            {chartType === 'area' ? (
                                <AreaChart data={data}>
                                    <defs>
                                        <linearGradient id="colorPrice" x1="0" y1="0" x2="0" y2="1">
                                            <stop offset="5%" stopColor="#3A6FF8" stopOpacity={0.3} />
                                            <stop offset="95%" stopColor="#3A6FF8" stopOpacity={0} />
                                        </linearGradient>
                                    </defs>
                                    <CartesianGrid strokeDasharray="3 3" stroke="#ffffff10" vertical={false} />
                                    <XAxis
                                        dataKey="time"
                                        tickFormatter={(t) => new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                                        stroke="#9E9E9E"
                                        tick={{ fill: '#9E9E9E', fontSize: 12 }}
                                        tickLine={false}
                                        axisLine={false}
                                        minTickGap={50}
                                    />
                                    <YAxis
                                        domain={['auto', 'auto']}
                                        orientation="right"
                                        stroke="#9E9E9E"
                                        tick={{ fill: '#9E9E9E', fontSize: 12 }}
                                        tickLine={false}
                                        axisLine={false}
                                        tickFormatter={(val) => val.toFixed(2)}
                                    />
                                    <Tooltip
                                        contentStyle={{ backgroundColor: '#1B2028', borderColor: '#31353F', borderRadius: '12px' }}
                                        itemStyle={{ color: '#fff' }}
                                        labelFormatter={(t) => new Date(t).toLocaleString()}
                                    />
                                    <Area
                                        type="monotone"
                                        dataKey="close"
                                        stroke="#3A6FF8"
                                        strokeWidth={2}
                                        fillOpacity={1}
                                        fill="url(#colorPrice)"
                                    />
                                </AreaChart>
                            ) : (
                                <BarChart data={data}>
                                    <CartesianGrid strokeDasharray="3 3" stroke="#ffffff10" vertical={false} />
                                    <XAxis
                                        dataKey="time"
                                        tickFormatter={(t) => new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                                        stroke="#9E9E9E"
                                        tick={{ fill: '#9E9E9E', fontSize: 12 }}
                                        tickLine={false}
                                        axisLine={false}
                                        minTickGap={50}
                                    />
                                    <YAxis
                                        domain={['auto', 'auto']}
                                        orientation="right"
                                        stroke="#9E9E9E"
                                        tick={{ fill: '#9E9E9E', fontSize: 12 }}
                                        tickLine={false}
                                        axisLine={false}
                                        tickFormatter={(val) => val.toFixed(2)}
                                    />
                                    <Tooltip
                                        cursor={{ fill: 'transparent' }}
                                        contentStyle={{ backgroundColor: '#1B2028', borderColor: '#31353F', borderRadius: '12px' }}
                                        itemStyle={{ color: '#fff' }}
                                        labelFormatter={(t) => new Date(t).toLocaleString()}
                                    />
                                    <Bar dataKey="candleBody" isAnimationActive={false}>
                                        {data.map((entry, index) => (
                                            <Cell key={`cell-${index}`} fill={entry.isUp ? '#1ECB4F' : '#F46D22'} />
                                        ))}
                                    </Bar>
                                </BarChart>
                            )}
                        </ResponsiveContainer>
                    )}
                </div>
            </div>

            {/* Scorecard Analysis */}
            {!loading && !error && (
                <Scorecard symbol={symbol || ""} />
            )}

            {/* Stats Grid - Only show if data is loaded */}
            {!loading && !error && stats && (
                <div className="grid grid-cols-3 gap-6">
                    <div className="bg-surface p-6 rounded-2xl shadow-card">
                        <p className="text-text-secondary text-sm mb-2">24h Volume (USDT)</p>
                        <p className="text-xl font-bold text-white">{fmtMoney(stats.volume)}</p>
                    </div>
                    <div className="bg-surface p-6 rounded-2xl shadow-card">
                        <p className="text-text-secondary text-sm mb-2">24h High</p>
                        <p className="text-xl font-bold text-white">${stats.high}</p>
                    </div>
                    <div className="bg-surface p-6 rounded-2xl shadow-card">
                        <p className="text-text-secondary text-sm mb-2">24h Low</p>
                        <p className="text-xl font-bold text-white">${stats.low}</p>
                    </div>
                </div>
            )}
        </div>
    );
}

function Scorecard({ symbol }: { symbol: string }) {
    const [coin, setCoin] = useState<any>(null);

    useEffect(() => {
        // Fetch coin analysis from backend
        fetch(`/api/coin/${symbol}`)
            .then(res => res.json())
            .then(data => setCoin(data))
            .catch(e => console.error("Failed to fetch coin analysis", e));
    }, [symbol]);

    if (!coin || !coin.features) return null;

    const f = coin.features;
    const s = coin.score;

    return (
        <div className="bg-surface rounded-2xl p-6 shadow-card">
            <h3 className="text-lg font-bold text-white mb-4 flex items-center justify-between">
                <span>Signal Analysis</span>
                <span className={clsx("px-3 py-1 rounded-lg text-sm", s >= 70 ? "bg-danger text-white" : "bg-warning text-white")}>
                    Score: {s}/100
                </span>
            </h3>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {/* Context */}
                <ReasonCard label="RSI (14)" value={f.rsi?.toFixed(1)} points={f.rsi > 70 ? "+10" : "0"} triggered={f.rsi > 70} />
                <ReasonCard label="Stoch RSI" value={f.rsi > 80 ? "Overbought" : "Normal"} points={f.rsi > 80 ? "+15" : "0"} triggered={f.rsi > 80} />
                <ReasonCard label="L/S Ratio" value={f.longShortRatio?.toFixed(2)} points={f.longShortRatio < 0.8 && f.longShortRatio > 0 ? "+15" : "0"} triggered={f.longShortRatio < 0.8 && f.longShortRatio > 0} />

                {/* Price Action */}
                <ReasonCard label="Wick Rejection" value={f.isRejectionWick ? "YES" : "NO"} points={f.isRejectionWick ? "+20" : "0"} triggered={f.isRejectionWick} />
                <ReasonCard label="Bearish Engulfing" value={f.isBearishEngulfing ? "YES" : "NO"} points={f.isBearishEngulfing ? "+25" : "0"} triggered={f.isBearishEngulfing} />
                <ReasonCard label="Volume Spike" value={`${f.volumeSpike?.toFixed(1)}x`} points={f.volumeSpike > 2 ? "+10" : "0"} triggered={f.volumeSpike > 2} />
            </div>
        </div>
    );
}

const ReasonCard = ({ label, value, points, triggered }: any) => (
    <div className={clsx("p-4 rounded-xl border flex justify-between items-center",
        triggered ? "bg-danger/10 border-danger/30" : "bg-background/50 border-white/5"
    )}>
        <div>
            <p className="text-xs text-text-secondary uppercase font-semibold">{label}</p>
            <p className={clsx("text-lg font-bold", triggered ? "text-white" : "text-text-muted")}>{value}</p>
        </div>
        {triggered && (
            <span className="text-danger font-bold text-sm bg-danger/10 px-2 py-1 rounded">
                {points}
            </span>
        )}
    </div>
);
