import { execFileSync } from 'node:child_process'
import { test, expect, _electron as electron } from '@playwright/test'
import { mkdtemp, writeFile, mkdir, readFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { startProvider } from '../fixtures/provider-server'

test('opens the real Electron desktop and creates persistent Prime sessions', async () => {
  test.setTimeout(120000)
  const editedSource =
    "export interface AppSettings {\n  name: string\n  autoSave: boolean\n}\n\nexport function createSettings(\n  name: string\n): AppSettings {\n  return { name, autoSave: false }\n}\n\nexport const settings = createSettings(\n  'Prime Desktop'\n)\n"
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
            },
            {
              id: 'secondary-fixture',
              name: 'Secondary Fixture',
              context_window: 131072,
              default_max_tokens: 4096
            },
            ...Array.from({ length: 20 }, (_, index) => ({
              id: 'extra-' + index,
              name: 'Extra Model ' + index,
              context_window: 64000,
              default_max_tokens: 8192
            }))
          ]
        }
      },
      models: {
        large: { provider: 'fixture', model: 'desktop-fixture' },
        small: { provider: 'fixture', model: 'desktop-fixture' }
      }
    })
  )
  await writeFile(
    join(project, 'config', 'mcp.json'),
    JSON.stringify({
      mcp: { 'Global Fixture MCP': { type: 'stdio', command: 'fixture-disabled', disabled: true } }
    })
  )
  await writeFile(join(project, 'hello.txt'), 'Desktop fixture')
  await writeFile(join(project, 'editor-example.ts'), 'export const answer: number = 42\n')
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
  const launch = () =>
    electron.launch({
      executablePath: process.env.PRIME_DESKTOP_PACKAGED_EXE,
      args: process.env.PRIME_DESKTOP_PACKAGED_EXE
        ? [`--user-data-dir=${join(project, 'electron-profile')}`]
        : [resolve('out/main/index.js'), `--user-data-dir=${join(project, 'electron-profile')}`],
      env
    })
  let desktop = await launch()
  const errors: string[] = []
  let page = await desktop.firstWindow()
  const workers: string[] = []
  page.on('worker', (worker) => workers.push(worker.url()))
  page.on('pageerror', (error) => errors.push(error.message))
  try {
    await expect(page.getByRole('heading', { name: 'С чего начнём?' })).toBeVisible()
    await expect
      .poll(() => page.evaluate(() => window.prime.info()), { timeout: 30000 })
      .toMatchObject({ runtime: 'ready' })
    await expect(page.getByRole('textbox', { name: 'Сообщение Prime' })).toBeEnabled()
    await expect(page.getByRole('button', { name: 'Поиск и команды' })).toHaveCount(0)
    await page.getByRole('button', { name: 'Ресурсы', exact: true }).click()
    await expect(page.getByText('Global Fixture MCP', { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Файлы', exact: true }).click()
    await page.getByRole('button', { name: 'Выбрать модель', exact: true }).click()
    await expect(page.getByRole('listbox', { name: 'Модели' })).toBeVisible()
    expect(
      await page
        .getByRole('listbox', { name: 'Модели' })
        .evaluate((element) => element.scrollHeight > element.clientHeight)
    ).toBe(true)
    await page.screenshot({ path: 'test-results/model-picker.png' })
    await page.getByRole('option', { name: /Desktop Fixture/ }).click()
    await page.getByRole('textbox', { name: 'Сообщение Prime' }).fill('Свободный чат без папки')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect(page.locator('.message.assistant .markdown')).toContainText(
      'Соединение работает.',
      { timeout: 20000 }
    )
    await page.reload()
    await expect(page.locator('.message.user')).toContainText('Свободный чат без папки')
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
    await expect(page.getByText('Shift Enter — новая строка', { exact: true })).toHaveCount(0)
    await expect(page.locator('.loading-banner')).toHaveCount(0)
    await expect(page.getByRole('textbox', { name: 'Поиск чатов' })).toHaveCount(0)
    await page.getByRole('button', { name: 'Найти чат', exact: true }).click()
    await page.getByRole('textbox', { name: 'Поиск чатов' }).fill('Проверка сессии')
    await expect(page.getByRole('button', { name: 'Проверка сессии', exact: true })).toBeVisible()
    await page.screenshot({ path: 'test-results/chat-search.png' })
    await page.getByRole('button', { name: 'Закрыть поиск', exact: true }).click()
    await page.getByRole('button', { name: 'Режим доступа', exact: true }).click()
    await page.getByRole('menuitemradio', { name: /Полный доступ/ }).click()
    await expect
      .poll(() => page.evaluate((id) => window.prime.call(id!, 'getSkip'), opened?.id))
      .toMatchObject({ skip: true })
    await page.getByRole('button', { name: 'Режим доступа', exact: true }).click()
    await page.screenshot({ path: 'test-results/permissions-menu.png' })
    await page.getByRole('menuitemradio', { name: /С подтверждением/ }).click()
    await page.getByRole('button', { name: 'editor-example.ts', exact: true }).click()
    const code = page.getByRole('textbox', { name: 'Код файла', exact: true })
    await expect(code).toBeAttached({ timeout: 30000 })
    await expect(page.locator('.file-editor .monaco-editor-background').first()).toHaveCSS(
      'background-color',
      'rgb(16, 16, 16)'
    )
    await code.press('ControlOrMeta+A')
    await desktop.evaluate(({ clipboard }, text) => clipboard.writeText(text), editedSource)
    await code.press('ControlOrMeta+V')
    await page.getByRole('button', { name: 'Закрыть файл editor-example.ts', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Сохранить изменения файла?' })).toBeVisible()
    await page.getByRole('button', { name: 'Отмена', exact: true }).click()
    await code.press('ControlOrMeta+s')
    await expect
      .poll(async () =>
        (await readFile(join(project, 'editor-example.ts'), 'utf8')).replaceAll('\r\n', '\n')
      )
      .toBe(editedSource)
    await expect
      .poll(() =>
        page
          .locator('.file-editor .view-lines span[class^=mtk]')
          .evaluateAll((items) => new Set(items.map((item) => item.className)).size)
      )
      .toBeGreaterThan(1)
    await expect.poll(() => workers.some((url) => url.startsWith('blob:'))).toBe(true)
    await page.screenshot({ path: 'test-results/file-editor.png' })
    await page.getByRole('button', { name: 'Закрыть файл editor-example.ts', exact: true }).click()
    await page.getByRole('button', { name: 'Действия: Проверка сессии', exact: true }).click()
    await page.getByRole('button', { name: 'Удалить чат', exact: true }).click()
    const deletion = page.getByRole('dialog', { name: 'Удалить чат и его историю?' })
    await expect(deletion).toBeVisible()
    await expect(deletion.getByText('Проверка сессии', { exact: true })).toBeVisible()
    await expect(deletion.getByRole('button', { name: 'Отмена', exact: true })).toBeFocused()
    const box = await deletion.boundingBox()
    const viewport = await page.evaluate(() => ({ width: innerWidth, height: innerHeight }))
    expect(Math.abs(box!.x + box!.width / 2 - viewport.width / 2)).toBeLessThan(2)
    expect(Math.abs(box!.y + box!.height / 2 - viewport.height / 2)).toBeLessThan(2)
    await page.screenshot({ path: 'test-results/delete-chat.png' })
    await page.keyboard.press('Escape')
    await expect(deletion).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Проверка сессии', exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Действия: Проверка сессии', exact: true }).click()
    await page.getByRole('button', { name: 'Удалить чат', exact: true }).click()
    await deletion.getByRole('button', { name: 'Удалить', exact: true }).click()
    await expect(deletion).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Проверка сессии', exact: true })).toHaveCount(0)
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
    await page.getByRole('combobox', { name: 'Источник сравнения' }).click()
    await page.screenshot({ path: 'test-results/compare-picker.png' })
    await page.getByRole('option', { name: 'Сравнить с Git HEAD' }).click()
    await page.getByRole('button', { name: 'created-by-tool.txt', exact: false }).click()
    await expect(page.locator('.file-editor .monaco-diff-editor')).toContainText(
      'Tool execution verified',
      { timeout: 20000 }
    )
    await page
      .getByRole('button', { name: 'Закрыть файл created-by-tool.txt', exact: true })
      .click()
    await page.getByRole('button', { name: 'Файлы', exact: true }).click()
    await page.getByRole('button', { name: 'Настройки', exact: false }).first().click()
    await expect(page.getByRole('heading', { name: 'Настройки', exact: true })).toBeVisible()
    const settingsBounds = await page.locator('.settings-modal').boundingBox()
    expect(settingsBounds!.width).toBeGreaterThan(1000)
    expect(settingsBounds!.height).toBeGreaterThan(700)
    await page.getByRole('combobox', { name: 'Язык интерфейса' }).click()
    await page.keyboard.press('Escape')
    await expect(page.getByRole('heading', { name: 'Настройки', exact: true })).toBeVisible()
    await expect(page.locator('.prime-popover')).toHaveCount(0)
    await page.screenshot({ path: 'test-results/settings-appearance.png' })
    await page.getByRole('button', { name: 'Провайдеры', exact: true }).click()
    await page.getByRole('button', { name: 'Изменить', exact: true }).click()
    await page.getByRole('tab', { name: 'Secondary Fixture', exact: false }).click()
    await expect(page.getByLabel('Контекст', { exact: true })).toHaveValue('131072')
    await page.getByLabel('Контекст', { exact: true }).fill('262144')
    await page.getByRole('button', { name: 'Сохранить изменения', exact: true }).click()
    await expect(page.getByRole('status')).toContainText('Настройки провайдера сохранены')
    const edited = await page.evaluate((id) => window.prime.call(id!, 'config'), opened?.id)
    expect(edited.providers.fixture.models).toHaveLength(22)
    expect(
      edited.providers.fixture.models.find((item: any) => item.id === 'secondary-fixture')
    ).toMatchObject({ context_window: 262144, default_max_tokens: 4096 })
    expect(
      edited.providers.fixture.models.find((item: any) => item.id === 'desktop-fixture')
    ).toMatchObject({ context_window: 128000, default_max_tokens: 8192 })
    await page.screenshot({ path: 'test-results/settings-providers.png' })
    await page.getByRole('button', { name: 'Интерфейс', exact: true }).click()
    await page.getByRole('button', { name: 'Светлая', exact: false }).click()
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
    await page.getByRole('button', { name: 'Тёмная', exact: false }).click()
    await page.getByRole('combobox', { name: 'Язык интерфейса' }).click()
    await page.getByRole('option', { name: 'Английский', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Settings', exact: true })).toBeVisible()
    await expect(page.locator('html')).toHaveAttribute('lang', 'en')
    await page.getByRole('button', { name: 'Close', exact: true }).click()
    await page.reload()
    await expect(page.getByRole('textbox', { name: 'Message Prime' })).toBeVisible()
    await page.getByRole('button', { name: 'Settings', exact: false }).first().click()
    await page.getByRole('combobox', { name: 'Interface language' }).click()
    await page.getByRole('option', { name: 'Russian', exact: true }).click()
    await page.getByRole('button', { name: 'Закрыть', exact: true }).click()
    await page.screenshot({ path: 'test-results/prime-desktop.png' })
    expect(errors).toEqual([])
    await page.evaluate(async (id) => {
      const personal = (await window.prime.projects()).find((item) => item.kind === 'chat')!
      const original = personal.currentSessionId!
      await window.prime.call(personal.id, 'createSession', { title: 'Newer personal chat' })
      await window.prime.call(personal.id, 'currentSession', { sessionId: original })
      const project = (await window.prime.projects()).find((item) => item.id === id)!
      await window.prime.call(project.id, 'currentSession', { sessionId: project.currentSessionId })
    }, opened?.id)
    await desktop.close()
    desktop = await launch()
    page = await desktop.firstWindow()
    await expect(page.getByRole('textbox', { name: 'Сообщение Prime' })).toBeEnabled({
      timeout: 20000
    })
    await page.getByRole('button', { name: 'Чат без проекта', exact: true }).click()
    await expect(
      page.locator('.message.user').getByText('Свободный чат без папки', { exact: true })
    ).toBeVisible({ timeout: 15000 })
    const before = provider.prompts.length
    await page
      .getByRole('textbox', { name: 'Сообщение Prime' })
      .fill('Возврат к личному чату после перезапуска')
    await page.getByRole('button', { name: 'Отправить сообщение', exact: true }).click()
    await expect.poll(() => provider.prompts.length).toBeGreaterThan(before)
    await expect(page.getByRole('button', { name: 'Остановить задачу', exact: true })).toHaveCount(
      0,
      { timeout: 15000 }
    )
  } finally {
    await desktop.close()
    await provider.close()
  }
})
