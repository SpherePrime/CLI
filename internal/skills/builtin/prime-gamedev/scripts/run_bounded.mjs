#!/usr/bin/env node
import { spawn, spawnSync } from 'node:child_process'
import { resolve } from 'node:path'
import { pathToFileURL } from 'node:url'

const FATAL_ENGINE_ERROR_RE =
  /(SCRIPT ERROR:|Parse Error:|Failed to load script|Unhandled Exception:|Fatal error:|Traceback \(most recent call last\))/i

function printUsage() {
  console.log(`Usage:
  node run_bounded.mjs [--timeout-ms 45000] [--idle-ms 15000] [--cwd <dir>] -- <command> [args...]`)
}

function killProcessTree(pid) {
  if (!pid) {
    return
  }
  if (process.platform === 'win32') {
    spawnSync('taskkill', ['/PID', String(pid), '/T', '/F'], {
      stdio: 'ignore',
      windowsHide: true
    })
    return
  }
  try {
    process.kill(-pid, 'SIGTERM')
  } catch {
    try {
      process.kill(pid, 'SIGTERM')
    } catch {
    }
  }
  setTimeout(() => {
    try {
      process.kill(-pid, 'SIGKILL')
    } catch {
      try {
        process.kill(pid, 'SIGKILL')
      } catch {
      }
    }
  }, 1000).unref()
}

function parseCliArgs(argv) {
  let timeoutMs = 45000
  let idleMs = 15000
  let cwd = process.cwd()
  let help = false
  const cmdParts = []

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i]
    if (arg === '--') {
      cmdParts.push(...argv.slice(i + 1))
      break
    }
    if (arg === '--help' || arg === '-h') {
      help = true
      continue
    }
    if (arg === '--timeout-ms' && argv[i + 1]) {
      timeoutMs = Math.max(1000, Number(argv[++i]) || 45000)
      continue
    }
    if (arg === '--idle-ms' && argv[i + 1]) {
      idleMs = Math.max(500, Number(argv[++i]) || 15000)
      continue
    }
    if (arg === '--cwd' && argv[i + 1]) {
      cwd = resolve(argv[++i])
      continue
    }
    cmdParts.push(...argv.slice(i))
    break
  }

  return { timeoutMs, idleMs, cwd, help, cmdParts }
}

export function runBoundedCommand({ command, args = [], cwd = process.cwd(), timeoutMs = 45000, idleMs = 15000 }) {
  return new Promise((resolvePromise) => {
    const child = spawn(command, args, {
      cwd,
      shell: args.length === 0,
      detached: process.platform !== 'win32',
      stdio: ['ignore', 'pipe', 'pipe']
    })

    let lastActivityAt = Date.now()
    let sawFatalErrorAt = 0
    let finished = false
    let abortReason = ''
    let recentStderr = ''

    const finish = (code, reason) => {
      if (finished) {
        return
      }
      finished = true
      clearTimeout(hardTimer)
      clearInterval(idleTimer)
      resolvePromise({ code, reason, recentStderr })
    }

    const abort = (reason) => {
      if (finished || abortReason) {
        return
      }
      abortReason = reason
      clearTimeout(hardTimer)
      clearInterval(idleTimer)
      console.error(`\n[run_bounded] ${reason}. Killing process tree (pid=${child.pid})...`)
      killProcessTree(child.pid)
      setTimeout(() => finish(124, reason), 1200).unref()
    }

    const onOutput = (chunk, isErr) => {
      const text = chunk.toString()
      lastActivityAt = Date.now()
      if (isErr) {
        process.stderr.write(chunk)
        recentStderr = (recentStderr + text).slice(-4000)
      } else {
        process.stdout.write(chunk)
      }
      if (!sawFatalErrorAt && FATAL_ENGINE_ERROR_RE.test(text)) {
        sawFatalErrorAt = Date.now()
      }
    }

    child.stdout?.on('data', (chunk) => onOutput(chunk, false))
    child.stderr?.on('data', (chunk) => onOutput(chunk, true))

    const hardTimer = setTimeout(() => {
      abort(`HARD TIMEOUT after ${timeoutMs}ms (app/test failed to exit cleanly)`)
    }, timeoutMs)

    const idleTimer = setInterval(() => {
      const now = Date.now()
      const silentFor = now - lastActivityAt
      if (sawFatalErrorAt && silentFor >= Math.min(idleMs, 3000)) {
        abort(`FATAL SCRIPT/ENGINE ERROR followed by ${silentFor}ms hang (grey/frozen screen detected)`)
        return
      }
      if (silentFor >= idleMs) {
        abort(`IDLE HANG DETECTED: no stdout/stderr output for ${silentFor}ms (limit ${idleMs}ms)`)
      }
    }, Math.min(500, Math.max(100, Math.floor(idleMs / 4))))

    child.on('error', (err) => {
      console.error(`[run_bounded] spawn error: ${err.message}`)
      finish(127, err.message)
    })

    child.on('exit', (code) => {
      if (abortReason) {
        finish(124, abortReason)
        return
      }
      finish(code ?? 1, '')
    })
  })
}

async function main() {
  const { timeoutMs, idleMs, cwd, help, cmdParts } = parseCliArgs(process.argv.slice(2))
  if (help || cmdParts.length === 0) {
    printUsage()
    process.exit(help ? 0 : 1)
  }
  const [command, ...args] = cmdParts
  const result = await runBoundedCommand({ command, args, cwd, timeoutMs, idleMs })
  process.exit(result.code)
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch((err) => {
    console.error(err?.stack || err?.message || err)
    process.exit(2)
  })
}
