import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// /api проксируется на бэкенд, поэтому CORS не нужен
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8980',
    },
  },
})
