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
  Search,
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
import { AgentMetricCards } from './components/agent-metric-cards'
import { AgentPagination } from './components/agent-pagination'
import { AgentProfilesTable } from './components/agent-profiles-table'
import { SupportUsersTable } from './components/support-users-table'
import {
  useAgentDailyMetrics,
  useAgentOverview,
  useAgentProfiles,
  useAgentStatus,
  useSupportUsers,
} from './hooks/use-agent-data'
import { formatAgentAmount } from './lib/format'

const PAGE_SIZE = 10

export function AgentManagement() {
  const { t } = useTranslation()
  const role = useAuthStore((state) => state.auth.user?.role ?? ROLE.GUEST)
  const isAdmin = role >= ROLE.ADMIN
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
  const users = useSupportUsers({
    page: userPage,
    pageSize: PAGE_SIZE,
    keyword: userKeyword,
    enabled: ready && userKeyword.length > 0,
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

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Agent Management')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {!status.isLoading && !ready ? (
          <Alert>
            <AlertTitle>{t('Agent module is not ready')}</AlertTitle>
            <AlertDescription>
              {t('Ask a super administrator to initialize the agent module.')}
            </AlertDescription>
          </Alert>
        ) : (
          <Tabs defaultValue={isAdmin ? 'overview' : 'assignments'}>
            <TabsList className='max-w-full overflow-x-auto'>
              {isAdmin && (
                <TabsTrigger value='overview'>{t('Overview')}</TabsTrigger>
              )}
              <TabsTrigger value='assignments'>
                {t('User assignments')}
              </TabsTrigger>
              <TabsTrigger value='balances'>{t('Agent balances')}</TabsTrigger>
            </TabsList>

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
                    <Button type='submit' disabled={!userInput.trim()}>
                      <Search data-icon='inline-start' />
                      {t('Search')}
                    </Button>
                  </form>
                  <SupportUsersTable
                    users={userData?.items ?? []}
                    loading={users.isLoading}
                    searched={userKeyword.length > 0}
                  />
                  {userKeyword && (
                    <AgentPagination
                      page={userPage}
                      pageSize={PAGE_SIZE}
                      total={userData?.total ?? 0}
                      onPageChange={setUserPage}
                    />
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value='balances'>
              <Card>
                <CardHeader>
                  <CardTitle>{t('Agent balances')}</CardTitle>
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
          </Tabs>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
