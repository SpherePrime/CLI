import type { AutomationRun, AutomationTrigger } from '../../../../shared/automations'

type Translate = (source: string, values?: Record<string, string | number>) => string
export const isActive = (run: AutomationRun) => run.status === 'running' || run.status === 'claimed'
export function statusLabel(run: AutomationRun, t: Translate) {
  return t(
    {
      claimed: 'Подготовка',
      running: 'Выполняется',
      completed: 'Завершено',
      failed: 'Ошибка',
      cancelled: 'Отменено',
      interrupted: 'Прервано'
    }[run.status]
  )
}
export function triggerLabel(trigger: AutomationTrigger, t: Translate) {
  if (trigger.type === 'interval') return t('Каждые {minutes} мин', { minutes: trigger.minutes })
  if (trigger.type === 'daily') return t('Ежедневно в {time}', { time: trigger.time })
  if (trigger.type === 'weekly')
    return `${t('Еженедельно')} · ${trigger.days.map((day) => t(['Вс', 'Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб'][day])).join(', ')} · ${trigger.time}`
  return t(trigger.type === 'filesChanged' ? 'Изменение файлов' : 'Ошибка запуска')
}
export function automationError(error: unknown, t: Translate) {
  const message = error instanceof Error ? error.message : String(error)
  const code = Object.keys(errorMessages).find((key) => message.includes(key))
  return code ? t(errorMessages[code]) : message
}
const errorMessages: Record<string, string> = {
  'automation.corrupt':
    'Файл автоматизаций повреждён. Восстановите его из резервной копии; исходный файл сохранён.',
  'automation.active': 'Сначала остановите активный запуск, затем удалите правило.',
  'automation.invalid': 'Проверьте название, задачу и расписание. Минимальный интервал — 1 минута.',
  'automation.notFound': 'Автоматизация больше не существует. Обновите список.',
  'automation.interrupted': 'Запуск прерван до отправки задачи.',
  'automation.recoveryFailed': 'Не удалось восстановить состояние запуска.',
  'automation.notLoaded': 'Автоматизации ещё загружаются.'
}
