import { useEffect, useState } from "react";
import { Header } from "../components/Header";
import { ArrowUp, ArrowDown } from "lucide-react";
import clsx from "clsx";

interface TradeLog {
    id: string;
    symbol: string;
    entryPrice: number;
    exitPrice: number;
    pnl: number;
    startTime: number;
    endTime: number;
    exitReason: string;
}

export function Logs() {
    const [logs, setLogs] = useState<TradeLog[]>([]);

    useEffect(() => {
        fetch("/api/logs")
            .then(res => res.json())
            .then(data => setLogs(data))
            .catch(err => console.error("Failed to fetch logs", err));
    }, []);

    return (
        <div>
            <Header title="Trade Logs" />

            <div className="bg-surface rounded-2xl p-6 shadow-card">
                <div className="flex justify-between items-center mb-6">
                    <h2 className="text-xl font-bold text-white">History (50x Leverage)</h2>
                </div>

                <div className="overflow-x-auto">
                    <table className="w-full text-left border-collapse">
                        <thead>
                            <tr className="text-text-secondary text-xs uppercase border-b border-white/5">
                                <th className="py-4 px-4 font-normal">Date</th>
                                <th className="py-4 px-4 font-normal">Symbol</th>
                                <th className="py-4 px-4 font-normal">Entry</th>
                                <th className="py-4 px-4 font-normal">Exit</th>
                                <th className="py-4 px-4 font-normal">Reason</th>
                                <th className="py-4 px-4 font-normal text-right">PnL</th>
                            </tr>
                        </thead>
                        <tbody className="text-sm">
                            {logs.map(log => (
                                <tr key={log.id} className="border-b border-white/5 hover:bg-white/5 transition-colors">
                                    <td className="py-4 px-4 text-text-muted">
                                        {new Date(log.endTime).toLocaleString()}
                                    </td>
                                    <td className="py-4 px-4 font-bold text-white">{log.symbol}</td>
                                    <td className="py-4 px-4 font-mono">${log.entryPrice.toFixed(4)}</td>
                                    <td className="py-4 px-4 font-mono">${log.exitPrice.toFixed(4)}</td>
                                    <td className="py-4 px-4 text-xs text-text-secondary w-32 truncate">{log.exitReason}</td>
                                    <td className="py-4 px-4 text-right">
                                        <div className={clsx(
                                            "flex items-center justify-end gap-1 font-bold",
                                            log.pnl >= 0 ? "text-secondary" : "text-danger"
                                        )}>
                                            {log.pnl >= 0 ? "+" : ""}{log.pnl.toFixed(2)}%
                                        </div>
                                    </td>
                                </tr>
                            ))}
                            {logs.length === 0 && (
                                <tr>
                                    <td colSpan={6} className="py-8 text-center text-text-muted italic">
                                        No trade history yet.
                                    </td>
                                </tr>
                            )}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    );
}
