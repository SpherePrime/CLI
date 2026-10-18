import { createReadStream } from 'node:fs'
import { readFile, stat } from 'node:fs/promises'
import { createHash } from 'node:crypto'
import { spawn, execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { join } from 'node:path'

export type Payload = {
  version: string
  sha256: string
  runtimeSha256: string
  installedSize: number
}
export function installerCommand(binary: string, directory: string) {
  return {
    binary,
    args: ['/S', `/D=${directory}`],
    options: {
      argv0: `"${binary}"`,
      windowsVerbatimArguments: true,
      windowsHide: true,
      stdio: 'ignore' as const
    }
  }
}
async function ensureDesktopClosed() {
  const { stdout } = await promisify(execFile)(
    'tasklist',
    ['/FI', 'IMAGENAME eq PrimeDesktop.exe', '/FO', 'CSV', '/NH'],
    { windowsHide: true }
  )
  if (/^"PrimeDesktop\.exe"/im.test(stdout))
    throw new Error(
      'Закройте Prime Desktop перед установкой, чтобы сохранить ваши задачи и изменения.'
    )
}
export async function readPayload(root: string): Promise<Payload> {
  const value = JSON.parse(await readFile(join(root, 'manifest.json'), 'utf8'))
  if (
    !/^\d+\.\d+\.\d+$/.test(value.version) ||
    !/^[a-f0-9]{64}$/.test(value.sha256) ||
    !/^[a-f0-9]{64}$/.test(value.runtimeSha256)
  )
    throw new Error('Повреждена конфигурация установщика')
  return value
}
async function checksum(path: string): Promise<string> {
  const hash = createHash('sha256')
  for await (const bytes of createReadStream(path)) hash.update(bytes)
  return hash.digest('hex')
}
export function payloadOperations(root: string, payload: Payload) {
  return {
    async verifyPayload() {
      if ((await checksum(join(root, 'PrimeDesktopSetup.exe'))) !== payload.sha256)
        throw new Error('Файл установки повреждён. Скачайте установщик заново.')
    },
    async launch(directory: string) {
      await ensureDesktopClosed()
      return new Promise<void>((resolve, reject) => {
        const command = installerCommand(join(root, 'PrimeDesktopSetup.exe'), directory)
        const child = spawn(command.binary, command.args, command.options)
        child.once('error', reject)
        child.once('exit', (code) =>
          code === 0
            ? resolve()
            : reject(
                new Error(
                  code === 4
                    ? 'Закройте Prime Desktop перед установкой и попробуйте снова.'
                    : `Установка не завершена (${code}). Проверьте доступ к выбранной папке.`
                )
              )
        )
      })
    },
    async verifyInstall(directory: string) {
      if (!(await stat(join(directory, 'PrimeDesktop.exe'))).isFile())
        throw new Error('Файлы приложения не найдены')
      if (
        (await checksum(join(directory, 'resources/runtime/prime.exe'))) !== payload.runtimeSha256
      )
        throw new Error('Не удалось проверить файлы Prime Desktop')
      const runtime = JSON.parse(
        await readFile(join(directory, 'resources/runtime/manifest.json'), 'utf8')
      )
      if (runtime.sha256 !== payload.runtimeSha256)
        throw new Error('Версии компонентов приложения не совпадают')
    }
  }
}
