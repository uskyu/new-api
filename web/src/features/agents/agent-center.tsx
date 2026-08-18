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
import { CircleDollarSign, Percent, Snowflake, WalletCards } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  AgentAdjustmentsTable,
  AgentDownlinesTable,
  AgentRebatesTable,
} from './components/agent-center-tables'
import { AgentDailyChart } from './components/agent-daily-chart'
import { AgentLeaderboardCard } from './components/agent-leaderboard-card'
import { AgentMetricCards } from './components/agent-metric-cards'
import { AgentPromoLinksTable } from './components/agent-promo-links-table'
import { useAgentCenterQueries } from './hooks/use-agent-data'
import { formatAgentAmount, formatAgentRate } from './lib/format'

const PAGE_SIZE = 10

function parseLocalDate(value: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) return undefined
  const date = new Date(
    Number(match[1]),
    Number(match[2]) - 1,
    Number(match[3])
  )
  if (
    date.getFullYear() !== Number(match[1]) ||
    date.getMonth() !== Number(match[2]) - 1 ||
    date.getDate() !== Number(match[3])
  ) {
    return undefined
  }
  return date
}

function addOneCalendarMonthClamped(start: Date) {
  const nextMonth = new Date(start.getFullYear(), start.getMonth() + 1, 1)
  const daysInNextMonth = new Date(
    nextMonth.getFullYear(),
    nextMonth.getMonth() + 1,
    0
  ).getDate()
  const boundary = new Date(
    nextMonth.getFullYear(),
    nextMonth.getMonth(),
    Math.min(start.getDate(), daysInNextMonth)
  )
  if (start.getDate() > daysInNextMonth) {
    boundary.setDate(boundary.getDate() + 1)
  }
  return boundary
}

function validateLeaderboardDateRange(startValue: string, endValue: string) {
  const start = parseLocalDate(startValue)
  const end = parseLocalDate(endValue)
  if (!start || !end) return 'Select a valid date range'
  if (start > end) return 'Start date must not be after end date'

  const today = new Date()
  today.setHours(0, 0, 0, 0)
  if (end > today) return 'Dates cannot be in the future'

  const endExclusive = new Date(end)
  endExclusive.setDate(endExclusive.getDate() + 1)
  if (endExclusive > addOneCalendarMonthClamped(start)) {
    return 'The selected period cannot exceed one calendar month'
  }
  return undefined
}

