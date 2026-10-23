import { invoke } from '@tauri-apps/api/core'
import { listen } from '@tauri-apps/api/event'
import type { InstallStatus, SetupApi } from '../shared/contracts'

function call<T>(command: string, args?: Record<string, unknown>) {
  return invoke<T>(command, args).catch((error) => {
    throw new Error(typeof error === 'string' ? error : error?.message || String(error))
  })
}

export const setupBridge: SetupApi = {
  info: () => call('setup_info'),
  chooseDirectory: (current) => call('setup_choose_directory', { current }),
  install: (directory) => call('setup_install', { directory }),
  open: () => call('setup_open'),
  close: () => call('setup_close'),
  onProgress: (listener) => {
    const unlisten = listen<InstallStatus>('setup:progress', (event) => listener(event.payload))
    return () => void unlisten.then((stop) => stop())
  }
}

window.setup = setupBridge
