import { invoke } from '@tauri-apps/api/core'
import { listen } from '@tauri-apps/api/event'
import { createNativeBridge } from './nativeBridge'

export async function installNativeHost() {
  const bridge = await createNativeBridge(invoke, listen)
  window.prime = bridge.api
  window.addEventListener('pagehide', bridge.dispose, { once: true })
}
