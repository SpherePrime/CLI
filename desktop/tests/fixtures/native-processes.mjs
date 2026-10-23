import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

const execute = promisify(execFile)

export async function nativeProcessFootprint(rootPid) {
  const { stdout } = await execute('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', 'Get-CimInstance Win32_Process | Select-Object ProcessId,ParentProcessId,Name,WorkingSetSize | ConvertTo-Json -Compress'], { windowsHide: true, maxBuffer: 2 * 1024 * 1024 })
  const processes = JSON.parse(stdout)
  const descendants = new Set([rootPid])
  let changed = true
  while (changed) {
    changed = false
    for (const process of processes) {
      if (descendants.has(process.ParentProcessId) && !descendants.has(process.ProcessId)) {
        descendants.add(process.ProcessId)
        changed = true
      }
    }
  }
  const selected = processes.filter((process) => descendants.has(process.ProcessId))
  return { processes: selected, workingSetBytes: selected.reduce((sum, process) => sum + Number(process.WorkingSetSize), 0) }
}
