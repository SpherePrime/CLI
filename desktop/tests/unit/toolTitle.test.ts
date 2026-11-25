import { describe, expect, it } from 'vitest'
import {
  AlertTriangle,
  AppWindow,
  ArrowRightLeft,
  Brain,
  ClipboardList,
  Cpu,
  FilePen,
  FileSpreadsheet,
  FileText,
  GitBranch,
  Github,
  Globe,
  History,
  Network,
  Plug,
  Presentation,
  Send,
  Server,
  ShieldAlert,
  UserCog,
  UserMinus,
  UserPlus,
  Users,
  Wrench
} from 'lucide-react'
import { translate } from '../../src/shared/localization'
import { toolTitle } from '../../src/renderer/src/features/chat/toolTitle'

const ru = (value: string) => translate('ru', value)
const en = (value: string) => translate('en', value)

describe('custom agent tool titles', () => {
  it('formats custom_agent_memory targets and operations in Russian and English', () => {
    const userReadRu = toolTitle('custom_agent_memory', { target: 'user', op: 'read' }, ru)
    expect(userReadRu).toEqual({
      icon: Brain,
      label: 'Память агентов',
      detail: 'Профиль пользователя · чтение'
    })

    const userReadEn = toolTitle('custom_agent_memory', { target: 'user', op: 'read' }, en)
    expect(userReadEn).toEqual({
      icon: Brain,
      label: 'Agent memory',
      detail: 'User profile · read'
    })

    const sharedAppendRu = toolTitle('custom_agent_memory', { target: 'memory', op: 'append' }, ru)
    expect(sharedAppendRu).toEqual({
      icon: Brain,
      label: 'Память агентов',
      detail: 'Общая память · дополнение'
    })

    const sharedAppendEn = toolTitle('custom_agent_memory', { target: 'memory', op: 'append' }, en)
    expect(sharedAppendEn).toEqual({
      icon: Brain,
      label: 'Agent memory',
      detail: 'Shared memory · append'
    })

    const agentWriteRu = toolTitle(
      'custom_agent_memory',
      { slug: 'auditor', op: 'replace' },
      ru
    )
    expect(agentWriteRu).toEqual({
      icon: Brain,
      label: 'Память агентов',
      detail: 'auditor · замена'
    })

    const idWriteEn = toolTitle(
      'custom_agent_memory',
      { agent_id: 'agent-42', op: 'write' },
      en
    )
    expect(idWriteEn).toEqual({
      icon: Brain,
      label: 'Agent memory',
      detail: 'agent-42 · write'
    })
  })

  it('formats all remaining custom_agent_* tools in Russian and English', () => {
    expect(toolTitle('custom_agent_list', {}, ru)).toEqual({
      icon: Users,
      label: 'Список агентов',
      detail: ''
    })
    expect(toolTitle('custom_agent_list', {}, en).label).toBe('Agent roster')

    expect(toolTitle('custom_agent_list_models', {}, ru)).toEqual({
      icon: Cpu,
      label: 'Модели агентов',
      detail: ''
    })
    expect(toolTitle('custom_agent_list_models', {}, en).label).toBe('Agent models')

    expect(toolTitle('custom_agent_create', { name: 'Build Auditor', slug: 'auditor' }, ru)).toEqual({
      icon: UserPlus,
      label: 'Создание агента',
      detail: 'Build Auditor'
    })
    expect(toolTitle('custom_agent_create', { slug: 'auditor' }, en).label).toBe('Create agent')

    expect(toolTitle('custom_agent_update', { slug: 'auditor', id: 'a1' }, ru)).toEqual({
      icon: UserCog,
      label: 'Обновление агента',
      detail: 'auditor'
    })
    expect(toolTitle('custom_agent_update', { id: 'a1' }, en)).toEqual({
      icon: UserCog,
      label: 'Update agent',
      detail: 'a1'
    })

    expect(toolTitle('custom_agent_delete', { slug: 'old-bot' }, ru)).toEqual({
      icon: UserMinus,
      label: 'Удаление агента',
      detail: 'old-bot'
    })
    expect(toolTitle('custom_agent_delete', { id: 'a9' }, en)).toEqual({
      icon: UserMinus,
      label: 'Delete agent',
      detail: 'a9'
    })

    expect(
      toolTitle('custom_agent_dispatch', { slug: 'reviewer', prompt: 'Check changed files' }, ru)
    ).toEqual({
      icon: Send,
      label: 'Запуск агента',
      detail: 'reviewer · Check changed files'
    })
    expect(
      toolTitle('custom_agent_dispatch', { agent_id: 'a2', prompt: 'Run audit' }, en).label
    ).toBe('Run agent')

    expect(toolTitle('custom_agent_swarm', { goal: 'Audit all modules' }, ru)).toEqual({
      icon: Network,
      label: 'Рой агентов',
      detail: 'Audit all modules'
    })
    expect(toolTitle('custom_agent_swarm', { goal: 'Audit all modules' }, en).label).toBe(
      'Agent swarm'
    )

    expect(
      toolTitle('custom_agent_workflow', { name: 'Release Flow', op: 'run' }, ru)
    ).toEqual({
      icon: GitBranch,
      label: 'Цепочка агентов',
      detail: 'Release Flow · run'
    })
    expect(
      toolTitle('custom_agent_workflow', { workflow_id: 'wf-1', op: 'list' }, en).label
    ).toBe('Agent workflow')

    expect(toolTitle('custom_agent_runs', { slug: 'auditor' }, ru)).toEqual({
      icon: History,
      label: 'История запусков',
      detail: 'auditor'
    })
    expect(toolTitle('custom_agent_runs', { agent_id: 'a1' }, en).label).toBe('Run history')

    expect(toolTitle('custom_agent_blackboard', { topic: 'findings', op: 'post' }, ru)).toEqual({
      icon: ClipboardList,
      label: 'Доска координации',
      detail: 'findings · post'
    })
    expect(toolTitle('custom_agent_blackboard', { op: 'read' }, en)).toEqual({
      icon: ClipboardList,
      label: 'Coordination board',
      detail: 'read'
    })
  })
})

