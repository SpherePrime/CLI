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

export type EngineEvent =
  | { type: "session.created"; session: { id: string } }
  | { type: "message.created"; message: { id: string; role: "user" | "assistant"; content?: string } }
  | { type: "started"; session_id: string; model: string }
  | { type: "text_delta"; text: string }
  | { type: "reasoning_delta"; text: string }
  | { type: "tool_call"; id: string; name: string; args: unknown }
  | { type: "tool_result"; id: string; name: string; ok: boolean; content: string; error?: string; ms: number }
  | { type: "permission_requested"; id: string; tool: string; scope: string; target: string; reason: string }
  | { type: "permission_resolved"; id: string; decision: string }
  | { type: "usage"; input_tokens: number; output_tokens: number }
  | { type: "finished"; stop_reason: string; input_tokens: number; output_tokens: number; iterations: number }
  | { type: "session.project_missing"; session: { id: string; project_name?: string }; project_path: string | null }
  | { type: "error"; message: string }
  | { type: "done" }

export type SessionMessage = EngineEvent

export type StoredMessage = {
  id: string
  role: string
  content?: string
  tool_calls?: unknown
  tool_call_id?: string | null
}

export type SessionDetail = SessionInfo & {
  messages: StoredMessage[]
}

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