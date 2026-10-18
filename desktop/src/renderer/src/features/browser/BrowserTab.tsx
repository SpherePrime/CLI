import { useEffect, useRef, useState } from 'react'
import { ArrowLeft, ArrowRight, ExternalLink, Globe, RotateCw, X } from 'lucide-react'
import type { BrowserState } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'

const overlaySelector =
  'dialog[open], .prime-popover, .file-context-menu, .select-options, .update-screen, .startup-screen:not(.hidden)'

const emptyState: BrowserState = {
  url: '',
  title: '',
  loading: false,
  canGoBack: false,
  canGoForward: false
}

export function BrowserTab() {
  const { t } = useI18n()
  const viewport = useRef<HTMLDivElement>(null)
  const [state, setState] = useState<BrowserState>(emptyState)
  const [address, setAddress] = useState('')
  const [editing, setEditing] = useState(false)
  const [error, setError] = useState('')
  useEffect(() => {
    void window.prime.browserState().then((value) => {
      setState(value)
      setAddress(value.url)
    })
    return window.prime.onEvent((event) => {
      if (event.type !== 'browser' || event.payload?.action !== 'state') return
      const { action: _action, ...next } = event.payload
      setState(next)
      setError('')
    })
  }, [])
  useEffect(() => {
    if (!editing) setAddress(state.url === 'about:blank' ? '' : state.url)
  }, [state.url, editing])
  useEffect(() => {
    const element = viewport.current
    if (!element) return
    let frame = 0
    const sync = () => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        const rect = element.getBoundingClientRect()
        const covered = Boolean(document.querySelector(overlaySelector))
        if (covered || rect.width < 2 || rect.height < 2 || !state.url)
          void window.prime.browserHide()
        else
          void window.prime.browserShow({
            x: rect.left,
            y: rect.top,
            width: rect.width,
            height: rect.height
          })
      })
    }
    sync()
    const resize = new ResizeObserver(sync)
    resize.observe(element)
    resize.observe(document.body)
    const overlays = new MutationObserver(sync)
    overlays.observe(document.body, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ['open', 'class']
    })
    window.addEventListener('resize', sync)
    return () => {
      cancelAnimationFrame(frame)
      resize.disconnect()
      overlays.disconnect()
      window.removeEventListener('resize', sync)
      void window.prime.browserHide()
    }
  }, [state.url])
  const navigate = (value: string) => {
    setEditing(false)
    setError('')
    void window.prime
      .browserNavigate(value)
      .then(setState)
      .catch((reason) => setError(String(reason?.message || reason).replace(/^.*Error: /, '')))
  }
  const command = (name: 'back' | 'forward' | 'reload') =>
    void window.prime.browserCommand(name).then(setState)
  return (
    <div className="browser-tab">
      <div className="browser-toolbar">
        <button
          className="icon small"
          aria-label={t('Назад')}
          title={t('Назад')}
          disabled={!state.canGoBack}
          onClick={() => command('back')}
        >
          <ArrowLeft size={14} />
        </button>
        <button
          className="icon small"
          aria-label={t('Вперёд')}
          title={t('Вперёд')}
          disabled={!state.canGoForward}
          onClick={() => command('forward')}
        >
          <ArrowRight size={14} />
        </button>
        <button
          className="icon small"
          aria-label={state.loading ? t('Остановить') : t('Обновить')}
          title={state.loading ? t('Остановить') : t('Обновить')}
          disabled={!state.url}
          onClick={() => command('reload')}
        >
          {state.loading ? <X size={14} /> : <RotateCw size={14} />}
        </button>
        <form
          className={`browser-address ${state.loading ? 'loading' : ''}`}
          onSubmit={(event) => {
            event.preventDefault()
            if (!address.trim()) return
            navigate(address)
            event.currentTarget.querySelector('input')?.blur()
          }}
        >
          <Globe size={13} />
          <input
            value={address}
            aria-label={t('Адрес страницы')}
            placeholder={t('Адрес или поиск')}
            spellCheck={false}
            onFocus={(event) => {
              setEditing(true)
              event.currentTarget.select()
            }}
            onBlur={() => setEditing(false)}
            onChange={(event) => setAddress(event.target.value)}
          />
        </form>
        <button
          className="icon small"
          aria-label={t('Открыть во внешнем браузере')}
          title={t('Открыть во внешнем браузере')}
          disabled={!/^https?:/.test(state.url)}
          onClick={() => void window.prime.browserOpenExternal()}
        >
          <ExternalLink size={14} />
        </button>
      </div>
      {error && <p className="browser-error">{error}</p>}
      <div ref={viewport} className="browser-viewport">
        {!state.url && (
          <div className="browser-empty">
            <Globe size={30} />
            <strong>{t('Встроенный браузер')}</strong>
            <p>
              {t(
                'Откройте адрес выше. Prime тоже работает здесь: открывает страницы, читает их, нажимает и вводит текст.'
              )}
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
