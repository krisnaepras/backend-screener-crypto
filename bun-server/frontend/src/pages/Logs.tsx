import { useEffect, useState } from "react";
import { Header } from "../components/Header";
import { Trash2, ChevronLeft, ChevronRight, CheckSquare, Square } from "lucide-react";
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
    const [selected, setSelected] = useState<Set<string>>(new Set());
    const [page, setPage] = useState(1);
    const ROWS_PER_PAGE = 10;

    useEffect(() => {
        fetchLogs();
        const interval = setInterval(fetchLogs, 1000); // Auto-refresh every 1s
        return () => clearInterval(interval);
    }, []);

    const fetchLogs = () => {
        fetch("/api/logs")
            .then(res => res.json())
            .then(data => setLogs(data))
            .catch(err => console.error("Failed to fetch logs", err));
    };

    const handleDelete = async () => {
        if (selected.size === 0) return;
        if (!confirm(`Delete ${selected.size} logs?`)) return;

        try {
            await fetch("/api/logs", {
                method: "DELETE",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ ids: Array.from(selected) })
            });
            setSelected(new Set());
            fetchLogs();
        } catch (e) {
            console.error("Failed to delete", e);
        }
    };

    const toggleSelect = (id: string) => {
        const newSelected = new Set(selected);
        if (newSelected.has(id)) newSelected.delete(id);
        else newSelected.add(id);
        setSelected(newSelected);
    };

    const toggleAll = () => {
        if (selected.size === paginatedLogs.length) {
            setSelected(new Set());
        } else {
            const newSelected = new Set(selected);
            paginatedLogs.forEach(l => newSelected.add(l.id));
            setSelected(newSelected);
        }
    };

    // Pagination Logic
    const totalPages = Math.ceil(logs.length / ROWS_PER_PAGE);
    const paginatedLogs = logs.slice((page - 1) * ROWS_PER_PAGE, page * ROWS_PER_PAGE);

    // Stats Logic
    const totalPnL = logs.reduce((acc, log) => acc + log.pnl, 0);
    const winCount = logs.filter(l => l.pnl > 0).length;
    const winRate = logs.length > 0 ? (winCount / logs.length) * 100 : 0;

    return (
        <div>
            <Header title="Trade History" />

            {/* Summary Stats */}
            <div className="grid grid-cols-3 gap-6 mb-8">
                <StatCard label="Total PnL" value={`${totalPnL.toFixed(2)}%`} color={totalPnL >= 0 ? "text-secondary" : "text-danger"} />
                <StatCard label="Win Rate" value={`${winRate.toFixed(1)}%`} color={winRate > 50 ? "text-secondary" : "text-warning"} />
                <StatCard label="Total Trades" value={logs.length.toString()} color="text-white" />
            </div>

            <div className="bg-surface rounded-2xl p-6 shadow-card">
                <div className="flex justify-between items-center mb-6">
                    <h2 className="text-xl font-bold text-white flex items-center gap-2">
                        History
                        <span className="text-xs bg-white/10 px-2 py-1 rounded text-text-muted">50x Lev</span>
                    </h2>

                    {selected.size > 0 && (
                        <button
                            onClick={handleDelete}
                            className="flex items-center gap-2 bg-danger/10 text-danger px-4 py-2 rounded-lg hover:bg-danger/20 transition-colors"
                        >
                            <Trash2 size={16} />
                            Delete ({selected.size})
                        </button>
                    )}
                </div>

                <div className="overflow-x-auto">
                    <table className="w-full text-left border-collapse">
                        <thead>
                            <tr className="text-text-secondary text-xs uppercase border-b border-white/5">
                                <th className="py-4 px-4 w-10">
                                    <button onClick={toggleAll} className="text-text-muted hover:text-white">
                                        {selected.size > 0 && selected.size === paginatedLogs.length ? <CheckSquare size={16} /> : <Square size={16} />}
                                    </button>
                                </th>
                                <th className="py-4 px-4 font-normal">Date</th>
                                <th className="py-4 px-4 font-normal">Symbol</th>
                                <th className="py-4 px-4 font-normal">Entry</th>
                                <th className="py-4 px-4 font-normal">Exit</th>
                                <th className="py-4 px-4 font-normal">Reason</th>
                                <th className="py-4 px-4 font-normal text-right">PnL</th>
                            </tr>
                        </thead>
                        <tbody className="text-sm">
                            {paginatedLogs.map(log => (
                                <tr key={log.id} className={clsx("border-b border-white/5 hover:bg-white/5 transition-colors", selected.has(log.id) && "bg-white/5")}>
                                    <td className="py-4 px-4">
                                        <button onClick={() => toggleSelect(log.id)} className={clsx("text-text-muted hover:text-white", selected.has(log.id) && "text-primary")}>
                                            {selected.has(log.id) ? <CheckSquare size={16} /> : <Square size={16} />}
                                        </button>
                                    </td>
                                    <td className="py-4 px-4 text-text-muted">
                                        {new Date(log.endTime).toLocaleString()}
                                    </td>
                                    <td className="py-4 px-4 font-bold text-white">{log.symbol}</td>
                                    <td className="py-4 px-4 font-mono">${log.entryPrice.toFixed(4)}</td>
                                    <td className="py-4 px-4 font-mono">${log.exitPrice.toFixed(4)}</td>
                                    <td className="py-4 px-4 text-xs text-text-secondary max-w-[150px] truncate" title={log.exitReason}>{log.exitReason}</td>
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
                                    <td colSpan={7} className="py-8 text-center text-text-muted italic">
                                        No trade history yet.
                                    </td>
                                </tr>
                            )}
                        </tbody>
                    </table>
                </div>

                {/* Pagination Controls */}
                {totalPages > 1 && (
                    <div className="flex justify-between items-center mt-6 border-t border-white/5 pt-4">
                        <span className="text-sm text-text-secondary">
                            Page {page} of {totalPages}
                        </span>
                        <div className="flex gap-2">
                            <button
                                disabled={page === 1}
                                onClick={() => setPage(p => Math.max(1, p - 1))}
                                className="p-2 rounded-lg bg-white/5 hover:bg-white/10 disabled:opacity-50 disabled:cursor-not-allowed text-white"
                            >
                                <ChevronLeft size={20} />
                            </button>
                            <button
                                disabled={page === totalPages}
                                onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                                className="p-2 rounded-lg bg-white/5 hover:bg-white/10 disabled:opacity-50 disabled:cursor-not-allowed text-white"
                            >
                                <ChevronRight size={20} />
                            </button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}

const StatCard = ({ label, value, color }: { label: string, value: string, color: string }) => (
    <div className="bg-surface p-6 rounded-2xl shadow-card">
        <p className="text-text-secondary text-sm mb-1">{label}</p>
        <p className={clsx("text-2xl font-bold", color)}>{value}</p>
    </div>
);
