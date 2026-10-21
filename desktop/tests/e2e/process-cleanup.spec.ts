import { test, expect } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import { resolve } from 'node:path'
import { launchWorkbench } from '../fixtures/workbench-app'

function processes(pattern: string) {
  const script = `Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -like '*${pattern}*' -and $_.Name -ne 'powershell.exe' } | ForEach-Object { $_.ProcessId }`
  return execFileSync('powershell', ['-NoProfile', '-Command', script], { encoding: 'utf8' })
    .split(/\s+/)
    .filter(Boolean)
}

async function launchWithServer(marker: string) {
  const workbench = await launchWorkbench({
    mcp: {
      child: {
        type: 'stdio',
        command: process.execPath,
        args: [resolve('tests/fixtures/mcp-child-server.cjs'), marker],
        timeout: 30
      }
    }
  })
  await expect
    .poll(() => processes(marker).length, { timeout: 30000 })
    .toBeGreaterThanOrEqual(2)
  const runtime = [
    ...new Set(
      processes(marker).map((pid) =>
        execFileSync(
          'powershell',
          [
            '-NoProfile',
            '-Command',
            `$p = Get-CimInstance Win32_Process -Filter "ProcessId=${pid}"; $q = Get-CimInstance Win32_Process -Filter "ProcessId=$($p.ParentProcessId)"; if ($q.Name -eq 'prime.exe') { $q.ProcessId }`
          ],
          { encoding: 'utf8' }
        ).trim()
      )
    )
  ].filter(Boolean)
  expect(runtime).toHaveLength(1)
  return { workbench, runtime: runtime[0] }
}

test.skip(process.platform !== 'win32', 'Windows process tree cleanup')

test('closing the app stops Prime and every MCP descendant', async () => {
  test.setTimeout(120000)
  const marker = `prime-cleanup-${Date.now()}`
  const { workbench, runtime } = await launchWithServer(marker)
  await workbench.close()
  await expect.poll(() => processes(marker).length, { timeout: 20000 }).toBe(0)
  await expect.poll(() => processes(`prime-desktop-`).includes(runtime), { timeout: 20000 }).toBe(false)
})

test('a crashed app does not leave Prime or MCP processes behind', async () => {
  test.setTimeout(120000)
  const marker = `prime-crash-${Date.now()}`
  const { workbench, runtime } = await launchWithServer(marker)
  const electronMain = execFileSync(
    'powershell',
    ['-NoProfile', '-Command', `(Get-CimInstance Win32_Process -Filter "ProcessId=${runtime}").ParentProcessId`],
    { encoding: 'utf8' }
  ).trim()
  execFileSync('taskkill', ['/F', '/PID', electronMain])
  await workbench.provider.close()
  await expect.poll(() => processes(`prime-desktop-`).includes(runtime), { timeout: 20000 }).toBe(false)
  await expect.poll(() => processes(marker).length, { timeout: 20000 }).toBe(0)
})
