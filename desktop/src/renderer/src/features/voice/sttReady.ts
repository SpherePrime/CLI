import { normalizeAsrLanguage, type AsrLanguage, type AsrStatus, type SttStatus } from '../../../../shared/stt'

export async function resolveAsrLanguage(
  call: (operation: 'stt') => Promise<unknown>
): Promise<AsrLanguage> {
  try {
    const catalog = (await call('stt')) as SttStatus | undefined
    return normalizeAsrLanguage(catalog?.language)
  } catch {
    return 'ru'
  }
}

export async function ensureSttReady(
  call: (operation: 'stt') => Promise<unknown>,
  t: (source: string) => string
): Promise<AsrLanguage> {
  const [language, status] = await Promise.all([
    resolveAsrLanguage(call),
    window.prime.asr({ action: 'status' }).catch(() => null) as Promise<AsrStatus | null>
  ])
  if (status && status.state === 'error') {
    throw new Error(status.message || t('Локальная модель распознавания речи недоступна'))
  }
  if (!status?.ready) {
    void window.prime.asr({ action: 'warmup' }).catch(() => {})
  }
  return language
}
