export type SessionInfo = {
  id: string
  title?: string
  created: string
  updated: string
  project_id?: string | null
  project_name?: string | null
  project_path?: string | null
  remote_url?: string | null
  project_exists?: boolean
}

export type ModelConfig = {
  provider: string
  model: string
  base_url?: string
  api_key_env?: string
  temperature?: number
  max_tokens?: number
}

export type WorkspaceInfo = {
  id: string
  name: string
  path: string
  remote_url?: string | null
}

export type ServerInfo = {
  name: string
  version: string
  model: ModelConfig
  workspace?: WorkspaceInfo
}

export type ProviderInfo = {
  id: string
  name: string
  connected: boolean
  custom: boolean
  api?: string | null
}

export type ModelInfo = {
  id: string
  name: string
  free: boolean
  provider: string
}

export type ModelGroup = {
  id: string
  name: string
  kind?: string
  base_url?: string | null
  connected: boolean
  models: ModelInfo[]
}

export type ModelsResponse = {
  data: ModelGroup[]
  current: { provider: string; model: string }
  favorites: string[]
}

export type SkillInfo = {
  name: string
  scope: string
  enabled: boolean
  description: string
}

export type PluginInfo = {
  name: string
  enabled: boolean
  version: string
  description: string
}

export type EventMeta = {
  protocol: number
  sequence: number
  turn_id: string
  item_id: string
  ts: number
}

export type FileChange = {
  path: string
  change: string
  diff?: string | null
  additions?: number
  deletions?: number
}

export type EngineEvent =
  | { type: "session.created"; session: { id: string } }
  | { type: "message.created"; message: { id: string; role: "user" | "assistant"; content?: string } }
  | { type: "done" }
  | { type: "session.project_missing"; session: { id: string; project_name?: string }; project_path: string | null }
  | { type: "started"; meta: EventMeta; model: string }
  | { type: "turn_started"; meta: EventMeta; model: string }
  | { type: "turn_completed"; meta: EventMeta }
  | { type: "turn_cancelled"; meta: EventMeta; reason?: string | null }
  | { type: "assistant_message_started"; meta: EventMeta; id: string }
  | { type: "text_delta"; meta: EventMeta; text: string }
  | { type: "assistant_message_completed"; meta: EventMeta }
  | { type: "reasoning_started"; meta: EventMeta }
  | { type: "reasoning_delta"; meta: EventMeta; text: string }
  | { type: "reasoning_completed"; meta: EventMeta }
  | { type: "tool_call_started"; meta: EventMeta; id: string; name: string; args: unknown }
  | { type: "tool_call_delta"; meta: EventMeta; index: number; id?: string | null; name?: string | null; args_delta: string }
  | { type: "tool_call_completed"; meta: EventMeta; id: string; name: string }
  | {
      type: "tool_result"
      meta: EventMeta
      id: string
      name: string
      ok: boolean
      content: string
      error?: string | null
      duration_ms: number
      summary?: string | null
      details?: string | null
      exit_code?: number | null
      truncated: boolean
      file_changes: FileChange[]
    }
  | { type: "activity_changed"; meta: EventMeta; activity: string; kind?: string | null }
  | { type: "permission_requested"; meta: EventMeta; id: string; tool: string; scope: string; target: string; reason: string }
  | { type: "permission_resolved"; meta: EventMeta; id: string; decision: string }
  | { type: "permission_mode_changed"; meta: EventMeta; mode: string }
  | { type: "usage"; meta: EventMeta; input_tokens: number; output_tokens: number }
  | { type: "finished"; meta: EventMeta; stop_reason: string; input_tokens: number; output_tokens: number; iterations: number }
  | { type: "error"; meta?: EventMeta; message: string }
  | { type: "session_title_changed"; meta: EventMeta; title: string }

export type SessionMessage = EngineEvent

export type TimelineEvent = EngineEvent

export type StoredMessage = {
  id: string
  role: string
  content?: string
  tool_calls?: unknown
  tool_call_id?: string | null
}

export type SessionDetail = SessionInfo & {
  messages: StoredMessage[]
  timeline?: TimelineEvent[]
  interrupted?: boolean
}

export type PermissionMode = "ask" | "auto_edit" | "full_access" | "deny"

export class AgentClient {
  constructor(
    private baseUrl: string,
    private token?: string,
  ) {}

  private authorized(headers: Record<string, string> = {}): Record<string, string> {
    if (this.token) headers["Authorization"] = `Bearer ${this.token}`
    return headers
  }

  async info(): Promise<ServerInfo> {
    return this.get<ServerInfo>("/")
  }

  async sessions(): Promise<SessionInfo[]> {
    const data = await this.get<{ data: SessionInfo[] }>("/session")
    return data.data
  }

  async createSession(): Promise<SessionInfo> {
    const data = await this.post<SessionInfo>("/session", {})
    return data
  }

  async getSession(id: string): Promise<SessionInfo> {
    return this.get<SessionInfo>(`/session/${id}`)
  }

  async getSessionDetail(id: string): Promise<SessionDetail> {
    return this.get<SessionDetail>(`/session/${id}`)
  }

  async renameSession(id: string, title: string): Promise<SessionInfo> {
    return this.post<SessionInfo>(`/session/${id}/rename`, { title })
  }

  async locateSession(id: string, path: string): Promise<SessionDetail> {
    return this.post<SessionDetail>(`/session/${id}/locate`, { path })
  }