describe('orchestrator, privilege and lsp_replace_symbol tool titles', () => {
  it('formats handoff_to_executor using first plan line or target_files fallback', () => {
    const withPlan = toolTitle(
      'handoff_to_executor',
      { plan: '\n  Update toolTitle mappings  \nSecond step' },
      ru
    )
    expect(withPlan).toEqual({
      icon: ArrowRightLeft,
      label: 'Передача исполнителю',
      detail: 'Update toolTitle mappings'
    })

    const withFiles = toolTitle(
      'handoff_to_executor',
      { target_files: ['desktop/src/toolTitle.ts', 'desktop/src/Approvals.tsx'] },
      en
    )
    expect(withFiles).toEqual({
      icon: ArrowRightLeft,
      label: 'Handoff to executor',
      detail: 'toolTitle.ts, Approvals.tsx'
    })
  })

  it('formats escalate_to_planner and sudo', () => {
    expect(
      toolTitle('escalate_to_planner', { reason: 'Missing API schema in backend' }, ru)
    ).toEqual({
      icon: AlertTriangle,
      label: 'Возврат планировщику',
      detail: 'Missing API schema in backend'
    })
    expect(toolTitle('escalate_to_planner', { reason: 'Blocked' }, en).label).toBe(
      'Escalate to planner'
    )

    expect(toolTitle('sudo', { description: 'Install system package' }, ru)).toEqual({
      icon: ShieldAlert,
      label: 'Команда администратора',
      detail: 'Install system package'
    })
    expect(toolTitle('sudo', { command: 'systemctl restart prime' }, en)).toEqual({
      icon: ShieldAlert,
      label: 'Admin command',
      detail: 'systemctl restart prime'
    })
  })

  it('formats lsp_replace_symbol without falling back to generic Edit label', () => {
    expect(
      toolTitle('lsp_replace_symbol', { symbol: 'toolTitle', file_path: 'src/toolTitle.ts' }, ru)
    ).toEqual({
      icon: FilePen,
      label: 'Замена символа',
      detail: 'toolTitle'
    })

    expect(
      toolTitle('lsp_replace_symbol', { file_path: 'desktop/src/toolTitle.ts' }, en)
    ).toEqual({
      icon: FilePen,
      label: 'Replace symbol',
      detail: 'toolTitle.ts'
    })
  })
})

