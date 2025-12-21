import { Search } from "lucide-react";
import { useState } from "react";
import { useNavigate, Link, useLocation } from "react-router-dom";
import clsx from "clsx";

export function Header({ title = "Dashboard" }: { title?: string }) {
    const [search, setSearch] = useState("");
    const navigate = useNavigate();
    const location = useLocation();

    const handleSearch = (e: React.KeyboardEvent) => {
        if (e.key === 'Enter' && search.trim()) {
            // uppercase the search term
            navigate(`/coin/${search.trim().toUpperCase()}`);
            setSearch("");
        }
    };

    const navs = [
        { label: "Dashboard", path: "/" },
        { label: "Transactions", path: "/transactions" },
        { label: "History", path: "/history" }
    ];

    return (
        <header className="flex justify-between items-center mb-10 pt-4">
            <div className="flex items-center gap-8">
                <h1 className="text-3xl font-bold text-white">{title}</h1>
                <nav className="flex gap-1 bg-surfaceHighlight p-1 rounded-lg">
                    {navs.map(n => (
                        <Link
                            key={n.path}
                            to={n.path}
                            className={clsx(
                                "px-4 py-2 rounded-md text-sm font-semibold transition-all",
                                location.pathname === n.path
                                    ? "bg-primary text-white shadow-lg"
                                    : "text-text-secondary hover:text-white hover:bg-white/5"
                            )}
                        >
                            {n.label}
                        </Link>
                    ))}
                </nav>
            </div>

            <div className="flex items-center gap-8">
                {/* Search Bar */}
                <div className="relative w-96 group">
                    <Search className="absolute left-4 top-1/2 -translate-y-1/2 text-text-secondary group-focus-within:text-primary transition-colors" size={20} />
                    <input
                        type="text"
                        value={search}
                        placeholder="Search Symbol (e.g. BTC)..."
                        onChange={(e) => setSearch(e.target.value)}
                        onKeyDown={handleSearch}
                        className="w-full bg-background border border-white/5 focus:border-primary/50 hover:border-white/10 rounded-xl py-3 pl-12 pr-4 text-sm text-white placeholder-text-muted outline-none transition-all duration-300 shadow-inner"
                    />
                </div>
            </div>
        </header>
    );
}
