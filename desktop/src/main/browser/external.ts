import { shell } from 'electron'
import { spawn } from 'node:child_process'
import { validateExternalUrl } from '../ipc/security'

function openWithExplorer(url: string) {
  return new Promise<void>((resolve, reject) => {
    const child = spawn('explorer.exe', [url], { detached: true, stdio: 'ignore' })
    child.once('error', reject)
    child.once('spawn', () => {
      child.unref()
      resolve()
    })
  })
}

export async function openInSystemBrowser(value: string) {
  const url = validateExternalUrl(value)
  try {
    await shell.openExternal(url)
  } catch (error) {
    if (process.platform !== 'win32') throw error
    await openWithExplorer(url)
  }
}
