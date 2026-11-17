import { invoke } from '@tauri-apps/api/core'
import { Minus, Square, X } from 'lucide-react'
import { useEffect, useRef, type MouseEvent } from 'react'
import { flushDrafts } from './flushDrafts'
import { shouldIgnoreNativeDrag } from './windowDrag'
import { useI18n } from '../i18n'
import './windowChrome.css'

function action(value: string) {
  return invoke('desktop_window', { action: value }).catch((error) =>
    window.dispatchEvent(new CustomEvent('prime:host-error', { detail: error }))
  )
}

export function dragNativeWindow(event: MouseEvent<HTMLElement>) {
  if (shouldIgnoreNativeDrag(event.target as Element, event.button)) return
  event.preventDefault()
  void action(event.detail === 2 ? 'maximize' : 'drag')
}

export function NativeWindowControls() {
  const { t } = useI18n()
  const closingRef = useRef(false)
  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        closingRef.current = false
      }
    }
    const handleFocus = () => {
      closingRef.current = false
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)
    window.addEventListener('focus', handleFocus)
    const unsubscribe = window.prime.onEvent((event) => {
      if ('type' in event && event.type === 'beforeClose') {
        closingRef.current = true
        void Promise.race([
          flushDrafts(),
          new Promise((resolve) => setTimeout(resolve, 600))
        ])
          .catch(() => {})
          .finally(() => {
            void invoke('desktop_window', { action: 'confirm_close' })
              .catch(() => {})
              .finally(() => {
                closingRef.current = false
              })
          })
      }
    })
    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      window.removeEventListener('focus', handleFocus)
      unsubscribe()
    }
  }, [])
  const handleClose = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation()
    if (closingRef.current) {
      void action('confirm_close').finally(() => {
        closingRef.current = false
      })
      return
    }
    closingRef.current = true
    void Promise.race([
      flushDrafts(),
      new Promise((resolve) => setTimeout(resolve, 300))
    ])
      .catch(() => {})
      .finally(() => {
        void action('confirm_close').finally(() => {
          closingRef.current = false
        })
      })
  }
  return (
    <div className="native-window-controls" onMouseDown={(event) => event.stopPropagation()}>
      <button
        type="button"
        aria-label={t('Свернуть')}
        title={t('Свернуть')}
        onClick={(event) => {
          event.stopPropagation()
          void action('minimize')
        }}
      >
        <Minus size={14} />
      </button>
      <button
        type="button"
        aria-label={t('Развернуть')}
        title={t('Развернуть')}
        onClick={(event) => {
          event.stopPropagation()
          void action('maximize')
        }}
      >
        <Square size={12} />
      </button>
      <button
        type="button"
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
