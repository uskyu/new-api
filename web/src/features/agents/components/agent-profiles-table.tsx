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
import { WalletCards } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

import { formatAgentAmount, formatAgentRate } from '../lib/format'
import type { AgentProfile } from '../types'
import { AgentBalanceDialog } from './agent-balance-dialog'

interface AgentProfilesTableProps {
  profiles: AgentProfile[]
  loading: boolean
  allowBalanceAdjustment: boolean
}

export function AgentProfilesTable(props: AgentProfilesTableProps) {
  const { t } = useTranslation()
  const [activeProfile, setActiveProfile] = useState<AgentProfile>()

  if (props.loading) {
    return <Skeleton className='h-72 w-full' />
  }

  return (
    <>
      <StaticDataTable
        data={props.profiles}
        getRowKey={(profile) => profile.user_id}
        emptyContent={t('No agents found')}
        columns={[
          {
            id: 'agent',
            header: t('Agent'),
            cell: (profile) => (
              <div className='flex min-w-40 flex-col'>
                <span className='font-medium'>
                  {profile.display_name || profile.username}
                </span>
                <span className='text-muted-foreground text-xs'>
                  #{profile.user_id} · {profile.username}
                </span>
              </div>
            ),
          },
          {
            id: 'upstream',
            header: t('Upstream Agent'),
            cell: (profile) =>
              profile.parent_agent_user_id ? (
                <span>
                  {profile.parent_agent_username || t('Agent')} (#
                  {profile.parent_agent_user_id})
                </span>
              ) : (
                <span className='text-muted-foreground'>
                  {t('No upstream agent')}
                </span>
              ),
          },
          {
            id: 'rate',
            header: t('Rebate rate'),
            cell: (profile) => formatAgentRate(profile.effective_rate),
          },
          {
            id: 'balance',
            header: t('Available / Frozen'),
            cell: (profile) => (
              <span className='tabular-nums'>
                {formatAgentAmount(profile.rebate_balance_amount)} /{' '}
                {formatAgentAmount(profile.rebate_frozen_amount)}
              </span>
            ),
          },
          {
            id: 'total',
            header: t('Total rebate'),
            cell: (profile) => formatAgentAmount(profile.rebate_total_amount),
          },
          {
            id: 'status',
            header: t('Status'),
            cell: (profile) => (
              <StatusBadge
                label={profile.status === 1 ? t('Enabled') : t('Disabled')}
                variant={profile.status === 1 ? 'success' : 'neutral'}
                copyable={false}
              />
            ),
          },
          {
            id: 'actions',
            header: t('Actions'),
            cell: (profile) =>
              props.allowBalanceAdjustment ? (
                <Button
                  size='sm'
                  variant='outline'
                  onClick={() => setActiveProfile(profile)}
                >
                  <WalletCards />
                  {t('Adjust balance')}
                </Button>
              ) : null,
          },
        ]}
      />

      {activeProfile && (
        <AgentBalanceDialog
          profile={activeProfile}
          open
          onOpenChange={(open) => !open && setActiveProfile(undefined)}
        />
      )}
    </>
  )
}
