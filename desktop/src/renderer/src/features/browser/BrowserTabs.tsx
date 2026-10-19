import { Globe, LoaderCircle, Plus, X } from 'lucide-react'
import type { BrowserState } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'

function label(tab: { url: string; title: string }, fallback: string) {
  if (tab.title && tab.title !== tab.url) return tab.title
  try {
    return new URL(tab.url).host || fallback
  } catch {
    return fallback
  }
}

export function BrowserTabs({ state, onChange }: { state: BrowserState; onChange(state: BrowserState): void }) {
  const { t } = useI18n()
  const tabs = state.tabs ?? []
  const run = (action: Promise<BrowserState>) => void action.then(onChange).catch(() => {})
  return (
    <div className="browser-tabs" role="tablist" aria-label={t('Вкладки браузера')}>
      {tabs.map((tab) => {
        const title = label(tab, t('Новая вкладка'))
        return (
          <div
            key={tab.id}
            role="tab"
            tabIndex={0}
            aria-selected={tab.id === state.activeTab}
            className={`browser-tab-item ${tab.id === state.activeTab ? 'active' : ''}`}
            title={tab.url || title}
            onClick={() => run(window.prime.browserTab('select', tab.id))}
            onKeyDown={(event) => {
              if (event.key === 'Enter' || event.key === ' ') run(window.prime.browserTab('select', tab.id))
            }}
            onAuxClick={(event) => {
              if (event.button === 1) run(window.prime.browserTab('close', tab.id))
            }}
          >
            {tab.loading ? <LoaderCircle size={12} className="spin" /> : <Globe size={12} />}
            <span>{title}</span>
            <button
              className="browser-tab-close"
              aria-label={t('Закрыть вкладку {value0}', { value0: title })}
              onClick={(event) => {
                event.stopPropagation()
                run(window.prime.browserTab('close', tab.id))
              }}
            >
              <X size={11} />
            </button>
          </div>
        )
      })}
      <button
        className="browser-tab-new"
        aria-label={t('Новая вкладка')}
        title={t('Новая вкладка')}
        disabled={tabs.length >= 12}
        onClick={() => run(window.prime.browserTab('new'))}
      >
        <Plus size={13} />
      </button>
    </div>
  )
}