describe('mcp_call, third-party mcp_*, and unknown tool fallbacks', () => {
  it('formats mcp_call both when unwrapping and when called directly', () => {
    const unwrapped = toolTitle('mcp_call', { tool: 'mcp_apps_app_click', arguments: { text: 'Save' } }, ru)
    expect(unwrapped).toEqual({
      icon: AppWindow,
      label: 'Проверка приложения',
      detail: 'Нажимает на элемент · Save'
    })

    const withServer = toolTitle('mcp_call', { server: 'my-docs', tool: 'search_api_docs' }, ru)
    expect(withServer).toEqual({
      icon: Plug,
      label: 'MCP · my docs',
      detail: 'search api docs'
    })

    const withoutServerRu = toolTitle('mcp_call', { tool: 'get_schema' }, ru)
    expect(withoutServerRu).toEqual({
      icon: Plug,
      label: 'Вызов MCP',
      detail: 'get schema'
    })

    const withoutServerEn = toolTitle('mcp_call', { tool: 'get_schema' }, en)
    expect(withoutServerEn).toEqual({
      icon: Plug,
      label: 'MCP call',
      detail: 'get schema'
    })
  })

  it('formats third-party mcp_* tools cleanly without raw snake_case', () => {
    expect(toolTitle('mcp_my-server_fetch_user_profile', {}, ru)).toEqual({
      icon: Plug,
      label: 'MCP · my server',
      detail: 'fetch user profile'
    })
  })

  it('humanizes unknown snake_case and kebab-case tool names and unknown lsp_* names', () => {
    expect(toolTitle('some_unknown_tool', {}, ru)).toEqual({
      icon: Wrench,
      label: 'Some unknown tool',
      detail: ''
    })

    expect(toolTitle('custom-helper_step', {}, en)).toEqual({
      icon: Wrench,
      label: 'Custom helper step',
      detail: ''
    })

    expect(toolTitle('lsp_type_definition', {}, ru)).toEqual({
      icon: Wrench,
      label: 'LSP · type definition',
      detail: ''
    })

    expect(toolTitle('', {}, ru)).toEqual({
      icon: Wrench,
      label: 'Инструмент',
      detail: ''
    })

    expect(toolTitle('', {}, en)).toEqual({
      icon: Wrench,
      label: 'Tool',
      detail: ''
    })
  })
})

