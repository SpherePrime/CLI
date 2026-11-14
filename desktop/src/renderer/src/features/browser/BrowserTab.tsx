import { useEffect, useRef, useState } from 'react'
import { ArrowLeft, ArrowRight, Crosshair, ExternalLink, Globe, RotateCw, X } from 'lucide-react'
import type { BrowserState, PickedElementResult } from '../../../../shared/contracts'
import { ElementPick } from './ElementPick'
import { BrowserTabs } from './BrowserTabs'
import { visibleRect } from './visibleRect'
import { isOverlayCovering } from './overlayCoverage'
import { PickedAttachments } from './pickedAttachments'
import type { Workspace } from '../../app/useWorkspace'
import { useI18n } from '../../i18n'

const emptyState: BrowserState = {
  url: '',
  title: '',
  loading: false,
  canGoBack: false,
  canGoForward: false
}

export function BrowserTab({
  workspace,
  onSend
}: {
  workspace?: Workspace
  onSend?(prompt: string, attachmentIds: string[]): Promise<unknown>
}) {
  const { t } = useI18n()
  const [picking, setPicking] = useState(false)
  const [picked, setPicked] = useState<PickedElementResult[] | null>(null)
  const pickedAttachments = useRef<PickedAttachments | null>(null)
  const updatePicked = (
    results: PickedElementResult[] | null,
    owner = pickedAttachments.current
  ) => {
    const ids =
      results?.flatMap((result) => (result.attachmentId ? [result.attachmentId] : [])) || []
    if (owner?.replace(ids)) setPicked(results)
  }
  useEffect(() => {
    if (!picking) return
    const key = (event: KeyboardEvent) => {
      if (event.key === 'Escape') void window.prime.browserPickCancel().catch(() => {})
    }
    window.addEventListener('keydown', key)
    return () => window.removeEventListener('keydown', key)
  }, [picking])
  useEffect(() => {
    const owner = new PickedAttachments((ids) => {
      void window.prime.releaseAttachments(ids).catch(() => {})
    })
    pickedAttachments.current = owner
    return () => {
      owner.dispose()
      pickedAttachments.current = null
      void window.prime.browserPickCancel().catch(() => {})
    }
  }, [])
  const pick = async () => {
    if (picking) {
      await window.prime.browserPickCancel()
      return
    }
    const owner = pickedAttachments.current
    updatePicked(null, owner)
    setPicking(true)
    try {
      updatePicked(await window.prime.browserPick(), owner)
    } catch (reason) {
      setError(String((reason as Error)?.message || reason).replace(/^.*Error: /, ''))
    } finally {
      setPicking(false)
    }
  }
  const viewport = useRef<HTMLDivElement>(null)
  const [state, setState] = useState<BrowserState>(emptyState)
  const [address, setAddress] = useState('')
  const [editing, setEditing] = useState(false)
  const [error, setError] = useState('')
  const stateLoadedRef = useRef(false)
  const syncRef = useRef<() => void>(() => {})
  useEffect(() => {
    void window.prime
      .browserState()
      .then((value) => {
        stateLoadedRef.current = true
        setState(value)
        setAddress(value.url)
        syncRef.current()
      })
      .catch((reason) => setError(String(reason?.message || reason).replace(/^.*Error: /, '')))
    return window.prime.onEvent((event) => {
      if (event.type !== 'browser' || event.payload?.action !== 'state') return
      const { action: _action, ...next } = event.payload
      stateLoadedRef.current = true
      setState(next)
      setError('')
      syncRef.current()
    })
  }, [])
  useEffect(() => {
    if (!editing) setAddress(state.url === 'about:blank' ? '' : state.url)
  }, [state.url, editing])
  const stateUrlRef = useRef(state.url)
  stateUrlRef.current = state.url
  useEffect(() => {
    const element = viewport.current
    if (!element) return
    let frame = 0
    let applied = ''
    const sync = () => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        if (!stateLoadedRef.current) return
        const rect = visibleRect(element)
        const covered = rect ? isOverlayCovering(rect) : false
        const hidden = covered || !rect || !stateUrlRef.current
        const next = hidden ? 'hidden' : JSON.stringify(rect)
        if (next === applied) return
        applied = next
        if (hidden) void window.prime.browserHide().catch(() => {})
        else void window.prime.browserShow(rect).catch(() => {})
      })
    }
    syncRef.current = sync
    sync()
    const resize = new ResizeObserver(sync)
    resize.observe(element)
    resize.observe(document.body)
    const overlays = new MutationObserver(sync)
    overlays.observe(document.body, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ['open', 'class', 'hidden']
    })
    window.addEventListener('resize', sync)
    return () => {
      cancelAnimationFrame(frame)
      resize.disconnect()
      overlays.disconnect()
      window.removeEventListener('resize', sync)
      void window.prime.browserHide().catch(() => {})
    }
  }, [])
  useEffect(() => {
    syncRef.current()
  }, [state.url])
  const navigate = (value: string) => {
    setEditing(false)
    setError('')
    void window.prime
      .browserNavigate(value)
      .then(setState)
      .catch((reason) => setError(String(reason?.message || reason).replace(/^.*Error: /, '')))
  }
  const command = (name: 'back' | 'forward' | 'reload' | 'fit') =>
    void window.prime
      .browserCommand(name)
      .then(setState)
      .catch((reason) => setError(String(reason?.message || reason).replace(/^.*Error: /, '')))
  return (
    <div className="browser-tab">
      <BrowserTabs
        state={state}
        onChange={(next) => {
          setState(next)
          setError('')
        }}
      />
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
        {onSend && (
          <button
            className={`icon small ${picking ? 'active' : ''}`}
            aria-label={t('Выбрать элемент')}
            aria-pressed={picking}
            title={t('Выбрать элемент')}
            disabled={!/^(https?|file):/.test(state.url)}
            onClick={() => void pick()}
          >
            <Crosshair size={14} />
          </button>
        )}
        <button
          className="icon small"
          aria-label={t('Открыть во внешнем браузере')}
          title={t('Открыть во внешнем браузере')}
          disabled={!/^https?:/.test(state.url)}
          onClick={() =>
            void window.prime
              .browserOpenExternal()
              .catch((reason) =>
                setError(String(reason?.message || reason).replace(/^.*Error: /, ''))
              )
          }
        >
          <ExternalLink size={14} />
        </button>
      </div>
      {error && <p className="browser-error">{error}</p>}
      {picking && (
        <p className="browser-pick-hint" role="status">
          {t('Нажмите на элемент. С Ctrl — несколько. Esc — отмена.')}
        </p>
      )}
      {picked && onSend && workspace && (
        <ElementPick
          results={picked}
          workspace={workspace}
          onSend={async (prompt, ids) => {
            const owner = pickedAttachments.current
            owner?.beginSend(ids)
            try {
              return await onSend(prompt, ids)
            } finally {
              owner?.endSend(ids)
            }
          }}
          onRemove={(index) => updatePicked(picked.filter((_, position) => position !== index))}
          onClose={() => updatePicked(null)}
        />
      )}
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
        {state.url && state.offscreenViewport && (
          <div className="browser-empty browser-offscreen">
            <Globe size={28} />
            <strong>
              {t('Проверка в ширине {value0} px', {
                value0: Math.round(state.offscreenViewport)
              })}
            </strong>
            <p>
              {t('Страница проверяется в фоне в запрошенной ширине, чтобы интерфейс не скакал.')}
            </p>
            <button type="button" className="quiet" onClick={() => command('fit')}>
              {t('Показать по ширине панели')}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
