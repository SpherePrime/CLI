import { expect, it } from 'vitest'
import { execFileSync, spawn } from 'node:child_process'
import { mkdtemp, mkdir, readFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { installerCommand } from '../../installer/src/main/payload'

it.skipIf(process.platform !== 'win32')(
  'passes a spaced installation directory in the raw format required by NSIS',
  async () => {
    const root = await mkdtemp(join(tmpdir(), 'prime setup command-'))
    await mkdir(join(root, 'spaced folder'))
    const binary = join(root, 'spaced folder', 'capture.exe')
    execFileSync(
      'go',
      ['build', '-o', binary, resolve('tests/fixtures/capture-commandline_windows.go')],
      { windowsHide: true }
    )
    const destination = 'C:\\Apps\\Prime Desktop Тест'
    const command = installerCommand(binary, destination)
    const capture = join(root, 'command.txt')
    await new Promise<void>((resolve, reject) => {
      const child = spawn(binary, command.args, {
        ...command.options,
        env: { ...process.env, PRIME_COMMAND_CAPTURE: capture }
      })
      child.once('error', reject)
      child.once('exit', (code) => (code === 0 ? resolve() : reject(new Error(`Exit ${code}`))))
    })
    const raw = await readFile(capture, 'utf8')
    expect(raw.startsWith(`"${binary}" /S /D=`)).toBe(true)
    expect(raw.slice(raw.lastIndexOf('/D=') + 3)).toBe(destination)
  },
  30000
)
