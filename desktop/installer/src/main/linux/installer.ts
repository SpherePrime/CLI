import { spawn } from 'node:child_process'
import { readFile, stat } from 'node:fs/promises'
import { join } from 'node:path'
import { downloadSpeechModel } from '../../../../src/main/speech/assets'
import { checksum } from '../checksum'
import type { PlatformInstaller } from '../platform'
import { appImageLaunch, dataHome, installAppImage } from './appImage'
import { detectPlan, type LinuxPackageKind, type LinuxPlan } from './distribution'
import { stagePackage, type LinuxPayload } from './packages'
import { ensureDesktopClosed, stopLeftoverRuntime } from './processes'

const systemDirectory = '/opt/Prime Desktop'
const systemBinary = join(systemDirectory, 'prime-desktop')
const authorizationDeclined = [126, 127]
const packageLabels: Record<LinuxPackageKind, string> = {
  deb: 'Пакет .deb',
  rpm: 'Пакет .rpm',
  pacman: 'Пакет pacman',
  appimage: 'AppImage для текущего пользователя'
}

function runPrivileged(command: string[]) {
  return new Promise<number>((resolve, reject) => {
    let output = ''
    const child = spawn('pkexec', command, { stdio: ['ignore', 'ignore', 'pipe'] })
    child.stderr.on('data', (chunk: Buffer) => {
      output = (output + chunk.toString('utf8')).slice(-600)
    })
    child.once('error', reject)
    child.once('exit', (code) => {
      if (code === 0 || authorizationDeclined.includes(code ?? -1)) resolve(code ?? -1)
      else reject(new Error(`Установка не завершена (${code}). ${output.trim()}`.trim()))
    })
  })
}

export function linuxInstaller(
  root: string,
  payload: LinuxPayload,
  icon: string
): PlatformInstaller {
  let plan: Promise<LinuxPlan> | undefined
  let staged: Awaited<ReturnType<typeof stagePackage>> | undefined
  let installed: { kind: LinuxPackageKind; directory: string } | undefined
  const currentPlan = () => (plan ||= detectPlan())
  const userDirectory = join(dataHome(), 'prime-desktop')
  async function installForUser(directory: string) {
    const appImage = await stagePackage(root, payload, 'appimage')
    try {
      await installAppImage(appImage.path, directory, icon)
    } finally {
      await appImage.cleanup()
    }
    installed = { kind: 'appimage', directory }
  }
  return {
    version: payload.version,
    installedSize: payload.installedSize,
    async method() {
      const { kind, distribution } = await currentPlan()
      return {
        label: `${packageLabels[kind]} · ${distribution}`,
        directoryLocked: kind !== 'appimage'
      }
    },
    async defaultDirectory() {
      return (await currentPlan()).kind === 'appimage' ? userDirectory : systemDirectory
    },
    async open(directory) {
      if ((installed?.kind ?? 'appimage') !== 'appimage') return { binary: systemBinary, args: [] }
      return appImageLaunch(installed?.directory ?? directory)
    },
    operations: {
      async verifyPayload() {
        const { kind } = await currentPlan()
        await staged?.cleanup()
        staged = undefined
        if (kind !== 'appimage') staged = await stagePackage(root, payload, kind)
      },
      async launch(directory: string) {
        await ensureDesktopClosed()
        const { kind, command } = await currentPlan()
        if (kind === 'appimage' || !command || !staged) return installForUser(directory)
        try {
          await stopLeftoverRuntime(systemDirectory)
          const code = await runPrivileged([...command, staged.path])
          if (code === 0) {
            installed = { kind, directory: systemDirectory }
            return
          }
        } finally {
          await staged.cleanup()
          staged = undefined
        }
        await installForUser(userDirectory)
        return {
          notice:
            'Системная установка отменена, Prime Desktop установлен для текущего пользователя.'
        }
      },
      async prepareVoice(progress: (percent: number) => void) {
        await downloadSpeechModel(progress)
      },
      async verifyInstall() {
        if (!installed) throw new Error('Файлы приложения не найдены')
        if (installed.kind === 'appimage') {
          const appImage = join(installed.directory, 'PrimeDesktop.AppImage')
          if ((await checksum(appImage)) !== payload.packages.appimage.sha256)
            throw new Error('Не удалось проверить файлы Prime Desktop')
          return
        }
        if (!(await stat(systemBinary)).isFile()) throw new Error('Файлы приложения не найдены')
        const runtime = join(systemDirectory, 'resources/runtime')
        if ((await checksum(join(runtime, 'prime'))) !== payload.runtimeSha256)
          throw new Error('Не удалось проверить файлы Prime Desktop')
        const manifest = JSON.parse(await readFile(join(runtime, 'manifest.json'), 'utf8'))
        if (manifest.sha256 !== payload.runtimeSha256)
          throw new Error('Версии компонентов приложения не совпадают')
      }
    }
  }
}
