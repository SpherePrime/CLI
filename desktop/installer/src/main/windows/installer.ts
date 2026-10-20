import { app } from 'electron'
import { spawn } from 'node:child_process'
import { readFile, stat } from 'node:fs/promises'
import { join } from 'node:path'
import { downloadSpeechModel } from '../../../../src/main/speech/assets'
import { checksum } from '../checksum'
import type { PlatformInstaller } from '../platform'
import {
  ensureDesktopClosed,
  installedDirectory,
  installerCommand,
  stopLeftoverRuntime
} from './process'

export type WindowsPayload = {
  version: string
  sha256: string
  runtimeSha256: string
  installedSize: number
}
export function readWindowsPayload(value: any): WindowsPayload {
  if (
    !/^\d+\.\d+\.\d+$/.test(value.version) ||
    !/^[a-f0-9]{64}$/.test(value.sha256) ||
    !/^[a-f0-9]{64}$/.test(value.runtimeSha256)
  )
    throw new Error('Повреждена конфигурация установщика')
  return value
}
export function windowsInstaller(root: string, payload: WindowsPayload): PlatformInstaller {
  const setup = join(root, 'PrimeDesktopSetup.exe')
  return {
    version: payload.version,
    installedSize: payload.installedSize,
    method: async () => ({ label: 'Ваше рабочее пространство', directoryLocked: false }),
    defaultDirectory: () =>
      installedDirectory(
        join(process.env.LOCALAPPDATA || app.getPath('home'), 'Programs', 'Prime Desktop')
      ),
    open: async (directory) => ({ binary: join(directory, 'PrimeDesktop.exe'), args: [] }),
    operations: {
      async verifyPayload() {
        if ((await checksum(setup)) !== payload.sha256)
          throw new Error('Файл установки повреждён. Скачайте установщик заново.')
      },
      async launch(directory: string) {
        await ensureDesktopClosed()
        await stopLeftoverRuntime(directory)
        await new Promise<void>((resolve, reject) => {
          const command = installerCommand(setup, directory)
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
      async prepareVoice(progress: (percent: number) => void) {
        await downloadSpeechModel(progress)
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
}
