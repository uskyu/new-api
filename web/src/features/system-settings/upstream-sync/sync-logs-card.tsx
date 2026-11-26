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
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { RefreshCw, RotateCcw, Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  DataTablePagination,
  StaticDataTable,
  useDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { DateTimePicker } from '@/components/datetime-picker'
import { ErrorState } from '@/components/error-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Skeleton } from '@/components/ui/skeleton'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatTimestampToDate } from '@/lib/format'
import { createServerError } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'

import { getUpstreamSyncLogs } from './api'
import {
  formatDurationMs,
  formatJsonDetail,
  getJsonEntryCount,
  getSyncTriggerLabel,
} from './helpers'
import { SyncLogDetailSheet } from './sync-log-detail-sheet'
import { SyncStatusBadge } from './sync-status-badge'
import type { UpstreamSyncLog, UpstreamSyncLogsPage } from './types'

type LogFilters = {
  status: string
  q: string
  start_time?: number
  end_time?: number
}

type LogFilterDraft = {
  status: string
  q: string
  start?: Date
  end?: Date
}

const EMPTY_FILTERS: LogFilters = { status: '', q: '' }
const EMPTY_DRAFT: LogFilterDraft = { status: '', q: '' }
const EMPTY_LOGS: UpstreamSyncLog[] = []

type SyncLogsCardProps = {
  activeTaskId: string
  runStartedAt: number
}

function CountCell(props: {
  value: unknown
  className?: string
  locale?: string
}) {
  const count = getJsonEntryCount(props.value)
  if (count === null) {
    return <span className='text-muted-foreground/60'>-</span>
  }
  return (
    <span className={cn('tabular-nums', count > 0 && props.className)}>
      {formatNumber(count, props.locale)}
    </span>
  )
}

function ErrorCountCell(props: { value: unknown; locale?: string }) {
  const count = getJsonEntryCount(props.value)
  if (count === null) {
    return <span className='text-muted-foreground/60'>-</span>
  }
  if (count === 0) {
    return (
      <span className='text-muted-foreground tabular-nums'>
        {formatNumber(0, props.locale)}
      </span>
    )
  }
  const detail = formatJsonDetail(props.value)
  return (
    <span title={detail.slice(0, 400)}>
      <Badge variant='destructive' className='tabular-nums'>
        {formatNumber(count, props.locale)}
      </Badge>
    </span>
  )
}

function FiltersBar(props: {
  draft: LogFilterDraft
  onDraftChange: (patch: Partial<LogFilterDraft>) => void
  onSearch: () => void
  onReset: () => void
}) {
  const { t } = useTranslation()

  return (
    <form
      aria-label={t('Filter sync logs')}
      className='bg-muted/20 grid min-w-0 gap-3 rounded-lg border p-3 sm:grid-cols-2 xl:grid-cols-[11rem_minmax(15rem,1fr)_minmax(15rem,1fr)_minmax(12rem,16rem)_auto] xl:items-end'
      onSubmit={(event) => {
        event.preventDefault()
        props.onSearch()
      }}
    >
      <div className='min-w-0 space-y-1.5'>
        <label className='text-muted-foreground text-xs font-medium'>
          {t('Status')}
        </label>
        <NativeSelect
          aria-label={t('Status')}
          className='w-full'
          value={props.draft.status}
          onChange={(event) =>
            props.onDraftChange({ status: event.target.value })
          }
        >
          <NativeSelectOption value=''>{t('All Status')}</NativeSelectOption>
          <NativeSelectOption value='success'>
            {t('Success')}
          </NativeSelectOption>
          <NativeSelectOption value='failed'>{t('Failed')}</NativeSelectOption>
          <NativeSelectOption value='partial'>
            {t('Partial')}
          </NativeSelectOption>
        </NativeSelect>
      </div>

      <div className='min-w-0 space-y-1.5'>
        <label className='text-muted-foreground text-xs font-medium'>
          {t('Start Time')}
        </label>
        <DateTimePicker
          value={props.draft.start}
          placeholder={t('Select start time')}
          onChange={(start) => props.onDraftChange({ start })}
          className='w-full'
        />
      </div>

      <div className='min-w-0 space-y-1.5'>
        <label className='text-muted-foreground text-xs font-medium'>
          {t('End Time')}
        </label>
        <DateTimePicker
          value={props.draft.end}
          placeholder={t('Select end time')}
          onChange={(end) => props.onDraftChange({ end })}
          className='w-full'
        />
      </div>

      <div className='min-w-0 space-y-1.5'>
        <label
          htmlFor='upstream-sync-log-keyword'
          className='text-muted-foreground text-xs font-medium'
        >
          {t('Keyword')}
        </label>
        <Input
          id='upstream-sync-log-keyword'
          aria-label={t('Keyword')}
          placeholder={t('Model or group')}
          value={props.draft.q}
          onChange={(event) => props.onDraftChange({ q: event.target.value })}
        />
      </div>

      <div className='flex items-end gap-2 sm:col-span-2 xl:col-span-1'>
        <Button type='submit' size='sm'>
          <Search data-icon='inline-start' />
          {t('Search')}
        </Button>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={props.onReset}
        >
          <RotateCcw data-icon='inline-start' />
          {t('Reset')}
        </Button>
      </div>
    </form>
  )
}

export function SyncLogsCard(props: SyncLogsCardProps) {
  const { t, i18n } = useTranslation()
  const intlLocale = toIntlLocale(i18n.language)
  const [draft, setDraft] = useState<LogFilterDraft>(EMPTY_DRAFT)
  const [filters, setFilters] = useState<LogFilters>(EMPTY_FILTERS)
  const [searchNonce, setSearchNonce] = useState(0)
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: 20 })
  const [detailLog, setDetailLog] = useState<UpstreamSyncLog | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)

  const logsQuery = useQuery({
    queryKey: ['upstream-sync', 'logs', filters, pagination, searchNonce],
    queryFn: async (): Promise<UpstreamSyncLogsPage> => {
      const res = await getUpstreamSyncLogs({
        page: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
        status: filters.status || undefined,
        start_time: filters.start_time,
        end_time: filters.end_time,
        q: filters.q || undefined,
      })
      if (!res.success || !res.data || !Array.isArray(res.data.items)) {
        throw createServerError(res, t('Failed to load sync logs'))
      }
      return res.data
    },
    placeholderData: keepPreviousData,
    retry: false,
    refetchInterval: () =>
      props.activeTaskId || props.runStartedAt > 0 ? 5000 : false,
  })

  const logs = logsQuery.data?.items ?? EMPTY_LOGS
  const { table } = useDataTable<UpstreamSyncLog>({
    data: logs,
    columns: [],
    totalCount: logsQuery.data?.total ?? 0,
    manualPagination: true,
    columnFilters: [],
    pagination,
    onPaginationChange: setPagination,
    columnVisibilityStorageKey: false,
    columnSizingStorageKey: false,
    ensurePageInRange: (pageCount) => {
      if (
        logsQuery.isSuccess &&
        !logsQuery.isFetching &&
        pagination.pageIndex >= Math.max(1, pageCount)
      ) {
        setPagination((previous) => ({
          ...previous,
          pageIndex: Math.max(0, pageCount - 1),
        }))
      }
    },
  })

  useEffect(() => {
    if (!props.runStartedAt) return
    setPagination((previous) =>
      previous.pageIndex === 0 ? previous : { ...previous, pageIndex: 0 }
    )
  }, [props.runStartedAt])

  const handleSearch = () => {
    if (
      draft.start &&
      draft.end &&
      draft.start.getTime() > draft.end.getTime()
    ) {
      toast.error(t('End time must be after start time'))
      return
    }

    setFilters({
      status: draft.status,
      q: draft.q.trim(),
      start_time: draft.start
        ? Math.floor(draft.start.getTime() / 1000)
        : undefined,
      end_time: draft.end ? Math.floor(draft.end.getTime() / 1000) : undefined,
    })
    setPagination((previous) => ({ ...previous, pageIndex: 0 }))
    setSearchNonce((previous) => previous + 1)
  }

  const handleReset = () => {
    setDraft(EMPTY_DRAFT)
    setFilters(EMPTY_FILTERS)
    setPagination((previous) => ({ ...previous, pageIndex: 0 }))
    setSearchNonce((previous) => previous + 1)
  }

  const columns: StaticDataTableColumn<UpstreamSyncLog>[] = [
    {
      id: 'started_at',
      header: t('Start Time'),
      className: 'w-[150px] px-4 text-xs',
      cell: (log) => formatTimestampToDate(log.started_at),
    },
    {
      id: 'finished_at',
      header: t('End Time'),
      className: 'w-[150px] text-xs',
      cell: (log) => formatTimestampToDate(log.finished_at ?? undefined),
    },
    {
      id: 'trigger',
      header: t('Trigger'),
      className: 'w-[90px] text-xs',
      cell: (log) => t(getSyncTriggerLabel(log.trigger)),
    },
    {
      id: 'status',
      header: t('Status'),
      className: 'w-[90px] text-xs',
      cell: (log) => <SyncStatusBadge status={log.status} />,
    },
    {
      id: 'groups_checked',
      header: t('Groups Checked'),
      className: 'w-[85px] text-right text-xs',
      cellClassName: 'text-right',
      cell: (log) => (
        <CountCell value={log.groups_checked} locale={intlLocale} />
      ),
    },
    {
      id: 'channels_changed',
      header: t('Channels Changed'),
      className: 'w-[75px] text-right text-xs',
      cellClassName: 'text-right',
      cell: (log) => (
        <CountCell value={log.channels_changed} locale={intlLocale} />
      ),
    },
    {
      id: 'added_models',
      header: t('Added Models'),
      className: 'w-[85px] text-right text-xs',
      cellClassName: 'text-right',
      cell: (log) => (
        <CountCell
          value={log.added_models}
          locale={intlLocale}
          className='text-emerald-600 dark:text-emerald-400'
        />
      ),
    },
    {
      id: 'removed_models',
      header: t('Removed Models'),
      className: 'w-[85px] text-right text-xs',
      cellClassName: 'text-right',
      cell: (log) => (
        <CountCell
          value={log.removed_models}
          locale={intlLocale}
          className='text-rose-600 dark:text-rose-400'
        />
      ),
    },
    {
      id: 'price_changes',
      header: t('Price Changes'),
      className: 'w-[85px] text-right text-xs',
      cellClassName: 'text-right',
      cell: (log) => (
        <CountCell
          value={log.price_changes}
          locale={intlLocale}
          className='text-sky-600 dark:text-sky-400'
        />
      ),
    },
    {
      id: 'errors',
      header: t('Errors'),
      className: 'w-[75px] text-right text-xs',
      cellClassName: 'text-right',
      cell: (log) => <ErrorCountCell value={log.errors} locale={intlLocale} />,
    },
    {
      id: 'duration_ms',
      header: t('Duration'),
      className: 'w-[75px] text-right text-xs',
      cellClassName: 'text-right',
      cell: (log) => formatDurationMs(log.duration_ms),
    },
    {
      id: 'actions',
      header: '',
      className: 'w-[70px] pr-4 text-xs',
      cellClassName: 'pr-4',
      cell: (log) => (
        <Button
          type='button'
          variant='ghost'
          size='sm'
          onClick={() => {
            setDetailLog(log)
            setDetailOpen(true)
          }}
        >
          {t('Details')}
        </Button>
      ),
    },
  ]

  return (
    <Card>
      <CardHeader className='border-b pb-4'>
        <CardTitle>{t('Sync Logs')}</CardTitle>
        <CardDescription>
          {t(
            'Inspect scheduled and manual upstream sync runs, including model and price changes.'
          )}
        </CardDescription>
        <CardAction>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => void logsQuery.refetch()}
            disabled={logsQuery.isFetching}
            aria-label={t('Refresh')}
          >
            <RefreshCw
              data-icon='inline-start'
              className={cn('size-3.5', logsQuery.isFetching && 'animate-spin')}
              aria-hidden='true'
            />
            {t('Refresh')}
          </Button>
        </CardAction>
      </CardHeader>

      <CardContent className='space-y-4'>
        <FiltersBar
          draft={draft}
          onDraftChange={(patch) =>
            setDraft((previous) => ({ ...previous, ...patch }))
          }
          onSearch={handleSearch}
          onReset={handleReset}
        />

        <div aria-busy={logsQuery.isFetching}>
          {logsQuery.isLoading && (
            <div className='space-y-2'>
              <Skeleton className='h-9 w-full rounded-md' />
              <Skeleton className='h-9 w-full rounded-md' />
              <Skeleton className='h-9 w-full rounded-md' />
              <Skeleton className='h-9 w-full rounded-md' />
              <Skeleton className='h-9 w-full rounded-md' />
            </div>
          )}

          {!logsQuery.isLoading && logsQuery.isError && (
            <ErrorState
              title={t('Failed to load sync logs')}
              description={
                logsQuery.error instanceof Error
                  ? logsQuery.error.message
                  : undefined
              }
              onRetry={() => void logsQuery.refetch()}
              className='min-h-[220px]'
            />
          )}

          {!logsQuery.isLoading && !logsQuery.isError && (
            <>
              <StaticDataTable
                columns={columns}
                data={logs}
                getRowKey={(log) => log.id}
                tableClassName='min-w-[1140px]'
                headerRowClassName='bg-muted/40 hover:bg-muted/40'
                emptyContent={
                  <span className='text-muted-foreground text-sm'>
                    {t('No sync logs found')}
                  </span>
                }
                emptyClassName='h-32'
              />
              <div className='mt-3'>
                <DataTablePagination table={table} />
              </div>
            </>
          )}
        </div>
      </CardContent>

      <SyncLogDetailSheet
        log={detailLog}
        open={detailOpen}
        onOpenChange={(open) => {
          setDetailOpen(open)
          if (!open) setDetailLog(null)
        }}
      />
    </Card>
  )
}
