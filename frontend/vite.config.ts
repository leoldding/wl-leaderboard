import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
    base: "/leaderboard",
    plugins: [react(), tailwindcss()],
    server: {
        host: true,
        proxy: {
            // Routes for leaderboard API → port 8085
            '/api/v1/leaderboard': {
                target: 'http://localhost:8085',
                changeOrigin: true,
                rewrite: (path) => path.replace(/^\/api\/v1\/leaderboard/, '/api/v1/leaderboard'),
            },

            // Routes for auth API → port 8080
            '/api/v1/auth': {
                target: 'http://localhost:8080',
                changeOrigin: true,
                rewrite: (path) => path.replace(/^\/api\/v1\/auth/, '/api/v1/auth'),
            },
        },
    },
})
