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
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

interface AgentPaginationProps {
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
}

export function AgentPagination(props: AgentPaginationProps) {
  const { t } = useTranslation()
  const totalPages = Math.max(1, Math.ceil(props.total / props.pageSize))

  return (
    <div className='flex items-center justify-between border-t px-1 pt-3'>
      <span className='text-muted-foreground text-sm'>
        {t('Total:')} {props.total.toLocaleString()}
      </span>
      <div className='flex items-center gap-2'>
        <Button
          variant='outline'
          size='icon-sm'
          aria-label={t('Go to previous page')}
          disabled={props.page <= 1}
          onClick={() => props.onPageChange(props.page - 1)}
        >
          <ChevronLeft />
        </Button>
        <span className='min-w-16 text-center text-sm tabular-nums'>
          {props.page} / {totalPages}
        </span>
        <Button
          variant='outline'
          size='icon-sm'
          aria-label={t('Go to next page')}
          disabled={props.page >= totalPages}
          onClick={() => props.onPageChange(props.page + 1)}
        >
          <ChevronRight />
        </Button>
      </div>
    </div>
  )
}
