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
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  CircleDollarSign,
  RefreshCw,
  Repeat2,
  Search,
  UserCheck,
  Users,
  WalletCards,
} from 'lucide-react'
import { useMemo, useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip as RechartsTooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { StaticDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import dayjs from '@/lib/dayjs'
import { formatNumber, formatQuota, formatTimestampToDate } from '@/lib/format'

import {
  useAdminAnalyticsOverview,
  useInactiveUserAnalytics,
} from '../../hooks/use-operations-analytics'
import type {
  AdminAnalyticsOverview,
  AdminAnalyticsRangeKey,
  InactiveAccountType,
  InactiveUserRow,
} from '../../types'
import { parseInactiveDays } from './inactive-days'

const RANGE_OPTIONS: Array<{
  value: AdminAnalyticsRangeKey
  label: string
}> = [
  { value: 'today', label: 'Today' },
  { value: 'yesterday', label: 'Yesterday' },
  { value: '7d', label: 'Last 7 days' },
  { value: '30d', label: 'Last 30 days' },
  { value: '90d', label: 'Last 90 days' },
  { value: 'custom', label: 'Custom' },
]

const CHART_COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
]

const INACTIVE_PAGE_SIZE = 10

const USER_TREND_LABELS: Record<string, string> = {
  newUsers: 'New users',
  activeUsers: 'Active users',
  calls: 'Calls',
}

const BALANCE_BUCKET_LABELS: Record<string, string> = {
  '<=0': 'No balance',
  '0-1 unit': 'Up to 1 unit',
  '1-10 units': '1 to 10 units',
  '10-100 units': '10 to 100 units',
  '>100 units': 'Over 100 units',
}

function ChartState(props: {
  loading: boolean
  empty: boolean
  children: ReactNode
}) {
  const { t } = useTranslation()
  if (props.loading) return <Skeleton className='h-full w-full' />
  if (props.empty) {
    return (
      <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
        {t('No Data')}
      </div>
    )
  }
  return props.children
}

function OperationsTrendCharts(props: {
  overview?: AdminAnalyticsOverview
  loading: boolean
}) {
  const { t } = useTranslation()
  const data = useMemo(
    () =>
      (props.overview?.trends ?? []).map((item) => ({
        time:
          (props.overview?.range.step ?? 0) < 24 * 3600
            ? dayjs.unix(item.start).format('MM-DD HH:mm')
            : dayjs.unix(item.start).format('MM-DD'),
        newUsers: item.new_user_count,
        activeUsers: item.active_user_count,
        calls: item.call_count,
        topup: item.topup_amount,
        consume: item.consume_quota,
      })),
    [props.overview]
  )

  return (
    <div className='grid gap-3 xl:grid-cols-2'>
      <Card>
        <CardHeader>
          <CardTitle>{t('User growth and activity')}</CardTitle>
        </CardHeader>
        <CardContent className='h-72 px-1 sm:px-4'>
          <ChartState loading={props.loading} empty={data.length === 0}>
            <ResponsiveContainer width='100%' height='100%'>
              <LineChart data={data} margin={{ top: 8, right: 8, left: 0 }}>
                <CartesianGrid strokeDasharray='3 3' vertical={false} />
                <XAxis dataKey='time' tickLine={false} axisLine={false} />
                <YAxis yAxisId='users' allowDecimals={false} width={42} />
                <YAxis
                  yAxisId='calls'
                  orientation='right'
                  tickFormatter={(value: number) => formatNumber(value)}
                  width={56}
                />
                <RechartsTooltip
                  formatter={(value, name) => [
                    formatNumber(Number(value)),
                    t(USER_TREND_LABELS[String(name)] ?? 'Calls'),
                  ]}
                />
                <Legend
                  formatter={(value) =>
                    t(USER_TREND_LABELS[String(value)] ?? 'Calls')
                  }
                />
                <Line
                  yAxisId='users'
                  dataKey='newUsers'
                  stroke='var(--chart-1)'
                  strokeWidth={2}
                  dot={false}
                />
                <Line
                  yAxisId='users'
                  dataKey='activeUsers'
                  stroke='var(--chart-2)'
                  strokeWidth={2}
                  dot={false}
                />
                <Line
                  yAxisId='calls'
                  dataKey='calls'
                  stroke='var(--chart-4)'
                  strokeWidth={2}
                  dot={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </ChartState>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('Top-up and consumption trend')}</CardTitle>
        </CardHeader>
        <CardContent className='h-72 px-1 sm:px-4'>
          <ChartState loading={props.loading} empty={data.length === 0}>
            <ResponsiveContainer width='100%' height='100%'>
              <LineChart data={data} margin={{ top: 8, right: 8, left: 0 }}>
                <CartesianGrid strokeDasharray='3 3' vertical={false} />
                <XAxis dataKey='time' tickLine={false} axisLine={false} />
                <YAxis
                  yAxisId='topup'
                  tickFormatter={(value: number) =>
                    formatBillingCurrencyFromUSD(value, { compact: true })
                  }
                  width={62}
                />
                <YAxis
                  yAxisId='consume'
                  orientation='right'
                  tickFormatter={(value: number) => formatQuota(value)}
                  width={70}
                />
                <RechartsTooltip
                  formatter={(value, name) =>
                    name === 'topup'
                      ? [
                          formatBillingCurrencyFromUSD(Number(value)),
                          t('Successful top-up amount'),
                        ]
                      : [formatQuota(Number(value)), t('Consumed quota')]
                  }
                />
                <Legend
                  formatter={(value) =>
                    value === 'topup'
                      ? t('Successful top-up amount')
                      : t('Consumed quota')
                  }
                />
                <Line
                  yAxisId='topup'
                  dataKey='topup'
                  stroke='var(--chart-3)'
                  strokeWidth={2}
                  dot={false}
                />
                <Line
                  yAxisId='consume'
                  dataKey='consume'
                  stroke='var(--chart-5)'
                  strokeWidth={2}
                  dot={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </ChartState>
        </CardContent>
      </Card>
    </div>
  )
}

function OperationsDistributionCharts(props: {
  overview?: AdminAnalyticsOverview
  loading: boolean
}) {
  const { t } = useTranslation()
  const payments = props.overview?.distributions.payment_method ?? []
  const balances = (props.overview?.distributions.balance_buckets ?? []).map(
    (item) => ({
      ...item,
      displayLabel: t(BALANCE_BUCKET_LABELS[item.label] ?? 'Over 100 units'),
    })
  )

  return (
    <div className='grid gap-3 xl:grid-cols-2'>
      <Card>
        <CardHeader>
          <CardTitle>{t('Payment method distribution')}</CardTitle>
        </CardHeader>
        <CardContent className='h-72'>
          <ChartState loading={props.loading} empty={payments.length === 0}>
            <ResponsiveContainer width='100%' height='100%'>
              <PieChart>
                <Pie
                  data={payments}
                  dataKey='topup_amount'
                  nameKey='payment_method'
                  innerRadius='48%'
                  outerRadius='76%'
                  paddingAngle={1}
                >
                  {payments.map((item, index) => (
                    <Cell
                      key={item.payment_method}
                      fill={CHART_COLORS[index % CHART_COLORS.length]}
                    />
                  ))}
                </Pie>
                <RechartsTooltip
                  formatter={(value) =>
                    formatBillingCurrencyFromUSD(Number(value))
                  }
                />
                <Legend />
              </PieChart>
            </ResponsiveContainer>
          </ChartState>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('User balance distribution')}</CardTitle>
        </CardHeader>
        <CardContent className='h-72 px-1 sm:px-4'>
          <ChartState loading={props.loading} empty={balances.length === 0}>
            <ResponsiveContainer width='100%' height='100%'>
              <BarChart data={balances} margin={{ top: 8, right: 8, left: 0 }}>
                <CartesianGrid strokeDasharray='3 3' vertical={false} />
                <XAxis
                  dataKey='displayLabel'
                  tickLine={false}
                  axisLine={false}
                  interval={0}
                  tick={{ fontSize: 11 }}
                />
                <YAxis allowDecimals={false} width={42} />
                <RechartsTooltip
                  formatter={(value) => [
                    formatNumber(Number(value)),
                    t('Users'),
                  ]}
                />
                <Bar
                  dataKey='user_count'
                  fill='var(--chart-2)'
                  radius={[4, 4, 0, 0]}
                />
              </BarChart>
            </ResponsiveContainer>
          </ChartState>
        </CardContent>
      </Card>
    </div>
  )
}

function ModelUsageRankingChart(props: {
  overview?: AdminAnalyticsOverview
  loading: boolean
}) {
  const { t } = useTranslation()
  const data = props.overview?.rankings.model_usage ?? []

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Model usage ranking')}</CardTitle>
      </CardHeader>
      <CardContent className='h-80 px-1 sm:h-96 sm:px-4'>
        <ChartState loading={props.loading} empty={data.length === 0}>
          <ResponsiveContainer width='100%' height='100%'>
            <BarChart
              data={data}
              layout='vertical'
              margin={{ top: 12, right: 12, bottom: 8, left: 8 }}
            >
              <CartesianGrid strokeDasharray='3 3' horizontal={false} />
              <XAxis
                xAxisId='quota'
                type='number'
                tickFormatter={(value: number) => formatQuota(value)}
                tickLine={false}
              />
              <XAxis
                xAxisId='calls'
                type='number'
                orientation='top'
                allowDecimals={false}
                tickFormatter={(value: number) => formatNumber(value)}
                tickLine={false}
              />
              <YAxis
                type='category'
                dataKey='model_name'
                width={132}
                tickLine={false}
                axisLine={false}
              />
              <RechartsTooltip
                formatter={(value, name) =>
                  name === 'consume_quota'
                    ? [formatQuota(Number(value)), t('Consumed quota')]
                    : [formatNumber(Number(value)), t('Calls')]
                }
              />
              <Legend
                formatter={(value) =>
                  value === 'consume_quota'
                    ? t('Consumed quota')
                    : t('Calls')
                }
              />
              <Bar
                xAxisId='quota'
                dataKey='consume_quota'
                fill='var(--chart-1)'
                radius={[0, 4, 4, 0]}
              />
              <Bar
                xAxisId='calls'
                dataKey='call_count'
                fill='var(--chart-4)'
                radius={[0, 4, 4, 0]}
              />
            </BarChart>
          </ResponsiveContainer>
        </ChartState>
      </CardContent>
    </Card>
  )
}

