type Translate = (key: string, values?: Record<string, string | number>) => string

export function normalizeProviderModels(models: Record<string, any>[], t: Translate) {
  return models.map((model) => {
    const normalized = { ...model }
    for (const field of ['context_window', 'default_max_tokens']) {
      const value = model[field]
      if (value === '' || value == null) {
        delete normalized[field]
        continue
      }
      if (!Number.isSafeInteger(value) || value < 0)
        throw new Error(
          t('Модель «{model}»: {field} должен быть целым неотрицательным числом', {
            model: model.name || model.id,
            field: t(field === 'context_window' ? 'Контекст' : 'Лимит ответа')
          })
        )
    }
    if (normalized.context_window > 0 && normalized.default_max_tokens > normalized.context_window)
      throw new Error(
        t('Модель «{model}»: лимит ответа превышает контекст', {
          model: model.name || model.id
        })
      )
    return normalized
  })
}
