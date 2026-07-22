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
import { Repeat2, Search } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'

import { useAgentDownlines } from '../hooks/use-agent-data'
import { formatAgentAmount } from '../lib/format'
import type { AgentDownlineUser, AgentProfile } from '../types'
import { AgentPagination } from './agent-pagination'
import { ChangeAgentDialog } from './change-agent-dialog'

const PAGE_SIZE = 10

interface AgentDownlinesDialogProps {
  profile: AgentProfile
  isAdmin: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function AgentDownlinesDialog(props: AgentDownlinesDialogProps) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [input, setInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [activeDownline, setActiveDownline] = useState<AgentDownlineUser>()
  const downlines = useAgentDownlines({
    agentUserId: props.profile.user_id,
    page,
    pageSize: PAGE_SIZE,
    keyword,
    enabled: props.open,
  })
  const data = downlines.data?.data

  const handleSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPage(1)
    setKeyword(input.trim())
  }

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      setPage(1)
      setInput('')
      setKeyword('')
      setActiveDownline(undefined)
    }
    props.onOpenChange(open)
  }

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={handleOpenChange}
        title={t('Agent downlines')}
        description={`${props.profile.display_name || props.profile.username} (#${props.profile.user_id})`}
        contentClassName='sm:max-w-6xl'
        contentHeight='min(70vh, 680px)'
      >
        <div className='flex flex-col gap-4'>
          <form className='flex gap-2' onSubmit={handleSearch}>
            <Input
              value={input}
              onChange={(event) => setInput(event.target.value)}
              placeholder={t('Search users by ID or name')}
            />
            <Button type='submit'>
              <Search />
              {t('Search')}
            </Button>
          </form>
          {downlines.isLoading ? (
            <Skeleton className='h-72 w-full' />
          ) : (
            <StaticDataTable
              data={data?.items ?? []}
              getRowKey={(item) => item.user_id}
              emptyContent={t('No invited users found')}
              columns={[
                {
                  id: 'user',
                  header: t('User'),
                  cell: (item) => (
                    <div className='flex min-w-40 items-center gap-2'>
                      <div className='flex min-w-0 flex-col'>
                        <span className='truncate font-medium'>
                          {item.display_name || item.username}
                        </span>
                        <span className='text-muted-foreground text-xs'>
                          #{item.user_id} / {item.username}
                        </span>
                      </div>
                      {item.is_agent && (
                        <StatusBadge
                          label={t('Agent')}
                          variant='info'
                          copyable={false}
                        />
                      )}
                    </div>
                  ),
                },
                {
                  id: 'promo',
                  header: t('Promotion link'),
                  cell: (item) => item.promo_link_name || '-',
                },
                ...(props.isAdmin
                  ? [
                      {
                        id: 'topups',
                        header: t('Top-ups'),
                        cell: (item: AgentDownlineUser) =>
                          item.topup_count.toLocaleString(),
                      },
                      {
                        id: 'amount',
                        header: t('Top-up amount'),
                        cell: (item: AgentDownlineUser) =>
                          formatAgentAmount(item.topup_amount),
                      },
                      {
                        id: 'rebate',
                        header: t('Rebate amount'),
                        cell: (item: AgentDownlineUser) =>
                          formatAgentAmount(item.rebate_amount),
                      },
                    ]
                  : []),
                {
                  id: 'actions',
                  header: t('Actions'),
                  cell: (item) => (
                    <Button
                      size='sm'
                      variant='outline'
                      onClick={() => setActiveDownline(item)}
                    >
                      <Repeat2 />
                      {item.is_agent
                        ? t('Change agent parent')
                        : t('Transfer user')}
                    </Button>
                  ),
                },
              ]}
            />
          )}
          <AgentPagination
            page={page}
            pageSize={PAGE_SIZE}
            total={data?.total ?? 0}
            onPageChange={setPage}
          />
        </div>
      </Dialog>

      {activeDownline && (
        <ChangeAgentDialog
          user={{
            id: activeDownline.user_id,
            username: activeDownline.username,
            display_name: activeDownline.display_name,
            inviter_id: activeDownline.inviter_id,
          }}
          sourceAgentUserId={props.profile.user_id}
          userIsAgent={activeDownline.is_agent}
          open
          onOpenChange={(open) => !open && setActiveDownline(undefined)}
        />
      )}
    </>
  )
}
