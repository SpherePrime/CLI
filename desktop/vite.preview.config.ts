import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

export default defineConfig({
  root: resolve(import.meta.dirname, 'src/renderer'),
  define: {
    'import.meta.env.VITE_NATIVE_HOST': JSON.stringify('false'),
    'import.meta.env.VITE_PREVIEW_HOST': JSON.stringify('true')
  },
  plugins: [
    react(),
    {
      name: 'preview-csp-policy',
      transformIndexHtml(html) {
        return html.replace("script-src 'self'", "script-src 'self' 'unsafe-inline'")
      }
    }
  ],
  server: { host: '127.0.0.1', port: 5173, strictPort: true }
})
