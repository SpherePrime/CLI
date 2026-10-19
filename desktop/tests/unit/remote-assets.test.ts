import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'

const directories: string[] = []
afterEach(async () => {
  await Promise.all(directories.splice(0).map((path) => rm(path, { recursive: true, force: true })))
})

async function runtimeFixture() {
  const root = await mkdtemp(join(tmpdir(), 'prime-remote-assets-'))
  directories.push(root)
  for (const [directory, platform, arch, filename] of [
    ['', process.platform, process.arch, 'prime.exe'],
    ['remotes/linux-x64', 'linux', 'x64', 'prime'],
    ['remotes/linux-arm64', 'linux', 'arm64', 'prime']
  ]) {
    const target = join(root, directory)
    const bytes = Buffer.from(`runtime-${platform}-${arch}`)
    await mkdir(target, { recursive: true })
    await writeFile(join(target, filename), bytes)
    await writeFile(
      join(target, 'manifest.json'),
      JSON.stringify({
        version: '1.2.3',
        commit: 'abc123',
        platform,
        arch,
        protocolVersion: 1,
        sha256: createHash('sha256').update(bytes).digest('hex')
      })
    )
  }
  return root
}

function verify(root: string) {
  try {
    return {
      ok: true,
      output: execFileSync(process.execPath, [resolve('scripts/verify-runtime.mjs'), root], {
        encoding: 'utf8',
        stdio: 'pipe'
      })
    }
  } catch (error) {
    const failure = error as { stderr: Buffer }
    return { ok: false, output: failure.stderr.toString() }
  }
}

describe('bundled remote runtimes', () => {
  it('verifies matching Windows and both Linux architectures', async () => {
    expect(verify(await runtimeFixture()).ok).toBe(true)
  })
  it('rejects a missing ARM64 runtime', async () => {
    const root = await runtimeFixture()
    await rm(join(root, 'remotes/linux-arm64/prime'))
    expect(verify(root).ok).toBe(false)
  })
  it('rejects a tampered Linux binary', async () => {
    const root = await runtimeFixture()
    await writeFile(join(root, 'remotes/linux-x64/prime'), 'corrupted')
    expect(verify(root).ok).toBe(false)
  })
  it.each(['version', 'commit', 'platform', 'arch', 'protocolVersion'])(
    'rejects inconsistent Linux %s metadata',
    async (field) => {
      const root = await runtimeFixture()
      const path = join(root, 'remotes/linux-x64/manifest.json')
      const manifest = JSON.parse(await readFile(path, 'utf8'))
      manifest[field] = field === 'protocolVersion' ? 2 : 'incorrect'
      await writeFile(path, JSON.stringify(manifest))
      expect(verify(root).ok).toBe(false)
    }
  )
})
