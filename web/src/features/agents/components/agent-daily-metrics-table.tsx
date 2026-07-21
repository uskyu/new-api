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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { Skeleton } from '@/components/ui/skeleton'

import { formatAgentAmount } from '../lib/format'
import type { AgentDailyMetric } from '../types'
import { AgentPagination } from './agent-pagination'

const PAGE_SIZE = 10

interface AgentDailyMetricsTableProps {
  items: AgentDailyMetric[]
  loading: boolean
}

export function AgentDailyMetricsTable(props: AgentDailyMetricsTableProps) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)

  useEffect(() => setPage(1), [props.items])

  if (props.loading) {
    return <Skeleton className='h-64 w-full' />
  }

  const pageItems = props.items.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

  return (
    <div className='flex flex-col gap-3'>
      <StaticDataTable
        data={pageItems}
        getRowKey={(item) => `${item.date}-${item.agent_user_id}`}
        emptyContent={t('No daily metrics found')}
        columns={[
          {
            id: 'date',
            header: t('Date'),
            cell: (item) => item.date,
          },
          {
            id: 'agent',
            header: t('Agent'),
            cell: (item) => (
              <div className='flex min-w-36 flex-col'>
                <span>{item.display_name || item.username}</span>
                <span className='text-muted-foreground text-xs'>
                  #{item.agent_user_id}
                </span>
              </div>
            ),
          },
          {
            id: 'new-users',
            header: t('New users'),
            cell: (item) => item.new_user_count.toLocaleString(),
          },
          {
            id: 'topups',
            header: t('Top-ups'),
            cell: (item) => item.topup_count.toLocaleString(),
          },
          {
            id: 'topup-amount',
            header: t('Top-up amount'),
            cell: (item) => formatAgentAmount(item.topup_amount),
          },
          {
            id: 'rebate',
            header: t('Rebate amount'),
            cell: (item) => formatAgentAmount(item.total_rebate_amount),
          },
        ]}
      />
      <AgentPagination
        page={page}
        pageSize={PAGE_SIZE}
        total={props.items.length}
        onPageChange={setPage}
      />
    </div>
  )
}
