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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { useIsAdmin } from '@/hooks/use-admin'

import {
  completeOrder,
  getAllBillingHistory,
  getRedemptionBillingHistory,
  getUserBillingHistory,
  getUserBillingHistoryByAdmin,
  isApiSuccess,
} from '../api'
import type { BillingHistoryScope, BillingRecordType } from '../types'

interface UseBillingHistoryOptions {
  scope?: BillingHistoryScope
  userId?: number
  enabled?: boolean
  initialPageSize?: number
}

export function useBillingHistory(options: UseBillingHistoryOptions = {}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isAdmin = useIsAdmin()
  const scope = options.scope ?? (isAdmin ? 'all' : 'self')
  const [recordType, setRecordType] = useState<BillingRecordType>('online')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(options.initialPageSize ?? 10)
  const [keyword, setKeyword] = useState('')
  const queryKey = [
    'billing-history',
    scope,
    options.userId,
    recordType,
    page,
    pageSize,
    keyword,
  ]

  const history = useQuery({
    queryKey,
    enabled:
      (options.enabled ?? true) &&
      (scope !== 'user' || (options.userId ?? 0) > 0),
    queryFn: async () => {
      if (recordType === 'redemption') {
        return getRedemptionBillingHistory(
          scope,
          page,
          pageSize,
          keyword,
          options.userId
        )
      }
      if (scope === 'all') {
        return getAllBillingHistory(page, pageSize, keyword)
      }
      if (scope === 'user' && options.userId) {
        return getUserBillingHistoryByAdmin(
          options.userId,
          page,
          pageSize,
          keyword
        )
      }
      return getUserBillingHistory(page, pageSize, keyword)
    },
  })

  const completion = useMutation({
    mutationFn: completeOrder,
    onSuccess: async (response) => {
      if (!isApiSuccess(response)) {
        toast.error(response.message || t('Failed to complete order'))
        return
      }
      toast.success(t('Order completed successfully'))
      await queryClient.invalidateQueries({
        queryKey: ['billing-history'],
      })
    },
    onError: () => toast.error(t('Failed to complete order')),
  })

  const handleRecordTypeChange = (value: BillingRecordType) => {
    setRecordType(value)
    setPage(1)
    setKeyword('')
  }

  const handlePageSizeChange = (value: number) => {
    setPageSize(value)
    setPage(1)
  }

  const handleSearch = (value: string) => {
    setKeyword(value)
    setPage(1)
  }

  return {
    response: history.data,
    loading: history.isLoading,
    recordType,
    page,
    pageSize,
    keyword,
    scope,
    isAdmin,
    completing: completion.isPending,
    handleRecordTypeChange,
    handlePageChange: setPage,
    handlePageSizeChange,
    handleSearch,
    handleCompleteOrder: async (tradeNo: string) => {
      const response = await completion.mutateAsync({ trade_no: tradeNo })
      return isApiSuccess(response)
    },
  }
}
