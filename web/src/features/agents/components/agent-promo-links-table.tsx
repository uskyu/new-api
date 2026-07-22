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
import { Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StaticDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { generateAffiliateLink } from '@/features/wallet/lib/affiliate'

import type { AgentPromoLink } from '../types'
import { AgentPagination } from './agent-pagination'

interface AgentPromoLinksTableProps {
  items: AgentPromoLink[]
  total: number
  page: number
  pageSize: number
  loading: boolean
  onPageChange: (page: number) => void
}

export function AgentPromoLinksTable(props: AgentPromoLinksTableProps) {
  const { t } = useTranslation()

  const copyLink = async (code: string) => {
    try {
      await navigator.clipboard.writeText(generateAffiliateLink(code))
      toast.success(t('Invitation link copied'))
    } catch {
      toast.error(t('Failed to copy invitation link'))
    }
  }

  if (props.loading) {
    return <Skeleton className='h-52 w-full' />
  }

  return (
    <div className='flex flex-col gap-3'>
      <StaticDataTable
        data={props.items}
        getRowKey={(link) => link.id}
        emptyContent={t('No promotion links found')}
        columns={[
          {
            id: 'name',
            header: t('Name'),
            cell: (link) => <span className='font-medium'>{link.name}</span>,
          },
          {
            id: 'code',
            header: t('Invitation code'),
            cell: (link) => (
              <span className='font-mono text-xs'>{link.code}</span>
            ),
          },
          {
            id: 'status',
            header: t('Status'),
            cell: (link) => (
              <StatusBadge
                label={link.status === 1 ? t('Enabled') : t('Disabled')}
                variant={link.status === 1 ? 'success' : 'neutral'}
                copyable={false}
              />
            ),
          },
          {
            id: 'actions',
            header: t('Actions'),
            cell: (link) => (
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      size='icon-sm'
                      variant='outline'
                      aria-label={t('Copy invitation link')}
                      onClick={() => copyLink(link.code)}
                    />
                  }
                >
                  <Copy />
                </TooltipTrigger>
                <TooltipContent>{t('Copy invitation link')}</TooltipContent>
              </Tooltip>
            ),
          },
        ]}
      />
      <AgentPagination
        page={props.page}
        pageSize={props.pageSize}
        total={props.total}
        onPageChange={props.onPageChange}
      />
    </div>
  )
}
