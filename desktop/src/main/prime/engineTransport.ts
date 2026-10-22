export interface EngineTransport {
  request<T = any>(
    method: string,
    path: string,
    body?: unknown,
    timeout?: number,
    signal?: AbortSignal
  ): Promise<T>
  subscribe(
    path: string,
    onEvent: (value: any) => void,
    onState: (state: string) => void
  ): () => void
}
