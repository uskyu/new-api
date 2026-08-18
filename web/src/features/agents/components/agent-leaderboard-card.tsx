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
import { useTranslation } from 'react-i18next'

import { DatePicker } from '@/components/date-picker'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import dayjs from '@/lib/dayjs'

import { useAgentLeaderboard } from '../hooks/use-agent-data'
import { formatAgentAmount } from '../lib/format'
import type { AgentLeaderboardEntry, AgentLeaderboardMetric } from '../types'
import { AgentPagination } from './agent-pagination'

const METRICS: Array<{
  value: AgentLeaderboardMetric
  label: string
  valueKey: keyof AgentLeaderboardEntry
  rankKey: keyof AgentLeaderboardEntry
}> = [
  {
    value: 'range_new_user_count',
    label: 'New users in selected period',
    valueKey: 'range_new_user_count',
    rankKey: 'range_new_user_rank',
  },
  {
    value: 'range_topup_amount',
    label: 'Online top-up in selected period',
    valueKey: 'range_topup_amount',
    rankKey: 'range_topup_amount_rank',
  },
  {
    value: 'range_second_topup_rate',
    label: 'Second top-up rate in selected period',
    valueKey: 'range_second_topup_rate',
    rankKey: 'range_second_topup_rank',
  },
  {
    value: 'range_third_topup_rate',
    label: 'Third top-up rate in selected period',
    valueKey: 'range_third_topup_rate',
    rankKey: 'range_third_topup_rank',
  },
  {
    value: 'range_fourth_topup_rate',
    label: 'Fourth top-up rate in selected period',
    valueKey: 'range_fourth_topup_rate',
    rankKey: 'range_fourth_topup_rank',
  },
]

type Props = {
  startDate: string
  endDate: string
  draftStartDate: string
  draftEndDate: string
  onDraftStartDateChange: (value: string) => void
  onDraftEndDateChange: (value: string) => void
  onApplyDates: () => boolean
  dateError?: string
  enabled: boolean
}

function metricConfig(metric: AgentLeaderboardMetric) {
  return METRICS.find((item) => item.value === metric) ?? METRICS[0]
}

function metricValue(
  item: AgentLeaderboardEntry,
  metric: AgentLeaderboardMetric
) {
  const config = metricConfig(metric)
  const value = item[config.valueKey] as number
  if (metric === 'range_topup_amount') return formatAgentAmount(value)
  if (metric.endsWith('_rate')) return `${(value * 100).toFixed(1)}%`
  return value.toLocaleString()
}

function metricRank(
  item: AgentLeaderboardEntry,
  metric: AgentLeaderboardMetric
) {
  return item[metricConfig(metric).rankKey] as number
}

function localDate(value: string) {
  const [year, month, date] = value.split('-').map(Number)
  return new Date(year, month - 1, date)
}

function localDateString(value?: Date) {
  return value ? dayjs(value).format('YYYY-MM-DD') : ''
}

function RankedMetric({
  item,
  metric,
}: {
  item: AgentLeaderboardEntry
  metric: AgentLeaderboardMetric
}) {
  return (
    <div className='flex items-center gap-2 whitespace-nowrap'>
      <Badge variant='secondary'>#{metricRank(item, metric)}</Badge>
      <span>{metricValue(item, metric)}</span>
    </div>
  )
}