export function AgentCenter() {
  const { t } = useTranslation()
  const initialStartDate = dayjs().subtract(6, 'day').format('YYYY-MM-DD')
  const initialEndDate = dayjs().format('YYYY-MM-DD')
  const [trendStartDate, setTrendStartDate] = useState(initialStartDate)
  const [trendEndDate, setTrendEndDate] = useState(initialEndDate)
  const [leaderboardStartDate, setLeaderboardStartDate] =
    useState(initialStartDate)
  const [leaderboardEndDate, setLeaderboardEndDate] = useState(initialEndDate)
  const [draftStartDate, setDraftStartDate] = useState(initialStartDate)
  const [draftEndDate, setDraftEndDate] = useState(initialEndDate)
  const [dateError, setDateError] = useState<string>()
  const [downlinePage, setDownlinePage] = useState(1)
  const [rebatePage, setRebatePage] = useState(1)
  const [adjustmentPage, setAdjustmentPage] = useState(1)
  const [promoLinkPage, setPromoLinkPage] = useState(1)
  const applyLeaderboardDates = () => {
    const error = validateLeaderboardDateRange(draftStartDate, draftEndDate)
    setDateError(error)
    if (error) return false
    setLeaderboardStartDate(draftStartDate)
    setLeaderboardEndDate(draftEndDate)
    return true
  }

  const updateDraftStartDate = (value: string) => {
    setDraftStartDate(value)
    if (dateError) {
      setDateError(validateLeaderboardDateRange(value, draftEndDate))
    }
  }

  const updateDraftEndDate = (value: string) => {
    setDraftEndDate(value)
    if (dateError) {
      setDateError(validateLeaderboardDateRange(draftStartDate, value))
    }
  }

  const queries = useAgentCenterQueries({
    startDate: trendStartDate,
    endDate: trendEndDate,
    downlinePage,
    rebatePage,
    adjustmentPage,
    promoLinkPage,
    pageSize: PAGE_SIZE,
  })

  if (queries.summary.isLoading) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Agent Center')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <Skeleton className='h-80 w-full' />
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  const summary = queries.summary.data?.data
  const profile = summary?.profile
  const moduleAvailable =
    summary?.agent_enabled === true && summary.agent_initialized === true
  const isAgent = summary?.is_agent === true && profile !== undefined

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Agent Center')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {!moduleAvailable && (
          <Alert>
            <AlertTitle>{t('Agent module is not available')}</AlertTitle>
            <AlertDescription>
              {t('The agent module has not been enabled by an administrator.')}
            </AlertDescription>
          </Alert>
        )}
        {moduleAvailable && !isAgent && (
          <Alert>
            <AlertTitle>{t('You are not an agent')}</AlertTitle>
            <AlertDescription>
              {t(
                'Agent data will appear here after your account becomes an agent.'
              )}
            </AlertDescription>
          </Alert>
        )}
        {moduleAvailable && isAgent && profile && (
          <div className='flex flex-col gap-4'>
            <AgentMetricCards
              items={[
                {
                  label: t('Available rebate'),
                  value: formatAgentAmount(profile.rebate_balance_amount),
                  icon: WalletCards,
                },
                {
                  label: t('Frozen rebate'),
                  value: formatAgentAmount(profile.rebate_frozen_amount),
                  icon: Snowflake,
                },
                {
                  label: t('Total rebate'),
                  value: formatAgentAmount(profile.rebate_total_amount),
                  icon: CircleDollarSign,
                },
                {
                  label: t('Rebate rate'),
                  value: formatAgentRate(profile.effective_rate),
                  icon: Percent,
                },
              ]}
            />

            <AgentLeaderboardCard
              startDate={leaderboardStartDate}
              endDate={leaderboardEndDate}
              draftStartDate={draftStartDate}
              draftEndDate={draftEndDate}
              onDraftStartDateChange={updateDraftStartDate}
              onDraftEndDateChange={updateDraftEndDate}
              onApplyDates={applyLeaderboardDates}
              dateError={dateError}
              enabled
            />

            <Card>
              <CardHeader>
                <CardTitle>{t('My promotion links')}</CardTitle>
              </CardHeader>
              <CardContent>
                <AgentPromoLinksTable
                  items={queries.promoLinks.data?.data?.items ?? []}
                  total={queries.promoLinks.data?.data?.total ?? 0}
                  page={promoLinkPage}
                  pageSize={PAGE_SIZE}
                  loading={queries.promoLinks.isLoading}
                  onPageChange={setPromoLinkPage}
                />
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>{t('Daily invitation and top-up trends')}</CardTitle>
              </CardHeader>
              <CardContent className='flex flex-col gap-4'>
                <div className='grid gap-3 sm:max-w-xl sm:grid-cols-2'>
                  <div className='flex flex-col gap-1.5'>
                    <Label htmlFor='agent-center-start'>
                      {t('Start date')}
                    </Label>
                    <Input
                      id='agent-center-start'
                      type='date'
                      value={trendStartDate}
                      max={trendEndDate}
                      onChange={(event) =>
                        setTrendStartDate(event.target.value)
                      }
                    />
                  </div>
                  <div className='flex flex-col gap-1.5'>
                    <Label htmlFor='agent-center-end'>{t('End date')}</Label>
                    <Input
                      id='agent-center-end'
                      type='date'
                      value={trendEndDate}
                      min={trendStartDate}
                      onChange={(event) => setTrendEndDate(event.target.value)}
                    />
                  </div>
                </div>
                <AgentDailyChart
                  items={queries.metrics.data?.data?.items ?? []}
                  loading={queries.metrics.isLoading}
                />
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>{t('Agent records')}</CardTitle>
              </CardHeader>
              <CardContent>
                <Tabs defaultValue='downlines'>
                  <TabsList className='max-w-full overflow-x-auto'>
                    <TabsTrigger value='downlines'>
                      {t('Invited users')}
                    </TabsTrigger>
                    <TabsTrigger value='rebates'>
                      {t('Rebate records')}
                    </TabsTrigger>
                    <TabsTrigger value='adjustments'>
                      {t('Balance adjustments')}
                    </TabsTrigger>
                  </TabsList>
                  <TabsContent value='downlines'>
                    <AgentDownlinesTable
                      items={queries.downlines.data?.data?.items ?? []}
                      total={queries.downlines.data?.data?.total ?? 0}
                      page={downlinePage}
                      pageSize={PAGE_SIZE}
                      loading={queries.downlines.isLoading}
                      onPageChange={setDownlinePage}
                    />
                  </TabsContent>
                  <TabsContent value='rebates'>
                    <AgentRebatesTable
                      items={queries.rebates.data?.data?.items ?? []}
                      total={queries.rebates.data?.data?.total ?? 0}
                      page={rebatePage}
                      pageSize={PAGE_SIZE}
                      loading={queries.rebates.isLoading}
                      onPageChange={setRebatePage}
                    />
                  </TabsContent>
                  <TabsContent value='adjustments'>
                    <AgentAdjustmentsTable
                      items={queries.adjustments.data?.data?.items ?? []}
                      total={queries.adjustments.data?.data?.total ?? 0}
                      page={adjustmentPage}
                      pageSize={PAGE_SIZE}
                      loading={queries.adjustments.isLoading}
                      onPageChange={setAdjustmentPage}
                    />
                  </TabsContent>
                </Tabs>
              </CardContent>
            </Card>
          </div>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
