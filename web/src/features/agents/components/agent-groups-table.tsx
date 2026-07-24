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
import { Pencil, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { useDeleteAgentRebateGroup } from '../hooks/use-agent-data'
import { formatAgentRate } from '../lib/format'
import type { AgentRebateGroup } from '../types'

interface AgentGroupsTableProps {
  groups: AgentRebateGroup[]
  loading: boolean
  onEdit: (group: AgentRebateGroup) => void
}

export function AgentGroupsTable(props: AgentGroupsTableProps) {
  const { t } = useTranslation()
  const [deleteTarget, setDeleteTarget] = useState<AgentRebateGroup>()
  const mutation = useDeleteAgentRebateGroup()

  if (props.loading) {
    return <Skeleton className='h-56 w-full' />
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      const response = await mutation.mutateAsync(deleteTarget.id)
      if (!response.success) {
        toast.error(response.message || t('Failed to delete rebate group'))
        return
      }
      toast.success(t('Rebate group deleted'))
      setDeleteTarget(undefined)
    } catch {
      toast.error(t('Failed to delete rebate group'))
    }
  }

  return (
    <>
      <StaticDataTable
        data={props.groups}
        getRowKey={(group) => group.id}
        emptyContent={t('No rebate groups found')}
        columns={[
          {
            id: 'name',
            header: t('Group name'),
            cell: (group) => (
              <div className='flex min-w-36 items-center gap-2'>
                <span className='font-medium'>{group.name}</span>
                {group.is_default && (
                  <StatusBadge
                    label={t('Default')}
                    variant='info'
                    copyable={false}
                  />
                )}
              </div>
            ),
          },
          {
            id: 'rate',
            header: t('Default rebate rate'),
            cell: (group) => formatAgentRate(group.rebate_rate),
          },
          {
            id: 'agents',
            header: t('Assigned agents'),
            cell: (group) => group.agent_count.toLocaleString(),
          },
          {
            id: 'status',
            header: t('Status'),
            cell: (group) => (
              <StatusBadge
                label={group.status === 1 ? t('Enabled') : t('Disabled')}
                variant={group.status === 1 ? 'success' : 'neutral'}
                copyable={false}
              />
            ),
          },
          {
            id: 'remark',
            header: t('Remark'),
            cell: (group) => group.remark || '-',
          },
          {
            id: 'actions',
            header: t('Actions'),
            cell: (group) => (
              <div className='flex items-center gap-2'>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        size='icon-sm'
                        variant='outline'
                        aria-label={t('Edit rebate group')}
                        onClick={() => props.onEdit(group)}
                      />
                    }
                  >
                    <Pencil />
                  </TooltipTrigger>
                  <TooltipContent>{t('Edit rebate group')}</TooltipContent>
                </Tooltip>
                {!group.is_default && (
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <Button
                          size='icon-sm'
                          variant='destructive'
                          aria-label={t('Delete rebate group')}
                          disabled={group.agent_count > 0}
                          onClick={() => setDeleteTarget(group)}
                        />
                      }
                    >
                      <Trash2 />
                    </TooltipTrigger>
                    <TooltipContent>
                      {group.agent_count > 0
                        ? t('This group is currently in use')
                        : t('Delete rebate group')}
                    </TooltipContent>
                  </Tooltip>
                )}
              </div>
            ),
          },
        ]}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(undefined)}
        title={t('Delete rebate group')}
        desc={t('This action cannot be undone. Agents must be moved first.')}
        destructive
        isLoading={mutation.isPending}
        confirmText={t('Delete')}
        handleConfirm={handleDelete}
      />
    </>
  )
}
