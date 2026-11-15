import {
  ArrowUpRight,
  LayoutDashboard,
  LayoutTemplate,
  Palette,
  Smartphone,
  Sparkles
} from 'lucide-react'
import { shortcutLabel } from '../../../../shared/keybindings'
import type { Workspace } from '../../app/useWorkspace'
import { useI18n } from '../../i18n'
import { useShortcuts } from '../shortcuts/shortcutStore'
import './designer.css'

export function DesignerWelcome({
  workspace: w,
  openCommands
}: {
  workspace: Workspace
  openCommands: () => void
}) {
  const { t } = useI18n()
  const shortcuts = useShortcuts()
  const starters = [
    {
      icon: LayoutTemplate,
      title: t('Главный экран сайта'),
      description: t('Композиция, типографика и акценты'),
      prompt: t(
        'Спроектируй главный экран (hero) современного сайта: продумай композицию, палитру, типографику и сгенерируй визуальный концепт через generate_image для: '
      )
    },
    {
      icon: LayoutDashboard,
      title: t('Интерфейс дашборда'),
      description: t('Сетка виджетов и состояния данных'),
      prompt: t(
        'Разработай UI-концепт дашборда: иерархию блоков, компоненты, палитру и визуальный макет через generate_image для: '
      )
    },
    {
      icon: Smartphone,
      title: t('Мобильный экран'),
      description: t('Навигация, карточки и сценарии'),
      prompt: t(
        'Спроектируй мобильный экран приложения с акцентом на эргономику и чистую иерархию и сгенерируй концепт через generate_image для: '
      )
    },
    {
      icon: Sparkles,
      title: t('Фирменный стиль и арт'),
      description: t('Палитра, образы и иллюстрации'),
      prompt: t(
        'Предложи визуальное направление, палитру и сгенерируй иллюстрацию через generate_image для: '
      )
    }
  ]

  return (
    <section className="welcome designer-welcome">
      <div className="welcome-content">
        <div className="welcome-wordmark designer-wordmark">
          <Palette size={22} />
        </div>
        <div className="eyebrow">{t('СТУДИЯ ДИЗАЙНА')}</div>
        <h1>
          {t('От идеи до макета.')}
          <br />
          <em>{t('И сразу в код.')}</em>
        </h1>
        <p>
          {t('Продумывайте интерфейсы, визуальные системы и иллюстрации в отдельном пространстве.')}{' '}
          <span className="designer-welcome-note">
            {t('Любая модель может обсуждать концепцию и генерировать изображения через')}{' '}
            <code>generate_image</code>.{' '}
            {t('Готовый вариант можно в один клик передать в проект.')}
          </span>
        </p>
        <div className="welcome-actions">
          <button
            className="primary open-project"
            disabled={w.loading || w.info.runtime !== 'ready'}
            onClick={() => void w.newDesignerChat()}
          >
            <Palette size={16} />
            {t('Новый дизайн')}
          </button>
          <button className="welcome-command" onClick={openCommands}>
            {t('Быстрые действия')}
            <kbd>{shortcutLabel(shortcuts.commands)}</kbd>
          </button>
        </div>
        <div className="starter-grid">
          {starters.map((item, index) => (
            <button
              key={item.title}
              style={{ '--card-index': index } as React.CSSProperties}
              onClick={() =>
                window.dispatchEvent(new CustomEvent('prime:draft', { detail: item.prompt }))
              }
            >
              <span className="starter-icon">
                <item.icon size={19} />
              </span>
              <strong>{item.title}</strong>
              <span>{item.description}</span>
              <ArrowUpRight className="starter-arrow" size={15} />
            </button>
          ))}
        </div>
      </div>
    </section>
  )
}
