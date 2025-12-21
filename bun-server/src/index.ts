import { ScreenerService } from "./services/screener";

const screener = new ScreenerService();
screener.start();

const server = Bun.serve({
    port: parseInt(process.env.PORT || "8080"),
    fetch(req, server) {
        const url = new URL(req.url);

        // WebSocket Upgrade
        if (url.pathname === "/ws") {
            const success = server.upgrade(req);
            return success ? undefined : new Response("WS Upgrade Failed", { status: 500 });
        }

        if (url.pathname === "/health") {
            return Response.json({ status: "ok" });
        }

        if (url.pathname === "/api/coins") {
            return Response.json(screener.coins);
        }

        return new Response("Not Found", { status: 404 });
    },
    websocket: {
        open(ws) {
            // Send initial data
            ws.send(JSON.stringify({ type: "initial", data: screener.coins }));
            ws.subscribe("updates");
        },
        message(ws, msg) {
            // Handle messages if needed
        }
    }
});

console.log(`Server running at http://localhost:${server.port}`);

// Broadcast updates
setInterval(() => {
    server.publish("updates", JSON.stringify({ type: "update", data: screener.coins }));
}, 5000);
