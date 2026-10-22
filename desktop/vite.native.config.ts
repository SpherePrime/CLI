import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

export default defineConfig({
  root: resolve(import.meta.dirname, 'src/renderer'),
  define: { 'import.meta.env.VITE_NATIVE_HOST': JSON.stringify('true') },
  plugins: [
    react(),
    {
      name: 'native-ipc-policy',
      transformIndexHtml(html) {
        return html.replace(
          "connect-src 'self' ws://127.0.0.1:*",
          "connect-src 'self' ipc: http://ipc.localhost ws://127.0.0.1:*"
        )
      }
    }
  ],
  server: { host: '127.0.0.1', port: 5173, strictPort: true },
  build: { outDir: resolve(import.meta.dirname, 'out/native-renderer'), emptyOutDir: true }
})
