import { projectName } from './projectName'
import {
  ArrowUpRight,
  Code2,
  FileSearch,
  FolderOpen,
  ListChecks,
  MessageSquare,
  ShieldCheck,
  Sparkles
} from 'lucide-react'
import { PrimeLogo } from '../../components/PrimeLogo'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'

export function Welcome({
  workspace: w,
  openCommands
}: {
  workspace: Workspace
  openCommands: () => void
}) {
  const { t } = useI18n()
  const hasFolder = Boolean(w.project && w.project.kind !== 'chat')
  const projectTitle = projectName(w.project)
  const starters = hasFolder
    ? [
        {
          icon: FileSearch,
          title: t('Разобраться в проекте'),
          description: t('Архитектура, файлы и связи'),
          prompt: t('Изучи проект и объясни его архитектуру и основные точки входа.')
        },
        {
          icon: Code2,
          title: t('Внести изменения'),
          description: t('От задачи к готовому решению'),
          prompt: t('Изучи проект и помоги реализовать следующую задачу: ')
        },
        {
          icon: ShieldCheck,
          title: t('Проверить качество'),
          description: t('Ошибки, риски и улучшения'),
          prompt: t(
            'Проведи ревью проекта: найди ошибки, риски безопасности и предложи приоритетные исправления.'
          )
        },
        {
          icon: ListChecks,
          title: t('Добавить проверки'),
          description: t('Тесты для важных сценариев'),
          prompt: t(
            'Изучи существующие тесты и предложи проверки для важных сценариев, которые ещё не покрыты.'
          )
        }
      ]
    : [
        {
          icon: Sparkles,
          title: t('Обсудить идею'),
          description: t('Найти направление и решение'),
          prompt: t('Помоги продумать идею: ')
        },
        {
          icon: FileSearch,
          title: t('Разобраться в вопросе'),
          description: t('Понятно объяснить сложное'),
          prompt: t('Помоги разобраться в следующем вопросе: ')
        },
        {
          icon: ListChecks,
          title: t('Составить план'),
          description: t('От замысла к конкретным шагам'),
          prompt: t('Помоги составить план для следующей задачи: ')
        },
        {
          icon: MessageSquare,
          title: t('Улучшить текст'),
          description: t('Структура, ясность и стиль'),
          prompt: t('Помоги улучшить структуру, ясность и стиль следующего текста: ')
        }
      ]
  return (
    <section className="welcome">
      <div className="welcome-content">
        <div className="welcome-wordmark">
          <PrimeLogo />
        </div>
        <div className="eyebrow">
          {hasFolder ? t('ПРОСТРАНСТВО ДЛЯ РАБОТЫ') : t('ПРОСТРАНСТВО ДЛЯ ИДЕЙ')}
        </div>
        <h1>
          {hasFolder ? t('Большие идеи.') : t('Меньше рутины.')}
          <br />
          <em>{hasFolder ? t('Конкретные шаги.') : t('Больше возможностей.')}</em>
        </h1>
        <p>
          {hasFolder ? (
            <>
              {t('Проект')} <strong>{projectTitle}</strong> {t('готов к работе.')}
              <br />
              {t('Начните с задачи — инструменты уже рядом.')}
            </>
          ) : (
            <>
              {t('Выберите модель и начните разговор.')}
              <br />
              {t('Папку можно подключить, когда понадобятся файлы проекта.')}
            </>
          )}
        </p>
        <div className="welcome-actions">
          {!hasFolder && (
            <button
              className="primary open-project"
              disabled={w.loading || w.info.runtime !== 'ready'}
              onClick={() => void w.openProject()}
            >
              <FolderOpen size={16} />
              {t('Открыть проект')}
              <ArrowUpRight size={15} />
            </button>
          )}
          <button className="welcome-command" onClick={openCommands}>
            {t('Быстрые действия')}
            <kbd>Ctrl K</kbd>
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