describe('desktop bridge tools (app_*, browser_resize, github_*, ssh_*)', () => {
  const appCases: [string, string, string][] = [
    ['app_launch', 'Запускает приложение', 'Starts app'],
    ['app_targets', 'Список окон приложений', 'Lists app windows'],
    ['app_attach', 'Подключается к окну', 'Attaches to window'],
    ['app_close', 'Закрывает приложение', 'Closes app'],
    ['app_output', 'Читает вывод приложения', 'Reads app output'],
    ['app_snapshot', 'Снимок интерфейса', 'UI snapshot'],
    ['app_find', 'Ищет элемент', 'Finds element'],
    ['app_wait', 'Ожидает элемент', 'Waits for element'],
    ['app_audit', 'Аудит макета', 'Layout audit'],
    ['app_check', 'Записывает проверку', 'Records check'],
    ['app_click', 'Нажимает на элемент', 'Clicks element'],
    ['app_type', 'Вводит текст', 'Types text'],
    ['app_set_value', 'Устанавливает значение', 'Sets value'],
    ['app_toggle', 'Переключает элемент', 'Toggles element'],
    ['app_select', 'Выбирает элемент', 'Selects element'],
    ['app_focus', 'Фокус на элементе', 'Focuses element'],
    ['app_scroll', 'Прокручивает область', 'Scrolls area'],
    ['app_expand', 'Раскрывает элемент', 'Expands element'],
    ['app_press_key', 'Нажимает клавишу', 'Presses a key'],
    ['app_screenshot', 'Снимок элемента', 'Element screenshot'],
    ['app_test_explore', 'Авто-обход интерфейса', 'Auto-explores UI'],
    ['app_test_list', 'Список сценариев', 'Lists scenarios'],
    ['app_test_run', 'Запускает сценарий', 'Runs scenario']
  ]

  it.each(appCases)('formats %s in Russian and English', (toolName, expectedRu, expectedEn) => {
    const ruResult = toolTitle(`mcp_apps_${toolName}`, {}, ru)
    expect(ruResult).toEqual({
      icon: AppWindow,
      label: 'Проверка приложения',
      detail: expectedRu
    })

    const enResult = toolTitle(toolName, {}, en)
    expect(enResult).toEqual({
      icon: AppWindow,
      label: 'App testing',
      detail: expectedEn
    })
  })

  it('formats browser_resize, github_* and ssh_* tools in Russian and English', () => {
    expect(toolTitle('mcp_browser_browser_resize', {}, ru)).toEqual({
      icon: Globe,
      label: 'Браузер',
      detail: 'Меняет ширину окна'
    })
    expect(toolTitle('browser_resize', {}, en)).toEqual({
      icon: Globe,
      label: 'Browser',
      detail: 'Resizes window'
    })

    expect(toolTitle('mcp_git_github_create_pull', { title: 'Fix labels' }, ru)).toEqual({
      icon: Github,
      label: 'GitHub',
      detail: 'Создаёт pull request · Fix labels'
    })
    expect(toolTitle('github_create_pull', {}, en).detail).toBe('Creates pull request')

    expect(toolTitle('mcp_git_github_comment', {}, ru).detail).toBe('Комментирует pull request')
    expect(toolTitle('github_comment', {}, en).detail).toBe('Comments on pull request')

    const sshCases: [string, string, string][] = [
      ['ssh_create_entry', 'Создаёт файл или папку', 'Creates file or folder'],
      ['ssh_download_file', 'Скачивает файл', 'Downloads file'],
      ['ssh_forward_port', 'Пробрасывает порт', 'Forwards port'],
      ['ssh_forwards', 'Активные пробросы', 'Active forwards'],
      ['ssh_close_forward', 'Закрывает проброс', 'Closes forward']
    ]

    for (const [toolName, expectedRu, expectedEn] of sshCases) {
      expect(toolTitle(`mcp_ssh_${toolName}`, {}, ru)).toEqual({
        icon: Server,
        label: 'SSH',
        detail: expectedRu
      })
      expect(toolTitle(toolName, {}, en)).toEqual({
        icon: Server,
        label: 'SSH',
        detail: expectedEn
      })
    }
  })

  it('normalizes hyphens and underscores for unknown desktop bridge actions', () => {
    expect(toolTitle('mcp_browser_browser_custom-check_action', {}, ru)).toEqual({
      icon: Globe,
      label: 'Браузер',
      detail: 'custom check action'
    })
  })

  it('formats built-in document tool titles in Russian and English', () => {
    expect(toolTitle('pdf_document', { action: 'create', output_path: 'reports/q4.pdf' }, ru)).toEqual({
      icon: FileText,
      label: 'PDF-документ',
      detail: 'создание · q4.pdf'
    })
    expect(toolTitle('pdf_document', { action: 'rotate', input_path: 'reports/q4.pdf' }, en)).toEqual({
      icon: FileText,
      label: 'PDF document',
      detail: 'rotate · q4.pdf'
    })
    expect(toolTitle('word_document', { action: 'fill_template', output_path: 'docs/contract.docx' }, ru)).toEqual({
      icon: FileText,
      label: 'Word-документ',
      detail: 'заполнение шаблона · contract.docx'
    })
    expect(toolTitle('word_document', { action: 'fill_template', output_path: 'docs/contract.docx' }, en)).toEqual({
      icon: FileText,
      label: 'Word document',
      detail: 'fill template · contract.docx'
    })
    expect(toolTitle('excel_workbook', { action: 'inspect', path: 'sheets/budget.xlsx' }, ru)).toEqual({
      icon: FileSpreadsheet,
      label: 'Excel-таблица',
      detail: 'просмотр · budget.xlsx'
    })
    expect(toolTitle('excel_workbook', { action: 'update', input_path: 'sheets/budget.xlsx' }, en)).toEqual({
      icon: FileSpreadsheet,
      label: 'Excel workbook',
      detail: 'update · budget.xlsx'
    })
    expect(toolTitle('powerpoint_presentation', { action: 'create', output_path: 'decks/pitch.pptx' }, ru)).toEqual({
      icon: Presentation,
      label: 'Презентация',
      detail: 'создание · pitch.pptx'
    })
    expect(toolTitle('powerpoint_presentation', { action: 'inspect', path: 'decks/pitch.pptx' }, en)).toEqual({
      icon: Presentation,
      label: 'Presentation',
      detail: 'inspect · pitch.pptx'
    })
  })
})
