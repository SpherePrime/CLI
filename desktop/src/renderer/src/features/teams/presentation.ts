import type { TeamTaskStatus } from '../../../../shared/teams'

const statuses: Record<TeamTaskStatus, string> = {
  queued: 'В очереди',
  starting: 'Запуск',
  running: 'Выполняется',
  completed: 'Готово',
  failed: 'Ошибка',
  cancelled: 'Остановлено',
  interrupted: 'Прервано'
}

const errors: Record<string, string> = {
  'team.invalid': 'Заполните название группы, задачи и инструкции.',
  'team.missing': 'Группа или исполнитель не найдены.',
  'team.active': 'Сначала остановите группу.',
  'team.limit': 'Слишком много активных групп.',
  'team.corrupt': 'Хранилище групп повреждено. Изменения заблокированы.',
  'team.notLoaded': 'Группы ещё загружаются.'
}

export function teamStatusLabel(status: TeamTaskStatus) {
  return statuses[status]
}

export function teamError(error: unknown, t: (text: string) => string) {
  const message = error instanceof Error ? error.message : String(error ?? '')
  const code = Object.keys(errors).find((key) => message.includes(key))
  return code ? t(errors[code]) : message
}