function OperationsRankings(props: {
  overview?: AdminAnalyticsOverview
  loading: boolean
}) {
  const { t } = useTranslation()
  const [ranking, setRanking] = useState<'topups' | 'consumption' | 'agents'>(
    'topups'
  )
  const rows = useMemo(() => {
    if (ranking === 'consumption') {
      return (props.overview?.rankings.user_consumptions ?? []).map(
        (item, index) => ({
          rank: index + 1,
          id: item.user_id,
          name: item.display_name || item.username || `#${item.user_id}`,
          primary: formatQuota(item.consume_quota),
          secondary: `${t('Calls')}: ${formatNumber(item.call_count)}`,
        })
      )
    }
    if (ranking === 'agents') {
      return (props.overview?.rankings.agent_contribution ?? []).map(
        (item, index) => ({
          rank: index + 1,
          id: item.agent_user_id,
          name: item.display_name || item.username || `#${item.agent_user_id}`,
          primary: formatBillingCurrencyFromUSD(item.pay_amount / 100),
          secondary: `${t('Rebate')}: ${formatBillingCurrencyFromUSD(item.rebate_amount / 100)}`,
        })
      )
    }
    return (props.overview?.rankings.user_topups ?? []).map((item, index) => ({
      rank: index + 1,
      id: item.user_id,
      name: item.display_name || item.username || `#${item.user_id}`,
      primary: formatBillingCurrencyFromUSD(item.topup_amount),
      secondary: `${t('Orders')}: ${formatNumber(item.topup_count)}`,
    }))
  }, [props.overview, ranking, t])

  return (
    <Card>
      <CardHeader className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <CardTitle>{t('Operational rankings')}</CardTitle>
        <Tabs
          value={ranking}
          onValueChange={(value) =>
            setRanking(value as 'topups' | 'consumption' | 'agents')
          }
        >
          <TabsList className='max-w-full overflow-x-auto'>
            <TabsTrigger value='topups'>{t('Top-up users')}</TabsTrigger>
            <TabsTrigger value='consumption'>
              {t('Consumption users')}
            </TabsTrigger>
            <TabsTrigger value='agents'>{t('Agent contribution')}</TabsTrigger>
          </TabsList>
        </Tabs>
      </CardHeader>
      <CardContent>
        {props.loading ? (
          <Skeleton className='h-64 w-full' />
        ) : (
          <StaticDataTable
            data={rows}
            getRowKey={(row) => `${ranking}-${row.id}`}
            emptyContent={t('No Data')}
            columns={[
              {
                id: 'rank',
                header: t('Rank'),
                cell: (row) => (
                  <StatusBadge
                    label={`#${row.rank}`}
                    variant={row.rank <= 3 ? 'info' : 'neutral'}
                    copyable={false}
                  />
                ),
              },
              {
                id: 'name',
                header: t('User'),
                cell: (row) => (
                  <div className='min-w-40'>
                    <div className='font-medium'>{row.name}</div>
                    <div className='text-muted-foreground text-xs'>
                      ID: {row.id}
                    </div>
                  </div>
                ),
              },
              {
                id: 'primary',
                header: t('Primary metric'),
                cell: (row) => (
                  <span className='font-medium'>{row.primary}</span>
                ),
              },
              {
                id: 'secondary',
                header: t('Additional metric'),
                cell: (row) => row.secondary,
              },
            ]}
          />
        )}
      </CardContent>
    </Card>
  )
}

