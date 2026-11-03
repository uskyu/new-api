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
import { createInstance } from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { afterEach, describe, expect, test } from 'vitest'

import { i18nOptions } from './config'
import {
  applyEnglishDefault,
  LANGUAGE_CACHE_KEY,
  LANGUAGE_DEFAULT_VERSION_KEY,
} from './default-language'

function setBrowserLanguages(languages: string[]) {
  Object.defineProperty(window.navigator, 'languages', {
    value: languages,
    configurable: true,
  })
  Object.defineProperty(window.navigator, 'language', {
    value: languages[0],
    configurable: true,
  })
}

/**
 * Resolve the startup language exactly like `./config` does, on a throwaway
 * i18next instance so the process-wide singleton (already initialized by the
 * test setup) cannot mask detection.
 */
async function detectStartupLanguage() {
  applyEnglishDefault()
  const instance = createInstance()
  await instance.use(LanguageDetector).init({ ...i18nOptions, debug: false })
  return instance.language
}

describe('i18n startup language', () => {
  afterEach(() => {
    window.localStorage.clear()
    setBrowserLanguages(['en-US'])
  })

  test('defaults to English on a Chinese browser with no stored choice', async () => {
    window.localStorage.clear()
    setBrowserLanguages(['zh-CN', 'zh'])

    await expect(detectStartupLanguage()).resolves.toBe('en')
  })

  test('drops the auto-detected language cached by older builds', async () => {
    window.localStorage.clear()
    window.localStorage.setItem(LANGUAGE_CACHE_KEY, 'zh-CN')
    setBrowserLanguages(['zh-CN'])

    await expect(detectStartupLanguage()).resolves.toBe('en')
  })

  test('keeps a language the visitor picked themselves', async () => {
    window.localStorage.clear()
    window.localStorage.setItem(LANGUAGE_DEFAULT_VERSION_KEY, '1')
    window.localStorage.setItem(LANGUAGE_CACHE_KEY, 'ja')
    setBrowserLanguages(['zh-CN'])

    await expect(detectStartupLanguage()).resolves.toBe('ja')
  })
})
