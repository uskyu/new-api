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
import { getRouteApi } from '@tanstack/react-router'
import type {
  ColumnDef,
  OnChangeFn,
  PaginationState,
} from '@tanstack/react-table'
import { useCallback, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { SectionPageLayout } from '@/components/layout'
import { createServerError } from '@/lib/server-error-message'

import { fetchLedger } from './api'
import { LedgerFilterBar } from './components/ledger-filter-bar'
import { useLedgerColumns } from './lib/columns'
import { DEFAULT_LEDGER_PAGE_SIZE } from './lib/constants'
import type { LedgerRow, LedgerStatus } from './types'

const route = getRouteApi('/_authenticated/harness-ledger/')

export function HarnessLedger() {
  const { t } = useTranslation()
  const search = route.useSearch()
  const navigate = route.useNavigate()

  const page = search.page ?? 1
  const pageSize = search.page_size ?? DEFAULT_LEDGER_PAGE_SIZE
  const statuses = useMemo<LedgerStatus[]>(
    () => (search.status ?? []) as LedgerStatus[],
    [search.status]
  )

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'harness-ledger',
      page,
      pageSize,
      search.request_id ?? '',
      search.model_name ?? '',
      statuses.join(','),
      search.start_time ?? 0,
      search.end_time ?? 0,
    ],
    queryFn: async () => {
      const result = await fetchLedger({
        page,
        pageSize,
        requestId: search.request_id,
        modelName: search.model_name,
        statuses,
        startTime: search.start_time,
        endTime: search.end_time,
      })
      if (!result.success) {
        throw createServerError(result, t('Failed to load the ledger'))
      }
      return result.data
    },
  })

  const rows = useMemo<LedgerRow[]>(() => data?.items ?? [], [data])
  const columns = useLedgerColumns()
  const tableColumns = columns as unknown as ColumnDef<
    Record<string, unknown>,
    unknown
  >[]

  const setSearch = useCallback(
    (patch: Record<string, unknown>) => {
      void navigate({
        search: (previous) =>
          ({
            ...previous,
            ...patch,
            // Any filter change returns to the first page, because the previous
            // page may not exist under the new filter.
            page: 1,
          }) as never,
      })
    },
    [navigate]
  )

  const pagination = useMemo<PaginationState>(
    () => ({ pageIndex: page - 1, pageSize }),
    [page, pageSize]
  )

  const onPaginationChange = useCallback<OnChangeFn<PaginationState>>(
    (updater) => {
      const next = typeof updater === 'function' ? updater(pagination) : updater
      void navigate({
        search: (previous) =>
          ({
            ...previous,
            page: next.pageIndex + 1,
            page_size: next.pageSize,
          }) as never,
      })
    },
    [navigate, pagination]
  )

  const { table } = useDataTable({
    data: rows as unknown as Record<string, unknown>[],
    columns: tableColumns,
    pagination,
    onPaginationChange,
    manualPagination: true,
    enableRowSelection: false,
    totalCount: data?.total ?? 0,
  })

  return (
    // The page shell must be SectionPageLayout: DataTablePage renders its pager
    // through the footer portal that shell provides, so mounting the table directly
    // as the page root silently drops the pagination controls.
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('Ledger')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <DataTablePage
          table={table}
          columns={tableColumns}
          isLoading={isLoading || (isFetching && !data)}
          isFetching={isFetching}
          emptyTitle={t('No ledger records found')}
          emptyDescription={t(
            'The ledger records every request the gateway billed. Records appear once traffic reaches the gateway.'
          )}
          skeletonKeyPrefix='harness-ledger-skeleton'
          toolbar={
            <LedgerFilterBar
              search={search}
              onChange={setSearch}
              total={data?.total ?? 0}
            />
          }
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
