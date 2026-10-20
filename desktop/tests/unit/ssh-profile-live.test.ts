import { expect, it, vi } from 'vitest'
import { mkdtemp, rm, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { RemoteManager } from '../../src/main/remote/manager'
import { SshAccess } from '../../src/main/remote/access'
import { createSshExecutionTools } from '../../src/main/remote/execution'
import { uploadLocalFile } from '../../src/main/remote/upload'
import { createSshLinuxFixture, detectSshLinuxFixture } from '../fixtures/ssh-linux'

it('uses a saved profile from a local chat over real SSH and uploads a binary without opening a workspace', async (context) => {
  const support = await detectSshLinuxFixture()
  if (!support.available) {
    context.skip(support.reason)
    return
  }
  const fixture = await createSshLinuxFixture()
  const local = await mkdtemp(join(tmpdir(), 'prime-profile-live-'))
  const onReady = vi.fn(async () => {})
  const manager = new RemoteManager({ path: join(local, 'profiles.json'), emit: () => {}, onReady })
  try {
    await manager.load()
    const saved = await manager.save({
      name: 'Fixture',
      host: fixture.host,
      port: fixture.port,
      user: fixture.user,
      root: fixture.root,
      auth: 'key',
      keyPath: fixture.keyPath,
      runtime: 'bundled'
    })
    const profileId = saved.profiles[0].id
    const access = new SshAccess({ manager: () => manager, byWorkspace: () => undefined })
    await expect(access.resolve('ordinary-chat', profileId)).rejects.toThrow('fingerprint')
    await manager.trust(profileId, manager.list().profiles[0].challenge!.fingerprint)
    const connection = await access.resolve('ordinary-chat', profileId)
    expect(onReady).not.toHaveBeenCalled()
    const tools = createSshExecutionTools((workspace, profile) =>
      access.resolve(workspace, profile)
    )
    const output = await tools
      .find((tool) => tool.name === 'ssh_exec')!
      .run({ profileId, command: 'pwd' }, { workspaceId: 'ordinary-chat' })
    expect(JSON.parse((output[0] as any).text).stdout.trim()).toBe(fixture.root)
    const source = join(local, 'sample.bin')
    await writeFile(source, Buffer.from([0, 255, 13, 10]))
    await uploadLocalFile(connection, source, 'sample.bin')
    expect(await fixture.execute(`od -An -t x1 '${fixture.root}/sample.bin'`)).toContain(
      '00 ff 0d 0a'
    )
    await expect(uploadLocalFile(connection, source, 'sample.bin')).rejects.toThrow()
    expect(onReady).not.toHaveBeenCalled()
  } finally {
    await manager.shutdown()
    await fixture.close()
    await rm(local, { recursive: true, force: true })
  }
}, 60000)
