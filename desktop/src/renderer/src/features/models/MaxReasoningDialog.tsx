import { useRef, useState } from 'react'
import { Target, Timer } from 'lucide-react'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'
import { ReasoningMeter } from './ReasoningMeter'
import { reasoningLevels } from './reasoning'

export function MaxReasoningDialog({
  close,
  confirm
}: {
  close: () => void
  confirm: (skipNextTime: boolean) => void
}) {
  const { t } = useI18n()
  const [skipNextTime, setSkipNextTime] = useState(false)
  const content = useRef<HTMLDivElement>(null)
  return (
    <Modal
      bare
      title={t('Максимальный уровень размышлений')}
      close={close}
      className="max-reasoning-dialog"
      initialFocus={content}
      lead={
        <span className="max-reasoning-badge">
          <ReasoningMeter level={reasoningLevels.length} total={reasoningLevels.length} />
        </span>
      }
    >
      <div ref={content} tabIndex={-1} className="max-reasoning-notice">
        <p className="max-reasoning-lead">
          {t('Модель размышляет дольше всего, а каждую часть задачи строго проверяют отдельные агенты-ревьюеры.')}
        </p>
        <ul className="max-reasoning-facts">
          <li>
            <span className="max-reasoning-fact-icon">
              <Timer size={15} />
            </span>
            <span>{t('Работа займёт больше времени и потратит заметно больше токенов.')}</span>
          </li>
          <li className="gain">
            <span className="max-reasoning-fact-icon">
              <Target size={15} />
            </span>
            <span>{t('Зато результат обычно точнее и аккуратнее.')}</span>
          </li>
        </ul>
        <p className="max-reasoning-hint">
          {t('Подходит для сложных и важных задач, для простых правок хватит высокого уровня.')}
        </p>
        <footer className="max-reasoning-footer">
          <label className="max-reasoning-skip">
            <input
              type="checkbox"
              checked={skipNextTime}
              onChange={(event) => setSkipNextTime(event.target.checked)}
            />
            {t('Больше не показывать')}
          </label>
          <div className="max-reasoning-actions">
            <button type="button" className="quiet" onClick={close}>
              {t('Отмена')}
            </button>
            <button type="button" className="primary" onClick={() => confirm(skipNextTime)}>
              {t('Включить')}
            </button>
          </div>
        </footer>
      </div>
    </Modal>
  )
}
