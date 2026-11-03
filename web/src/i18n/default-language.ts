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
/**
 * i18next-browser-languagedetector persists the active language under this key.
 */
export const LANGUAGE_CACHE_KEY = 'i18nextLng'

/**
 * Marks that the English-first default has been applied once in this browser.
 */
export const LANGUAGE_DEFAULT_VERSION_KEY = 'interface_language_default_version'

const LANGUAGE_DEFAULT_VERSION = '1'

type LanguageStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

/**
 * English is the default interface language for every first-time visitor.
 *
 * Language detection no longer reads `navigator.language`, so the only source
 * of a non-English UI is the localStorage cache written when someone picks a
 * language themselves. Older builds cached the auto-detected browser language
 * too, which is indistinguishable from a real choice, so the cache is dropped
 * once (after that, every cached value is a deliberate choice and is kept).
 *
 * Safe to call when storage is unavailable (private mode, disabled cookies):
 * the detector simply falls back to English again.
 */
export function applyEnglishDefault(storage?: LanguageStorage | null): void {
  let target: LanguageStorage | null
  try {
    target = storage ?? window.localStorage
  } catch {
    return
  }
  if (!target) return

  try {
    if (
      target.getItem(LANGUAGE_DEFAULT_VERSION_KEY) === LANGUAGE_DEFAULT_VERSION
    ) {
      return
    }
    target.removeItem(LANGUAGE_CACHE_KEY)
    target.setItem(LANGUAGE_DEFAULT_VERSION_KEY, LANGUAGE_DEFAULT_VERSION)
  } catch {
    // Storage can be read-only or over quota; the default still applies because
    // nothing is cached for the detector to find.
  }
}
