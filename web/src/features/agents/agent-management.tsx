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
import dayjs from 'dayjs'
import {
  CircleDollarSign,
  Database,
  ReceiptText,
  Search,
  UserPlus,
  UserRoundPlus,
  UsersRound,
  WalletCards,
} from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { AgentDailyChart } from './components/agent-daily-chart'
import { AgentDailyMetricsTable } from './components/agent-daily-metrics-table'
import { AgentDownlinesDialog } from './components/agent-downlines-dialog'
import { AgentInitializeDialog } from './components/agent-initialize-dialog'
import { AgentMetricCards } from './components/agent-metric-cards'
import { AgentPagination } from './components/agent-pagination'
import { AgentProfileDialog } from './components/agent-profile-dialog'
import { AgentProfilesTable } from './components/agent-profiles-table'
import { AgentWithdrawalsPanel } from './components/agent-withdrawals-panel'
import { AssignDownlineDialog } from './components/assign-downline-dialog'
import { SupportUsersTable } from './components/support-users-table'
import {
  useAgentAssignments,
  useAgentDailyMetrics,
  useAgentOverview,
  useAgentProfiles,
  useAgentStatus,
} from './hooks/use-agent-data'
import { formatAgentAmount } from './lib/format'
import type { AgentProfile } from './types'

const PAGE_SIZE = 10

