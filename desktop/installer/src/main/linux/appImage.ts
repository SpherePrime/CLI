import { execFile } from 'node:child_process'
import { existsSync } from 'node:fs'
import { chmod, copyFile, mkdir, readFile, rename, writeFile } from 'node:fs/promises'
import { homedir } from 'node:os'
import { join } from 'node:path'
import { promisify } from 'node:util'

export const appImageName = 'PrimeDesktop.AppImage'
const run = promisify(execFile)

export function dataHome() {
  return process.env.XDG_DATA_HOME || join(homedir(), '.local', 'share')
}

async function hasFuse2() {
  for (const ldconfig of ['ldconfig', '/sbin/ldconfig', '/usr/sbin/ldconfig']) {
    try {
      const { stdout } = await run(ldconfig, ['-p'])
      return /libfuse\.so\.2\b/.test(stdout)
    } catch {}
  }
  return false
}

async function restrictsUserNamespaces() {
  const value = await readFile(
    '/proc/sys/kernel/apparmor_restrict_unprivileged_userns',
    'utf8'
  ).catch(() => '0')
  return value.trim() === '1'
}

export async function appImageLaunch(directory: string) {
  const env: NodeJS.ProcessEnv = (await hasFuse2()) ? {} : { APPIMAGE_EXTRACT_AND_RUN: '1' }
  const args = (await restrictsUserNamespaces()) ? ['--no-sandbox'] : []
  return { binary: join(directory, appImageName), args, env }
}

const quoteExec = (value: string) => `"${value.replace(/(["`$\\])/g, '\\$1')}"`

async function desktopFolder() {
  try {
    const { stdout } = await run('xdg-user-dir', ['DESKTOP'])
    const folder = stdout.trim()
    if (folder && folder !== homedir() && existsSync(folder)) return folder
  } catch {}
  const fallback = join(homedir(), 'Desktop')
  return existsSync(fallback) ? fallback : undefined
}

export async function installAppImage(source: string, directory: string, icon: string) {
  await mkdir(directory, { recursive: true })
  const target = join(directory, appImageName)
  await copyFile(source, `${target}.partial`)
  await chmod(`${target}.partial`, 0o755)
  await rename(`${target}.partial`, target)
  const icons = join(dataHome(), 'icons', 'hicolor', '256x256', 'apps')
  await mkdir(icons, { recursive: true })
  await copyFile(icon, join(icons, 'prime-desktop.png'))
  const launch = await appImageLaunch(directory)
  const exec = [
    ...Object.entries(launch.env).map(([key, value]) => `env ${key}=${value}`),
    quoteExec(launch.binary),
    ...launch.args,
    '%U'
  ].join(' ')
  const entry = [
    '[Desktop Entry]',
    'Type=Application',
    'Name=Prime Desktop',
    'Comment=Desktop workspace for Prime',
    `Exec=${exec}`,
    'Icon=prime-desktop',
    'Terminal=false',
    'Categories=Development;',
    ''
  ].join('\n')
  const applications = join(dataHome(), 'applications')
  await mkdir(applications, { recursive: true })
  await writeFile(join(applications, 'prime-desktop.desktop'), entry, { mode: 0o755 })
  await run('update-desktop-database', [applications]).catch(() => {})
  const desktop = await desktopFolder()
  if (desktop) {
    const shortcut = join(desktop, 'prime-desktop.desktop')
    await writeFile(shortcut, entry, { mode: 0o755 })
    await run('gio', ['set', shortcut, 'metadata::trusted', 'true']).catch(() => {})
  }
  return target
}
