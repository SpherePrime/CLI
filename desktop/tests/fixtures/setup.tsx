import React from 'react'
import { createRoot } from 'react-dom/client'
import '@fontsource/inter/400.css'
import '@fontsource/inter/500.css'
import '@fontsource/inter/600.css'
import { SetupApp } from '../../installer/src/renderer/App'
import '../../installer/src/renderer/style.css'
const listeners = new Set<(value: any) => void>()
Object.assign(window, {
  fixtureSetupCalls: [],
  setup: {
    info: async () => ({
      version: '0.4.51',
      directory: 'C:\\Users\\User\\AppData\\Local\\Programs\\Prime Desktop',
      installedSize: 320 * 1024 * 1024
    }),
    chooseDirectory: async () => 'C:\\Apps\\Prime Desktop',
    onProgress: (listener: (value: any) => void) => {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
    install: async (directory: string) => {
      ;(window as any).fixtureSetupCalls.push(directory)
      const value = { phase: 'installing', directory }
      listeners.forEach((listener) => listener(value))
      return value
    },
    open: async () => {
      ;(window as any).fixtureSetupCalls.push('open')
    },
    close: async () => {}
  }
})
window.addEventListener('fixture:install-status', ((event: CustomEvent) =>
  listeners.forEach((listener) => listener(event.detail))) as EventListener)
createRoot(document.getElementById('root')!).render(<SetupApp />)
