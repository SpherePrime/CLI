import { isDeviceAttributesReply, isTerminalReport } from '../../../../shared/terminalReports'
import { useEffect, useRef } from 'react'
import type { ITheme, Terminal } from '@xterm/xterm'
import type { TerminalApi, TerminalRead, TerminalSnapshot } from '../../../../shared/terminal'
import { useI18n } from '../../i18n'
import { TerminalReplay } from './replay'
import { bindTerminalClipboard } from './clipboardKeys'
import type { TerminalSubscribe } from './useTerminalWorkspace'

export type TerminalHandle = { clear(): void; copy(): Promise<void>; focus(): void }
export type TerminalTheme = 'dark' | 'light' | 'system'

type Props = {
  terminal: TerminalSnapshot
  active: boolean
  api: TerminalApi
  subscribe: TerminalSubscribe
  theme?: TerminalTheme
  onReady: (id: string, handle?: TerminalHandle) => void
  onError: (error: unknown) => void
}

function colors(element: HTMLElement): ITheme {
  const styles = getComputedStyle(element)
  const token = (name: string, fallback: string) => styles.getPropertyValue(name).trim() || fallback
  return {
    background: token('--bg', '#101010'),
    foreground: token('--text', '#f1efe9'),
    cursor: token('--accent', '#ff8c60'),
    cursorAccent: token('--bg', '#101010'),
    selectionBackground: token('--accent-bg', '#36251e'),
    black: '#252320',
    red: token('--red', '#ec9390'),
    green: token('--green', '#95bba0'),
    yellow: token('--amber', '#d5b380'),
    blue: '#89aaca',
    magenta: '#c39acb',
    cyan: '#83b7b4',
    white: '#c6c2b9',
    brightBlack: '#817c72',
    brightRed: '#f3a3a0',
    brightGreen: '#acd0b4',
    brightYellow: '#e5c796',
    brightBlue: '#a2c5e7',
    brightMagenta: '#dbb4e4',
    brightCyan: '#a0d2ce',
    brightWhite: '#f1efe9'
  }
}