  async deleteSession(id: string): Promise<{ deleted: string }> {
    return this.post<{ deleted: string }>(`/session/${id}/delete`, {})
  }

  async forkSession(id: string): Promise<SessionInfo> {
    return this.post<SessionInfo>(`/session/${id}/fork`, {})
  }

  async compactSession(id: string): Promise<SessionDetail> {
    return this.post<SessionDetail>(`/session/${id}/compact`, {})
  }

  async getPermissionMode(sessionId: string): Promise<{ mode: PermissionMode }> {
    return this.get<{ mode: PermissionMode }>(`/session/${sessionId}/permission-mode`)
  }

  async setPermissionMode(
    sessionId: string,
    mode: PermissionMode,
    rememberProject?: boolean,
  ): Promise<{ applied: boolean; mode: PermissionMode }> {
    return this.post<{ applied: boolean; mode: PermissionMode }>(`/session/${sessionId}/permission-mode`, {
      mode,
      remember_project: rememberProject ?? false,
    })
  }

  async cancelMessage(sessionId: string | undefined): Promise<{ cancelled: boolean }> {
    return this.post<{ cancelled: boolean }>("/cancel", { session_id: sessionId })
  }

  async resolvePermission(
    sessionId: string | undefined,
    id: string,
    decision: "allow" | "once" | "deny" | "reject",
    remember?: boolean,
  ): Promise<{ resolved: boolean }> {
    return this.post<{ resolved: boolean }>("/permission", {
      session_id: sessionId,
      id,
      decision,
      remember: remember ?? false,
    })
  }

  async updateModel(model: ModelConfig): Promise<void> {
    await this.post<unknown>("/config", { model })
  }

  async listProviders(): Promise<ProviderInfo[]> {
    const data = await this.get<{ data: ProviderInfo[] }>("/providers")
    return data.data
  }

  async listModels(): Promise<ModelsResponse> {
    return this.get<ModelsResponse>("/models")
  }

  async connectProvider(input: { id: string; api_key?: string; base_url?: string }): Promise<void> {
    await this.post<unknown>("/provider", input)
  }

  async selectModel(provider: string, model: string): Promise<ModelConfig> {
    const data = await this.post<{ model: ModelConfig }>("/model", { provider, model })
    return data.model
  }

  async toggleFavorite(provider: string, model: string): Promise<string[]> {
    const data = await this.post<{ favorites: string[] }>("/favorite", { provider, model })
    return data.favorites
  }

  async listSkills(): Promise<SkillInfo[]> {
    const data = await this.get<{ data: SkillInfo[] }>("/skills")
    return data.data
  }

  async toggleSkill(name: string, enabled: boolean): Promise<void> {
    await this.post<unknown>("/skill", { name, enabled })
  }

  async listPlugins(): Promise<PluginInfo[]> {
    const data = await this.get<{ data: PluginInfo[] }>("/plugins")
    return data.data
  }

  async togglePlugin(name: string, enabled: boolean): Promise<void> {
    await this.post<unknown>("/plugin", { name, enabled })
  }

  async findFiles(query: string): Promise<{ path: string; type: string }[]> {
    const data = await this.get<{ data: { path: string; type: string }[] }>(
      `/fs/find?query=${encodeURIComponent(query)}`,
    )
    return data.data
  }

  async sendMessage(text: string, sessionId?: string): Promise<SessionMessage[]> {
    const response = await fetch(`${this.baseUrl}/message`, {
      method: "POST",
      headers: this.authorized({ "Content-Type": "application/json" }),
      body: JSON.stringify({ text, session_id: sessionId }),
    })
    if (!response.ok) throw new Error(`HTTP ${response.status}`)
    const body = await response.text()
    const events: SessionMessage[] = []
    for (const line of body.split("\n")) {
      const trimmed = line.trim()
      if (!trimmed) continue
      try {
        events.push(JSON.parse(trimmed))
      } catch {}
    }
    return events
  }

  async streamMessage(
    text: string,
    sessionId: string | undefined,
    onEvent: (event: SessionMessage) => void,
    options?: { readonly?: boolean },
  ): Promise<void> {
    const response = await fetch(`${this.baseUrl}/message`, {
      method: "POST",
      headers: this.authorized({ "Content-Type": "application/json" }),
      body: JSON.stringify({ text, session_id: sessionId, readonly: options?.readonly ?? false }),
    })
    if (!response.ok || !response.body) throw new Error(`HTTP ${response.status}`)

    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ""

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      let index: number
      while ((index = buffer.indexOf("\n")) !== -1) {
        const line = buffer.slice(0, index).trim()
        buffer = buffer.slice(index + 1)
        if (!line) continue
        try {
          onEvent(JSON.parse(line))
        } catch {}
      }
    }
  }

  private async get<T>(path: string): Promise<T> {
    const response = await fetch(`${this.baseUrl}${path}`, {
      headers: this.authorized(),
    })
    if (!response.ok) throw new Error(`HTTP ${response.status}`)
    return response.json()
  }

  private async post<T>(path: string, body: unknown): Promise<T> {
    const response = await fetch(`${this.baseUrl}${path}`, {
      method: "POST",
      headers: this.authorized({ "Content-Type": "application/json" }),
      body: JSON.stringify(body),
    })
    if (!response.ok) throw new Error(`HTTP ${response.status}`)
    return response.json()
  }
}