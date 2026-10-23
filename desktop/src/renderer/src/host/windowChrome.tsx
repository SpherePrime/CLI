import { invoke } from '@tauri-apps/api/core'
import { Minus, Square, X } from 'lucide-react'
import { useEffect, type MouseEvent } from 'react'
import { flushDrafts } from './flushDrafts'
import { useI18n } from '../i18n'
import './windowChrome.css'

function action(value: string) {
  void invoke('desktop_window', { action: value }).catch((error) =>
    window.dispatchEvent(new CustomEvent('prime:host-error', { detail: error }))
  )
}

export function dragNativeWindow(event: MouseEvent<HTMLElement>) {
  if (
    event.button !== 0 ||
    (event.target as Element).closest('button,a,input,select,textarea,[role="button"],.breadcrumbs')
  )
    return
  event.preventDefault()
  action(event.detail === 2 ? 'maximize' : 'drag')
}

export function NativeWindowControls() {
  const { t } = useI18n()
  useEffect(() => window.prime.onEvent(event => {
    if (event.type === 'beforeClose') {
      void flushDrafts().then(() => invoke('desktop_window', { action: 'confirm_close' }))
        .catch(error => window.dispatchEvent(new CustomEvent('prime:host-error', { detail: error })))
    }
  }), [])
  return (
    <div className="native-window-controls">
      <button aria-label={t('Свернуть')} title={t('Свернуть')} onClick={() => action('minimize')}>
        <Minus size={14} />
      </button>
      <button
        aria-label={t('Развернуть')}
        title={t('Развернуть')}
        onClick={() => action('maximize')}
      >
        <Square size={12} />
      </button>
      <button
        className="native-window-close"
        aria-label={t('Закрыть')}
        title={t('Закрыть')}
        onClick={() => action('close')}
      >
        <X size={16} />
      </button>
    </div>
  )
}
