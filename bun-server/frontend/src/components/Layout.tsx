import { Outlet } from "react-router-dom";
import { Sidebar } from "./Sidebar";

export function Layout() {
    return (
        <div className="min-h-screen bg-background font-sans text-text-primary flex">
            <Sidebar />
            <main className="flex-1 ml-64 p-8 relative z-0">
                <div className="max-w-[1600px] mx-auto">
                    <Outlet />
                </div>
            </main>
        </div>
    );
}
