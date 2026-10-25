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

// The gateway ledger is the operator's own bookkeeping of what it charged the
// platform and what it consumed. Token counts are nullable on purpose: null means
// the gateway never measured that figure, which is a different statement from a
// measured zero.

export interface LedgerUsage {
  input_tokens: number | null
  output_tokens: number | null
  cache_read_tokens: number | null
  cache_write_tokens: number | null
  total_tokens?: number | null
  split_known: boolean
}

export interface LedgerBilling {
  base_amount: string
  group_ratio: number
  tax_rate_percent: number
  total_amount: string
  currency: string
}

export interface LedgerRow {
  id: number
  request_id: string
  harness_request_id?: string
  harness_run_id?: string
  status: string
  model: string
  channel_id: number
  pricing_group?: string
  usage: LedgerUsage
  billing: LedgerBilling
  created_at: number
  settled_at?: number
}

// The paged envelope is the project's shared PageInfo shape, not a bespoke one.
export interface LedgerPage {
  items: LedgerRow[]
  total: number
  page: number
  page_size: number
}

// These are the settlement states the gateway actually stores. `adjusted` is a
// ledger-event status, not a settlement status, so it must not appear here: a
// filter offering it would answer 400 from the backend.
export type LedgerStatus = 'pending' | 'settled' | 'refunded' | 'failed'

export const LEDGER_STATUS_VALUES: LedgerStatus[] = [
  'pending',
  'settled',
  'refunded',
  'failed',
]
