import { spawn } from "bun";

console.log("🚀 Starting Crypto Watchtower (Full Stack)...");

// 1. Start Backend
const backend = spawn([process.execPath, "run", "src/index.ts"], {
    stdout: "inherit",
    stderr: "inherit",
    env: { ...process.env, PORT: "8181" }
});

// 2. Start Frontend (Vite)
const frontend = spawn([process.execPath, "run", "dev:frontend"], {
    stdout: "inherit",
    stderr: "inherit",
});

console.log(`
✅ Services Started:
   - Backend API: http://localhost:8181
   - Frontend UI: http://localhost:5173
`);

// Handle Exit
process.on("SIGINT", () => {
    backend.kill();
    frontend.kill();
    process.exit();
});
