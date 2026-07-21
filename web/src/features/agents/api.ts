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
  AgentAdjustment,
  AgentAdminOverview,
  AgentApiResponse,
  AgentBootstrapStatus,
  AgentDailyMetrics,
  AgentDownlineUser,
  AgentProfile,
  AgentRebateRecord,
  AgentSelfSummary,
  PagedAgentData,
  SupportManagedUser,
} from './types'

export async function getAgentStatus() {
  const response =
    await api.get<AgentApiResponse<AgentBootstrapStatus>>('/api/agent/status')
  return response.data
}

export async function getAgentOverview() {
  const response = await api.get<AgentApiResponse<AgentAdminOverview>>(
    '/api/agent/overview'
  )
  return response.data
}

export async function getAgentProfiles(params: {
  page: number
  pageSize: number
  keyword?: string
}) {
  const response = await api.get<
    AgentApiResponse<PagedAgentData<AgentProfile>>
  >('/api/agent/profiles', {
    params: {
      p: params.page,
      page_size: params.pageSize,
      keyword: params.keyword || undefined,
    },
  })
  return response.data
}

export async function getAgentDailyMetrics(params: {
  agentUserId?: number
  startDate: string
  endDate: string
}) {
  const response = await api.get<AgentApiResponse<AgentDailyMetrics>>(
    '/api/agent/daily-metrics',
    {
      params: {
        agent_user_id: params.agentUserId || undefined,
        start_date: params.startDate,
        end_date: params.endDate,
      },
    }
  )
  return response.data
}

export async function adjustAgentBalance(payload: {
  agentUserId: number
  amount: string
  reason: string
}) {
  const response = await api.post<AgentApiResponse<AgentAdjustment>>(
    '/api/agent/adjust',
    {
      agent_user_id: payload.agentUserId,
      amount: payload.amount,
      reason: payload.reason,
    }
  )
  return response.data
}

export async function searchSupportUsers(params: {
  page: number
  pageSize: number
  keyword: string
}) {
  const response = await api.get<
    AgentApiResponse<PagedAgentData<SupportManagedUser>>
  >('/api/support/users/search', {
    params: {
      p: params.page,
      page_size: params.pageSize,
      keyword: params.keyword,
    },
  })
  return response.data
}

export async function changeUserAgent(payload: {
  downlineUserId: number
  targetAgentUserId: number
  remark: string
}) {
  const response = await api.post<AgentApiResponse<unknown>>(
    '/api/agent/downline/change',
    {
      downline_user_id: payload.downlineUserId,
      target_agent_user_id: payload.targetAgentUserId,
      remark: payload.remark,
    }
  )
  return response.data
}

export async function getAgentSelfSummary() {
  const response =
    await api.get<AgentApiResponse<AgentSelfSummary>>('/api/agent/self')
  return response.data
}

export async function getAgentSelfDailyMetrics(params: {
  startDate: string
  endDate: string
}) {
  const response = await api.get<AgentApiResponse<AgentDailyMetrics>>(
    '/api/agent/self/daily-metrics',
    { params: { start_date: params.startDate, end_date: params.endDate } }
  )
  return response.data
}

export async function getAgentSelfDownlines(params: {
  page: number
  pageSize: number
}) {
  const response = await api.get<
    AgentApiResponse<PagedAgentData<AgentDownlineUser>>
  >('/api/agent/self/downlines', {
    params: { p: params.page, page_size: params.pageSize },
  })
  return response.data
}

export async function getAgentSelfRebates(params: {
  page: number
  pageSize: number
}) {
  const response = await api.get<
    AgentApiResponse<PagedAgentData<AgentRebateRecord>>
  >('/api/agent/self/rebates', {
    params: { p: params.page, page_size: params.pageSize },
  })
  return response.data
}

export async function getAgentSelfAdjustments(params: {
  page: number
  pageSize: number
}) {
  const response = await api.get<
    AgentApiResponse<PagedAgentData<AgentAdjustment>>
  >('/api/agent/self/adjustments', {
    params: { p: params.page, page_size: params.pageSize },
  })
  return response.data
}
