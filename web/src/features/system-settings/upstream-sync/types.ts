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
export type UpstreamSyncConfig = {
  enabled: boolean
  interval_minutes: number
  last_run_at: number | null
  last_status: string
  active_task_id: string
}

export type UpstreamSyncConfigResponse = {
  success: boolean
  message?: string
  data?: UpstreamSyncConfig
}

export type UpdateUpstreamSyncConfigRequest = {
  enabled: boolean
  interval_minutes: number
}

export type UpstreamSyncPriceChange = {
  model_name: string
  fields?: string[]
  before?: Record<string, unknown>
  after?: Record<string, unknown>
}

export type UpstreamSyncIssue = {
  level?: string
  scope?: string
  base_url?: string
  channel_id?: number
  channel_name?: string
  group?: string
  message: string
}

export type UpstreamSyncLog = {
  id: number
  started_at: number
  finished_at: number | null
  trigger: string
  status: string
  base_url: string
  pricing_version: string
  groups_checked: number
  channels_changed: number
  added_models: string[]
  removed_models: string[]
  price_changes: UpstreamSyncPriceChange[]
  errors: UpstreamSyncIssue[]
  duration_ms: number
  message: string
}

export type UpstreamSyncLogsPage = {
  items: UpstreamSyncLog[]
  total: number
  page: number
  page_size: number
}

export type UpstreamSyncLogsWireResponse = {
  success: boolean
  message?: string
  data?: UpstreamSyncLog[] | UpstreamSyncLogsPage
  total?: number
  page?: number
  page_size?: number
}

export type UpstreamSyncLogsResponse = {
  success: boolean
  message?: string
  data?: UpstreamSyncLogsPage
}

export type UpstreamSyncLogQuery = {
  page: number
  page_size: number
  status?: string
  start_time?: number
  end_time?: number
  q?: string
}

export type UpstreamSyncRunResponse = {
  success: boolean
  message?: string
  data?: {
    task_id: string
  }
}
