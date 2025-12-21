import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
    plugins: [react()],
    root: 'frontend',
    resolve: {
        alias: {
            "@": path.resolve(__dirname, "./src"),
        },
    },
    server: {
        proxy: {
            '/api': {
                target: 'http://localhost:8181',
                changeOrigin: true,
            },
            '/ws': {
                target: 'ws://localhost:8181',
                ws: true,
            }
        }
    },
    build: {
        outDir: '../public',
        emptyOutDir: true,
    }
})
