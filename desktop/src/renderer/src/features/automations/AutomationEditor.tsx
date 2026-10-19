import { useId, useState, type FormEvent } from 'react'
import type {
  AutomationInput,
  AutomationRule,
  AutomationTrigger
} from '../../../../shared/automations'
import { useI18n } from '../../i18n'
import { Select } from '../../components/Select'

type Props = {
  rule?: AutomationRule
  busy: boolean
  onSave(input: AutomationInput): Promise<void>
  onCancel(): void
}
export function AutomationEditor({ rule, busy, onSave, onCancel }: Props) {
  const { t } = useI18n()
  const formId = useId()
  const [title, setTitle] = useState(rule?.title ?? '')
  const [prompt, setPrompt] = useState(rule?.prompt ?? '')
  const [enabled, setEnabled] = useState(rule?.enabled ?? true)
  const [type, setType] = useState<AutomationTrigger['type']>(rule?.trigger.type ?? 'interval')
  const [minutes, setMinutes] = useState(
    rule?.trigger.type === 'interval'
      ? rule.trigger.minutes
      : rule?.trigger.type === 'filesChanged' || rule?.trigger.type === 'runFailed'
        ? rule.trigger.cooldownMinutes
        : 30
  )
  const [time, setTime] = useState(
    rule?.trigger.type === 'daily' || rule?.trigger.type === 'weekly' ? rule.trigger.time : '09:00'
  )
  const [days, setDays] = useState(
    rule?.trigger.type === 'weekly' ? rule.trigger.days : [1, 2, 3, 4, 5]
  )
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    const trigger: AutomationTrigger =
      type === 'interval'
        ? { type, minutes }
        : type === 'daily'
          ? { type, time }
          : type === 'weekly'
            ? { type, time, days }
            : { type, cooldownMinutes: minutes }
    await onSave({ id: rule?.id, title, prompt, enabled, trigger })
  }
  return (
    <form
      className="automation-editor"
      onSubmit={submit}
      aria-label={t(rule ? 'Изменить автоматизацию' : 'Новая автоматизация')}
    >
      <div className="automation-section-heading">
        <h3>{t(rule ? 'Изменить автоматизацию' : 'Новая автоматизация')}</h3>
        <button type="button" onClick={onCancel}>
          {t('Закрыть')}
        </button>
      </div>
      <label htmlFor={`${formId}-title`}>{t('Название')}</label>
      <input
        id={`${formId}-title`}
        autoFocus
        required
        maxLength={160}
        value={title}
        onChange={(event) => setTitle(event.target.value)}
      />
      <label htmlFor={`${formId}-prompt`}>{t('Задача для Prime')}</label>
      <textarea
        id={`${formId}-prompt`}
        required
        maxLength={100000}
        rows={4}
        value={prompt}
        onChange={(event) => setPrompt(event.target.value)}
      />
      <div className="automation-form-grid">
        <label>
          {t('Триггер')}
          <Select
            label={t('Триггер')}
            value={type}
            onChange={(value) => setType(value as AutomationTrigger['type'])}
            options={[
              { value: 'interval', label: t('Интервал') },
              { value: 'daily', label: t('Ежедневно') },
              { value: 'weekly', label: t('Еженедельно') },
              { value: 'filesChanged', label: t('Изменение файлов') },
              { value: 'runFailed', label: t('Ошибка запуска') }
            ]}
          />
        </label>
        {(type === 'interval' || type === 'filesChanged' || type === 'runFailed') && (
          <label>
            {t(type === 'interval' ? 'Интервал, минуты' : 'Пауза между событиями, минуты')}
            <input
              type="number"
              min={1}
              max={525600}
              required
              value={minutes}
              onChange={(event) => setMinutes(Number(event.target.value))}
            />
          </label>
        )}
        {(type === 'daily' || type === 'weekly') && (
          <label>
            {t('Время')}
            <input
              type="time"
              required
              value={time}
              onChange={(event) => setTime(event.target.value)}
            />
          </label>
        )}
      </div>
      {type === 'weekly' && (
        <fieldset className="automation-days">
          <legend>{t('Дни недели')}</legend>
          {['Вс', 'Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб'].map((day, index) => (
            <label key={day}>
              <input
                type="checkbox"
                checked={days.includes(index)}
                onChange={(event) =>
                  setDays(
                    event.target.checked
                      ? [...days, index]
                      : days.filter((candidate) => candidate !== index)
                  )
                }
              />
              {t(day)}
            </label>
          ))}
        </fieldset>
      )}
      <p className="automation-muted">
        {t('Местное время')} · {Intl.DateTimeFormat().resolvedOptions().timeZone}
      </p>
      <label className="automation-checkbox">
        <input
          type="checkbox"
          checked={enabled}
          onChange={(event) => setEnabled(event.target.checked)}
        />
        {t('Включено')}
      </label>
      <p className="automation-muted">
        {t(
          'Приостановка правила не останавливает текущую задачу. Разрешения Prime действуют как обычно.'
        )}
      </p>
      <div className="automation-actions">
        <button
          className="automation-primary"
          type="submit"
          disabled={busy || (type === 'weekly' && !days.length)}
        >
          {t(busy ? 'Сохранение…' : 'Сохранить')}
        </button>
        <button type="button" onClick={onCancel} disabled={busy}>
          {t('Отмена')}
        </button>
      </div>
    </form>
  )
}
