import type { Operation } from '../../shared/contracts'
type Route = [string, string, any?]
export function operationRoute(operation: Operation, args: any = {}): Route {
  const sid = encodeURIComponent(args.sessionId || '')
  const routes: Record<Operation, () => Route> = {
    channelMessages: () => ['GET', 'channel-messages'],
    sessions: () => ['GET', 'sessions'],
    createSession: () => ['POST', 'sessions', { title: args.title || 'Новый чат' }],
    renameSession: () => ['PUT', `sessions/${sid}`, args.session],
    deleteSession: () => ['DELETE', `sessions/${sid}`],
    messages: () => ['GET', `sessions/${sid}/messages`],
    history: () => ['GET', `sessions/${sid}/history`],
    context: () => ['GET', `sessions/${sid}/context`],
    agent: () => ['GET', 'agent'],
    init: () => ['POST', 'agent/init', { interactive: true }],
    send: () => [
      'POST',
      'agent',
      {
        session_id: args.sessionId,
        run_id: args.runId,
        reasoning_mode: args.reasoningMode,
        prompt: args.prompt,
        attachments: args.attachments || []
      }
    ],
    cancel: () => ['POST', `agent/sessions/${sid}/cancel`],
    mode: () => ['POST', 'agent/main', { agent_id: args.mode }],
    queue: () => ['GET', `agent/sessions/${sid}/prompts/entries`],
    runQueuedNow: () => ['POST', `agent/sessions/${sid}/prompts/run-now`, { id: args.id }],
    clearQueue: () => ['POST', `agent/sessions/${sid}/prompts/clear`],
    summarize: () => ['POST', `agent/sessions/${sid}/summarize`],
    shell: () => [
      'POST',
      `agent/sessions/${sid}/shell`,
      { session_id: args.sessionId, command: args.command, term_width: 120 }
    ],
    permission: () => [
      'POST',
      'permissions/grant',
      { permission: args.permission, action: args.action }
    ],
    getSkip: () => ['GET', 'permissions/skip'],
    skip: () => ['POST', 'permissions/skip', { skip: args.skip }],
    answer: () => ['POST', 'questions/answer', args],
    cancelQuestion: () => ['POST', 'questions/cancel', { batch_request_id: args.id }],
    config: () => ['GET', 'config'],
    providers: () => ['GET', 'providers'],
    providerSave: () => [
      'POST',
      'config/provider-save',
      {
        scope: args.scope ?? 0,
        original_id: args.originalId,
        provider: args.provider,
        api_key: args.apiKey
      }
    ],
    configSet: () => [
      'POST',
      'config/set',
      { scope: args.scope ?? 0, key: args.key, value: args.value }
    ],
    configRemove: () => ['POST', 'config/remove', { scope: args.scope ?? 0, key: args.key }],
    model: () => [
      'POST',
      'config/model',
      { scope: args.scope ?? 0, model_type: args.modelType || 'large', model: args.model }
    ],
    providerKey: () => [
      'POST',
      'config/provider-key',
      { scope: args.scope ?? 0, provider_id: args.providerId, kind: 'string', api_key: args.key }
    ],
    reasoning: () => ['POST', 'config/reasoning', args],
    models: () => ['GET', 'models/capabilities'],
    mcp: () => ['GET', 'mcp/states'],
    lsp: () => ['GET', 'lsps'],
    skills: () => ['GET', 'skills'],
    readSkill: () => ['POST', 'skills/read', { skill_id: args.id }],
    restart: () => ['POST', 'systems/restart'],
    mcpPrompts: () => ['GET', 'mcp/prompts'],
    mcpPrompt: () => ['POST', 'mcp/get-prompt', args],
    mcpResource: () => ['POST', 'mcp/read-resource', args],
    mcpRefresh: () => ['POST', 'mcp/refresh-tools', { name: args.name }],
    mcpAuth: () => ['POST', 'mcp/auth', { name: args.name }],
    mcpAuthUrl: () => ['GET', `mcp/auth-url?name=${encodeURIComponent(args.name)}`],
    lspStart: () => ['POST', 'lsps/start', { path: args.path }],
    lspStop: () => ['POST', 'lsps/stop'],
    diagnostics: () => ['GET', `lsps/${encodeURIComponent(args.name)}/diagnostics`],
    dockerEnable: () => ['POST', 'mcp/docker/enable'],
    dockerDisable: () => ['POST', 'mcp/docker/disable'],
    plugins: () => ['GET', 'plugins'],
    pluginEnable: () => ['POST', `plugins/${encodeURIComponent(args.name)}/enable`],
    pluginDisable: () => ['POST', `plugins/${encodeURIComponent(args.name)}/disable`],
    pluginTasks: () => ['GET', 'plugins/tasks'],
    voice: () => ['POST', 'voice/transcribe', args],
    pending: () => ['GET', 'pending'],
    runs: () => ['GET', args.runId ? `runs/${encodeURIComponent(args.runId)}` : 'runs'],
    commands: () => ['GET', 'commands'],
    jobs: () => ['GET', `jobs${args.sessionId ? `?session_id=${sid}` : ''}`],
    currentSession: () => [
      'POST',
      `current-session?client_id=${encodeURIComponent(args.clientId)}`,
      { session_id: args.sessionId || '' }
    ],
    importCopilot: () => ['POST', 'config/import-copilot'],
    refreshOAuth: () => [
      'POST',
      'config/refresh-oauth',
      { scope: args.scope ?? 0, provider_id: args.providerId }
    ]
  }
  if (!Object.hasOwn(routes, operation)) throw new Error('Неизвестное действие')
  return routes[operation]()
}
export function redact(value: any): any {
  if (Array.isArray(value)) return value.map(redact)
  if (!value || typeof value !== 'object') return value
  return Object.fromEntries(
    Object.entries(value)
      .filter(
        ([key, item]) =>
          (key === 'oauth' && typeof item === 'boolean') ||
          !/^(api_?key|access_token|refresh_token|id_token|secret|authorization|headers|env|oauth|oauth_token|oauth_client_secret)$/i.test(
            key
          )
      )
      .map(([key, item]) => [key, redact(item)])
  )
}
