import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// The SPA never talks business rules: the server computes, the front
// displays (C-26). In dev, /api proxies to the Go binary.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    allowedHosts: ['.trycloudflare.com', '.simptom.fr'],
    proxy: {
      '/api': 'http://localhost:8099',
    },
  },
})
