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
import { Check, ChevronLeft, ChevronRight, Copy, Search } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
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
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { formatNumber, formatQuota } from '@/lib/format'

import { useBillingHistory } from '../../hooks/use-billing-history'
import {
  formatTimestamp,
  getPaymentMethodName,
  getStatusConfig,
} from '../../lib/billing'
import type {
  BillingHistoryScope,
  BillingRecordType,
  RedemptionBillingRecord,
  TopupRecord,
} from '../../types'

interface BillingHistoryDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  scope?: BillingHistoryScope
  userId?: number
  subjectName?: string
}

export function BillingHistoryDialog(props: BillingHistoryDialogProps) {
  const { t } = useTranslation()
  const history = useBillingHistory({
    scope: props.scope,
    userId: props.userId,
    enabled: props.open,
  })
  const [searchInput, setSearchInput] = useState('')
  const [confirmTradeNo, setConfirmTradeNo] = useState<string | null>(null)
  const { copyToClipboard, copiedText } = useCopyToClipboard({ notify: false })
  const items = history.response?.data?.items ?? []
  const total = history.response?.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / history.pageSize))
  const showUserId = history.scope === 'all'
  const allowCompleteOrder = history.scope === 'all' && history.isAdmin

  const handleSearchSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    history.handleSearch(searchInput.trim())
  }

  const handleTypeChange = (value: BillingRecordType) => {
    setSearchInput('')
    history.handleRecordTypeChange(value)
  }

  const handleConfirmComplete = async () => {
    if (!confirmTradeNo) return
    const success = await history.handleCompleteOrder(confirmTradeNo)
    if (success) setConfirmTradeNo(null)
  }

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        title={t('Billing History')}
        description={
          props.subjectName
            ? t('View all billing records for {{name}}', {
                name: props.subjectName,
              })
            : t('View online and redemption top-up records')
        }
        contentClassName='flex max-h-[calc(100dvh-2rem)] flex-col max-sm:w-screen max-sm:max-w-none max-sm:rounded-none max-sm:p-4 sm:max-w-4xl'
        contentHeight='auto'
        bodyClassName='flex min-h-0 flex-col gap-3'
      >
        <Tabs value={history.recordType} onValueChange={handleTypeChange}>
          <TabsList>
            <TabsTrigger value='online'>{t('Online top-ups')}</TabsTrigger>
            <TabsTrigger value='redemption'>
              {t('Redemption top-ups')}
            </TabsTrigger>
          </TabsList>
        </Tabs>

        <form className='flex items-center gap-2' onSubmit={handleSearchSubmit}>
          <div className='relative flex-1'>
            <Search
              aria-hidden='true'
              className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2'
            />
            <Input
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              placeholder={
                history.recordType === 'online'
                  ? t('Search by order number...')
                  : t('Search by ID, name, or redemption code...')
              }
              className='pl-9'
            />
          </div>
          <Button type='submit' size='icon' aria-label={t('Search')}>
            <Search />
          </Button>
          <Select
            value={String(history.pageSize)}
            onValueChange={(value) =>
              value !== null && history.handlePageSizeChange(Number(value))
            }
          >
            <SelectTrigger className='w-28'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {[10, 20, 50, 100].map((size) => (
                  <SelectItem key={size} value={String(size)}>
                    {t('{{count}} / page', { count: size })}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </form>

        <div className='max-h-[min(54vh,520px)] min-h-40 overflow-y-auto pr-1'>
          {history.loading && (
            <div className='flex flex-col gap-3'>
              {Array.from({ length: 4 }, (_, index) => (
                <Skeleton key={index} className='h-28 w-full' />
              ))}
            </div>
          )}
          {!history.loading && items.length === 0 && (
            <div className='text-muted-foreground flex min-h-40 items-center justify-center text-sm'>
              {t('No billing records found')}
            </div>
          )}
          {!history.loading &&
            items.length > 0 &&
            history.recordType === 'online' && (
              <div className='flex flex-col gap-3'>
                {(items as TopupRecord[]).map((record) => {
                  const statusConfig = getStatusConfig(record.status)
                  return (
                    <div
                      key={record.id}
                      className='rounded-lg border p-3 sm:p-4'
                    >
                      <div className='flex items-start justify-between gap-2'>
                        <div className='flex min-w-0 flex-col gap-1'>
                          <div className='flex min-w-0 items-center gap-1'>
                            <code className='truncate font-mono text-sm'>
                              {record.trade_no}
                            </code>
                            <Button
                              variant='ghost'
                              size='icon-xs'
                              aria-label={t('Copy order number')}
                              onClick={() => copyToClipboard(record.trade_no)}
                            >
                              {copiedText === record.trade_no ? (
                                <Check />
                              ) : (
                                <Copy />
                              )}
                            </Button>
                            {showUserId && (
                              <StatusBadge
                                label={`${t('User ID')}: ${record.user_id}`}
                                variant='neutral'
                                size='sm'
                                copyText={String(record.user_id)}
                              />
                            )}
                          </div>
                          <span className='text-muted-foreground text-xs'>
                            {formatTimestamp(record.create_time)}
                          </span>
                        </div>
                        <StatusBadge
                          label={statusConfig.label}
                          variant={statusConfig.variant}
                          showDot
                          copyable={false}
                        />
                      </div>
                      <div className='mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3'>
                        <BillField
                          label={t('Payment Method')}
                          value={getPaymentMethodName(record.payment_method, t)}
                        />
                        <BillField
                          label={t('Amount')}
                          value={formatCurrencyFromUSD(record.amount, {
                            digitsLarge: 2,
                            digitsSmall: 2,
                            abbreviate: false,
                          })}
                        />
                        <BillField
                          label={t('Payment')}
                          value={formatNumber(record.money)}
                        />
                      </div>
                      {allowCompleteOrder && record.status === 'pending' && (
                        <div className='mt-3 flex justify-end'>
                          <Button
                            size='sm'
                            variant='outline'
                            onClick={() => setConfirmTradeNo(record.trade_no)}
                            disabled={history.completing}
                          >
                            {t('Complete Order')}
                          </Button>
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
          {!history.loading &&
            items.length > 0 &&
            history.recordType === 'redemption' && (
              <div className='flex flex-col gap-3'>
                {(items as RedemptionBillingRecord[]).map((record) => (
                  <div key={record.id} className='rounded-lg border p-3 sm:p-4'>
                    <div className='flex items-start justify-between gap-2'>
                      <div className='flex min-w-0 flex-col gap-1'>
                        <span className='truncate font-medium'>
                          {record.name || '-'}
                        </span>
                        <span className='text-muted-foreground text-xs'>
                          {formatTimestamp(record.redeemed_time)}
                        </span>
                      </div>
                      {showUserId && (
                        <StatusBadge
                          label={`${t('User ID')}: ${record.user_id}`}
                          variant='neutral'
                          size='sm'
                          copyText={String(record.user_id)}
                        />
                      )}
                    </div>
                    <div className='mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3'>
                      <BillField
                        label={t('Record ID')}
                        value={`#${record.id}`}
                      />
                      <BillField
                        label={t('Redemption code')}
                        value={record.code}
                      />
                      <BillField
                        label={t('Credited quota')}
                        value={formatQuota(record.quota)}
                      />
                    </div>
                  </div>
                ))}
              </div>
            )}
        </div>

        {!history.loading && items.length > 0 && (
          <>
            <Separator />
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground text-sm'>
                {t('Showing {{start}}-{{end}} of {{total}}', {
                  start: (history.page - 1) * history.pageSize + 1,
                  end: Math.min(history.page * history.pageSize, total),
                  total,
                })}
              </span>
              <div className='flex items-center gap-2'>
                <Button
                  variant='outline'
                  size='icon-sm'
                  aria-label={t('Go to previous page')}
                  disabled={history.page <= 1}
                  onClick={() => history.handlePageChange(history.page - 1)}
                >
                  <ChevronLeft />
                </Button>
                <span className='min-w-14 text-center text-sm tabular-nums'>
                  {history.page} / {totalPages}
                </span>
                <Button
                  variant='outline'
                  size='icon-sm'
                  aria-label={t('Go to next page')}
                  disabled={history.page >= totalPages}
                  onClick={() => history.handlePageChange(history.page + 1)}
                >
                  <ChevronRight />
                </Button>
              </div>
            </div>
          </>
        )}
      </Dialog>

      <AlertDialog
        open={confirmTradeNo !== null}
        onOpenChange={(open) => !open && setConfirmTradeNo(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Complete Order')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Are you sure you want to manually complete this order? The user will be credited with the corresponding quota.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={history.completing}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={handleConfirmComplete}
              disabled={history.completing}
            >
              {history.completing ? t('Processing...') : t('Confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

interface BillFieldProps {
  label: string
  value: string
}

function BillField(props: BillFieldProps) {
  return (
    <div className='flex min-w-0 flex-col gap-1'>
      <Label className='text-muted-foreground text-xs'>{props.label}</Label>
      <span className='truncate text-sm font-medium'>{props.value}</span>
    </div>
  )
}
