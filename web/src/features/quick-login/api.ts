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
import { api } from '@/lib/api'

export interface QuickLoginAuthorization {
  client_name: string
  redirect_uri: string
  cancel_url: string
  expires_at: number
}

interface ApiResponse<T> {
  success: boolean
  message?: string
  data?: T
}

export async function getQuickLoginAuthorization(code: string) {
  const response = await api.get<ApiResponse<QuickLoginAuthorization>>(
    '/api/quick-login/authorize',
    {
      params: { code },
      skipErrorHandler: true,
    }
  )
  return response.data
}

export async function approveQuickLogin(code: string) {
  const response = await api.post<ApiResponse<{ callback_url: string }>>(
    '/api/quick-login/approve',
    { code },
    { skipErrorHandler: true }
  )
  return response.data
}
