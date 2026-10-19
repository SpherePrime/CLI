import koffi from 'koffi'

function bindings() {
  const kernel = koffi.load('kernel32.dll')
  const basicLimits = koffi.struct({
    PerProcessUserTimeLimit: 'int64_t',
    PerJobUserTimeLimit: 'int64_t',
    LimitFlags: 'uint32_t',
    MinimumWorkingSetSize: 'size_t',
    MaximumWorkingSetSize: 'size_t',
    ActiveProcessLimit: 'uint32_t',
    Affinity: 'uintptr_t',
    PriorityClass: 'uint32_t',
    SchedulingClass: 'uint32_t'
  })
  const counters = koffi.struct({
    ReadOperationCount: 'uint64_t',
    WriteOperationCount: 'uint64_t',
    OtherOperationCount: 'uint64_t',
    ReadTransferCount: 'uint64_t',
    WriteTransferCount: 'uint64_t',
    OtherTransferCount: 'uint64_t'
  })
  const extendedLimits = koffi.struct({
    BasicLimitInformation: basicLimits,
    IoInfo: counters,
    ProcessMemoryLimit: 'size_t',
    JobMemoryLimit: 'size_t',
    PeakProcessMemoryUsed: 'size_t',
    PeakJobMemoryUsed: 'size_t'
  })
  const accounting = koffi.struct({
    TotalUserTime: 'int64_t',
    TotalKernelTime: 'int64_t',
    ThisPeriodTotalUserTime: 'int64_t',
    ThisPeriodTotalKernelTime: 'int64_t',
    TotalPageFaultCount: 'uint32_t',
    TotalProcesses: 'uint32_t',
    ActiveProcesses: 'uint32_t',
    TotalTerminatedProcesses: 'uint32_t'
  })
  return {
    create: kernel.func('__stdcall', 'CreateJobObjectW', 'void *', ['void *', 'str16']),
    setLimits: kernel.func('__stdcall', 'SetInformationJobObject', 'int', [
      'void *',
      'int',
      koffi.pointer(extendedLimits),
      'uint32_t'
    ]),
    assign: kernel.func('__stdcall', 'AssignProcessToJobObject', 'int', ['void *', 'void *']),
    openProcess: kernel.func('__stdcall', 'OpenProcess', 'void *', ['uint32_t', 'int', 'uint32_t']),
    terminateProcess: kernel.func('__stdcall', 'TerminateProcess', 'int', ['void *', 'uint32_t']),
    terminateJob: kernel.func('__stdcall', 'TerminateJobObject', 'int', ['void *', 'uint32_t']),
    query: kernel.func('__stdcall', 'QueryInformationJobObject', 'int', [
      'void *',
      'int',
      koffi.out(koffi.pointer(accounting)),
      'uint32_t',
      'void *'
    ]),
    close: kernel.func('__stdcall', 'CloseHandle', 'int', ['void *']),
    lastError: kernel.func('__stdcall', 'GetLastError', 'uint32_t', []),
    limitsSize: koffi.sizeof(extendedLimits),
    accountingSize: koffi.sizeof(accounting)
  }
}

let windows: ReturnType<typeof bindings> | undefined

export interface TerminalJob {
  close(): Promise<void>
}

export function createWindowsJob(pid: number): TerminalJob | undefined {
  if (process.platform !== 'win32') return undefined
  const api = (windows ??= bindings())
  const job = api.create(null, null)
  if (!job) throw new Error(`Could not create terminal job (${api.lastError()})`)
  let assigned = false
  try {
    if (!api.setLimits(job, 9, { BasicLimitInformation: { LimitFlags: 0x2000 } }, api.limitsSize))
      throw new Error(`Could not configure terminal job (${api.lastError()})`)
    const root = api.openProcess(0x101, 0, pid)
    if (!root)
      throw new Error(`Could not open terminal process for job assignment (${api.lastError()})`)
    try {
      if (!api.assign(job, root)) {
        const code = api.lastError()
        api.terminateProcess(root, 1)
        throw new Error(`Could not assign terminal process to job (${code})`)
      }
      assigned = true
    } finally {
      api.close(root)
    }
  } finally {
    if (!assigned) api.close(job)
  }
  let closing: Promise<void> | undefined
  return {
    close() {
      return (closing ??= (async () => {
        try {
          if (!api.terminateJob(job, 1))
            throw new Error(`Could not terminate terminal job (${api.lastError()})`)
          const deadline = Date.now() + 5000
          while (true) {
            const result: { ActiveProcesses?: number } = {}
            if (!api.query(job, 1, result, api.accountingSize, null))
              throw new Error(`Could not inspect terminal job (${api.lastError()})`)
            if (result.ActiveProcesses === 0) return
            if (Date.now() >= deadline) throw new Error('Terminal job cleanup timed out')
            await new Promise((resolve) => setTimeout(resolve, 20))
          }
        } finally {
          if (!api.close(job)) throw new Error(`Could not close terminal job (${api.lastError()})`)
        }
      })())
    }
  }
}
