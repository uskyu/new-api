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

import type { LedgerPage, LedgerStatus } from './types'

export interface LedgerQuery {
  page: number
  pageSize: number
  requestId?: string
  modelName?: string
  statuses?: LedgerStatus[]
  startTime?: number
  endTime?: number
}

export interface LedgerResult {
  success: boolean
  message?: string
  data?: LedgerPage
}

// fetchLedger reads one page of the gateway ledger. The admin endpoint answers
// with the project's standard envelope, so a business failure arrives as
// success:false with a message rather than as a thrown error.
export async function fetchLedger(query: LedgerQuery): Promise<LedgerResult> {
  const params = new URLSearchParams()
  params.set('p', String(query.page))
  params.set('page_size', String(query.pageSize))
  if (query.requestId?.trim()) {
    params.set('request_id', query.requestId.trim())
  }
  if (query.modelName?.trim()) {
    params.set('model_name', query.modelName.trim())
  }
  for (const status of query.statuses ?? []) {
    params.append('status', status)
  }
  if (query.startTime) {
    params.set('start_time', String(query.startTime))
  }
  if (query.endTime) {
    params.set('end_time', String(query.endTime))
  }
  const res = await api.get(`/api/harness/ledger/?${params.toString()}`)
  return res.data as LedgerResult
}
