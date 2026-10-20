import type { SpawnOptions } from 'node:child_process'

const bundleEnvironment = ['APPIMAGE', 'APPDIR', 'OWD', 'ARGV0', 'APPIMAGE_EXTRACT_AND_RUN']

export function desktopLaunchOptions(extra: NodeJS.ProcessEnv = {}): SpawnOptions {
  const env = { ...process.env }
  for (const name of bundleEnvironment) delete env[name]
  return {
    detached: true,
    windowsHide: false,
    stdio: 'ignore',
    env: { ...env, ...extra }
  }
}
