import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'

export function ToolModeSetting({
  smart,
  disabled,
  onChange
}: {
  smart: boolean
  disabled: boolean
  onChange: (smart: boolean) => void
}) {
  const { t } = useI18n()
  return (
    <label className="form-label">
      {t('Режим инструментов')}
      <Select
        label={t('Режим инструментов')}
        value={smart ? 'smart' : 'standard'}
        disabled={disabled}
        onChange={(value) => onChange(value === 'smart')}
        options={[
          { value: 'standard', label: t('Обычный') },
          { value: 'smart', label: 'SmartTools' }
        ]}
      />
      <small className="muted">
        {smart
          ? t('Агент находит нужные инструменты и скиллы через поиск перед использованием.')
          : t('Агент получает список инструментов и описания доступных скиллов сразу.')}
      </small>
      {disabled && <small className="muted">{t('Сначала завершите активные задачи')}</small>}
    </label>
  )
}
