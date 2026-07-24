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
import { useQuery } from '@tanstack/react-query'

import { getAdminAnalyticsOverview, getInactiveUserAnalytics } from '../api'
import type { AdminAnalyticsRangeKey, InactiveAccountType } from '../types'

export function useAdminAnalyticsOverview(params: {
  range: AdminAnalyticsRangeKey
  startTimestamp?: number
  endTimestamp?: number
}) {
  return useQuery({
    queryKey: ['dashboard', 'operations', 'overview', params],
    queryFn: () => getAdminAnalyticsOverview({ ...params, limit: 10 }),
    staleTime: 60_000,
    enabled:
      params.range !== 'custom' ||
      Boolean(params.startTimestamp && params.endTimestamp),
  })
}

export function useInactiveUserAnalytics(params: {
  days: number
  accountType: InactiveAccountType
  keyword: string
  page: number
  pageSize: number
}) {
  return useQuery({
    queryKey: ['dashboard', 'operations', 'inactive-users', params],
    queryFn: () => getInactiveUserAnalytics(params),
    staleTime: 60_000,
  })
}
