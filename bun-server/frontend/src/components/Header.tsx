import { Search } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";

export function Header({ title = "Dashboard" }: { title?: string }) {
    const [search, setSearch] = useState("");
    const navigate = useNavigate();

    const handleSearch = (e: React.KeyboardEvent) => {
        if (e.key === 'Enter' && search.trim()) {
            // uppercase the search term
            navigate(`/coin/${search.trim().toUpperCase()}`);
            setSearch("");
        }
    };

    return (
        <header className="flex justify-between items-center mb-10 pt-4">
            <div>
                <h1 className="text-3xl font-bold text-white">{title}</h1>
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
