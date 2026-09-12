import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: { outDir: 'dist', emptyOutDir: true, sourcemap: false },
  server: { port: 5173, proxy: { '/rpc': 'http://127.0.0.1:8080', '/help': 'http://127.0.0.1:8080', '/schema': 'http://127.0.0.1:8080' } },
  test: { environment: 'jsdom', setupFiles: './src/test/setup.ts' },
})
