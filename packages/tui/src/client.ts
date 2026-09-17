export type SessionInfo = {
  id: string
  title?: string
  created: number
  updated: number
}

export type ModelConfig = {
  provider: string
  model: string
  base_url?: string
  api_key_env?: string
  temperature?: number
  max_tokens?: number
}

export type ServerInfo = {
  name: string
  version: string
  model: ModelConfig
}

export type SessionMessage = {
  type: string
  session?: { id: string }
  message?: { id: string; role: "user" | "assistant"; content?: string }
  error?: string
  delta?: string
  part?: { type: string; text?: string }
}

export class AgentClient {
  constructor(private baseUrl: string) {}

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

  async updateModel(model: ModelConfig): Promise<void> {
    await this.post<unknown>("/config", { model })
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
      headers: { "Content-Type": "application/json" },
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
  ): Promise<void> {
    const response = await fetch(`${this.baseUrl}/message`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text, session_id: sessionId }),
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
    const response = await fetch(`${this.baseUrl}${path}`)
    if (!response.ok) throw new Error(`HTTP ${response.status}`)
    return response.json()
  }

  private async post<T>(path: string, body: unknown): Promise<T> {
    const response = await fetch(`${this.baseUrl}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    })
    if (!response.ok) throw new Error(`HTTP ${response.status}`)
    return response.json()
  }
}