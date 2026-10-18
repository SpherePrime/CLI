import { defineConfig } from 'electron-vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

const root = resolve(import.meta.dirname)
export default defineConfig({
  main: {
    build: {
      outDir: resolve(root, 'out/main'),
      rollupOptions: { input: resolve(root, 'src/main/index.ts') }
    }
  },
  preload: {
    build: {
      outDir: resolve(root, 'out/preload'),
      rollupOptions: {
        input: resolve(root, 'src/preload/index.ts'),
        output: { format: 'cjs', entryFileNames: 'index.cjs' }
      }
    }
  },
  renderer: {
    root: resolve(root, 'src/renderer'),
    plugins: [react()],
    build: {
      outDir: resolve(root, 'out/renderer'),
      rollupOptions: { input: resolve(root, 'src/renderer/index.html') }
    }
  }
})