function InactiveUsersPanel() {
  const { t } = useTranslation()
  const [days, setDays] = useState(30)
  const [dayMode, setDayMode] = useState('30')
  const [customDays, setCustomDays] = useState('120')
  const [accountType, setAccountType] = useState<InactiveAccountType>('all')
  const [searchInput, setSearchInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const query = useInactiveUserAnalytics({
    days,
    accountType,
    keyword,
    page,
    pageSize: INACTIVE_PAGE_SIZE,
  })
  const result = query.data?.data
  const pageCount = Math.max(
    1,
    Math.ceil((result?.total ?? 0) / INACTIVE_PAGE_SIZE)
  )
  const parsedCustomDays = parseInactiveDays(customDays)

  const handleSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPage(1)
    setKeyword(searchInput.trim())
  }
  const handleDaysChange = (value: string) => {
    setDayMode(value)
    if (value === 'custom') return
    setPage(1)
    setDays(Number(value))
  }
  const handleApplyCustomDays = () => {
    if (parsedCustomDays == null) return
    setPage(1)
    setDays(parsedCustomDays)
  }
  const handleAccountTypeChange = (value: string | null) => {
    if (!value) return
    setPage(1)
    setAccountType(value as InactiveAccountType)
  }

  return (
    <Card>
      <CardHeader className='gap-3'>
        <CardTitle>{t('Zombie users')}</CardTitle>
        <div className='flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between'>
          <div className='flex flex-wrap items-end gap-2'>
            <Tabs value={dayMode} onValueChange={handleDaysChange}>
              <TabsList className='max-w-full overflow-x-auto'>
                {[7, 30, 60, 90].map((value) => (
                  <TabsTrigger key={value} value={String(value)}>
                    {t('{{count}} days', { count: value })}
                  </TabsTrigger>
                ))}
                <TabsTrigger value='custom'>{t('Custom')}</TabsTrigger>
              </TabsList>
            </Tabs>
            {dayMode === 'custom' && (
              <div className='flex items-end gap-2'>
                <div className='flex flex-col gap-1'>
                  <Label htmlFor='inactive-custom-days'>
                    {t('Custom days')}
                  </Label>
                  <Input
                    id='inactive-custom-days'
                    type='number'
                    min={1}
                    max={3650}
                    value={customDays}
                    onChange={(event) => setCustomDays(event.target.value)}
                    className='w-28'
                  />
                </div>
                <Button
                  type='button'
                  variant='outline'
                  disabled={parsedCustomDays == null || query.isFetching}
                  onClick={handleApplyCustomDays}
                >
                  {t('Apply')}
                </Button>
              </div>
            )}
          </div>
          <form
            className='grid gap-2 sm:grid-cols-[10rem_minmax(12rem,1fr)_auto]'
            onSubmit={handleSearch}
          >
            <Select
              items={[
                { value: 'all', label: t('All accounts') },
                { value: 'users', label: t('Regular users') },
                { value: 'agents', label: t('Agents') },
              ]}
              value={accountType}
              onValueChange={handleAccountTypeChange}
            >
              <SelectTrigger aria-label={t('Account type')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value='all'>{t('All accounts')}</SelectItem>
                  <SelectItem value='users'>{t('Regular users')}</SelectItem>
                  <SelectItem value='agents'>{t('Agents')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
            <Input
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              placeholder={t('Search users by ID or name')}
              aria-label={t('Search users')}
            />
            <Button type='submit'>
              <Search data-icon='inline-start' />
              {t('Search')}
            </Button>
          </form>
        </div>
      </CardHeader>
      <CardContent className='flex flex-col gap-4'>
        {query.isError && (
          <div className='border-destructive/40 bg-destructive/5 text-destructive rounded-lg border px-4 py-3 text-sm'>
            {t('Failed to load inactive users')}
          </div>
        )}
        <div className='grid grid-cols-2 overflow-hidden rounded-lg border md:grid-cols-4'>
          {[
            {
              label: t('Inactive users'),
              value: formatNumber(result?.summary.inactive_user_count ?? 0),
            },
            {
              label: t('Inactive ratio'),
              value: Intl.NumberFormat(undefined, {
                style: 'percent',
                maximumFractionDigits: 1,
              }).format(result?.summary.inactive_ratio ?? 0),
            },
            {
              label: t('Balance held by inactive users'),
              value: formatQuota(result?.summary.inactive_balance_quota ?? 0),
            },
            {
              label: t('Never logged in'),
              value: formatNumber(result?.summary.never_logged_in_count ?? 0),
            },
          ].map((item) => (
            <div
              key={item.label}
              className='border-border/60 min-w-0 border-b p-3 even:border-l md:border-b-0 md:border-l md:first:border-l-0'
            >
              <div className='text-muted-foreground truncate text-xs'>
                {item.label}
              </div>
              <div className='mt-1 truncate text-lg font-semibold'>
                {item.value}
              </div>
            </div>
          ))}
        </div>

        {query.isLoading ? (
          <Skeleton className='h-72 w-full' />
        ) : (
          <StaticDataTable<InactiveUserRow>
            data={result?.items ?? []}
            getRowKey={(row) => row.id}
            emptyContent={t('No inactive users found')}
            columns={[
              {
                id: 'user',
                header: t('User'),
                cell: (row) => (
                  <div className='min-w-40'>
                    <div className='font-medium'>
                      {row.display_name || row.username}
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      {row.username} · ID: {row.id}
                    </div>
                  </div>
                ),
              },
              {
                id: 'type',
                header: t('Account type'),
                cell: (row) => (
                  <StatusBadge
                    label={row.is_agent ? t('Agent') : t('Regular user')}
                    variant={row.is_agent ? 'info' : 'neutral'}
                    copyable={false}
                  />
                ),
              },
              {
                id: 'balance',
                header: t('Current balance'),
                cell: (row) => formatQuota(row.quota),
              },
              {
                id: 'last-login',
                header: t('Last login'),
                cell: (row) =>
                  row.last_login_at > 0
                    ? formatTimestampToDate(row.last_login_at)
                    : t('Never logged in'),
              },
              {
                id: 'inactive-days',
                header: t('Inactive days'),
                cell: (row) =>
                  t('{{count}} days', { count: row.inactive_days }),
              },
              {
                id: 'created',
                header: t('Registration time'),
                cell: (row) => formatTimestampToDate(row.created_at),
              },
              {
                id: 'inviter',
                header: t('Upstream agent'),
                cell: (row) =>
                  row.inviter_id > 0
                    ? `${row.inviter_username || '-'} (#${row.inviter_id})`
                    : '-',
              },
            ]}
          />
        )}

        <div className='flex flex-wrap items-center justify-between gap-2'>
          <span className='text-muted-foreground text-sm'>
            {t('{{count}} records', { count: result?.total ?? 0 })}
          </span>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='icon-sm'
              aria-label={t('Previous page')}
              disabled={page <= 1 || query.isFetching}
              onClick={() => setPage((current) => Math.max(1, current - 1))}
            >
              <ChevronLeft />
            </Button>
            <span className='min-w-20 text-center text-sm'>
              {page} / {pageCount}
            </span>
            <Button
              variant='outline'
              size='icon-sm'
              aria-label={t('Next page')}
              disabled={page >= pageCount || query.isFetching}
              onClick={() => setPage((current) => current + 1)}
            >
              <ChevronRight />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function OperationsOverview() {
  const { t } = useTranslation()
  const [range, setRange] = useState<AdminAnalyticsRangeKey>('7d')
  const [startDate, setStartDate] = useState(() =>
    dayjs().subtract(6, 'day').format('YYYY-MM-DD')
  )
  const [endDate, setEndDate] = useState(() => dayjs().format('YYYY-MM-DD'))
  const rangeParams = useMemo(
    () => ({
      range,
      startTimestamp:
        range === 'custom' ? dayjs(startDate).startOf('day').unix() : undefined,
      endTimestamp:
        range === 'custom' ? dayjs(endDate).endOf('day').unix() : undefined,
    }),
    [endDate, range, startDate]
  )
  const overviewQuery = useAdminAnalyticsOverview(rangeParams)
  const overview = overviewQuery.data?.data
  const metrics = overview?.metrics

  const metricCards = [
    {
      label: t('Successful top-up amount'),
      value: formatBillingCurrencyFromUSD(
        metrics?.successful_topup_amount ?? 0
      ),
      detail: t('{{count}} orders', {
        count: metrics?.successful_topup_count ?? 0,
      }),
      icon: CircleDollarSign,
    },
    {
      label: t('Repurchase rate'),
      value: Intl.NumberFormat(undefined, {
        style: 'percent',
        maximumFractionDigits: 1,
      }).format(metrics?.repurchase_rate ?? 0),
      detail: t('{{count}} repeat users', {
        count: metrics?.repeat_topup_user_count ?? 0,
      }),
      icon: Repeat2,
    },
    {
      label: t('Consumed quota'),
      value: formatQuota(metrics?.consume_quota ?? 0),
      detail: t('{{count}} calls', { count: metrics?.call_count ?? 0 }),
      icon: Activity,
    },
    {
      label: t('Active users'),
      value: formatNumber(metrics?.active_user_count ?? 0),
      detail: t('{{count}} new users', {
        count: metrics?.new_user_count ?? 0,
      }),
      icon: UserCheck,
    },
    {
      label: t('Remaining balance pool'),
      value: formatQuota(metrics?.user_balance_quota ?? 0),
      detail: t('Used: {{value}}', {
        value: formatQuota(metrics?.user_used_quota ?? 0),
      }),
      icon: WalletCards,
    },
    {
      label: t('Total users'),
      value: formatNumber(metrics?.user_count ?? 0),
      detail: t('{{count}} paying users', {
        count: metrics?.topup_user_count ?? 0,
      }),
      icon: Users,
    },
  ]

  return (
    <div className='space-y-3'>
      <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
        <Tabs
          value={range}
          onValueChange={(value) => setRange(value as AdminAnalyticsRangeKey)}
        >
          <TabsList className='max-w-full overflow-x-auto'>
            {RANGE_OPTIONS.map((option) => (
              <TabsTrigger key={option.value} value={option.value}>
                {t(option.label)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <div className='flex flex-wrap items-end gap-2'>
          {range === 'custom' && (
            <>
              <div className='flex flex-col gap-1'>
                <Label htmlFor='operations-start'>{t('Start date')}</Label>
                <Input
                  id='operations-start'
                  type='date'
                  value={startDate}
                  max={endDate}
                  onChange={(event) => setStartDate(event.target.value)}
                />
              </div>
              <div className='flex flex-col gap-1'>
                <Label htmlFor='operations-end'>{t('End date')}</Label>
                <Input
                  id='operations-end'
                  type='date'
                  value={endDate}
                  min={startDate}
                  max={dayjs().format('YYYY-MM-DD')}
                  onChange={(event) => setEndDate(event.target.value)}
                />
              </div>
            </>
          )}
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  variant='outline'
                  size='icon'
                  aria-label={t('Refresh')}
                  disabled={overviewQuery.isFetching}
                  onClick={() => void overviewQuery.refetch()}
                />
              }
            >
              <RefreshCw
                className={overviewQuery.isFetching ? 'animate-spin' : ''}
              />
            </TooltipTrigger>
            <TooltipContent>{t('Refresh')}</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {overviewQuery.isError && (
        <div className='border-destructive/40 bg-destructive/5 text-destructive rounded-lg border px-4 py-3 text-sm'>
          {t('Failed to load operational analytics')}
        </div>
      )}

      <div className='grid grid-cols-2 gap-2 md:grid-cols-3 xl:grid-cols-6'>
        {metricCards.map((item) => (
          <Card key={item.label}>
            <CardContent className='p-3 sm:p-4'>
              <div className='text-muted-foreground flex items-center justify-between gap-2 text-xs'>
                <span className='truncate'>{item.label}</span>
                <item.icon className='size-4 shrink-0' />
              </div>
              {overviewQuery.isLoading ? (
                <Skeleton className='mt-3 h-7 w-24' />
              ) : (
                <div className='mt-2 truncate text-xl font-semibold'>
                  {item.value}
                </div>
              )}
              <div className='text-muted-foreground mt-1 truncate text-xs'>
                {item.detail}
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      <OperationsTrendCharts
        overview={overview}
        loading={overviewQuery.isLoading}
      />
      <ModelUsageRankingChart
        overview={overview}
        loading={overviewQuery.isLoading}
      />
      <OperationsDistributionCharts
        overview={overview}
        loading={overviewQuery.isLoading}
      />
      <OperationsRankings
        overview={overview}
        loading={overviewQuery.isLoading}
      />
    </div>
  )
}

export function OperationsAnalytics() {
  const { t } = useTranslation()
  const [view, setView] = useState<'overview' | 'zombie-users'>('overview')

  return (
    <div className='space-y-3'>
      <Tabs
        value={view}
        onValueChange={(value) =>
          setView(value as 'overview' | 'zombie-users')
        }
      >
        <TabsList>
          <TabsTrigger value='overview'>
            {t('Operational overview')}
          </TabsTrigger>
          <TabsTrigger value='zombie-users'>{t('Zombie users')}</TabsTrigger>
        </TabsList>
      </Tabs>
      {view === 'overview' ? <OperationsOverview /> : <InactiveUsersPanel />}
    </div>
  )
}
