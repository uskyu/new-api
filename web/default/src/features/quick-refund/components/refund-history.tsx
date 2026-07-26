import type { ReactNode } from 'react'
import { AlertCircle, CheckCircle2, Clock, Loader2, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { RefundBatch, RefundBatchPage, RefundItem } from '../types'

type RefundHistoryProps = {
  pageInfo?: RefundBatchPage
  loading: boolean
  fetching: boolean
  selectedBatch: RefundBatch | null
  selectedLoading: boolean
  onSelect: (batch: RefundBatch) => void
  onCloseDetails: () => void
  onPageChange: (page: number) => void
  onDetailPageChange: (page: number) => void
  onRefresh: () => void
}

const statusConfig: Record<
  string,
  {
    label: string
    variant: 'default' | 'secondary' | 'destructive' | 'outline'
  }
> = {
  completed: { label: 'Completed', variant: 'default' },
  failed: { label: 'Failed', variant: 'destructive' },
  pending: { label: 'Pending', variant: 'secondary' },
  running: { label: 'Running', variant: 'secondary' },
  skipped: { label: 'Skipped', variant: 'outline' },
  success: { label: 'Success', variant: 'default' },
}

function formatTime(timestamp: number): string {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

function StatusBadge(props: { status: string }) {
  const { t } = useTranslation()
  const config = statusConfig[props.status] ?? {
    label: props.status || 'Unknown',
    variant: 'outline' as const,
  }
  return <Badge variant={config.variant}>{t(config.label)}</Badge>
}

function parseJsonList(value: string): string[] {
  if (!value) return []
  try {
    const parsed = JSON.parse(value)
    if (Array.isArray(parsed)) return parsed.map((item) => String(item))
  } catch {
    return []
  }
  return []
}

function BatchSummary(props: { batch: RefundBatch; className?: string }) {
  const { t } = useTranslation()
  const { batch } = props
  return (
    <div className={cn('grid gap-2 sm:grid-cols-4', props.className)}>
      <Metric label={t('Matched items')} value={batch.total_items} />
      <Metric label={t('Success')} value={batch.success_items} />
      <Metric label={t('Failed')} value={batch.failed_items} />
      <Metric
        label={t('Refunded quota')}
        value={formatQuota(batch.refunded_quota)}
      />
    </div>
  )
}

function Metric(props: { label: string; value: ReactNode }) {
  return (
    <div className='bg-muted/30 min-w-0 rounded-md border p-3'>
      <div className='text-muted-foreground truncate text-xs'>
        {props.label}
      </div>
      <div className='mt-1 min-w-0 truncate text-lg font-medium'>
        {props.value}
      </div>
    </div>
  )
}

function RefundItemCard(props: { item: RefundItem }) {
  const { t } = useTranslation()
  const { item } = props
  return (
    <div className='flex min-w-0 flex-col gap-2 rounded-md border p-3'>
      <div className='flex min-w-0 items-start justify-between gap-2'>
        <div className='min-w-0'>
          <div className='truncate font-mono text-xs'>
            #{item.source_log_id}
          </div>
          <div className='text-muted-foreground mt-0.5 truncate text-xs'>
            {item.model_name || '-'}
          </div>
        </div>
        <StatusBadge status={item.status} />
      </div>
      <div className='grid grid-cols-2 gap-2 text-xs'>
        <div>
          <span className='text-muted-foreground'>{t('User ID')}: </span>
          <span className='font-mono'>{item.user_id}</span>
        </div>
        <div>
          <span className='text-muted-foreground'>{t('Channel')}: </span>
          <span className='font-mono'>#{item.channel_id}</span>
        </div>
        <div>
          <span className='text-muted-foreground'>{t('Source quota')}: </span>
          <span>{formatQuota(item.source_quota)}</span>
        </div>
        <div>
          <span className='text-muted-foreground'>{t('Refund quota')}: </span>
          <span>{formatQuota(item.refund_quota)}</span>
        </div>
      </div>
      {item.message && (
        <div className='text-muted-foreground min-w-0 text-xs break-words'>
          {item.message}
        </div>
      )}
    </div>
  )
}

export function RefundHistory(props: RefundHistoryProps) {
  const { t } = useTranslation()
  const pageInfo = props.pageInfo
  const batches = pageInfo?.items ?? []
  const page = pageInfo?.page ?? 1
  const pageSize = pageInfo?.page_size ?? 20
  const total = pageInfo?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const itemPage = props.selectedBatch?.item_page ?? 1
  const itemPageSize = props.selectedBatch?.item_page_size ?? 20
  const itemTotal = props.selectedBatch?.item_total ?? 0
  const itemTotalPages = Math.max(1, Math.ceil(itemTotal / itemPageSize))

  return (
    <>
      <Card>
        <CardHeader className='gap-2'>
          <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
            <div>
              <CardTitle>{t('Refund batch history')}</CardTitle>
              <CardDescription>
                {t('View each refund batch and its item results.')}
              </CardDescription>
            </div>
            <Button
              type='button'
              variant='outline'
              onClick={props.onRefresh}
              disabled={props.fetching}
            >
              {props.fetching ? (
                <Loader2 className='animate-spin' />
              ) : (
                <Clock />
              )}
              {t('Refresh')}
            </Button>
          </div>
        </CardHeader>
        <CardContent className='flex flex-col gap-4'>
          {props.loading ? (
            <div className='text-muted-foreground flex items-center gap-2 text-sm'>
              <Loader2 className='animate-spin' />
              {t('Loading...')}
            </div>
          ) : batches.length === 0 ? (
            <Empty className='border'>
              <EmptyHeader>
                <EmptyMedia variant='icon'>
                  <Search />
                </EmptyMedia>
                <EmptyTitle>{t('No refund batches yet.')}</EmptyTitle>
                <EmptyDescription>
                  {t('Create a preview first, then confirm the refund batch.')}
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : (
            <>
              <div className='hidden md:block'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Batch')}</TableHead>
                      <TableHead>{t('Created at')}</TableHead>
                      <TableHead>{t('Ratio')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                      <TableHead>{t('Items')}</TableHead>
                      <TableHead>{t('Refunded quota')}</TableHead>
                      <TableHead className='text-right'>
                        {t('Actions')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {batches.map((batch) => (
                      <TableRow key={batch.id}>
                        <TableCell className='font-mono'>#{batch.id}</TableCell>
                        <TableCell>{formatTime(batch.created_at)}</TableCell>
                        <TableCell>{batch.ratio}%</TableCell>
                        <TableCell>
                          <StatusBadge status={batch.status} />
                        </TableCell>
                        <TableCell>
                          {batch.success_items}/{batch.total_items}
                          {batch.failed_items > 0 && (
                            <span className='text-muted-foreground ml-1'>
                              ({t('Failed')} {batch.failed_items})
                            </span>
                          )}
                        </TableCell>
                        <TableCell>
                          {formatQuota(batch.refunded_quota)}
                        </TableCell>
                        <TableCell className='text-right'>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            onClick={() => props.onSelect(batch)}
                          >
                            {t('Details')}
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              <div className='flex flex-col gap-3 md:hidden'>
                {batches.map((batch) => (
                  <div
                    key={batch.id}
                    className='flex flex-col gap-3 rounded-md border p-3'
                  >
                    <div className='flex items-start justify-between gap-2'>
                      <div className='min-w-0'>
                        <div className='font-mono text-sm'>#{batch.id}</div>
                        <div className='text-muted-foreground text-xs'>
                          {formatTime(batch.created_at)}
                        </div>
                      </div>
                      <StatusBadge status={batch.status} />
                    </div>
                    <BatchSummary batch={batch} className='grid-cols-2' />
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => props.onSelect(batch)}
                    >
                      {t('Details')}
                    </Button>
                  </div>
                ))}
              </div>

              <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                <div className='text-muted-foreground text-sm'>
                  {t('Page {{page}} of {{totalPages}}', { page, totalPages })}
                </div>
                <div className='flex gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={page <= 1}
                    onClick={() => props.onPageChange(page - 1)}
                  >
                    {t('Previous')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={page >= totalPages}
                    onClick={() => props.onPageChange(page + 1)}
                  >
                    {t('Next')}
                  </Button>
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      <Dialog
        open={props.selectedBatch !== null || props.selectedLoading}
        onOpenChange={(open) => !open && props.onCloseDetails()}
      >
        <DialogContent className='max-h-[88vh] overflow-y-auto sm:max-w-4xl'>
          <DialogHeader>
            <DialogTitle>
              {t('Refund batch details')}
              {props.selectedBatch ? ` #${props.selectedBatch.id}` : ''}
            </DialogTitle>
            <DialogDescription>
              {props.selectedBatch?.reason ||
                t('Loading refund batch details.')}
            </DialogDescription>
          </DialogHeader>

          {props.selectedLoading && (
            <div className='text-muted-foreground flex items-center gap-2 text-sm'>
              <Loader2 className='animate-spin' />
              {t('Loading...')}
            </div>
          )}

          {props.selectedBatch && !props.selectedLoading && (
            <div className='flex min-w-0 flex-col gap-4'>
              <div className='flex flex-wrap items-center gap-2'>
                <StatusBadge status={props.selectedBatch.status} />
                {props.selectedBatch.status === 'completed' && <CheckCircle2 />}
                {props.selectedBatch.status === 'failed' && <AlertCircle />}
              </div>
              <BatchSummary batch={props.selectedBatch} />
              <div className='grid gap-3 text-sm sm:grid-cols-2'>
                <div className='min-w-0 rounded-md border p-3'>
                  <div className='text-muted-foreground text-xs'>
                    {t('Time range')}
                  </div>
                  <div className='mt-1 break-words'>
                    {formatTime(props.selectedBatch.start_time)} -{' '}
                    {formatTime(props.selectedBatch.end_time)}
                  </div>
                </div>
                <div className='min-w-0 rounded-md border p-3'>
                  <div className='text-muted-foreground text-xs'>
                    {t('Channels')}
                  </div>
                  <div className='mt-1 break-words'>
                    {parseJsonList(props.selectedBatch.channel_ids).join(
                      ', '
                    ) || '-'}
                  </div>
                </div>
                <div className='min-w-0 rounded-md border p-3 sm:col-span-2'>
                  <div className='text-muted-foreground text-xs'>
                    {t('Models')}
                  </div>
                  <div className='mt-1 break-words'>
                    {parseJsonList(props.selectedBatch.model_names).join(
                      ', '
                    ) || t('All eligible models')}
                  </div>
                </div>
                {props.selectedBatch.error && (
                  <div className='border-destructive text-destructive min-w-0 rounded-md border p-3 sm:col-span-2'>
                    <div className='text-xs'>{t('Error')}</div>
                    <div className='mt-1 break-words'>
                      {props.selectedBatch.error}
                    </div>
                  </div>
                )}
              </div>

              <div className='hidden md:block'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Log ID')}</TableHead>
                      <TableHead>{t('User ID')}</TableHead>
                      <TableHead>{t('Channel')}</TableHead>
                      <TableHead>{t('Model')}</TableHead>
                      <TableHead>{t('Source quota')}</TableHead>
                      <TableHead>{t('Refund quota')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                      <TableHead>{t('Message')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(props.selectedBatch.items ?? []).map((item) => (
                      <TableRow key={item.id}>
                        <TableCell className='font-mono'>
                          #{item.source_log_id}
                        </TableCell>
                        <TableCell className='font-mono'>
                          {item.user_id}
                        </TableCell>
                        <TableCell className='font-mono'>
                          #{item.channel_id}
                        </TableCell>
                        <TableCell className='max-w-56 truncate'>
                          {item.model_name || '-'}
                        </TableCell>
                        <TableCell>{formatQuota(item.source_quota)}</TableCell>
                        <TableCell>{formatQuota(item.refund_quota)}</TableCell>
                        <TableCell>
                          <StatusBadge status={item.status} />
                        </TableCell>
                        <TableCell className='max-w-64 truncate'>
                          {item.message || '-'}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
              <div className='flex flex-col gap-3 md:hidden'>
                {(props.selectedBatch.items ?? []).map((item) => (
                  <RefundItemCard key={item.id} item={item} />
                ))}
              </div>
              {itemTotal > itemPageSize && (
                <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                  <div className='text-muted-foreground text-sm'>
                    {t('Page {{page}} of {{totalPages}}', {
                      page: itemPage,
                      totalPages: itemTotalPages,
                    })}
                  </div>
                  <div className='flex gap-2'>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      disabled={itemPage <= 1 || props.selectedLoading}
                      onClick={() => props.onDetailPageChange(itemPage - 1)}
                    >
                      {t('Previous')}
                    </Button>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      disabled={
                        itemPage >= itemTotalPages || props.selectedLoading
                      }
                      onClick={() => props.onDetailPageChange(itemPage + 1)}
                    >
                      {t('Next')}
                    </Button>
                  </div>
                </div>
              )}
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  )
}
