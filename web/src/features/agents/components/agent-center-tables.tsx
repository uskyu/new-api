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
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota } from '@/lib/format'

import { formatAgentAmount, formatAgentRate } from '../lib/format'
import type {
  AgentAdjustment,
  AgentDownlineUser,
  AgentRebateRecord,
} from '../types'
import { AgentPagination } from './agent-pagination'

interface PagedTableProps {
  page: number
  pageSize: number
  total: number
  loading: boolean
  onPageChange: (page: number) => void
}

export function AgentDownlinesTable(
  props: PagedTableProps & { items: AgentDownlineUser[] }
) {
  const { t } = useTranslation()

  if (props.loading) return <Skeleton className='h-64 w-full' />

  return (
    <div className='flex flex-col gap-3'>
      <StaticDataTable
        data={props.items}
        getRowKey={(item) => item.user_id}
        emptyContent={t('No invited users found')}
        columns={[
          {
            id: 'user',
            header: t('User'),
            cell: (item) => (
              <div className='flex min-w-36 flex-col'>
                <span>{item.display_name || item.username}</span>
                <span className='text-muted-foreground text-xs'>
                  #{item.user_id} · {item.username}
                </span>
              </div>
            ),
          },
          {
            id: 'source',
            header: t('Promotion link'),
            cell: (item) => item.promo_link_name || '-',
          },
          {
            id: 'topups',
            header: t('Top-ups'),
            cell: (item) => item.topup_count.toLocaleString(),
          },
          {
            id: 'amount',
            header: t('Top-up amount'),
            cell: (item) => formatAgentAmount(item.topup_amount),
          },
          {
            id: 'rebate',
            header: t('Rebate amount'),
            cell: (item) => formatAgentAmount(item.rebate_amount),
          },
        ]}
      />
      <AgentPagination {...props} />
    </div>
  )
}

export function AgentRebatesTable(
  props: PagedTableProps & { items: AgentRebateRecord[] }
) {
  const { t } = useTranslation()

  if (props.loading) return <Skeleton className='h-64 w-full' />

  return (
    <div className='flex flex-col gap-3'>
      <StaticDataTable
        data={props.items}
        getRowKey={(item) => item.id}
        emptyContent={t('No rebate records found')}
        columns={[
          {
            id: 'trade',
            header: t('Order number'),
            cell: (item) => item.trade_no,
          },
          {
            id: 'source',
            header: t('Source'),
            cell: (item) => item.source_type,
          },
          {
            id: 'user',
            header: t('Invited user ID'),
            cell: (item) => `#${item.invitee_user_id}`,
          },
          {
            id: 'quota',
            header: t('Credited quota'),
            cell: (item) => formatQuota(item.redeem_quota),
          },
          {
            id: 'rate',
            header: t('Rebate rate'),
            cell: (item) => formatAgentRate(item.rebate_rate),
          },
          {
            id: 'amount',
            header: t('Rebate amount'),
            cell: (item) => formatAgentAmount(item.rebate_amount),
          },
          {
            id: 'status',
            header: t('Status'),
            cell: (item) => (
              <StatusBadge
                label={t(item.status === 'settled' ? 'Settled' : 'Pending')}
                variant={item.status === 'settled' ? 'success' : 'neutral'}
                copyable={false}
              />
            ),
          },
        ]}
      />
      <AgentPagination {...props} />
    </div>
  )
}

export function AgentAdjustmentsTable(
  props: PagedTableProps & { items: AgentAdjustment[] }
) {
  const { t } = useTranslation()

  if (props.loading) return <Skeleton className='h-64 w-full' />

  return (
    <div className='flex flex-col gap-3'>
      <StaticDataTable
        data={props.items}
        getRowKey={(item) => item.id}
        emptyContent={t('No balance adjustments found')}
        columns={[
          {
            id: 'change',
            header: t('Change'),
            cell: (item) => formatAgentAmount(item.delta_amount),
          },
          {
            id: 'before',
            header: t('Balance before'),
            cell: (item) => formatAgentAmount(item.balance_before),
          },
          {
            id: 'after',
            header: t('Balance after'),
            cell: (item) => formatAgentAmount(item.balance_after),
          },
          {
            id: 'reason',
            header: t('Reason'),
            cell: (item) => item.reason,
          },
          {
            id: 'time',
            header: t('Created At'),
            cell: (item) => new Date(item.created_at * 1000).toLocaleString(),
          },
        ]}
      />
      <AgentPagination {...props} />
    </div>
  )
}
