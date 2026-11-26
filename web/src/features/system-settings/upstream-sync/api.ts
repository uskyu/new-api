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

import type {
  UpdateUpstreamSyncConfigRequest,
  UpstreamSyncConfigResponse,
  UpstreamSyncLogQuery,
  UpstreamSyncLogsResponse,
  UpstreamSyncLogsWireResponse,
  UpstreamSyncRunResponse,
} from './types'

const UPSTREAM_SYNC_BASE_PATH = '/api/upstream-sync'

export async function getUpstreamSyncConfig() {
  const res = await api.get<UpstreamSyncConfigResponse>(
    `${UPSTREAM_SYNC_BASE_PATH}/config`
  )
  return res.data
}

export async function updateUpstreamSyncConfig(
  request: UpdateUpstreamSyncConfigRequest
) {
  const res = await api.put<UpstreamSyncConfigResponse>(
    `${UPSTREAM_SYNC_BASE_PATH}/config`,
    request
  )
  return res.data
}

export async function getUpstreamSyncLogs(
  query: UpstreamSyncLogQuery
): Promise<UpstreamSyncLogsResponse> {
  const res = await api.get<UpstreamSyncLogsWireResponse>(
    `${UPSTREAM_SYNC_BASE_PATH}/logs`,
    { params: query }
  )
  const body = res.data

  // The documented contract nests paging inside `data`, while the current
  // backend returns `data` as the item array plus top-level paging fields.
  if (Array.isArray(body.data)) {
    return {
      ...body,
      data: {
        items: body.data,
        total: typeof body.total === 'number' ? body.total : body.data.length,
        page: body.page ?? query.page,
        page_size: body.page_size ?? query.page_size,
      },
    }
  }

  return body as UpstreamSyncLogsResponse
}

export async function runUpstreamSync() {
  const res = await api.post<UpstreamSyncRunResponse>(
    `${UPSTREAM_SYNC_BASE_PATH}/run`
  )
  return res.data
}