export function AgentLeaderboardCard(props: Props) {
  const { t } = useTranslation()
  const [metric, setMetric] = useState<AgentLeaderboardMetric>(METRICS[0].value)
  const [page, setPage] = useState(1)
  const pageSize = 10
  const query = useAgentLeaderboard({
    startDate: props.startDate,
    endDate: props.endDate,
    metric,
    page,
    pageSize,
    enabled: props.enabled,
  })
  const result = query.data?.data
  const selectedMetric = metricConfig(metric)

  const apply = () => {
    if (props.onApplyDates()) setPage(1)
  }

  return (
    <Card>
      <CardHeader className='gap-3'>
        <div className='flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between'>
          <div>
            <CardTitle>{t('Agent leaderboard')}</CardTitle>
            <p className='text-muted-foreground mt-1 text-sm'>
              {t(
                'Only settled online top-ups are counted. Redemption codes and manual top-ups are excluded.'
              )}
            </p>
          </div>
          <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
            <DatePicker
              selected={
                props.draftStartDate
                  ? localDate(props.draftStartDate)
                  : undefined
              }
              onSelect={(date) =>
                props.onDraftStartDateChange(localDateString(date))
              }
              className='w-full sm:w-[150px]'
            />
            <span className='text-muted-foreground hidden sm:inline'>–</span>
            <DatePicker
              selected={
                props.draftEndDate ? localDate(props.draftEndDate) : undefined
              }
              onSelect={(date) =>
                props.onDraftEndDateChange(localDateString(date))
              }
              className='w-full sm:w-[150px]'
            />
            <Button size='sm' onClick={apply}>
              {t('Apply')}
            </Button>
          </div>
        </div>
        {props.dateError && (
          <p className='text-destructive text-sm'>{t(props.dateError)}</p>
        )}
      </CardHeader>
      <CardContent className='space-y-4'>
        <Tabs
          value={metric}
          onValueChange={(value) => {
            setMetric(value as AgentLeaderboardMetric)
            setPage(1)
          }}
        >
          <div className='overflow-x-auto pb-1'>
            <TabsList className='min-w-max justify-start'>
              {METRICS.map((item) => (
                <TabsTrigger key={item.value} value={item.value}>
                  {t(item.label)}
                </TabsTrigger>
              ))}
            </TabsList>
          </div>
        </Tabs>

        {query.isLoading && <Skeleton className='h-64 w-full' />}
        {query.isError && (
          <Alert variant='destructive'>
            <AlertTitle>{t('Unable to load leaderboard')}</AlertTitle>
            <AlertDescription>{t('Please try again later.')}</AlertDescription>
          </Alert>
        )}

        {!query.isLoading && !query.isError && result?.self && (
          <div className='bg-primary/5 grid gap-3 rounded-lg border p-4 sm:grid-cols-3'>
            <div>
              <p className='text-muted-foreground text-sm'>
                {t('Your ranking')}
              </p>
              <p className='text-2xl font-semibold'>
                #{metricRank(result.self, metric)}
              </p>
            </div>
            <div>
              <p className='text-muted-foreground text-sm'>
                {t(selectedMetric.label)}
              </p>
              <p className='font-medium'>{metricValue(result.self, metric)}</p>
            </div>
            <div>
              <p className='text-muted-foreground text-sm'>
                {t('Selected period')}
              </p>
              <p className='font-medium'>
                {result.start_date} – {result.end_date}
              </p>
            </div>
          </div>
        )}

        {!query.isLoading &&
          !query.isError &&
          result &&
          result.items.length === 0 && (
            <p className='text-muted-foreground py-10 text-center'>
              {t('No leaderboard data found')}
            </p>
          )}

        {!query.isLoading &&
          !query.isError &&
          result &&
          result.items.length > 0 && (
            <>
              <div className='hidden overflow-x-auto md:block'>
                <table className='w-full text-sm'>
                  <thead>
                    <tr className='border-b text-left'>
                      <th className='p-2'>{t('Agent')}</th>
                      {METRICS.map((item) => (
                        <th className='p-2' key={item.value}>
                          {t(item.label)}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {result.items.map((item) => (
                      <tr
                        className={
                          item.is_self ? 'bg-primary/5 border-b' : 'border-b'
                        }
                        key={item.row_key}
                      >
                        <td className='p-2 font-medium'>
                          <span className='flex items-center gap-2'>
                            {item.agent_label}
                            {item.is_self && <Badge>{t('You')}</Badge>}
                          </span>
                        </td>
                        {METRICS.map((entry) => (
                          <td
                            className={
                              entry.value === metric
                                ? 'bg-primary/5 p-2 font-medium'
                                : 'p-2'
                            }
                            key={entry.value}
                          >
                            <RankedMetric item={item} metric={entry.value} />
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <div className='divide-y rounded-lg border md:hidden'>
                {result.items.map((item) => (
                  <div className='space-y-3 p-3' key={item.row_key}>
                    <div className='flex items-center justify-between gap-3'>
                      <span className='flex items-center gap-2 font-medium'>
                        {item.agent_label}
                        {item.is_self && <Badge>{t('You')}</Badge>}
                      </span>
                      <Badge variant='secondary'>
                        #{metricRank(item, metric)}
                      </Badge>
                    </div>
                    <div>
                      <p className='text-muted-foreground text-xs'>
                        {t(selectedMetric.label)}
                      </p>
                      <p className='text-lg font-semibold'>
                        {metricValue(item, metric)}
                      </p>
                    </div>
                    <div className='grid grid-cols-2 gap-2 text-xs'>
                      {METRICS.map((entry) => (
                        <div key={entry.value}>
                          <p className='text-muted-foreground'>
                            {t(entry.label)}
                          </p>
                          <RankedMetric item={item} metric={entry.value} />
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
              </div>

              <AgentPagination
                page={page}
                pageSize={pageSize}
                total={result.total}
                onPageChange={setPage}
              />
            </>
          )}
      </CardContent>
    </Card>
  )
}
