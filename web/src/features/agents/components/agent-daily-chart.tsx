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
import { useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

import { formatAgentAmount } from '../lib/format'
import type { AgentDailyMetric } from '../types'

interface AgentDailyChartProps {
  items: AgentDailyMetric[]
  loading?: boolean
}

export function AgentDailyChart(props: AgentDailyChartProps) {
  const { t } = useTranslation()
  const data = useMemo(() => {
    const totals = new Map<
      string,
      { date: string; newUsers: number; topupAmount: number }
    >()
    for (const item of props.items) {
      const current = totals.get(item.date) ?? {
        date: item.date.slice(5),
        newUsers: 0,
        topupAmount: 0,
      }
      current.newUsers += item.new_user_count
      current.topupAmount += item.topup_amount
      totals.set(item.date, current)
    }
    return [...totals.entries()]
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([, value]) => value)
  }, [props.items])
  let chartContent: ReactNode
  if (props.loading) {
    chartContent = <Skeleton className='h-full w-full' />
  } else if (data.length === 0) {
    chartContent = (
      <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
        {t('No Data')}
      </div>
    )
  } else {
    chartContent = (
      <ResponsiveContainer width='100%' height='100%'>
        <LineChart
          data={data}
          margin={{ top: 8, right: 8, bottom: 0, left: 0 }}
        >
          <CartesianGrid strokeDasharray='3 3' vertical={false} />
          <XAxis dataKey='date' tickLine={false} axisLine={false} />
          <YAxis yAxisId='users' allowDecimals={false} width={32} />
          <YAxis
            yAxisId='topup'
            orientation='right'
            tickFormatter={(value: number) => formatAgentAmount(value)}
            width={72}
          />
          <Tooltip
            formatter={(value, name) => {
              if (name === 'topupAmount') {
                return [formatAgentAmount(Number(value)), t('Top-up amount')]
              }
              return [Number(value).toLocaleString(), t('New users')]
            }}
          />
          <Legend
            formatter={(value) =>
              value === 'topupAmount' ? t('Top-up amount') : t('New users')
            }
          />
          <Line
            yAxisId='users'
            type='monotone'
            dataKey='newUsers'
            stroke='var(--chart-1)'
            strokeWidth={2}
            dot={false}
          />
          <Line
            yAxisId='topup'
            type='monotone'
            dataKey='topupAmount'
            stroke='var(--chart-2)'
            strokeWidth={2}
            dot={false}
          />
        </LineChart>
      </ResponsiveContainer>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Daily growth trends')}</CardTitle>
      </CardHeader>
      <CardContent className='h-72'>{chartContent}</CardContent>
    </Card>
  )
}
