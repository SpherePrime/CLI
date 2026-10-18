import { english } from './translations'
export type Language = 'ru' | 'en'
export type TranslationValues = Record<string, string | number>

export function translate(
  language: Language | undefined,
  source: string,
  values: TranslationValues = {}
) {
  const text = language === 'en' ? (english[source] ?? source) : source
  return text.replace(/\{(\w+)\}/g, (placeholder, key: string) =>
    Object.prototype.hasOwnProperty.call(values, key) ? String(values[key]) : placeholder
  )
}
