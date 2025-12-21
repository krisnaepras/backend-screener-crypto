import { Elysia } from "elysia";
import { swagger } from "@elysiajs/swagger";
import { staticPlugin } from "@elysiajs/static";
import { ScreenerService } from "./services/screener";

// Initialize Service
const screener = new ScreenerService();

// Start Background Loop
screener.start();

// Initialize Elysia App
const app = new Elysia()
    .use(swagger({
        path: "/swagger",
        documentation: {
            info: {
                title: "Crypto Screener API",
                version: "1.0.0",
                description: "Real-time Crypto Screener API with WebSocket support. Powered by Bun & Elysia."
            }
        }
    }))
    .get("/health", () => ({ status: "ok" }), {
        detail: {
            summary: "Health Check",
            description: "Returns the health status of the server."
        }
    })
    .get("/api/coins", () => screener.coins, {
        detail: {
            summary: "Get Screened Coins",
            description: "Returns the latest snapshot of all screened coins."
        }
    })
    .ws("/ws", {
        open(ws) {
            ws.send(JSON.stringify({ type: "initial", data: screener.coins }));
            ws.subscribe("updates");
        },
        message(ws, message) {
            // Handle messages
        },
        detail: {
            summary: "WebSocket Endpoint",
            description: "Connect here for real-time updates."
        }
    })
    .use(staticPlugin({
        assets: "public",
        prefix: "/"
    }))
    .get("/", () => Bun.file("public/index.html")) // Fallback for SPA
    .get("/api/logs", async () => {
        return await screener.getTradeLogs();
    })
    .get("/swagger", () => ({ status: "OK" })) // Temp swagger placeholder
    .listen(8181);

console.log(`🦊 Server running at http://localhost:8181`);
console.log(`📘 Swagger UI at http://localhost:8181/swagger`);

// Broadcast updates
setInterval(() => {
    if (app.server) {
        app.server.publish("updates", JSON.stringify({ type: "update", data: screener.coins }));
    }
}, 5000);
