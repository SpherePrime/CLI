import type { DesktopApi } from './contracts'
import manifest from './hostMethods.json'

export const hostMethods = manifest as Record<Exclude<keyof DesktopApi, 'onEvent'>, true>

export type HostMethod = keyof typeof hostMethods
