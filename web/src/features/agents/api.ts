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
  AgentPromoLink,
  AgentRebateGroup,
  AgentRebateRecord,
  AgentSelfSummary,
  AgentWithdrawImportResult,
  AgentWithdrawRequest,
  AgentWithdrawStatus,
  PagedAgentData,
  SupportManagedUser,
} from './types'

export async function getAgentStatus() {
  const response =
    await api.get<AgentApiResponse<AgentBootstrapStatus>>('/api/agent/status')
  return response.data
}

export async function initializeAgentModule(defaultRebateRate: number) {
  const response = await api.post<AgentApiResponse<AgentBootstrapStatus>>(
    '/api/agent/init',
    { default_rebate_rate: defaultRebateRate }
  )
  return response.data
}

export async function getAgentRebateGroups() {
  const response =
    await api.get<AgentApiResponse<AgentRebateGroup[]>>('/api/agent/groups')
  return response.data
}

export async function upsertAgentProfile(payload: {
  userId: number
  status: number
  rebateGroupId: number
  customRate: number
  remark: string
}) {
  const response = await api.post<AgentApiResponse<AgentProfile>>(
    '/api/agent/profile',
    {
      user_id: payload.userId,
      status: payload.status,
      rebate_group_id: payload.rebateGroupId,
      custom_rate: payload.customRate,
      remark: payload.remark,
    }
  )
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

export async function getAgentDownlines(params: {
  agentUserId: number
  page: number
  pageSize: number
  keyword?: string
}) {
  const response = await api.get<
    AgentApiResponse<PagedAgentData<AgentDownlineUser>>
  >('/api/agent/downlines', {
    params: {
      agent_user_id: params.agentUserId,
      p: params.page,
      page_size: params.pageSize,
      keyword: params.keyword || undefined,
    },
  })
  return response.data
}

export async function assignAgentDownline(payload: {
  targetAgentUserId: number
  downlineUserId: number
  remark: string
}) {
  const response = await api.post<AgentApiResponse<unknown>>(
    '/api/agent/downline/assign',
    {
      target_agent_user_id: payload.targetAgentUserId,
      downline_user_id: payload.downlineUserId,
      remark: payload.remark,
    }
  )
  return response.data
}

export async function transferAgentDownline(payload: {
  sourceAgentUserId: number
  targetAgentUserId: number
  downlineUserId: number
  remark: string
}) {
  const response = await api.post<AgentApiResponse<unknown>>(
    '/api/agent/downline/transfer',
    {
      source_agent_user_id: payload.sourceAgentUserId,
      target_agent_user_id: payload.targetAgentUserId,
      downline_user_id: payload.downlineUserId,
      remark: payload.remark,
    }
  )
  return response.data
}

export async function getAgentWithdrawRequests(params: {
  page: number
  pageSize: number
  status?: AgentWithdrawStatus | ''
  startDate?: string
  endDate?: string
}) {
  const response = await api.get<
    AgentApiResponse<PagedAgentData<AgentWithdrawRequest>>
  >('/api/agent/withdraw-requests', {
    params: {
      p: params.page,
      page_size: params.pageSize,
      status: params.status || undefined,
      start_date: params.startDate || undefined,
      end_date: params.endDate || undefined,
    },
  })
  return response.data
}

export async function exportAgentWithdrawRequests(params: {
  status?: AgentWithdrawStatus | ''
  startDate?: string
  endDate?: string
}) {
  const response = await api.get<Blob>('/api/agent/withdraw-requests/export', {
    params: {
      status: params.status || undefined,
      start_date: params.startDate || undefined,
      end_date: params.endDate || undefined,
    },
    responseType: 'blob',
  })
  return response.data
}

export async function importAgentWithdrawResults(file: File) {
  const formData = new FormData()
  formData.append('file', file)
  const response = await api.post<AgentApiResponse<AgentWithdrawImportResult>>(
    '/api/agent/withdraw-requests/import',
    formData
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

export async function decreaseSupportUserQuota(payload: {
  userId: number
  quota: number
  reason: string
}) {
  const response = await api.post<
    AgentApiResponse<{
      user_id: number
      quota_delta: number
      quota_before: number
      quota_after: number
    }>
  >(`/api/support/users/${payload.userId}/quota/decrease`, {
    quota: payload.quota,
    reason: payload.reason,
  })
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

export async function getAgentSelfPromoLinks(params: {
  page: number
  pageSize: number
}) {
  const response = await api.get<
    AgentApiResponse<PagedAgentData<AgentPromoLink>>
  >('/api/agent/self/promo-links', {
    params: { p: params.page, page_size: params.pageSize },
  })
  return response.data
}
