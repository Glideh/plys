import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true,
    // Let the API answer CORS preflights for paired peers; Vite's default answers them without the peer origin.
    cors: false,
    proxy: {
      '/api': { target: process.env.API_PROXY_TARGET ?? 'http://localhost:8080' },
    },
  },
  test: {
    environment: 'node',
  },
})
