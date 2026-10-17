import { execFileSync } from 'node:child_process'
import { test, expect, _electron as electron } from '@playwright/test'
import { mkdtemp, writeFile, mkdir, readFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { startProvider } from '../fixtures/provider-server'

test('opens the real Electron desktop and creates persistent Prime sessions', async () => {
  const project = await mkdtemp(join(tmpdir(), 'prime-desktop-проект-'))
  const provider = await startProvider()
  await mkdir(join(project, 'config'))
  await writeFile(
    join(project, 'config', 'prime.json'),
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
  await writeFile(join(project, 'hello.txt'), 'Desktop fixture')
  await writeFile(
    join(project, '.gitignore'),
    'config/\ncache/\nglobal-data/\nelectron-profile/\nxdg-*/\n.prime/\n'
  )
  const git = (...args: string[]) => execFileSync('git', args, { cwd: project, windowsHide: true })
  git('init')
  git('config', 'core.autocrlf', 'false')
  git('add', '.')
  git('-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.test', 'commit', '-m', 'fixture')
  const env: Record<string, string> = {
    ...Object.fromEntries(
      Object.entries(process.env).filter(
        (entry): entry is [string, string] => entry[1] !== undefined
      )
    ),
    PRIME_DESKTOP_TEST_PROJECT: project,
    PRIME_GLOBAL_CONFIG: join(project, 'config'),
    PRIME_GLOBAL_DATA: join(project, 'global-data'),
    PRIME_CACHE_DIR: join(project, 'cache'),
    XDG_CONFIG_HOME: join(project, 'xdg-config'),
    XDG_DATA_HOME: join(project, 'xdg-data')
  }
  delete env.ELECTRON_RUN_AS_NODE
  const desktop = await electron.launch({
    executablePath: process.env.PRIME_DESKTOP_PACKAGED_EXE,
    args: process.env.PRIME_DESKTOP_PACKAGED_EXE
      ? [`--user-data-dir=${join(project, 'electron-profile')}`]
      : [resolve('out/main/index.js'), `--user-data-dir=${join(project, 'electron-profile')}`],
    env
  })
  const errors: string[] = []
  const page = await desktop.firstWindow()
  page.on('pageerror', (error) => errors.push(error.message))
  try {
    await expect(page.getByRole('heading', { name: 'От идеи к работающему коду.' })).toBeVisible()
    await expect
      .poll(() => page.evaluate(() => window.prime.info()), { timeout: 30000 })
      .toMatchObject({ runtime: 'ready' })
    const opened = await page.evaluate((project) => window.prime.openProject(project), project)
    expect(opened?.path).toBe(project)
    const session = await page.evaluate(
      (id) => window.prime.call(id!, 'createSession', { title: 'Проверка сессии' }),
      opened?.id
    )
    expect(session.id).toBeTruthy()
    const sessions = await page.evaluate((id) => window.prime.call(id!, 'sessions'), opened?.id)
    expect(sessions.some((item: any) => item.id === session.id)).toBe(true)
    const files = await page.evaluate((id) => window.prime.files(id!), opened?.id)
    expect(files.some((file: any) => file.name === 'hello.txt')).toBe(true)
    await desktop.evaluate(({ dialog }, path) => {
      dialog.showOpenDialog = (async () => ({
        canceled: false,
        filePaths: [path]
      })) as typeof dialog.showOpenDialog
    }, project)
    await page.getByRole('button', { name: 'Открыть проект', exact: true }).first().click()
    await expect(page.getByRole('heading', { name: 'Что сделаем в проекте?' })).toBeVisible({
      timeout: 20000
    })
    await expect(page.getByRole('textbox', { name: 'Сообщение Prime' })).toBeEnabled({
      timeout: 20000
    })
    await page.getByRole('textbox', { name: 'Сообщение Prime' }).fill('Проверка связи через CLI')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect(
      page
        .locator('.message.assistant .markdown')
        .getByText('Соединение работает. Prime Desktop получил ответ через встроенный CLI.', {
          exact: true
        })
    ).toBeVisible({ timeout: 30000 })
    expect(provider.prompts.length).toBeGreaterThan(0)
    await page.reload()
    await expect(
      page
        .locator('.message.assistant .markdown')
        .getByText('Соединение работает. Prime Desktop получил ответ через встроенный CLI.', {
          exact: true
        })
    ).toBeVisible({ timeout: 15000 })
    await page
      .getByRole('textbox', { name: 'Сообщение Prime' })
      .fill('TOOL_FIXTURE Создай тестовый файл')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    try {
      await expect(page.getByRole('heading', { name: 'Разрешить действие?' })).toBeVisible({
        timeout: 15000
      })
    } catch (error) {
      console.log(
        'PENDING',
        await page.evaluate((id) => window.prime.call(id!, 'pending'), opened?.id)
      )
      console.log('PROJECTS', await page.evaluate(() => window.prime.projects()))
      throw error
    }
    await page.getByRole('button', { name: 'Разрешить', exact: true }).click()
    await expect
      .poll(() => readFile(join(project, 'created-by-tool.txt'), 'utf8'))
      .toBe('Tool execution verified')
    await expect(page.getByRole('button', { name: 'Остановить задачу', exact: true })).toHaveCount(
      0,
      { timeout: 15000 }
    )
    await page
      .getByRole('textbox', { name: 'Сообщение Prime' })
      .fill('QUESTION_FIXTURE Задай вопрос')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect(page.getByText('Выберите вариант проверки', { exact: true })).toBeVisible({
      timeout: 15000
    })
    await page.getByRole('radio', { name: 'Первый вариант', exact: true }).check()
    await page.getByRole('button', { name: 'Ответить', exact: true }).click()
    await expect(page.getByRole('button', { name: 'Остановить задачу', exact: true })).toHaveCount(
      0,
      { timeout: 15000 }
    )
    await page.getByRole('button', { name: 'Изменения', exact: true }).click()
    await page.getByRole('combobox', { name: 'Источник сравнения' }).selectOption('git')
    await page.getByRole('button', { name: 'created-by-tool.txt', exact: false }).click()
    await expect(page.locator('.diff-preview')).toContainText('+Tool execution verified')
    await page.getByRole('button', { name: 'Закрыть', exact: true }).click()
    await page.getByRole('button', { name: 'Файлы', exact: true }).click()
    await page.getByRole('button', { name: 'Настройки', exact: false }).first().click()
    await expect(page.getByRole('heading', { name: 'Настройки', exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Светлая', exact: false }).click()
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
    await page.getByRole('button', { name: 'Тёмная', exact: false }).click()
    await page.getByLabel('Язык интерфейса').selectOption('en')
    await expect(page.getByRole('heading', { name: 'Settings', exact: true })).toBeVisible()
    await expect(page.locator('html')).toHaveAttribute('lang', 'en')
    await page.getByRole('button', { name: 'Close', exact: true }).click()
    await page.reload()
    await expect(page.getByRole('textbox', { name: 'Message Prime' })).toBeVisible()
    await page.getByRole('button', { name: 'Settings', exact: false }).first().click()
    await page.getByLabel('Interface language').selectOption('ru')
    await page.getByRole('button', { name: 'Закрыть', exact: true }).click()
    await page.screenshot({ path: 'test-results/prime-desktop.png' })
    expect(errors).toEqual([])
    await page.evaluate((id) => window.prime.closeProject(id!), opened?.id)
  } finally {
    await desktop.close()
    await provider.close()
  }
})
