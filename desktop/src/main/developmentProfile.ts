import { mkdirSync } from 'node:fs'

type ProfileApp = {
  isPackaged: boolean
  commandLine: { hasSwitch(name: string): boolean }
  getPath(name: 'userData' | 'sessionData'): string
  setPath(name: 'userData' | 'sessionData', value: string): void
}

export function configureDevelopmentProfile(app: ProfileApp): void {
  if (app.isPackaged || app.commandLine.hasSwitch('user-data-dir')) return
  const profile = app.getPath('userData') + '-dev'
  mkdirSync(profile, { recursive: true })
  app.setPath('userData', profile)
  app.setPath('sessionData', profile)
}
