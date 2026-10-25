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
import { Search, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { LEDGER_STATUS_PRESENTATION } from '../lib/constants'
import { LEDGER_STATUS_VALUES, type LedgerStatus } from '../types'

interface LedgerFilterBarProps {
  search: Record<string, unknown>
  onChange: (patch: Record<string, unknown>) => void
  total: number
}

// The "any status" sentinel is a value the backend never receives; the item label
// is what the closed trigger must show.
const ALL_STATUSES = 'all'

export function LedgerFilterBar(props: LedgerFilterBarProps) {
  const { t } = useTranslation()
  const [requestId, setRequestId] = useState(
    () => (props.search.request_id as string) ?? ''
  )
  const [modelName, setModelName] = useState(
    () => (props.search.model_name as string) ?? ''
  )

  // The URL is the source of truth, so a browser back/forward or a pasted link
  // must be reflected in the inputs rather than leaving stale text behind.
  useEffect(() => {
    setRequestId((props.search.request_id as string) ?? '')
  }, [props.search.request_id])
  useEffect(() => {
    setModelName((props.search.model_name as string) ?? '')
  }, [props.search.model_name])

  const status = (props.search.status as LedgerStatus[] | undefined) ?? []
  const startTime = props.search.start_time as number | undefined
  const endTime = props.search.end_time as number | undefined
  const selectedStatus = status.length === 1 ? status[0] : ALL_STATUSES

  const hasActiveFilters =
    Boolean(props.search.request_id) ||
    Boolean(props.search.model_name) ||
    status.length > 0 ||
    Boolean(startTime) ||
    Boolean(endTime)

  const submit = () => {
    props.onChange({
      request_id: requestId.trim() || undefined,
      model_name: modelName.trim() || undefined,
    })
  }

  const reset = () => {
    setRequestId('')
    setModelName('')
    props.onChange({
      request_id: undefined,
      model_name: undefined,
      status: undefined,
      start_time: undefined,
      end_time: undefined,
    })
  }

  return (
    <div className='flex flex-wrap items-end gap-2'>
      <div className='flex min-w-[190px] flex-1 flex-col gap-1'>
        <label
          className='text-muted-foreground text-[11px] font-medium'
          htmlFor='ledger-request-id'
        >
          {t('Request ID')}
        </label>
        <Input
          id='ledger-request-id'
          value={requestId}
          placeholder={t('Search by request ID prefix')}
          autoComplete='off'
          onChange={(event) => setRequestId(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              submit()
            }
          }}
          className='h-8 text-xs'
        />
      </div>

      <div className='flex min-w-[150px] flex-1 flex-col gap-1'>
        <label
          className='text-muted-foreground text-[11px] font-medium'
          htmlFor='ledger-model-name'
        >
          {t('Model')}
        </label>
        <Input
          id='ledger-model-name'
          value={modelName}
          placeholder={t('Search by model')}
          autoComplete='off'
          onChange={(event) => setModelName(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              submit()
            }
          }}
          className='h-8 text-xs'
        />
      </div>

      <div className='flex flex-col gap-1'>
        <span className='text-muted-foreground text-[11px] font-medium'>
          {t('Status')}
        </span>
        <Select
          value={selectedStatus}
          onValueChange={(value) =>
            props.onChange({
              status: value === ALL_STATUSES ? undefined : [value],
            })
          }
        >
          <SelectTrigger className='h-8 w-[130px] text-xs'>
            {/* The trigger renders its children when given any, so the label is
                stated explicitly instead of letting the raw value show through. */}
            <SelectValue>
              {selectedStatus === ALL_STATUSES
                ? t('All statuses')
                : t(
                    LEDGER_STATUS_PRESENTATION[selectedStatus]?.labelKey ??
                      selectedStatus
                  )}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_STATUSES}>{t('All statuses')}</SelectItem>
            {LEDGER_STATUS_VALUES.map((value) => (
              <SelectItem key={value} value={value}>
                {t(LEDGER_STATUS_PRESENTATION[value]?.labelKey ?? value)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className='flex flex-col gap-1'>
        <label
          className='text-muted-foreground text-[11px] font-medium'
          htmlFor='ledger-start-date'
        >
          {t('From')}
        </label>
        <Input
          id='ledger-start-date'
          type='date'
          className='h-8 w-[140px] text-xs'
          value={startTime ? toDateInputValue(startTime) : ''}
          onChange={(event) =>
            props.onChange({
              start_time: fromDateInputValue(event.target.value, false),
            })
          }
        />
      </div>

      <div className='flex flex-col gap-1'>
        <label
          className='text-muted-foreground text-[11px] font-medium'
          htmlFor='ledger-end-date'
        >
          {t('To')}
        </label>
        <Input
          id='ledger-end-date'
          type='date'
          className='h-8 w-[140px] text-xs'
          value={endTime ? toDateInputValue(endTime) : ''}
          onChange={(event) =>
            props.onChange({
              end_time: fromDateInputValue(event.target.value, true),
            })
          }
        />
      </div>

      <Button size='sm' className='h-8' onClick={submit}>
        <Search className='mr-1 size-3.5' aria-hidden='true' />
        {t('Search')}
      </Button>
      <Button
        size='sm'
        variant='ghost'
        className='h-8'
        onClick={reset}
        disabled={!hasActiveFilters}
      >
        <X className='mr-1 size-3.5' aria-hidden='true' />
        {t('Reset')}
      </Button>

      <Badge variant='secondary' className='ml-auto h-8 px-2 text-xs'>
        {t('{{count}} records', { count: props.total })}
      </Badge>
    </div>
  )
}

// The API filters created_at as unix seconds, while a date input works in local
// calendar days. The end bound is shifted to the next midnight so the chosen day is
// included, matching what an operator means by "to this date".
function toDateInputValue(unixSeconds: number): string {
  const date = new Date(unixSeconds * 1000)
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

function fromDateInputValue(
  value: string,
  endOfDay: boolean
): number | undefined {
  if (!value) {
    return undefined
  }
  const date = new Date(`${value}T00:00:00`)
  if (Number.isNaN(date.getTime())) {
    return undefined
  }
  if (endOfDay) {
    date.setDate(date.getDate() + 1)
  }
  return Math.floor(date.getTime() / 1000)
}
