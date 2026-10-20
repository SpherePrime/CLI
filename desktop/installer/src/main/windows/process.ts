import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

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
export async function ensureDesktopClosed() {
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
export async function stopLeftoverRuntime(directory: string) {
  const script = `$root = [IO.Path]::GetFullPath($env:PRIME_INSTALL_DIR).TrimEnd('\\') + '\\'
Get-CimInstance Win32_Process -Filter "Name='prime.exe'" |
  Where-Object { $_.ExecutablePath -and $_.ExecutablePath.StartsWith($root, [StringComparison]::OrdinalIgnoreCase) } |
  ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }`
  await promisify(execFile)(
    'powershell.exe',
    ['-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-Command', script],
    { windowsHide: true, env: { ...process.env, PRIME_INSTALL_DIR: directory } }
  ).catch(() => {})
}
export async function installedDirectory(fallback: string) {
  try {
    const { stdout } = await promisify(execFile)(
      'reg',
      [
        'query',
        'HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall',
        '/s',
        '/f',
        'Prime Desktop',
        '/d',
        '/e'
      ],
      { windowsHide: true }
    )
    const key = stdout.split(/\r?\n/).find((line) => line.startsWith('HKEY_CURRENT_USER\\'))
    if (key) {
      const result = await promisify(execFile)(
        'reg',
        ['query', key.trim(), '/v', 'InstallLocation'],
        { windowsHide: true }
      )
      const location = /InstallLocation\s+REG_SZ\s+(.+)/.exec(result.stdout)?.[1]?.trim()
      if (location) return location
    }
  } catch {}
  return fallback
}
