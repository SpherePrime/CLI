import { readdir, readFile, readlink } from 'node:fs/promises'

async function processes() {
  const result: { pid: number; name: string; executable: string }[] = []
  for (const entry of await readdir('/proc').catch(() => [] as string[])) {
    const pid = Number(entry)
    if (!Number.isInteger(pid) || pid === process.pid) continue
    const name = (await readFile(`/proc/${pid}/comm`, 'utf8').catch(() => '')).trim()
    const executable = await readlink(`/proc/${pid}/exe`).catch(() => '')
    result.push({ pid, name, executable })
  }
  return result
}

export async function ensureDesktopClosed() {
  if ((await processes()).some((item) => item.name === 'prime-desktop'))
    throw new Error(
      'Закройте Prime Desktop перед установкой, чтобы сохранить ваши задачи и изменения.'
    )
}

export async function stopLeftoverRuntime(directory: string) {
  const root = directory.replace(/\/+$/, '') + '/'
  for (const item of await processes())
    if (item.executable.startsWith(root) && item.executable.endsWith('/prime'))
      try {
        process.kill(item.pid, 'SIGKILL')
      } catch {}
}
