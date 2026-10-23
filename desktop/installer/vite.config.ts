import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

const root = resolve(import.meta.dirname, 'src/renderer')
export default defineConfig({
  root,
  base: './',
  plugins: [react()],
  server: { host: '127.0.0.1', port: 5174, strictPort: true },
  build: { outDir: resolve(import.meta.dirname, 'out'), emptyOutDir: true }
})
