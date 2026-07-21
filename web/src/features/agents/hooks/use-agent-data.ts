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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  adjustAgentBalance,
  changeUserAgent,
  getAgentDailyMetrics,
  getAgentOverview,
  getAgentProfiles,
  getAgentSelfAdjustments,
  getAgentSelfDailyMetrics,
  getAgentSelfDownlines,
  getAgentSelfRebates,
  getAgentSelfSummary,
  getAgentStatus,
  searchSupportUsers,
} from '../api'

export function useAgentStatus() {
  return useQuery({
    queryKey: ['agents', 'status'],
    queryFn: getAgentStatus,
  })
}

export function useAgentOverview(enabled: boolean) {
  return useQuery({
    queryKey: ['agents', 'overview'],
    queryFn: getAgentOverview,
    enabled,
  })
}

export function useAgentProfiles(params: {
  page: number
  pageSize: number
  keyword?: string
  enabled?: boolean
}) {
  return useQuery({
    queryKey: ['agents', 'profiles', params],
    queryFn: () => getAgentProfiles(params),
    enabled: params.enabled ?? true,
  })
}

export function useAgentDailyMetrics(params: {
  agentUserId?: number
  startDate: string
  endDate: string
  enabled: boolean
}) {
  return useQuery({
    queryKey: ['agents', 'daily-metrics', params],
    queryFn: () => getAgentDailyMetrics(params),
    enabled: params.enabled,
  })
}

export function useSupportUsers(params: {
  page: number
  pageSize: number
  keyword: string
  enabled: boolean
}) {
  return useQuery({
    queryKey: ['agents', 'support-users', params],
    queryFn: () => searchSupportUsers(params),
    enabled: params.enabled,
  })
}

export function useAdjustAgentBalance() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: adjustAgentBalance,
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['agents', 'profiles'] }),
        queryClient.invalidateQueries({ queryKey: ['agents', 'overview'] }),
        queryClient.invalidateQueries({ queryKey: ['agents', 'self'] }),
      ])
    },
  })
}

export function useChangeUserAgent() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: changeUserAgent,
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['agents', 'support-users'],
        }),
        queryClient.invalidateQueries({ queryKey: ['users'] }),
        queryClient.invalidateQueries({ queryKey: ['agents', 'profiles'] }),
      ])
    },
  })
}

export function useAgentCenterQueries(params: {
  startDate: string
  endDate: string
  downlinePage: number
  rebatePage: number
  adjustmentPage: number
  pageSize: number
}) {
  const summary = useQuery({
    queryKey: ['agents', 'self', 'summary'],
    queryFn: getAgentSelfSummary,
  })
  const enabled = summary.data?.data?.is_agent === true

  const metrics = useQuery({
    queryKey: [
      'agents',
      'self',
      'daily-metrics',
      params.startDate,
      params.endDate,
    ],
    queryFn: () => getAgentSelfDailyMetrics(params),
    enabled,
  })
  const downlines = useQuery({
    queryKey: [
      'agents',
      'self',
      'downlines',
      params.downlinePage,
      params.pageSize,
    ],
    queryFn: () =>
      getAgentSelfDownlines({
        page: params.downlinePage,
        pageSize: params.pageSize,
      }),
    enabled,
  })
  const rebates = useQuery({
    queryKey: ['agents', 'self', 'rebates', params.rebatePage, params.pageSize],
    queryFn: () =>
      getAgentSelfRebates({
        page: params.rebatePage,
        pageSize: params.pageSize,
      }),
    enabled,
  })
  const adjustments = useQuery({
    queryKey: [
      'agents',
      'self',
      'adjustments',
      params.adjustmentPage,
      params.pageSize,
    ],
    queryFn: () =>
      getAgentSelfAdjustments({
        page: params.adjustmentPage,
        pageSize: params.pageSize,
      }),
    enabled,
  })

  return { summary, metrics, downlines, rebates, adjustments }
}
