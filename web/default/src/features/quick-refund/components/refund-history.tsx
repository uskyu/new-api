import { useTranslation } from 'react-i18next'
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatQuota } from '@/lib/format'
import type { RefundBatch } from '../types'

type RefundHistoryProps = {
  batches: RefundBatch[]
  loading: boolean
  selectedBatch: RefundBatch | null
  onSelect: (batch: RefundBatch) => void
  onCloseDetails: () => void
}

function formatTime(timestamp: number): string {
  return new Date(timestamp * 1000).toLocaleString()
}

function StatusBadge(props: { status: string }) {
  const { t } = useTranslation()
  let variant: 'default' | 'secondary' | 'destructive' | 'outline' = 'outline'
  if (props.status === 'completed') variant = 'default'
  if (props.status === 'running' || props.status === 'pending') variant = 'secondary'
  if (props.status === 'failed') variant = 'destructive'
  return <Badge variant={variant}>{t(props.status)}</Badge>
}

export function RefundHistory(props: RefundHistoryProps) {
  const { t } = useTranslation()
  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t('Refund batch history')}</CardTitle>
          <CardDescription>
            {t('The latest 20 quick refund batches are shown here.')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Batch')}</TableHead>
                <TableHead>{t('Created at')}</TableHead>
                <TableHead>{t('Ratio')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Success')}</TableHead>
                <TableHead>{t('Refunded quota')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {props.loading && (
                <TableRow>
                  <TableCell colSpan={7}>{t('Loading...')}</TableCell>
                </TableRow>
              )}
              {!props.loading && props.batches.length === 0 && (
                <TableRow>
                  <TableCell colSpan={7}>{t('No refund batches yet.')}</TableCell>
                </TableRow>
              )}
              {props.batches.map((batch) => (
                <TableRow key={batch.id}>
                  <TableCell>#{batch.id}</TableCell>
                  <TableCell>{formatTime(batch.created_at)}</TableCell>
                  <TableCell>{batch.ratio}%</TableCell>
                  <TableCell><StatusBadge status={batch.status} /></TableCell>
                  <TableCell>{batch.success_items}/{batch.total_items}</TableCell>
                  <TableCell>{formatQuota(batch.refunded_quota)}</TableCell>
                  <TableCell className='text-right'>
                    <Button variant='outline' size='sm' onClick={() => props.onSelect(batch)}>
                      {t('Details')}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Dialog open={props.selectedBatch !== null} onOpenChange={(open) => !open && props.onCloseDetails()}>
        <DialogContent className='max-h-[80vh] overflow-y-auto sm:max-w-3xl'>
          <DialogHeader>
            <DialogTitle>{t('Refund batch details')} #{props.selectedBatch?.id}</DialogTitle>
            <DialogDescription>{props.selectedBatch?.reason}</DialogDescription>
          </DialogHeader>
          {props.selectedBatch && (
            <div className='space-y-4'>
              <div className='grid gap-3 sm:grid-cols-3'>
                <div><div className='text-muted-foreground text-xs'>{t('Status')}</div><StatusBadge status={props.selectedBatch.status} /></div>
                <div><div className='text-muted-foreground text-xs'>{t('Refunded quota')}</div><div>{formatQuota(props.selectedBatch.refunded_quota)}</div></div>
                <div><div className='text-muted-foreground text-xs'>{t('Failed')}</div><div>{props.selectedBatch.failed_items}</div></div>
              </div>
              <Table>
                <TableHeader><TableRow><TableHead>{t('Log ID')}</TableHead><TableHead>{t('User ID')}</TableHead><TableHead>{t('Model')}</TableHead><TableHead>{t('Refund quota')}</TableHead><TableHead>{t('Status')}</TableHead><TableHead>{t('Message')}</TableHead></TableRow></TableHeader>
                <TableBody>
                  {(props.selectedBatch.items ?? []).map((item) => (
                    <TableRow key={item.id}><TableCell>{item.source_log_id}</TableCell><TableCell>{item.user_id}</TableCell><TableCell>{item.model_name}</TableCell><TableCell>{formatQuota(item.refund_quota)}</TableCell><TableCell><StatusBadge status={item.status} /></TableCell><TableCell className='max-w-48 truncate'>{item.message || '-'}</TableCell></TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  )
}
