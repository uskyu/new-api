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
import { describe, expect, test } from 'vitest'

import {
  applyEnglishDefault,
  LANGUAGE_CACHE_KEY,
  LANGUAGE_DEFAULT_VERSION_KEY,
} from './default-language'

function createStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial))
  return {
    data,
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => {
      data.set(key, value)
    },
    removeItem: (key: string) => {
      data.delete(key)
    },
  }
}

describe('applyEnglishDefault', () => {
  test('clears the cache on a first visit so English applies', () => {
    const storage = createStorage({ [LANGUAGE_CACHE_KEY]: 'zhCN' })

    applyEnglishDefault(storage)

    expect(storage.getItem(LANGUAGE_CACHE_KEY)).toBeNull()
    expect(storage.getItem(LANGUAGE_DEFAULT_VERSION_KEY)).toBe('1')
  })

  test('drops a legacy auto-detected cache only once', () => {
    const storage = createStorage()

    applyEnglishDefault(storage)
    storage.setItem(LANGUAGE_CACHE_KEY, 'ja')

    applyEnglishDefault(storage)

    expect(storage.getItem(LANGUAGE_CACHE_KEY)).toBe('ja')
  })

  test('keeps the language chosen after the migration', () => {
    const storage = createStorage({
      [LANGUAGE_DEFAULT_VERSION_KEY]: '1',
      [LANGUAGE_CACHE_KEY]: 'fr',
    })

    applyEnglishDefault(storage)

    expect(storage.getItem(LANGUAGE_CACHE_KEY)).toBe('fr')
  })

  test('swallows unavailable or broken storage', () => {
    expect(() => applyEnglishDefault(null)).not.toThrow()

    const broken = {
      getItem: () => {
        throw new Error('denied')
      },
      setItem: () => {
        throw new Error('denied')
      },
      removeItem: () => {
        throw new Error('denied')
      },
    }

    expect(() => applyEnglishDefault(broken)).not.toThrow()
  })
})
