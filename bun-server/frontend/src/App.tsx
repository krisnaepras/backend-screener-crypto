import { BrowserRouter, Routes, Route } from "react-router-dom";
import { Layout } from "./components/Layout";
import { Dashboard } from "./pages/Dashboard";
import { CoinDetail } from "./pages/CoinDetail";
import { Logs } from "./pages/Logs";
import { Transactions } from "./pages/Transactions";

function App() {
    return (
        <BrowserRouter>
            <Routes>
                <Route path="/" element={<Layout />}>
                    <Route index element={<Dashboard />} />
                    <Route path="coin/:symbol" element={<CoinDetail />} />
                    <Route path="history" element={<Logs />} />
                    <Route path="transactions" element={<Transactions />} />
                    {/* Placeholders for other menu items */}
                    <Route path="chart" element={<Dashboard />} />
                    <Route path="wallet" element={<Dashboard />} />
                    <Route path="news" element={<Dashboard />} />
                    <Route path="mail" element={<Dashboard />} />
                </Route>
            </Routes>
        </BrowserRouter>
    );
}

export default App;
