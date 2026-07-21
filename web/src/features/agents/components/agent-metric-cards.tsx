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
import type { LucideIcon } from 'lucide-react'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

export interface AgentMetricCardItem {
  label: string
  value: string | number
  icon: LucideIcon
}

interface AgentMetricCardsProps {
  items: AgentMetricCardItem[]
  loading?: boolean
}

export function AgentMetricCards(props: AgentMetricCardsProps) {
  return (
    <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
      {props.items.map((item) => (
        <Card key={item.label} size='sm'>
          <CardHeader className='grid grid-cols-[1fr_auto] items-center gap-2'>
            <CardTitle className='text-muted-foreground font-normal'>
              {item.label}
            </CardTitle>
            <item.icon className='text-muted-foreground size-4' />
          </CardHeader>
          <CardContent>
            {props.loading ? (
              <Skeleton className='h-7 w-24' />
            ) : (
              <div className='text-2xl font-semibold tabular-nums'>
                {item.value}
              </div>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
