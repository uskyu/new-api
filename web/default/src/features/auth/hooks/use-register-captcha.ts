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
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { useStatus } from '@/hooks/use-status'

interface RegisterCaptchaData {
  captcha_id: string
  image_base64: string
}

interface RegisterCaptchaResponse {
  success: boolean
  message?: string
  data?: RegisterCaptchaData
}

/**
 * Hook for managing the one-time image captcha required by registration.
 *
 * The backend rate limits captcha generation, so consumers should only call
 * `refresh()` once on mount and on explicit user action (clicking the image).
 */
export function useRegisterCaptcha() {
  const { t } = useTranslation()
  const { status } = useStatus()

  const [captchaId, setCaptchaId] = useState('')
  const [captchaImage, setCaptchaImage] = useState('')
  const [captchaAnswer, setCaptchaAnswer] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  const isRegisterCaptchaEnabled = Boolean(
    status?.register_captcha_enabled ??
    status?.data?.register_captcha_enabled ??
    false
  )
  const captchaKind =
    status?.register_captcha_kind ??
    status?.data?.register_captcha_kind ??
    'math'

  /**
   * Fetch a fresh captcha challenge and clear the previous answer.
   */
  const refresh = useCallback(async () => {
    if (!isRegisterCaptchaEnabled) return

    setIsLoading(true)
    try {
      const res = await api.get<RegisterCaptchaResponse>(
        '/api/user/register/captcha',
        { skipBusinessError: true, skipErrorHandler: true }
      )
      if (res.data?.success && res.data.data) {
        setCaptchaId(res.data.data.captcha_id)
        setCaptchaImage(res.data.data.image_base64)
        setCaptchaAnswer('')
        return
      }
      setCaptchaId('')
      setCaptchaImage('')
      setCaptchaAnswer('')
      toast.error(t('Failed to fetch registration captcha'))
    } catch (_error) {
      setCaptchaId('')
      setCaptchaImage('')
      setCaptchaAnswer('')
      toast.error(t('Failed to fetch registration captcha'))
    } finally {
      setIsLoading(false)
    }
  }, [isRegisterCaptchaEnabled, t])

  /**
   * Validate that the captcha has been answered when it is required.
   */
  const validate = useCallback((): boolean => {
    if (!isRegisterCaptchaEnabled) return true
    if (!captchaAnswer.trim()) {
      toast.error(t('Please enter the captcha'))
      return false
    }
    return true
  }, [captchaAnswer, isRegisterCaptchaEnabled, t])

  const resetAnswer = useCallback(() => {
    setCaptchaAnswer('')
  }, [])

  return {
    isRegisterCaptchaEnabled,
    captchaKind,
    captchaId,
    captchaImage,
    captchaAnswer,
    setCaptchaAnswer,
    isLoading,
    refresh,
    validate,
    resetAnswer,
  }
}
