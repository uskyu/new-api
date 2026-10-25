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
import type { ColumnDef } from '@tanstack/react-table'

import { StatusBadge } from '@/components/status-badge'
import { formatTimestamp } from '@/lib/format'
import { useTranslation } from 'react-i18next'

import { LEDGER_STATUS_PRESENTATION } from './constants'
import type { LedgerRow } from '../types'

// A null token count means the gateway never measured it. Rendering it as 0 would
// claim a measurement that was never made, so the cell states the unknown case in
// words instead.
export function useLedgerColumns(): ColumnDef<LedgerRow, unknown>[] {
  const { t } = useTranslation()

  return [
    {
      accessorKey: 'created_at',
      header: t('Time'),
      size: 150,
      cell: ({ row }) => (
        <span className='text-muted-foreground whitespace-nowrap text-xs tabular-nums'>
          {formatTimestamp(row.original.created_at)}
        </span>
      ),
    },
    {
      accessorKey: 'request_id',
      header: t('Request ID'),
      size: 260,
      cell: ({ row }) => (
        <div className='flex flex-col gap-0.5'>
          <span className='font-mono text-xs'>{row.original.request_id}</span>
          <span className='text-muted-foreground font-mono text-[11px]'>
            {row.original.harness_request_id || t('Internal request')}
          </span>
        </div>
      ),
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      size: 110,
      cell: ({ row }) => {
        const presentation = LEDGER_STATUS_PRESENTATION[row.original.status]
        return (
          <StatusBadge
            label={presentation ? t(presentation.labelKey) : row.original.status}
            variant={presentation?.variant ?? 'neutral'}
            size='sm'
            copyable={false}
          />
        )
      },
    },
    {
      accessorKey: 'model',
      header: t('Model'),
      size: 160,
      cell: ({ row }) => (
        <div className='flex flex-col gap-0.5'>
          <span className='text-xs'>{row.original.model}</span>
          {row.original.pricing_group ? (
            <span className='text-muted-foreground text-[11px]'>
              {row.original.pricing_group}
            </span>
          ) : null}
        </div>
      ),
    },
    {
      id: 'tokens',
      header: t('Tokens (in / out)'),
      size: 150,
      cell: ({ row }) => {
        const usage = row.original.usage
        if (!usage.split_known) {
          // Only a total was reported, so the split is unknown and the total is the
          // only honest figure to show.
          return (
            <span className='text-xs tabular-nums'>
              {usage.total_tokens != null
                ? t('{{count}} total', { count: usage.total_tokens })
                : t('Not measured')}
            </span>
          )
        }
        return (
          <span className='text-xs tabular-nums'>
            {usage.input_tokens ?? '-'} / {usage.output_tokens ?? '-'}
          </span>
        )
      },
    },
    {
      id: 'billing',
      header: t('Procurement cost'),
      size: 200,
      cell: ({ row }) => {
        const billing = row.original.billing
        return (
          <div className='flex flex-col gap-0.5'>
            <span className='text-xs font-medium tabular-nums'>
              {billing.total_amount} {billing.currency}
            </span>
            <span className='text-muted-foreground text-[11px] tabular-nums'>
              {billing.base_amount} x {billing.group_ratio} +{' '}
              {billing.tax_rate_percent}%
            </span>
          </div>
        )
      },
    },
  ]
}
