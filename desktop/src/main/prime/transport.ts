import http from 'node:http'
import { SseDecoder } from './sse'
import type { Duplex } from 'node:stream'

export type TransportConnector = (callback: (error: Error | null, stream?: Duplex) => void) => void

function socketLike(stream: Duplex): Duplex {
  const socket = stream as Duplex & Record<string, any>
  let timer: NodeJS.Timeout | undefined
  socket.setTimeout ??= (timeout: number, listener?: () => void) => {
    if (listener) socket.once('timeout', listener)
    if (timer) clearTimeout(timer)
    timer = timeout > 0 ? setTimeout(() => socket.emit('timeout'), timeout) : undefined
    return socket
  }
  socket.setNoDelay ??= () => socket
  socket.setKeepAlive ??= () => socket
  socket.ref ??= () => socket
  socket.unref ??= () => socket
  socket.once('close', () => timer && clearTimeout(timer))
  return socket
}

export class PrimeTransport {
  constructor(readonly socketPath: string, private readonly connector?: TransportConnector) {}
  private connectionOptions(): http.RequestOptions {
    if (!this.connector) return { socketPath: this.socketPath }
    return { createConnection: (_options, callback) => {
      this.connector!((error, stream) => callback?.(error, stream ? socketLike(stream) : stream!))
      return undefined
    } }
  }
  request<T = any>(
    method: string,
    path: string,
    body?: unknown,
    timeout = 60000,
    signal?: AbortSignal
  ): Promise<T> {
    return new Promise((resolve, reject) => {
      const encoded = body === undefined ? undefined : JSON.stringify(body)
      const request = http.request(
        {
          ...this.connectionOptions(),
          path,
          method,
          signal,
          headers: encoded
            ? { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(encoded) }
            : {}
        },
        (response) => {
          const chunks: Buffer[] = []
          let length = 0
          response.on('data', (chunk: Buffer) => {
            length += chunk.length
            if (length > 64 * 1024 * 1024) request.destroy(new Error('Ответ Prime слишком большой'))
            else chunks.push(chunk)
          })
          response.on('error', reject)
          response.on('end', () => {
            const text = Buffer.concat(chunks).toString('utf8')
            let data: any
            try {
              data = text ? JSON.parse(text) : null
            } catch {
              reject(new Error(`Prime вернул некорректный ответ (${response.statusCode})`))
              return
            }
            if ((response.statusCode || 500) >= 400)
              reject(
                Object.assign(
                  new Error(data?.message || data?.error || `Prime: HTTP ${response.statusCode}`),
                  { status: response.statusCode }
                )
              )
            else resolve(data)
          })
        }
      )
      request.setTimeout(timeout, () =>
        request.destroy(
          new Error(
            'Время ожидания Prime истекло. Проверьте состояние задачи перед повторной отправкой.'
          )
        )
      )
      request.on('error', reject)
      request.end(encoded)
    })
  }
  subscribe(
    path: string,
    onEvent: (value: any) => void,
    onState: (state: string) => void
  ): () => void {
    let closed = false
    let retry = 0
    let timer: NodeJS.Timeout | undefined
    let active: http.ClientRequest | undefined
    const connect = () => {
      if (closed) return
      const decoder = new SseDecoder()
      let ended = false
      const reconnect = () => {
        if (ended || closed) return
        ended = true
        onState('reconnecting')
        timer = setTimeout(connect, Math.min(5000, 250 * 2 ** retry++))
      }
      active = http.get(
        { ...this.connectionOptions(), path, headers: { Accept: 'text/event-stream' } },
        (response) => {
          if (response.statusCode !== 200) {
            response.resume()
            reconnect()
            return
          }
          retry = 0
          onState('ready')
          response.on('data', (chunk: Buffer) => {
            try {
              for (const data of decoder.push(chunk)) onEvent(JSON.parse(data))
            } catch {
              active?.destroy()
              reconnect()
            }
          })
          response.on('end', reconnect)
          response.on('error', reconnect)
        }
      )
      active.on('error', reconnect)
    }
    connect()
    return () => {
      closed = true
      clearTimeout(timer)
      active?.destroy()
    }
  }
}
