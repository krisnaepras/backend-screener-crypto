import { useEffect, useState } from "react";
import { CoinData } from "../types";
import { CoinCard } from "../components/CoinCard";
import { Header } from "../components/Header";
import { ArrowUp, ArrowDown, Activity } from "lucide-react";
import clsx from "clsx";

export function Dashboard() {
    const [coins, setCoins] = useState<CoinData[]>([]);
    const [connected, setConnected] = useState(false);

    useEffect(() => {
        let ws: WebSocket;

        const connect = () => {
            const url = window.location.hostname === "localhost"
                ? "ws://localhost:8181/ws"
                // eslint-disable-next-line @typescript-eslint/ban-ts-comment
                // @ts-ignore
                : `ws://${window.location.host}/ws`;

            ws = new WebSocket(url);

            ws.onopen = () => {
                console.log("Connected to Watchtower");
                setConnected(true);
            };

            ws.onmessage = (event) => {
                try {
                    const payload = JSON.parse(event.data);
                    if (payload.type === "initial" || payload.type === "update") {
                        setCoins(payload.data);
                    }
                } catch (e) {
                    console.error("Parse error", e);
                }
            };

            ws.onclose = () => {
                setConnected(false);
                setTimeout(connect, 3000);
            };
        };

        connect();
        return () => ws?.close();
    }, []);

    const triggerCoins = coins.filter(c => c.status === "TRIGGER");
    const setupCoins = coins.filter(c => c.status === "SETUP");
    const displayCoins = [...triggerCoins, ...setupCoins];
    const watchCoins = coins.filter(c => c.status === "WATCH" || c.status === "AVOID");

    return (
        <div>
            <Header title="Dashboard" />

            {/* Top Summary Cards */}
            <div className="grid grid-cols-4 gap-6 mb-10">
                <SummaryCard
                    label="Active Pumps"
                    value={triggerCoins.length}
                    trend="up"
                    color="text-secondary"
                />
                <SummaryCard
                    label="Potential Setups"
                    value={setupCoins.length}
                    trend="neutral"
                    color="text-warning"
                />
                <SummaryCard
                    label="Scanned (1H)"
                    value={coins.length}
                    trend="up"
                    color="text-primary"
                />
                <SummaryCard
                    label="System Status"
                    value={connected ? "Online" : "Offline"}
                    trend={connected ? "up" : "down"}
                    color={connected ? "text-secondary" : "text-danger"}
                />
            </div>

            {/* Active Signals Grid */}
            {(displayCoins.length > 0) && (
                <div className="mb-12">
                    <h2 className="text-xl font-bold text-white mb-6 flex items-center gap-3">
                        <Activity className="text-primary" />
                        Active Signals
                    </h2>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
                        {displayCoins.map(coin => (
                            <CoinCard key={coin.symbol} coin={coin} />
                        ))}
                    </div>
                </div>
            )}

            {/* Live Market Table */}
            <div className="bg-surface rounded-2xl p-6 shadow-card">
                <div className="flex justify-between items-center mb-6">
                    <h2 className="text-xl font-bold text-white">Live Market</h2>
                    <button className="px-4 py-2 rounded-lg border border-white/10 text-xs font-semibold text-text-secondary hover:text-white transition-colors">
                        View More
                    </button>
                </div>

                <div className="overflow-x-auto">
                    <table className="w-full text-left border-collapse">
                        <thead>
                            <tr className="text-text-secondary text-xs uppercase border-b border-white/5">
                                <th className="py-4 px-4 font-normal">Details</th>
                                <th className="py-4 px-4 font-normal">Price</th>
                                <th className="py-4 px-4 font-normal">24h Change</th>
                                <th className="py-4 px-4 font-normal">Score</th>
                                <th className="py-4 px-4 font-normal">Funding</th>
                                <th className="py-4 px-4 font-normal text-right">Action</th>
                            </tr>
                        </thead>
                        <tbody className="text-sm">
                            {watchCoins.slice(0, 10).map(coin => (
                                <tr key={coin.symbol} className="border-b border-white/5 hover:bg-white/5 transition-colors group">
                                    <td className="py-4 px-4">
                                        <div className="flex items-center gap-3">
                                            <div className="w-8 h-8 rounded-full bg-surfaceHighlight flex items-center justify-center text-xs font-bold text-white">
                                                {coin.symbol[0]}
                                            </div>
                                            <div>
                                                <p className="font-semibold text-white">{coin.symbol}</p>
                                                <p className="text-xs text-text-secondary">USDT</p>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="py-4 px-4 font-medium text-white">${coin.price}</td>
                                    <td className="py-4 px-4">
                                        <div className={clsx("flex items-center gap-1", coin.priceChangePercent >= 0 ? "text-secondary" : "text-danger")}>
                                            {coin.priceChangePercent >= 0 ? <ArrowUp size={14} /> : <ArrowDown size={14} />}
                                            {coin.priceChangePercent}%
                                        </div>
                                    </td>
                                    <td className="py-4 px-4">
                                        <span className={clsx("px-2 py-1 rounded-md text-xs font-semibold",
                                            coin.score > 50 ? "bg-primary/20 text-primary" : "bg-white/5 text-text-secondary"
                                        )}>
                                            {coin.score.toFixed(0)}
                                        </span>
                                    </td>
                                    <td className="py-4 px-4 text-text-muted">{(coin.fundingRate * 100).toFixed(4)}%</td>
                                    <td className="py-4 px-4 text-right">
                                        <button className="text-primary hover:text-white text-xs font-semibold">Trade</button>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    );
}

function SummaryCard({ label, value, trend, color }: any) {
    return (
        <div className="bg-surface rounded-2xl p-6 shadow-card hover:-translate-y-1 transition-transform duration-300">
            <div className="flex justify-between items-start mb-4">
                <div className={clsx("w-10 h-10 rounded-xl flex items-center justify-center bg-surfaceHighlight text-white", color)}>
                    {trend === 'up' ? <ArrowUp size={20} /> : <ArrowDown size={20} />}
                </div>
                <span className={clsx("text-sm font-semibold px-2 py-1 rounded bg-white/5", color)}>
                    {trend === 'up' ? '+2.5%' : '-1.2%'}
                </span>
            </div>
            <div>
                <p className="text-2xl font-bold text-white mb-1">{value}</p>
                <p className="text-sm text-text-secondary">{label}</p>
            </div>
        </div>
    );
}
