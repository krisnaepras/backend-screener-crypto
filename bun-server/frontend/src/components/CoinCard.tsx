import { CoinData } from "../types";
import { TrendingUp, Activity, AlertTriangle, ArrowDown, TrendingDown } from "lucide-react";
import clsx from "clsx";

interface Props {
    coin: CoinData;
}

import { Link } from "react-router-dom";

export const CoinCard = ({ coin }: Props) => {
    const isTrigger = coin.status === "TRIGGER";
    const f = coin.features;

    return (
        <Link
            to={`/coin/${coin.symbol}`}
            className={clsx(
                "block bg-surface p-5 rounded-2xl shadow-card relative overflow-hidden group hover:-translate-y-1 transition-all duration-300",
                isTrigger && "ring-1 ring-danger/50"
            )}
        >
            {/* Glow Effect for Trigger */}
            {isTrigger && (
                <>
                    <div className="absolute -right-10 -top-10 w-32 h-32 bg-danger/10 blur-3xl rounded-full" />
                    <div className="absolute top-0 right-0 left-0 bg-danger text-white text-[10px] font-bold text-center py-1 tracking-widest uppercase">
                        Short Now
                    </div>
                </>
            )}

            {/* Header */}
            <div className="flex justify-between items-start mb-4">
                <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-xl bg-surfaceHighlight flex items-center justify-center text-white font-bold">
                        {coin.symbol[0]}
                    </div>
                    <div>
                        <h3 className="text-lg font-bold text-white tracking-wide">{coin.symbol.replace("USDT", "")}</h3>
                        <p className="text-xs text-text-secondary mt-1 font-mono">${coin.price.toFixed(coin.price < 1 ? 4 : 2)}</p>
                    </div>
                </div>
                <div className="flex flex-col items-end">
                    <div className={clsx(
                        "px-2 py-1 rounded-lg text-xs font-bold",
                        coin.score >= 70 ? "bg-danger/20 text-danger" :
                            coin.score >= 50 ? "bg-warning/20 text-warning" : "bg-white/10 text-text-muted"
                    )}>
                        {Math.floor(coin.score)} PTS
                    </div>
                </div>
            </div>

            {/* Price Change & Sparkline Placeholder */}
            <div className="mb-4">
                <div className={clsx("flex items-center gap-1 text-sm font-semibold", coin.priceChangePercent >= 0 ? "text-secondary" : "text-danger")}>
                    {coin.priceChangePercent >= 0 ? <TrendingUp size={16} /> : <TrendingDown size={16} />}
                    {Math.abs(coin.priceChangePercent).toFixed(2)}% (24h)
                </div>
            </div>

            {/* Key Metrics */}
            {f && (
                <div className="grid grid-cols-2 gap-3 mb-4">
                    <Metric label="Vol Spike" value={`${f.volumeSpike?.toFixed(1)}x`} active={f.isVolumeExhaustion} />
                    <Metric label="RSI" value={f.rsi?.toFixed(0)} active={f.isRsiBearishDiv || f.rsi > 70} warning={f.rsi > 70} />
                </div>
            )}
        </Link>
    );
};

// ... Metric and Badge components (Badge used below? No, Badge removed from main view for clean look, metrics sufficient)
const Metric = ({ label, value, active, warning }: { label: string, value: string, active?: boolean, warning?: boolean }) => (
    <div className={clsx(
        "flex flex-col p-2 rounded-lg bg-background/50 border border-white/5",
        active && "border-white/20 bg-white/5"
    )}>
        <span className="text-[10px] text-text-muted uppercase">{label}</span>
        <span className={clsx(
            "text-sm font-semibold font-mono",
            active ? "text-white" : "text-text-secondary",
            warning && "text-yellow-400"
        )}>{value}</span>
    </div>
);

const Badge = ({ icon: Icon, label, color, bg }: any) => (
    <div className={clsx("flex items-center gap-1.5 px-2 py-1 rounded-md text-[10px] font-medium border border-white/5", color, bg)}>
        <Icon size={10} />
        {label}
    </div>
);
