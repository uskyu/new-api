/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import i18n, { type InitOptions } from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'

import { applyEnglishDefault } from './default-language'
import { convertDetectedLanguage } from './languages'
import en from './locales/en.json'
import fr from './locales/fr.json'
import ja from './locales/ja.json'
import ru from './locales/ru.json'
import vi from './locales/vi.json'
import zhTW from './locales/zh-TW.json'
import zhCN from './locales/zh.json'

export const resources = {
  en,
  zhCN,
  fr,
  ru,
  ja,
  vi,
  zhTW,
} as const

// English is the default interface language for every first-time visitor.
applyEnglishDefault()

export const i18nOptions: InitOptions = {
  resources,
  fallbackLng: 'en',
  supportedLngs: ['en', 'zhCN', 'fr', 'ru', 'ja', 'vi', 'zhTW'],
  load: 'currentOnly',
  nsSeparator: false, // Allow literal colons in keys (e.g., URLs, labels)
  debug: import.meta.env.DEV,
  interpolation: {
    escapeValue: false, // not needed for react as it escapes by default
  },
  detection: {
    // Only an explicit choice (cached by the language switcher / profile page)
    // may override the English default. `navigator.language` is deliberately not
    // consulted: a zh-CN browser would otherwise start in Chinese before anyone
    // picks a language.
    order: ['localStorage'],
    caches: ['localStorage'],
    // Cached values from older builds can still be BCP-47 tags (`zh-CN`); map
    // them onto our `zhCN`/`zhTW` codes (other codes pass through unchanged).
    convertDetectedLanguage,
  },
}

i18n.use(LanguageDetector).use(initReactI18next).init(i18nOptions)

export default i18n
