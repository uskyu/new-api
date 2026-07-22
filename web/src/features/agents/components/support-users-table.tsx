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
import { CircleMinus, Repeat2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota } from '@/lib/format'

import type { SupportManagedUser } from '../types'
import { ChangeAgentDialog } from './change-agent-dialog'
import { SupportQuotaDialog } from './support-quota-dialog'

interface SupportUsersTableProps {
  users: SupportManagedUser[]
  loading: boolean
  searched: boolean
}

export function SupportUsersTable(props: SupportUsersTableProps) {
  const { t } = useTranslation()
  const [activeUser, setActiveUser] = useState<SupportManagedUser>()
  const [quotaUser, setQuotaUser] = useState<SupportManagedUser>()

  if (props.loading) {
    return <Skeleton className='h-64 w-full' />
  }

  return (
    <>
      <StaticDataTable
        data={props.users}
        getRowKey={(user) => user.id}
        emptyContent={props.searched ? t('No users found') : t('Search users')}
        columns={[
          {
            id: 'user',
            header: t('User'),
            cell: (user) => (
              <div className='flex min-w-40 flex-col'>
                <span className='font-medium'>
                  {user.display_name || user.username}
                </span>
                <span className='text-muted-foreground text-xs'>
                  #{user.id} / {user.username}
                </span>
              </div>
            ),
          },
          {
            id: 'identity',
            header: t('Identity'),
            cell: (user) =>
              user.is_agent ? (
                <StatusBadge
                  label={t('Agent')}
                  variant='info'
                  copyable={false}
                />
              ) : (
                <StatusBadge
                  label={t('User')}
                  variant='neutral'
                  copyable={false}
                />
              ),
          },
          {
            id: 'quota',
            header: t('Quota'),
            cell: (user) => formatQuota(user.quota),
          },
          {
            id: 'upstream',
            header: t('Upstream Agent'),
            cell: (user) =>
              user.inviter_id > 0
                ? `${user.inviter_display_name || user.inviter_username || t('Agent')} (#${user.inviter_id})`
                : t('No upstream agent'),
          },
          {
            id: 'actions',
            header: t('Actions'),
            cell: (user) => (
              <div className='flex items-center gap-2'>
                <Button
                  size='sm'
                  variant='outline'
                  onClick={() => setActiveUser(user)}
                >
                  <Repeat2 />
                  {t('Change')}
                </Button>
                {!user.is_agent && (
                  <Button
                    size='sm'
                    variant='outline'
                    onClick={() => setQuotaUser(user)}
                  >
                    <CircleMinus />
                    {t('Decrease balance')}
                  </Button>
                )}
              </div>
            ),
          },
        ]}
      />

      {activeUser && (
        <ChangeAgentDialog
          user={activeUser}
          open
          onOpenChange={(open) => !open && setActiveUser(undefined)}
        />
      )}

      {quotaUser && (
        <SupportQuotaDialog
          user={quotaUser}
          open
          onOpenChange={(open) => !open && setQuotaUser(undefined)}
        />
      )}
    </>
  )
}