export function AgentManagement() {
  const { t } = useTranslation()
  const currentUser = useAuthStore((state) => state.auth.user)
  const role = currentUser?.role ?? ROLE.GUEST
  const isAdmin = role >= ROLE.ADMIN
  const isRoot = role === ROLE.SUPER_ADMIN
  const [initializeOpen, setInitializeOpen] = useState(false)
  const [profileDialogOpen, setProfileDialogOpen] = useState(false)
  const [editingProfile, setEditingProfile] = useState<AgentProfile>()
  const [downlineProfile, setDownlineProfile] = useState<AgentProfile>()
  const [assignProfile, setAssignProfile] = useState<AgentProfile>()
  const [profilePage, setProfilePage] = useState(1)
  const [profileInput, setProfileInput] = useState('')
  const [profileKeyword, setProfileKeyword] = useState('')
  const [userPage, setUserPage] = useState(1)
  const [userInput, setUserInput] = useState('')
  const [userKeyword, setUserKeyword] = useState('')
  const [startDate, setStartDate] = useState(() =>
    dayjs().subtract(6, 'day').format('YYYY-MM-DD')
  )
  const [endDate, setEndDate] = useState(() => dayjs().format('YYYY-MM-DD'))
  const [agentIdInput, setAgentIdInput] = useState('')
  const [metricAgentId, setMetricAgentId] = useState<number>()

  const status = useAgentStatus()
  const ready =
    status.data?.data?.enabled === true &&
    status.data.data.initialized === true &&
    status.data.data.migration_ready === true
  const overview = useAgentOverview(isAdmin && ready)
  const profiles = useAgentProfiles({
    page: profilePage,
    pageSize: PAGE_SIZE,
    keyword: profileKeyword,
    enabled: ready,
  })
  const metrics = useAgentDailyMetrics({
    agentUserId: metricAgentId,
    startDate,
    endDate,
    enabled: isAdmin && ready,
  })
  const users = useAgentAssignments({
    page: userPage,
    pageSize: PAGE_SIZE,
    keyword: userKeyword,
    enabled: isAdmin && ready,
  })

  const handleProfileSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setProfilePage(1)
    setProfileKeyword(profileInput.trim())
  }

  const handleUserSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setUserPage(1)
    setUserKeyword(userInput.trim())
  }

  const handleMetricFilter = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const parsedAgentId = Number(agentIdInput)
    setMetricAgentId(parsedAgentId > 0 ? parsedAgentId : undefined)
  }

  const overviewData = overview.data?.data
  const metricSummary = metrics.data?.data?.summary
  const profileData = profiles.data?.data
  const userData = users.data?.data
  const bootstrapStatus = status.data?.data

  const openNewAgent = () => {
    setEditingProfile(undefined)
    setProfileDialogOpen(true)
  }

  const openEditAgent = (profile: AgentProfile) => {
    setEditingProfile(profile)
    setProfileDialogOpen(true)
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Agent Management')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {status.isLoading && (
          <div className='bg-muted h-32 animate-pulse rounded-lg' />
        )}
        {!status.isLoading && !ready && (
          <Alert>
            <AlertTitle>{t('Agent module is not ready')}</AlertTitle>
            <AlertDescription className='flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between'>
              <span>
                {isRoot
                  ? t('Initialize the module before managing agents.')
                  : t(
                      'Ask a super administrator to initialize the agent module.'
                    )}
              </span>
              {isRoot && (
                <Button size='sm' onClick={() => setInitializeOpen(true)}>
                  <Database />
                  {t('Initialize')}
                </Button>
              )}
            </AlertDescription>
          </Alert>
        )}
        {!status.isLoading && ready && (
          <Tabs defaultValue={isAdmin ? 'overview' : 'balances'}>
            {isAdmin && (
              <TabsList className='max-w-full overflow-x-auto'>
                <TabsTrigger value='overview'>{t('Overview')}</TabsTrigger>
                <TabsTrigger value='assignments'>
                  {t('User assignments')}
                </TabsTrigger>
                <TabsTrigger value='balances'>
                  {t('Agent balances')}
                </TabsTrigger>
                <TabsTrigger value='withdrawals'>
                  {t('Withdrawal receipts')}
                </TabsTrigger>
              </TabsList>
            )}

            {isAdmin && (
              <TabsContent value='overview' className='flex flex-col gap-4'>
                <AgentMetricCards
                  loading={overview.isLoading}
                  items={[
                    {
                      label: t('Agents'),
                      value: overviewData?.agent_count ?? 0,
                      icon: UsersRound,
                    },
                    {
                      label: t('Invited users'),
                      value: overviewData?.downline_user_count ?? 0,
                      icon: UserRoundPlus,
                    },
                    {
                      label: t('Available rebate'),
                      value: formatAgentAmount(
                        overviewData?.rebate_balance_amount
                      ),
                      icon: WalletCards,
                    },
                    {
                      label: t('Total rebate'),
                      value: formatAgentAmount(
                        overviewData?.rebate_total_amount
                      ),
                      icon: CircleDollarSign,
                    },
                  ]}
                />

                <Card>
                  <CardHeader>
                    <CardTitle>{t('Daily agent metrics')}</CardTitle>
                  </CardHeader>
                  <CardContent className='flex flex-col gap-4'>
                    <form
                      className='grid gap-3 sm:grid-cols-2 xl:grid-cols-[1fr_1fr_1fr_auto]'
                      onSubmit={handleMetricFilter}
                    >
                      <div className='flex flex-col gap-1.5'>
                        <Label htmlFor='agent-metric-start'>
                          {t('Start date')}
                        </Label>
                        <Input
                          id='agent-metric-start'
                          type='date'
                          value={startDate}
                          max={endDate}
                          onChange={(event) => setStartDate(event.target.value)}
                        />
                      </div>
                      <div className='flex flex-col gap-1.5'>
                        <Label htmlFor='agent-metric-end'>
                          {t('End date')}
                        </Label>
                        <Input
                          id='agent-metric-end'
                          type='date'
                          value={endDate}
                          min={startDate}
                          onChange={(event) => setEndDate(event.target.value)}
                        />
                      </div>
                      <div className='flex flex-col gap-1.5'>
                        <Label htmlFor='agent-metric-id'>{t('Agent ID')}</Label>
                        <Input
                          id='agent-metric-id'
                          inputMode='numeric'
                          value={agentIdInput}
                          onChange={(event) =>
                            setAgentIdInput(event.target.value)
                          }
                          placeholder={t('All agents')}
                        />
                      </div>
                      <Button type='submit' className='self-end'>
                        <Search data-icon='inline-start' />
                        {t('Apply')}
                      </Button>
                    </form>

                    <AgentMetricCards
                      loading={metrics.isLoading}
                      items={[
                        {
                          label: t('New users'),
                          value: metricSummary?.new_user_count ?? 0,
                          icon: UserRoundPlus,
                        },
                        {
                          label: t('Top-ups'),
                          value: metricSummary?.topup_count ?? 0,
                          icon: WalletCards,
                        },
                        {
                          label: t('Top-up amount'),
                          value: formatAgentAmount(metricSummary?.topup_amount),
                          icon: CircleDollarSign,
                        },
                        {
                          label: t('Rebate amount'),
                          value: formatAgentAmount(
                            metricSummary?.total_rebate_amount
                          ),
                          icon: CircleDollarSign,
                        },
                      ]}
                    />
                    <AgentDailyChart
                      items={metrics.data?.data?.items ?? []}
                      loading={metrics.isLoading}
                    />
                    <AgentDailyMetricsTable
                      items={metrics.data?.data?.items ?? []}
                      loading={metrics.isLoading}
                    />
                  </CardContent>
                </Card>
              </TabsContent>
            )}

            {isAdmin && (
              <TabsContent value='assignments'>
                <Card>
                  <CardHeader>
                    <CardTitle>
                      {t('Search and change upstream agents')}
                    </CardTitle>
                  </CardHeader>
                  <CardContent className='flex flex-col gap-4'>
                    <form className='flex gap-2' onSubmit={handleUserSearch}>
                      <Input
                        value={userInput}
                        onChange={(event) => setUserInput(event.target.value)}
                        placeholder={t('Search users by ID or name')}
                        aria-label={t('Search users')}
                      />
                      <Button type='submit'>
                        <Search data-icon='inline-start' />
                        {t('Search')}
                      </Button>
                    </form>
                    <SupportUsersTable
                      users={userData?.items ?? []}
                      loading={users.isLoading}
                      searched
                      allowQuotaDecrease={false}
                    />
                    <AgentPagination
                      page={userPage}
                      pageSize={PAGE_SIZE}
                      total={userData?.total ?? 0}
                      onPageChange={setUserPage}
                    />
                  </CardContent>
                </Card>
              </TabsContent>
            )}

            <TabsContent value='balances'>
              <Card>
                <CardHeader className='flex flex-row items-center justify-between gap-3'>
                  <CardTitle>{t('Agent balances')}</CardTitle>
                  {isAdmin && (
                    <Button size='sm' onClick={openNewAgent}>
                      <UserPlus />
                      {t('Add agent')}
                    </Button>
                  )}
                </CardHeader>
                <CardContent className='flex flex-col gap-4'>
                  <form className='flex gap-2' onSubmit={handleProfileSearch}>
                    <Input
                      value={profileInput}
                      onChange={(event) => setProfileInput(event.target.value)}
                      placeholder={t('Search agents by ID or name')}
                      aria-label={t('Search agents')}
                    />
                    <Button type='submit'>
                      <Search data-icon='inline-start' />
                      {t('Search')}
                    </Button>
                  </form>
                  <AgentProfilesTable
                    profiles={profileData?.items ?? []}
                    loading={profiles.isLoading}
                    allowBalanceAdjustment
                    allowProfileEditing={isAdmin}
                    showFinancialDetails={isAdmin}
                    showAvailableBalance={!isAdmin}
                    onEditProfile={openEditAgent}
                    onViewDownlines={setDownlineProfile}
                    onAssignDownline={setAssignProfile}
                    assignDisabledUserId={isAdmin ? undefined : currentUser?.id}
                  />
                  <AgentPagination
                    page={profilePage}
                    pageSize={PAGE_SIZE}
                    total={profileData?.total ?? 0}
                    onPageChange={setProfilePage}
                  />
                </CardContent>
              </Card>
            </TabsContent>

            {isAdmin && (
              <TabsContent value='withdrawals'>
                <Card>
                  <CardHeader>
                    <CardTitle className='flex items-center gap-2'>
                      <ReceiptText className='size-5' />
                      {t('Withdrawal receipts')}
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <AgentWithdrawalsPanel enabled={ready} />
                  </CardContent>
                </Card>
              </TabsContent>
            )}
          </Tabs>
        )}
        <AgentInitializeDialog
          open={initializeOpen}
          defaultRate={bootstrapStatus?.default_rate ?? 1000}
          onOpenChange={setInitializeOpen}
        />
        <AgentProfileDialog
          open={profileDialogOpen}
          profile={editingProfile}
          defaultGroupId={bootstrapStatus?.default_group_id ?? 0}
          onOpenChange={setProfileDialogOpen}
        />
        {downlineProfile && (
          <AgentDownlinesDialog
            profile={downlineProfile}
            isAdmin={isAdmin}
            open
            onOpenChange={(open) => !open && setDownlineProfile(undefined)}
          />
        )}
        {assignProfile && (
          <AssignDownlineDialog
            profile={assignProfile}
            open
            onOpenChange={(open) => !open && setAssignProfile(undefined)}
          />
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
