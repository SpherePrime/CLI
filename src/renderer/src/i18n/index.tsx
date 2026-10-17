import { createContext, useContext, useEffect, useMemo, type ReactNode } from 'react'
import { english } from './catalog'

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

function createI18n(language: Language) {
  return {
    language,
    locale: language === 'en' ? 'en-US' : 'ru-RU',
    t: (source: string, values?: TranslationValues) => translate(language, source, values)
  }
}

const I18nContext = createContext(createI18n('ru'))

export function I18nProvider({
  language = 'ru',
  children
}: {
  language?: Language
  children: ReactNode
}) {
  const value = useMemo(() => createI18n(language), [language])
  useEffect(() => {
    document.documentElement.lang = language
  }, [language])
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n() {
  return useContext(I18nContext)
}