export function TerminalView(props: Props) {
  const { t } = useI18n()
  const container = useRef<HTMLDivElement>(null)
  const emulator = useRef<Terminal | null>(null)
  const fitNow = useRef<() => void>(() => {})
  const latest = useRef(props)
  latest.current = props

  useEffect(() => {
    const element = container.current
    if (!element) return
    let disposed = false
    let cleanup = () => {}
    void Promise.all([import('@xterm/xterm'), import('@xterm/addon-fit')])
      .then(([{ Terminal: Emulator }, { FitAddon }]) => {
        if (disposed) return
        const terminal = new Emulator({
          cursorBlink: true,
          cursorStyle: 'bar',
          fontFamily: "'JetBrains Mono', Consolas, monospace",
          fontSize: 12,
          lineHeight: 1.35,
          scrollback: 2000,
          theme: colors(element),
          allowProposedApi: false,
          disableStdin: true
        })
        const fit = new FitAddon()
        terminal.loadAddon(fit)
        terminal.open(element)
        emulator.current = terminal
        const unbindClipboard = bindTerminalClipboard(terminal, element, (error) => latest.current.onError(error))
        const replay = new TerminalReplay()
        let restored = false
        const restore = (snapshot: { output: string; sequence: number }) => {
          terminal.write(replay.restore(snapshot), () => {
            if (disposed) return
            restored = true
            terminal.options.disableStdin =
              latest.current.terminal.owner !== 'user' ||
              latest.current.terminal.status !== 'running'
          })
        }
        const report = (error: unknown) => {
          if (!disposed) latest.current.onError(error)
        }
        const unsubscribe = latest.current.subscribe((event) => {
          if (
            disposed ||
            event.workspaceId !== props.terminal.workspaceId ||
            event.type !== 'terminal'
          )
            return
          const payload = event.payload
          if (payload?.action === 'data' && payload.id === props.terminal.id) {
            const data = replay.receive(payload)
            if (data) terminal.write(data)
          }
        })
        const input = terminal.onData((data) => {
          if (isTerminalReport(data)) {
            if (latest.current.terminal.status === 'running' && !isDeviceAttributesReply(data))
              void latest.current.api({ action: 'report', id: props.terminal.id, data }).catch(() => {})
            return
          }
          if (
            !restored ||
            latest.current.terminal.owner !== 'user' ||
            latest.current.terminal.status !== 'running'
          )
            return
          void latest.current.api({ action: 'write', id: props.terminal.id, data }).catch(report)
        })
        let frame = 0
        const resize = () => {
          cancelAnimationFrame(frame)
          frame = requestAnimationFrame(() => {
            if (
              disposed ||
              !latest.current.active ||
              element.clientWidth < 40 ||
              element.clientHeight < 30
            )
              return
            fit.fit()
            if (latest.current.terminal.status === 'running')
              void latest.current
                .api({
                  action: 'resize',
                  id: props.terminal.id,
                  cols: terminal.cols,
                  rows: terminal.rows
                })
                .catch(report)
          })
        }
        fitNow.current = resize
        const observer = new ResizeObserver(resize)
        observer.observe(element)
        const themeObserver = new MutationObserver(() => {
          terminal.options.theme = colors(element)
        })
        themeObserver.observe(document.documentElement, {
          attributes: true,
          attributeFilter: ['data-theme']
        })
        latest.current.onReady(props.terminal.id, {
          clear: () => terminal.clear(),
          focus: () => terminal.focus(),
          copy: async () => {
            let output = terminal.getSelection()
            if (!output) {
              const buffer = terminal.buffer.active
              const lines: string[] = []
              for (let row = 0; row < buffer.length; row += 1)
                lines.push(buffer.getLine(row)?.translateToString(true) ?? '')
              output = lines.join('\n').trimEnd()
            }
            await navigator.clipboard.writeText(output)
          }
        })
        void latest.current
          .api({ action: 'read', id: props.terminal.id })
          .then((response) => {
            if (disposed) return
            if (!response || !('output' in response))
              throw new Error('Invalid terminal read response')
            const snapshot = response as TerminalRead
            restore(snapshot)
            resize()
          })
          .catch((error) => {
            if (!disposed) restore({ output: '', sequence: -1 })
            report(error)
          })
        resize()
        cleanup = () => {
          cancelAnimationFrame(frame)
          observer.disconnect()
          themeObserver.disconnect()
          unbindClipboard()
          unsubscribe()
          input.dispose()
          terminal.dispose()
          emulator.current = null
          fitNow.current = () => {}
          latest.current.onReady(props.terminal.id)
        }
      })
      .catch((error) => {
        if (!disposed) latest.current.onError(error)
      })
    return () => {
      disposed = true
      cleanup()
    }
  }, [props.terminal.id, props.terminal.workspaceId])

  useEffect(() => {
    if (!emulator.current) return
    emulator.current.options.disableStdin =
      props.terminal.owner !== 'user' || props.terminal.status !== 'running'
  }, [props.terminal.owner, props.terminal.status])

  useEffect(() => {
    if (props.active) {
      fitNow.current()
      if (props.terminal.owner === 'user') emulator.current?.focus()
    }
  }, [props.active, props.terminal.owner])

  useEffect(() => {
    if (emulator.current && container.current)
      emulator.current.options.theme = colors(container.current)
  }, [props.theme])

  return (
    <div
      className="terminal-session"
      hidden={!props.active}
      role="tabpanel"
      id={`terminal-panel-${props.terminal.id}`}
      aria-labelledby={`terminal-tab-${props.terminal.id}`}
    >
      <div className="terminal-emulator" ref={container} aria-label={t('Вывод терминала')} />
    </div>
  )
}
