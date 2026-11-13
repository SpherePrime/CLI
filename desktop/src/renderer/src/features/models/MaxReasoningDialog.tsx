import { useRef, useState } from 'react'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'

export function MaxReasoningDialog({
  close,
  confirm
}: {
  close: () => void
  confirm: (skipNextTime: boolean) => void
}) {
  const { t } = useI18n()
  const [skipNextTime, setSkipNextTime] = useState(false)
  const enableButton = useRef<HTMLButtonElement>(null)
  return (
    <Modal
      title={t('Максимальный уровень размышлений')}
      close={close}
      className="max-reasoning-dialog"
      initialFocus={enableButton}
    >
      <div className="max-reasoning-notice">
        <p>
          {t(
            'Модель размышляет дольше всего, а каждую часть задачи строго проверяют отдельные агенты-ревьюеры. Они делают скриншоты на разных размерах экрана, в том числе крупные планы отдельных элементов, и возвращают замечания автору, пока результат их не устроит.'
          )}
        </p>
        <p>
          {t(
            'Работа займёт больше времени и потратит заметно больше токенов, зато результат обычно точнее и аккуратнее. Подходит для сложных и важных задач, для простых правок хватит высокого уровня.'
          )}
        </p>
        <label className="checkbox-label">
          <input
            type="checkbox"
            checked={skipNextTime}
            onChange={(event) => setSkipNextTime(event.target.checked)}
          />
          {t('Больше не показывать')}
        </label>
        <div className="approval-actions">
          <button type="button" className="quiet" onClick={close}>
            {t('Отмена')}
          </button>
          <button
            ref={enableButton}
            type="button"
            className="primary"
            onClick={() => confirm(skipNextTime)}
          >
            {t('Включить')}
          </button>
        </div>
      </div>
    </Modal>
  )
}
