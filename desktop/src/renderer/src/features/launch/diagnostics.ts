import type { BrowserLog } from '../../../../shared/contracts'

type Diagnostics = {
  name: string
  command: string
  status: string
  exitCode?: number
  url?: string
  output?: string
  browser?: BrowserLog[]
}

const ansi = /\u001b\[[0-9;?]*[ -/]*[@-~]|\u001b\][^\u0007]*(\u0007|\u001b\\)/g

export function diagnosticsPrompt(input: Diagnostics): string {
  const sections = [
    `Diagnose and fix the problem in the running application "${input.name}".`,
    `Command: ${input.command}\nStatus: ${input.status}${input.exitCode !== undefined ? ` (exit code ${input.exitCode})` : ''}${input.url ? `\nURL: ${input.url}` : ''}`
  ]
  const output = input.output?.replace(ansi, '').replace(/\r/g, '').trim()
  if (output) sections.push('Server log (last lines):\n```\n' + output.split('\n').slice(-80).join('\n') + '\n```')
  const browser = input.browser?.slice(-30)
  if (browser?.length)
    sections.push(
      'Browser console:\n```\n' +
        browser.map((log) => `[${log.level}] ${log.message}${log.source ? ` (${log.source}:${log.line ?? 0})` : ''}`).join('\n') +
        '\n```'
    )
  return sections.join('\n\n')
}
