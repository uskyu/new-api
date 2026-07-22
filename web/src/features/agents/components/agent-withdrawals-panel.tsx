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
import { Download, Search, Upload } from 'lucide-react'
import { useRef, useState, type ChangeEvent, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StaticDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Skeleton } from '@/components/ui/skeleton'

import { exportAgentWithdrawRequests, importAgentWithdrawResults } from '../api'
import { useAgentWithdrawRequests } from '../hooks/use-agent-data'
import { formatAgentAmount } from '../lib/format'
import type { AgentWithdrawStatus } from '../types'
import { AgentPagination } from './agent-pagination'

const PAGE_SIZE = 10

interface WithdrawFilters {
  status: AgentWithdrawStatus | ''
  startDate: string
  endDate: string
}

interface AgentWithdrawalsPanelProps {
  enabled: boolean
}

export function AgentWithdrawalsPanel(props: AgentWithdrawalsPanelProps) {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement>(null)
  const [page, setPage] = useState(1)
  const [draft, setDraft] = useState<WithdrawFilters>({
    status: '',
    startDate: dayjs().subtract(29, 'day').format('YYYY-MM-DD'),
    endDate: dayjs().format('YYYY-MM-DD'),
  })
  const [filters, setFilters] = useState(draft)
  const [exporting, setExporting] = useState(false)
  const [importing, setImporting] = useState(false)
  const requests = useAgentWithdrawRequests({
    page,
    pageSize: PAGE_SIZE,
    ...filters,
    enabled: props.enabled,
  })
  const data = requests.data?.data

  const applyFilters = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setPage(1)
    setFilters(draft)
  }

  const handleExport = async () => {
    setExporting(true)
    try {
      const blob = await exportAgentWithdrawRequests(filters)
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = `agent-withdraw-${Date.now()}.csv`
      anchor.click()
      URL.revokeObjectURL(url)
      toast.success(t('Withdrawal data exported'))
      await requests.refetch()
    } catch {
      toast.error(t('Failed to export withdrawal data'))
    } finally {
      setExporting(false)
    }
  }

  const handleImport = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    setImporting(true)
    try {
      const response = await importAgentWithdrawResults(file)
      if (!response.success) {
        toast.error(
          response.message || t('Failed to import withdrawal receipt')
        )
        return
      }
      toast.success(
        t('Withdrawal receipt imported: {{count}} processed', {
          count: response.data?.processed ?? 0,
        })
      )
      await requests.refetch()
    } catch {
      toast.error(t('Failed to import withdrawal receipt'))
    } finally {
      setImporting(false)
    }
  }

  const statusLabel = (status: AgentWithdrawStatus) => {
    if (status === 'paid') return t('Paid')
    if (status === 'exported') return t('Exported')
    return t('Pending')
  }

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between'>
        <form
          className='grid flex-1 gap-3 sm:grid-cols-3 lg:max-w-3xl'
          onSubmit={applyFilters}
        >
          <div className='space-y-1.5'>
            <Label htmlFor='withdraw-status'>{t('Status')}</Label>
            <NativeSelect
              id='withdraw-status'
              className='w-full'
              value={draft.status}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  status: event.target.value as AgentWithdrawStatus | '',
                }))
              }
            >
              <NativeSelectOption value=''>
                {t('All statuses')}
              </NativeSelectOption>
              <NativeSelectOption value='pending'>
                {t('Pending')}
              </NativeSelectOption>
              <NativeSelectOption value='exported'>
                {t('Exported')}
              </NativeSelectOption>
              <NativeSelectOption value='paid'>{t('Paid')}</NativeSelectOption>
            </NativeSelect>
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='withdraw-start'>{t('Start date')}</Label>
            <Input
              id='withdraw-start'
              type='date'
              value={draft.startDate}
              max={draft.endDate}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  startDate: event.target.value,
                }))
              }
            />
          </div>
          <div className='flex items-end gap-2'>
            <div className='flex-1 space-y-1.5'>
              <Label htmlFor='withdraw-end'>{t('End date')}</Label>
              <Input
                id='withdraw-end'
                type='date'
                value={draft.endDate}
                min={draft.startDate}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    endDate: event.target.value,
                  }))
                }
              />
            </div>
            <Button type='submit' size='icon' aria-label={t('Apply')}>
              <Search />
            </Button>
          </div>
        </form>
        <div className='flex gap-2'>
          <input
            ref={inputRef}
            type='file'
            accept='.csv,text/csv'
            className='hidden'
            onChange={handleImport}
          />
          <Button
            variant='outline'
            disabled={importing}
            onClick={() => inputRef.current?.click()}
          >
            <Upload />
            {importing ? t('Importing...') : t('Import receipt')}
          </Button>
          <Button disabled={exporting} onClick={handleExport}>
            <Download />
            {exporting ? t('Exporting...') : t('Export requests')}
          </Button>
        </div>
      </div>

      {requests.isLoading ? (
        <Skeleton className='h-72 w-full' />
      ) : (
        <StaticDataTable
          data={data?.items ?? []}
          getRowKey={(request) => request.id}
          emptyContent={t('No withdrawal requests found')}
          columns={[
            {
              id: 'request',
              header: t('Request'),
              cell: (request) => (
                <div className='flex min-w-36 flex-col'>
                  <span className='font-medium'>#{request.id}</span>
                  <span className='text-muted-foreground text-xs'>
                    {new Date(request.created_at * 1000).toLocaleString()}
                  </span>
                </div>
              ),
            },
            {
              id: 'agent',
              header: t('Agent'),
              cell: (request) => (
                <div className='flex min-w-32 flex-col'>
                  <span>{request.display_name || request.username}</span>
                  <span className='text-muted-foreground text-xs'>
                    #{request.agent_user_id}
                  </span>
                </div>
              ),
            },
            {
              id: 'account',
              header: t('Payout account'),
              cell: (request) => (
                <div className='flex min-w-40 flex-col'>
                  <span>{request.account_name_snapshot}</span>
                  <span className='text-muted-foreground text-xs'>
                    {request.account_no_snapshot}
                  </span>
                </div>
              ),
            },
            {
              id: 'amount',
              header: t('Amount'),
              cell: (request) => formatAgentAmount(request.amount),
            },
            {
              id: 'status',
              header: t('Status'),
              cell: (request) => (
                <StatusBadge
                  label={statusLabel(request.status)}
                  variant={request.status === 'paid' ? 'success' : 'neutral'}
                  copyable={false}
                />
              ),
            },
            {
              id: 'external',
              header: t('External order number'),
              cell: (request) => request.external_order_no || '-',
            },
          ]}
        />
      )}
      <AgentPagination
        page={page}
        pageSize={PAGE_SIZE}
        total={data?.total ?? 0}
        onPageChange={setPage}
      />
    </div>
  )
}
