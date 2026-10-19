import { _electron as electron, expect } from '@playwright/test'
import { mkdtemp, mkdir, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { execFileSync } from 'node:child_process'
import { startProvider } from './provider-server'
import type { Project } from '../../src/shared/contracts'

export async function launchWorkbench() {
  const root = await mkdtemp(join(tmpdir(), 'prime-workbench-'))
  const provider = await startProvider()
  await mkdir(join(root, 'config'))
  await writeFile(
    join(root, 'config', 'prime.json'),
    JSON.stringify({
      options: {
        disable_default_providers: true,
        disable_provider_auto_update: true,
        disable_metrics: true,
        disable_auto_summarize: true
      },
      providers: {
        fixture: {
          id: 'fixture',
          name: 'Fixture',
          type: 'openai-compat',
          base_url: provider.url,
          api_key: 'fixture-key',
          discover_models: false,
          models: [
            {
              id: 'desktop-fixture',
              name: 'Desktop Fixture',
              context_window: 128000,
              default_max_tokens: 8192
            }
          ]
        }
      },
      models: {
        large: { provider: 'fixture', model: 'desktop-fixture' },
        small: { provider: 'fixture', model: 'desktop-fixture' }
      }
    })
  )
  await writeFile(join(root, 'app.ts'), 'export const version = 1\n')
  execFileSync('git', ['init', '-q'], { cwd: root })
  execFileSync('git', ['add', 'app.ts'], { cwd: root })
  execFileSync(
    'git',
    [
      '-c',
      'user.name=Fixture',
      '-c',
      'user.email=fixture@example.test',
      'commit',
      '-qm',
      'fixture'
    ],
    { cwd: root }
  )
  const env = {
    ...Object.fromEntries(
      Object.entries(process.env).filter(
        (entry): entry is [string, string] =>
          entry[1] !== undefined && entry[0] !== 'ELECTRON_RUN_AS_NODE'
      )
    ),
    PRIME_DESKTOP_TEST_PROJECT: root,
    PRIME_GLOBAL_CONFIG: join(root, 'config'),
    PRIME_GLOBAL_DATA: join(root, 'data'),
    PRIME_CACHE_DIR: join(root, 'cache'),
    XDG_CONFIG_HOME: join(root, 'xdg-config'),
    XDG_DATA_HOME: join(root, 'xdg-data')
  }
  const app = await electron.launch({
    args: [resolve('out/main/index.js'), `--user-data-dir=${join(root, 'profile')}`],
    env
  })
  const page = await app.firstWindow()
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  let project: Project | null = null
  try {
    await expect
      .poll(
        async () => {
          try {
            project = await page.evaluate((root) => window.prime.openProject(root), root)
            return Boolean(project)
          } catch {
            return false
          }
        },
        { timeout: 30000 }
      )
      .toBe(true)
  } catch (error) {
    await app.close()
    await provider.close()
    throw error
  }
  await page.reload()
  await page
    .getByRole('textbox', { name: 'Сообщение Prime', exact: true })
    .waitFor({ state: 'visible' })
  return {
    root,
    provider,
    app,
    page,
    project: project!,
    errors,
    close: async () => {
      await page.evaluate(() => window.prime.editorDirty(false)).catch(() => {})
      await app.close()
      await provider.close()
    }
  }
}
