import { test, expect } from '@playwright/test'
import { generateKeyPairSync } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { join } from 'node:path'
import ssh2, { type Connection } from 'ssh2'
import { launchWorkbench } from '../fixtures/workbench-app'

test('SSH passwords survive app restart in OS-encrypted storage without being returned to the renderer', async () => {
  const key = generateKeyPairSync('rsa', { modulusLength: 2048 }).privateKey.export({
    type: 'pkcs1',
    format: 'pem'
  })
  const clients: Connection[] = []
  const passwords: string[] = []
  const server = new ssh2.Server({ hostKeys: [key] }, (client) => {
    clients.push(client)
    client.on('error', () => {})
    client.on('authentication', (context) => {
      if (context.method === 'password') passwords.push(context.password)
      context.reject()
    })
  })
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  const fixture = await launchWorkbench()
  try {
    const initial = await fixture.page.evaluate(
      (port) =>
        window.prime.remote({
          action: 'save',
          input: {
            name: 'Credential fixture',
            host: '127.0.0.1',
            port,
            user: 'fixture',
            root: '/project',
            auth: 'password',
            runtime: 'bundled'
          }
        }),
      (server.address() as { port: number }).port
    )
    const id = initial.profiles[0].id
    const snapshot = await fixture.page.evaluate(
      (id) =>
        window.prime.remote({
          action: 'connect',
          id,
          credentials: { password: 'fixture-private-password' }
        }),
      id
    )
    expect(snapshot.profiles[0].hasSavedCredentials).toBe(true)
    await fixture.page.evaluate(
      ({ id, fingerprint }) => window.prime.remote({ action: 'trust', id, fingerprint }),
      { id, fingerprint: snapshot.profiles[0].challenge!.fingerprint }
    )
    const bytes = await readFile(join(fixture.root, 'profile', 'remote-credentials.json'), 'utf8')
    expect(bytes).not.toContain('fixture-private-password')
    const { page } = await fixture.restart()
    const restored = await page.evaluate(() => window.prime.remote({ action: 'list' }))
    expect(restored.profiles[0].hasSavedCredentials).toBe(true)
    expect(JSON.stringify(restored)).not.toContain('fixture-private-password')
    await page.evaluate((id) => window.prime.remote({ action: 'connect', id }), id)
    expect(passwords).toContain('fixture-private-password')
    await page.evaluate((id) => window.prime.remote({ action: 'forgetCredentials', id }), id)
    expect(
      (await page.evaluate(() => window.prime.remote({ action: 'list' }))).profiles[0]
        .hasSavedCredentials
    ).toBe(false)
  } finally {
    await fixture.close()
    clients.forEach((client) => client.end())
    await new Promise<void>((resolve) => server.close(() => resolve()))
  }
})
