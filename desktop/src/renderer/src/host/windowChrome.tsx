import { invoke } from '@tauri-apps/api/core'
import { Minus, Square, X } from 'lucide-react'
import { useEffect, useRef, type MouseEvent } from 'react'
import { flushDrafts } from './flushDrafts'
import { shouldIgnoreNativeDrag } from './windowDrag'
import { useI18n } from '../i18n'
import './windowChrome.css'

function action(value: string) {
  void invoke('desktop_window', { action: value }).catch((error) =>
    window.dispatchEvent(new CustomEvent('prime:host-error', { detail: error }))
  )
}

export function dragNativeWindow(event: MouseEvent<HTMLElement>) {
  if (shouldIgnoreNativeDrag(event.target as Element, event.button)) return
  event.preventDefault()
  action(event.detail === 2 ? 'maximize' : 'drag')
}

export function NativeWindowControls() {
  const { t } = useI18n()
  const closingRef = useRef(false)
  useEffect(() => window.prime.onEvent(event => {
    if (event.type === 'beforeClose') {
      closingRef.current = true
      void Promise.race([
        flushDrafts(),
        new Promise((resolve) => setTimeout(resolve, 600))
      ])
        .catch(() => {})
        .finally(() => {
          invoke('desktop_window', { action: 'confirm_close' }).catch(() => {})
        })
    }
  }), [])
  const handleClose = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation()
    if (closingRef.current) {
      action('confirm_close')
      return
    }
    closingRef.current = true
    void Promise.race([
      flushDrafts(),
      new Promise((resolve) => setTimeout(resolve, 300))
    ])
      .catch(() => {})
      .finally(() => {
        action('confirm_close')
      })
  }
  return (
    <div className="native-window-controls" onMouseDown={(event) => event.stopPropagation()}>
      <button
        aria-label={t('Свернуть')}
        title={t('Свернуть')}
        onClick={(event) => {
          event.stopPropagation()
          action('minimize')
        }}
      >
        <Minus size={14} />
      </button>
      <button
        aria-label={t('Развернуть')}
        title={t('Развернуть')}
        onClick={(event) => {
          event.stopPropagation()
          action('maximize')
        }}
      >
        <Square size={12} />
      </button>
      <button
        className="native-window-close"
        aria-label={t('Закрыть')}
        title={t('Закрыть')}
        onClick={handleClose}
      >
        <X size={16} />
      </button>
    </div>
  )
}
