import type { SttStatus } from '../../../../shared/stt'

export async function ensureSttReady(
  call: (operation: 'stt') => Promise<unknown>,
  t: (source: string) => string
) {
  const catalog = (await call('stt')) as SttStatus | undefined
  const engine = catalog?.engine || 'auto'
  if (engine === 'whisper' || engine === 'vosk') {
    const models = catalog?.providers?.find((item) => item.id === engine)?.models || []
    if (!models.some((item) => item.installed)) {
      throw new Error(t('Для выбранного провайдера скачайте модель в настройках распознавания речи'))
    }
  }
}
