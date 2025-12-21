import {
    LayoutGrid,
    PieChart,
    Layers,
    Wallet,
    Newspaper,
    Mail,
    Settings,
    LogOut,
    History,
    LayoutDashboard
} from "lucide-react";
import { Link, useLocation } from "react-router-dom";
import clsx from "clsx";

export function Sidebar() {
    const location = useLocation();

    const menuItems = [
        { icon: LayoutGrid, label: "Overview", path: "/" },
        { icon: PieChart, label: "Chart", path: "/chart" },
        { icon: Layers, label: "Transactions", path: "/transactions" },
        { icon: Wallet, label: "Wallet", path: "/wallet" },
        { icon: Newspaper, label: "News", path: "/news" },
        { icon: Mail, label: "Mail Box", path: "/mail" },
    ];

    return (
        <aside className="w-64 bg-surface h-screen fixed left-0 top-0 flex flex-col z-50">
            {/* Logo */}
            <div className="h-24 flex items-center px-8">
                <div className="flex items-center gap-2">
                    <div className="w-8 h-8 bg-primary rounded-lg flex items-center justify-center relative overflow-hidden">
                        <div className="absolute inset-0 bg-white/20 transform rotate-45 translate-x-1/2"></div>
                    </div>
                    <span className="font-display font-bold text-xl text-white">Logoipsm</span>
                </div>
            </div>

            {/* Menu */}
            <nav className="flex-1 px-4 py-4 space-y-2">
                {menuItems.map((item) => {
                    const isActive = location.pathname === item.path;
                    return (
                        <Link
                            key={item.path}
                            to={item.path}
                            className={clsx(
                                "flex items-center gap-4 px-4 py-3 rounded-xl transition-all duration-200 group relative overflow-hidden",
                                isActive
                                    ? "bg-primary text-white shadow-glow"
                                    : "text-text-secondary hover:text-white"
                            )}
                        >
                            <item.icon size={20} className={clsx(isActive ? "text-white" : "text-text-secondary group-hover:text-white")} />
                            <span className="font-medium text-sm">{item.label}</span>

                            {/* Hover effect background */}
                            {!isActive && (
                                <div className="absolute inset-0 bg-white/5 opacity-0 group-hover:opacity-100 transition-opacity" />
                            )}
                        </Link>
                    )
                })}
            </nav>

            {/* Bottom Actions */}
            <div className="px-4 py-8 space-y-2">
                <Link to="/" className="flex items-center gap-3 px-4 py-3 bg-primary/10 text-primary rounded-lg text-sm font-medium">
                    <LayoutDashboard size={18} />
                    Dashboard
                </Link>
                <Link to="/history" className="flex items-center gap-3 px-4 py-3 text-text-secondary hover:text-white hover:bg-white/5 rounded-lg text-sm font-medium transition-colors">
                    <History size={18} />
                    Trading History
                </Link>
                <div className="flex items-center gap-3 px-4 py-3 text-text-secondary hover:text-white hover:bg-white/5 rounded-lg text-sm font-medium transition-colors cursor-not-allowed opacity-50">
                    <Settings size={18} />
                    Configuration
                </div>
                <button className="w-full flex items-center gap-3 px-4 py-3 text-text-secondary hover:text-white hover:bg-white/5 rounded-lg text-sm font-medium transition-colors">
                    <LogOut size={18} />
                    Logout
                </button>
            </div>
        </aside>
    );
}
