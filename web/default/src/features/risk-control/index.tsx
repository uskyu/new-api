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

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { RefreshCw, Search, ShieldAlert, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { SectionPageLayout } from '@/components/layout'
import { formatTimestamp } from '@/lib/format'
import { getRiskInviters, getRiskOverview, getSharedIPs } from './api'
import type { RiskInviter, RiskSharedIP, RiskUser } from './types'

const PAGE_SIZE = 10

function UsersList({ users }: { users: RiskUser[] }) {
  return (
    <div className='flex min-w-0 flex-wrap gap-1'>
      {users.map((user) => (
        <Badge
          key={`${user.user_id}-${user.token_id}`}
          variant='outline'
          className='max-w-full font-normal'
        >
          <span className='truncate'>#{user.user_id} {user.username}</span>
        </Badge>
      ))}
    </div>
  )
}

function SourceBadge({ source }: { source: 'login' | 'token' | 'all' }) {
  const { t } = useTranslation()
  return <Badge variant='secondary'>{source === 'login' ? t('Login') : source === 'token' ? t('Token') : t('All Sources')}</Badge>
}

function SharedIPMobile({ items }: { items: RiskSharedIP[] }) {
  const { t } = useTranslation()
  return (
    <div className='grid gap-3 md:hidden'>
      {items.map((item) => (
        <div key={`${item.source}-${item.ip}`} className='rounded-lg border p-3'>
          <div className='flex items-start justify-between gap-3'>
            <span className='min-w-0 break-all font-mono text-sm font-medium'>{item.ip}</span>
            <SourceBadge source={item.source} />
          </div>
          <div className='mt-3 grid grid-cols-3 gap-2 text-center'>
            <Stat label={t('Users')} value={item.user_count} />
            <Stat label={t('Samples')} value={item.event_count} />
            <Stat label={t('Last Seen')} value={formatTimestamp(item.last_seen_at)} small />
          </div>
          <div className='mt-3 border-t pt-3'><UsersList users={item.users} /></div>
        </div>
      ))}
    </div>
  )
}

function InviterMobile({ items }: { items: RiskInviter[] }) {
  const { t } = useTranslation()
  return (
    <div className='grid gap-3 md:hidden'>
      {items.map((item) => (
        <div key={item.inviter_id} className='rounded-lg border p-3'>
          <div className='flex items-start justify-between gap-3'>
            <div className='min-w-0'>
              <div className='truncate font-medium'>#{item.inviter_id} {item.username}</div>
              {item.display_name && <div className='text-muted-foreground truncate text-xs'>{item.display_name}</div>}
            </div>
            <Badge variant={item.suspicious ? 'destructive' : 'secondary'}>
              {item.suspicious ? t('Review') : t('Normal')}
            </Badge>
          </div>
          <div className='mt-3 flex flex-wrap gap-2'>
            <Badge variant='outline'>{t('Direct Invites')}: {item.direct_invite_count}</Badge>
            <Badge variant={item.shared_ip_count ? 'destructive' : 'outline'}>
              {t('Shared IPs')}: {item.shared_ip_count}
            </Badge>
          </div>
          <div className='mt-3 border-t pt-3'><UsersList users={item.invitees} /></div>
        </div>
      ))}
    </div>
  )
}

function Stat({ label, value, small }: { label: string; value: string | number; small?: boolean }) {
  return (
    <div className='min-w-0'>
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className={small ? 'mt-1 truncate text-xs' : 'mt-1 font-medium tabular-nums'}>{value}</div>
    </div>
  )
}

export function RiskControl() {
  const { t } = useTranslation()
  const [tab, setTab] = useState<'shared' | 'inviters'>('shared')
  const [source, setSource] = useState('all')
  const [keyword, setKeyword] = useState('')
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)

  const overviewQuery = useQuery({
    queryKey: ['risk-control-overview'],
    queryFn: async () => (await getRiskOverview()).data,
  })
  const listQuery = useQuery({
    queryKey: ['risk-control-list', tab, source, search, page],
    queryFn: async () => {
      if (tab === 'shared') {
        return (await getSharedIPs({
          p: page,
          page_size: PAGE_SIZE,
          source: source === 'all' ? undefined : source,
          keyword: search || undefined,
          min_users: 2,
        })).data
      }
      return (await getRiskInviters({ p: page, page_size: PAGE_SIZE, keyword: search || undefined })).data
    },
  })

  const refresh = () => {
    overviewQuery.refetch()
    listQuery.refetch()
  }
  const submitSearch = () => {
    setPage(1)
    setSearch(keyword.trim())
  }
  const overview = overviewQuery.data
  const total = listQuery.data?.total || 0
  const sharedItems = tab === 'shared' ? (listQuery.data?.items || []) as RiskSharedIP[] : []
  const inviterItems = tab === 'inviters' ? (listQuery.data?.items || []) as RiskInviter[] : []
  const isLoading = overviewQuery.isFetching || listQuery.isFetching

  const stats = [
    [t('Tracked Users'), overview?.tracked_users || 0],
    [t('Shared IP Groups'), overview?.shared_ip_groups || 0],
    [t('Inviters'), overview?.inviter_count || 0],
    [t('Active Records (24h)'), overview?.active_records_24h || 0],
  ]

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Risk Control')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Review shared IP and invitation relationships without automatic enforcement')}
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-7xl flex-col gap-4'>
          <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
            {stats.map(([label, value]) => (
              <Card key={label}>
                <CardContent className='p-4'>
                  <div className='text-muted-foreground text-xs'>{label}</div>
                  <div className='mt-1 text-xl font-semibold tabular-nums'>{value}</div>
                </CardContent>
              </Card>
            ))}
          </div>

          <Card>
            <CardHeader className='gap-3'>
              <div className='flex flex-wrap items-center justify-between gap-3'>
                <CardTitle className='flex items-center gap-2 text-base'>
                  <ShieldAlert className='size-4' />{t('Risk Records')}
                </CardTitle>
                <Button variant='outline' size='sm' onClick={refresh} disabled={isLoading}>
                  <RefreshCw className={isLoading ? 'size-4 animate-spin' : 'size-4'} />{t('Refresh')}
                </Button>
              </div>
              <Tabs value={tab} onValueChange={(value) => { setTab(value as typeof tab); setPage(1) }}>
                <TabsList>
                  <TabsTrigger value='shared'><ShieldAlert className='size-4' />{t('Shared IP Users')}</TabsTrigger>
                  <TabsTrigger value='inviters'><Users className='size-4' />{t('Invitation Relations')}</TabsTrigger>
                </TabsList>
              </Tabs>
              <div className='flex flex-col gap-2 sm:flex-row'>
                {tab === 'shared' && (
                  <Select value={source} onValueChange={(value) => { setSource(value ?? 'all'); setPage(1) }}>
                    <SelectTrigger className='w-full sm:w-40'><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value='all'>{t('All Sources')}</SelectItem>
                      <SelectItem value='login'>{t('Login')}</SelectItem>
                      <SelectItem value='token'>{t('Token')}</SelectItem>
                    </SelectContent>
                  </Select>
                )}
                <div className='relative flex-1'>
                  <Search className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2' />
                  <Input className='pl-9' value={keyword} onChange={(event) => setKeyword(event.target.value)} onKeyDown={(event) => event.key === 'Enter' && submitSearch()} placeholder={tab === 'shared' ? t('Search IP') : t('Search inviter')} />
                </div>
                <Button onClick={submitSearch}>{t('Search')}</Button>
              </div>
            </CardHeader>
            <CardContent className='space-y-4'>
              {tab === 'shared' ? <SharedIPMobile items={sharedItems} /> : <InviterMobile items={inviterItems} />}
              <div className='hidden overflow-x-auto md:block'>
                {tab === 'shared' ? (
                  <Table>
                    <TableHeader><TableRow><TableHead>{t('IP Address')}</TableHead><TableHead>{t('Source')}</TableHead><TableHead>{t('Associated Users')}</TableHead><TableHead>{t('Users')}</TableHead><TableHead>{t('Samples')}</TableHead><TableHead>{t('Last Seen')}</TableHead></TableRow></TableHeader>
                    <TableBody>{sharedItems.map((item) => <TableRow key={`${item.source}-${item.ip}`}><TableCell className='font-mono'>{item.ip}</TableCell><TableCell><SourceBadge source={item.source} /></TableCell><TableCell><UsersList users={item.users} /></TableCell><TableCell>{item.user_count}</TableCell><TableCell>{item.event_count}</TableCell><TableCell>{formatTimestamp(item.last_seen_at)}</TableCell></TableRow>)}</TableBody>
                  </Table>
                ) : (
                  <Table>
                    <TableHeader><TableRow><TableHead>{t('Inviter')}</TableHead><TableHead>{t('Direct Invites')}</TableHead><TableHead>{t('Shared IPs')}</TableHead><TableHead>{t('Invitees')}</TableHead><TableHead>{t('Status')}</TableHead></TableRow></TableHeader>
                    <TableBody>{inviterItems.map((item) => <TableRow key={item.inviter_id}><TableCell className='font-medium'>#{item.inviter_id} {item.username}</TableCell><TableCell>{item.direct_invite_count}</TableCell><TableCell>{item.shared_ip_count}</TableCell><TableCell><UsersList users={item.invitees} /></TableCell><TableCell><Badge variant={item.suspicious ? 'destructive' : 'secondary'}>{item.suspicious ? t('Review') : t('Normal')}</Badge></TableCell></TableRow>)}</TableBody>
                  </Table>
                )}
              </div>
              {listQuery.data?.items?.length === 0 && <div className='text-muted-foreground py-10 text-center text-sm'>{t('No risk records found')}</div>}
              <div className='flex items-center justify-between gap-3 border-t pt-4'>
                <span className='text-muted-foreground text-sm'>{t('Total')}: {total}</span>
                <div className='flex gap-2'>
                  <Button variant='outline' size='sm' disabled={page <= 1} onClick={() => setPage((value) => value - 1)}>{t('Previous')}</Button>
                  <Button variant='outline' size='sm' disabled={page * PAGE_SIZE >= total} onClick={() => setPage((value) => value + 1)}>{t('Next')}</Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
