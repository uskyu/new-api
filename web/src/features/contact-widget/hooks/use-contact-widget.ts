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
import i18next from 'i18next'
import { useEffect, useState } from 'react'

import { handleServerError } from '@/lib/handle-server-error'

import { getContactWidget } from '../api'
import { parseContactWidgetConfig } from '../lib/config'
import type { ContactWidgetConfig } from '../types'

const STORAGE_KEY = 'contact_widget'

export type ContactWidgetResult = {
  config: ContactWidgetConfig | null
  isLoaded: boolean
}

/**
 * Loads the contact widget configuration for public pages, caching the raw
 * option value in localStorage for immediate first paint.
 */
export function useContactWidget(): ContactWidgetResult {
  const [config, setConfig] = useState<ContactWidgetConfig | null>(null)
  const [isLoaded, setIsLoaded] = useState(false)

  useEffect(() => {
    let mounted = true

    const loadConfig = async () => {
      const cached = localStorage.getItem(STORAGE_KEY)
      if (cached && mounted) {
        setConfig(parseContactWidgetConfig(cached))
      }

      try {
        const response = await getContactWidget()

        if (!mounted) return

        if (response.success && response.data) {
          const parsed = parseContactWidgetConfig(response.data)
          setConfig(parsed)
          if (parsed) {
            localStorage.setItem(STORAGE_KEY, response.data)
          } else {
            localStorage.removeItem(STORAGE_KEY)
          }
        } else {
          setConfig(null)
          localStorage.removeItem(STORAGE_KEY)
        }
      } catch (error) {
        if (!mounted) return
        handleServerError(error, i18next.t('Failed to load contact widget'))
      } finally {
        if (mounted) {
          setIsLoaded(true)
        }
      }
    }

    loadConfig()

    return () => {
      mounted = false
    }
  }, [])

  return { config, isLoaded }
}
