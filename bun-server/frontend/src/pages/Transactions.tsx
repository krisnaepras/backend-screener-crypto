
import { useEffect, useState } from "react";
import { CoinData, TradeLog } from "../types";
import { Header } from "../components/Header";
import { ArrowUp, ArrowDown, History, Activity, XCircle } from "lucide-react";
import clsx from "clsx";

export function Transactions() {
    const [coins, setCoins] = useState<CoinData[]>([]);
    const [history, setHistory] = useState<TradeLog[]>([]);
    const [connected, setConnected] = useState(false);

    // 1. WebSocket for Active Trades
    useEffect(() => {
        let ws: WebSocket;
        const connect = () => {
            const url = window.location.hostname === "localhost"
                ? "ws://localhost:8181/ws"
                // eslint-disable-next-line @typescript-eslint/ban-ts-comment
                // @ts-ignore
                : `ws://${window.location.host}/ws`;

            ws = new WebSocket(url);
            ws.onopen = () => setConnected(true);
            ws.onmessage = (e) => {
                const p = JSON.parse(e.data);
                if (p.type === "initial" || p.type === "update") setCoins(p.data);
            };
            ws.onclose = () => {
                setConnected(false);
                setTimeout(connect, 3000);
            };
        };
        connect();
        return () => ws?.close();
    }, []);

    // 2. Fetch History
    useEffect(() => {
        fetch("/api/logs").then(res => res.json()).then(setHistory).catch(console.error);
        // Refresh history every 5s
        const int = setInterval(() => {
            fetch("/api/logs").then(res => res.json()).then(setHistory).catch(console.error);
        }, 5000);
        return () => clearInterval(int);
    }, []);

    const activeTrades = coins.filter(c => c.tradeActive);

    const handleClose = async (symbol: string) => {
        if (!confirm(`Force Close ${symbol}?`)) return;
        await fetch(`/api/trade/close/${symbol}`, { method: "POST" });
        // WS will update UI
    };

    return (
        <div>
            <Header title="Transactions" />

            {/* SECTION 1: ACTIVE POSITIONS */}
            <div className="mb-12">
                <h2 className="text-xl font-bold text-white mb-6 flex items-center gap-3">
                    <Activity className="text-secondary animate-pulse" />
                    Active Positions ({activeTrades.length})
                </h2>

                {activeTrades.length === 0 ? (
                    <div className="p-10 rounded-2xl bg-surface border border-white/5 text-center text-text-muted">
                        No active trades running.
                    </div>
                ) : (
                    <div className="bg-surface rounded-2xl overflow-hidden shadow-card border border-white/5">
                        <table className="w-full text-left border-collapse">
                            <thead className="bg-white/5 text-xs uppercase text-text-secondary font-semibold">
                                <tr>
                                    <th className="p-5 font-medium">Symbol</th>
                                    <th className="p-5 font-medium text-right">Entry</th>
                                    <th className="p-5 font-medium text-right">Current</th>
                                    <th className="p-5 font-medium text-right">PnL (50x)</th>
                                    <th className="p-5 font-medium text-right">Action</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-white/5 text-sm">
                                {activeTrades.map(coin => (
                                    <tr key={coin.symbol} className="hover:bg-white/5 transition-colors group">
                                        <td className="p-5">
                                            <div className="flex items-center gap-4">
                                                <div className="w-10 h-10 rounded-xl bg-surfaceHighlight flex items-center justify-center text-white font-bold shadow-md">
                                                    {coin.symbol[0]}
                                                </div>
                                                <div>
                                                    <h3 className="text-base font-bold text-white">{coin.symbol}</h3>
                                                    <span className="text-[10px] bg-danger/10 text-danger border border-danger/20 px-1.5 py-0.5 rounded uppercase font-bold tracking-wider">SHORT</span>
                                                </div>
                                            </div>
                                        </td>
                                        <td className="p-5 text-right font-mono text-text-secondary">
                                            ${coin.tradeEntryPrice?.toFixed(5)}
                                        </td>
                                        <td className="p-5 text-right font-mono font-bold text-white">
                                            ${coin.price.toFixed(5)}
                                        </td>
                                        <td className="p-5 text-right">
                                            <div className="flex flex-col items-end gap-1">
                                                <span className={clsx(
                                                    "text-lg font-mono font-bold tracking-tight",
                                                    (coin.currentPnL || 0) >= 0 ? "text-secondary" : "text-danger"
                                                )}>
                                                    {(coin.currentPnL || 0) > 0 ? "+" : ""}{(coin.currentPnL || 0).toFixed(2)}%
                                                </span>
                                                {/* ROI Preview */}
                                                <span className="text-[10px] text-text-muted">
                                                    Unrealized
                                                </span>
                                            </div>
                                        </td>
                                        <td className="p-5 text-right">
                                            <button
                                                onClick={() => handleClose(coin.symbol)}
                                                className="bg-danger hover:bg-danger/80 text-white px-4 py-2 rounded-lg font-bold text-xs flex items-center gap-2 ml-auto shadow-lg hover:shadow-danger/20 transition-all active:scale-95"
                                            >
                                                <XCircle size={14} />
                                                CLOSE
                                            </button>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}
            </div>

            {/* SECTION 2: HISTORY */}
            <div>
                <h2 className="text-xl font-bold text-white mb-6 flex items-center gap-3">
                    <History className="text-primary" />
                    Trade History
                </h2>
                <div className="bg-surface rounded-2xl overflow-hidden shadow-card border border-white/5">
                    <table className="w-full text-left">
                        <thead className="bg-white/5 text-xs uppercase text-text-secondary font-semibold">
                            <tr>
                                <th className="p-5 font-medium">Time</th>
                                <th className="p-5 font-medium">Symbol</th>
                                <th className="p-5 font-medium text-right">Entry</th>
                                <th className="p-5 font-medium text-right">Exit</th>
                                <th className="p-5 font-medium text-right">PnL</th>
                                <th className="p-5 font-medium">Reason</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-white/5 text-sm">
                            {history.slice(0, 20).map(log => (
                                <tr key={log.id} className="hover:bg-white/5 transition-colors">
                                    <td className="p-5 text-text-muted font-mono text-xs">{new Date(log.endTime).toLocaleTimeString()}</td>
                                    <td className="p-5 font-bold text-white">{log.symbol}</td>
                                    <td className="p-5 text-right font-mono text-text-secondary">${log.entryPrice.toFixed(4)}</td>
                                    <td className="p-5 text-right font-mono text-text-secondary">${log.exitPrice.toFixed(4)}</td>
                                    <td className={clsx("p-5 text-right font-bold font-mono", log.pnl >= 0 ? "text-secondary" : "text-danger")}>
                                        {log.pnl > 0 ? "+" : ""}{log.pnl.toFixed(2)}%
                                    </td>
                                    <td className="p-5">
                                        <span className="px-2 py-1 rounded bg-white/5 text-[10px] text-text-secondary border border-white/10 uppercase tracking-wide">
                                            {log.exitReason}
                                        </span>
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
