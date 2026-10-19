const { TerminalManager } = require(process.env.PRIME_TERMINAL_MANAGER)

async function until(check) {
  const deadline = Date.now() + 15000
  while (!check()) {
    if (Date.now() > deadline) throw new Error('Terminal runtime smoke timed out')
    await new Promise((resolve) => setTimeout(resolve, 30))
  }
}

async function run() {
  const events = []
  const manager = new TerminalManager({ emit: (event) => events.push(event) })
  try {
    const terminal = manager.create('runtime-smoke', process.cwd())
    manager.resize(terminal.id, 100, 30)
    let gated = false
    try { manager.write(terminal.id, 'invalid', 'agent') } catch { gated = true }
    if (!gated) throw new Error('Agent write was not gated')
    manager.setOwner(terminal.id, 'agent')
    manager.write(terminal.id, process.platform === 'win32'
      ? "Write-Output ('ELECTRON_'+'PTY_OK')\r"
      : "printf 'ELECTRON_%s\\n' PTY_OK\n", 'agent')
    await until(() => manager.read(terminal.id).output.includes('ELECTRON_PTY_OK'))
    let childPid = 0
    if (process.platform === 'win32') {
      manager.write(terminal.id, "$child = Start-Process powershell.exe -WindowStyle Hidden -ArgumentList '-NoLogo -NoProfile -Command Start-Sleep -Seconds 60' -PassThru; Write-Output ('RUNTIME_'+'CHILD=' + $child.Id)\r", 'agent')
      await until(() => /RUNTIME_CHILD=\d+/.test(manager.read(terminal.id).output))
      childPid = Number(manager.read(terminal.id).output.match(/RUNTIME_CHILD=(\d+)/)[1])
      process.kill(childPid, 0)
    }
    manager.rebindWorkspace('runtime-smoke', 'runtime-recovered')
    const rebound = manager.list('runtime-recovered')[0]
    if (rebound.id !== terminal.id || rebound.pid !== terminal.pid) throw new Error('Workspace rebind replaced the terminal')
    let workspaceDenied = false
    try { manager.read(terminal.id, 'runtime-smoke') } catch { workspaceDenied = true }
    if (!workspaceDenied) throw new Error('Old workspace retained terminal access')
    manager.write(terminal.id, process.platform === 'win32' ? 'exit 9\r' : 'exit 9\n', 'agent')
    await until(() => manager.read(terminal.id).terminal.status === 'exited')
    if (manager.read(terminal.id).terminal.exitCode !== 9) throw new Error('Unexpected PTY exit code')
    await manager.close(terminal.id)
    if (childPid) {
      try {
        process.kill(childPid, 0)
        throw new Error('Detached child survived natural root exit and terminal close')
      } catch (error) {
        if (error.code !== 'ESRCH') throw error
      }
    }
    const cleanup = manager.create('runtime-smoke', process.cwd())
    await manager.close(cleanup.id)
    try {
      process.kill(cleanup.pid, 0)
      throw new Error('Terminal process survived cleanup')
    } catch (error) {
      if (error.code !== 'ESRCH') throw error
    }
    console.log(JSON.stringify({ result: 'terminal-runtime-ok', electron: process.versions.electron, node: process.versions.node, modules: process.versions.modules, events: events.length, roundtrip: true, resize: true, ownership: true, exitCode: 9, cleanup: true, detachedChildCleanup: true, workspaceRebind: true }))
  } finally {
    await manager.dispose()
  }
}

const electronApp = process.versions.electron ? require('electron').app : undefined
Promise.resolve(electronApp?.whenReady()).then(run).then(() => {
  if (electronApp) electronApp.exit(0)
}).catch((error) => {
  console.error(error)
  if (electronApp) electronApp.exit(1)
  else process.exitCode = 1
})
