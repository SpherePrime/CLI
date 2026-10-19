type Translate = (source: string, values?: Record<string, string | number>) => string

export function terminalError(error: unknown, t: Translate) {
  const message = error instanceof Error ? error.message : String(error)
  if (/ownership/i.test(message)) return t('Сначала верните управление терминалом')
  if (/limit reached/i.test(message))
    return t('Достигнут лимит терминалов. Закройте одну из вкладок')
  if (/another workspace|access denied/i.test(message))
    return t('Терминал принадлежит другому проекту')
  if (/not found/i.test(message)) return t('Терминал уже закрыт')
  if (/exited|closed/i.test(message)) return t('Процесс завершён. Откройте новый терминал')
  if (/spawn|CreateProcess|ENOENT/i.test(message))
    return t('Не удалось запустить оболочку. Проверьте её установку')
  if (/clipboard|permission.*denied/i.test(message))
    return t('Не удалось скопировать вывод в буфер обмена')
  return t('Ошибка терминала: {message}', { message })
}
