import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { formatQuota } from '@/lib/format'
import {
  createRefundBatch,
  getRefundBatch,
  getRefundBatches,
  getRefundOptions,
  previewRefund,
} from './api'
import { RefundHistory } from './components/refund-history'
import {
  quickRefundSchema,
  type QuickRefundFormValues,
} from './lib/schema'
import type { RefundBatch, RefundPayload } from './types'

function toLocalInput(date: Date): string {
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

const now = new Date()
const defaultStart = new Date(now.getTime() - 60 * 60 * 1000)

export function QuickRefund() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [selectedBatch, setSelectedBatch] = useState<RefundBatch | null>(null)
  const form = useForm<QuickRefundFormValues>({
    resolver: zodResolver(quickRefundSchema),
    defaultValues: {
      startTime: toLocalInput(defaultStart),
      endTime: toLocalInput(now),
      channelIds: [],
      modelNames: [],
      ratio: 100,
      reason: '',
    },
  })
  const values = form.watch()
  const startTimestamp = Math.floor(new Date(values.startTime).getTime() / 1000)
  const endTimestamp = Math.floor(new Date(values.endTime).getTime() / 1000)
  const validRange = startTimestamp > 0 && endTimestamp >= startTimestamp

  const optionsQuery = useQuery({
    queryKey: ['quick-refund', 'options', startTimestamp, endTimestamp],
    queryFn: () => getRefundOptions(startTimestamp, endTimestamp),
    enabled: validRange,
  })
  const batchesQuery = useQuery({
    queryKey: ['quick-refund', 'batches'],
    queryFn: getRefundBatches,
    refetchInterval: (query) => {
      const batches = query.state.data?.data.items ?? []
      return batches.some((batch) => ['pending', 'running'].includes(batch.status))
        ? 3000
        : false
    },
  })
  const options = optionsQuery.data?.data

  useEffect(() => {
    if (!options) return
    const availableChannels = new Set(options.channels.map((channel) => channel.id))
    const availableModels = new Set(options.models)
    form.setValue(
      'channelIds',
      form.getValues('channelIds').filter((id) => availableChannels.has(id))
    )
    form.setValue(
      'modelNames',
      form.getValues('modelNames').filter((model) => availableModels.has(model))
    )
  }, [options, form])

  const payload = useMemo<RefundPayload>(() => ({
    start_time: startTimestamp,
    end_time: endTimestamp,
    channel_ids: values.channelIds,
    model_names: values.modelNames,
    ratio: Number(values.ratio),
    reason: values.reason.trim(),
  }), [endTimestamp, startTimestamp, values.channelIds, values.modelNames, values.ratio, values.reason])

  const previewMutation = useMutation({
    mutationFn: previewRefund,
    onSuccess: (result) => {
      if (result.success) toast.success(t('Refund preview updated.'))
    },
  })
  const createMutation = useMutation({
    mutationFn: createRefundBatch,
    onSuccess: async (result) => {
      if (!result.success) return
      toast.success(t('Refund batch created.'))
      setConfirmOpen(false)
      previewMutation.reset()
      await queryClient.invalidateQueries({ queryKey: ['quick-refund', 'batches'] })
    },
  })

  const submitPreview = form.handleSubmit((data) => {
    previewMutation.mutate({ ...payload, ratio: data.ratio, reason: data.reason.trim() })
  })
  const preview = previewMutation.data?.data

  const toggleNumber = (field: 'channelIds', value: number, checked: boolean) => {
    const current = form.getValues(field)
    form.setValue(field, checked ? [...current, value] : current.filter((item) => item !== value), { shouldValidate: true })
    previewMutation.reset()
  }
  const toggleString = (value: string, checked: boolean) => {
    const current = form.getValues('modelNames')
    form.setValue('modelNames', checked ? [...current, value] : current.filter((item) => item !== value))
    previewMutation.reset()
  }
  const openBatch = async (batch: RefundBatch) => {
    const result = await getRefundBatch(batch.id)
    if (result.success) setSelectedBatch(result.data)
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Quick Refund')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button variant='outline' onClick={() => batchesQuery.refetch()} disabled={batchesQuery.isFetching}>
          <RefreshCw className='size-4' /> {t('Refresh history')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <Card>
            <CardHeader>
              <CardTitle>{t('Create a quick refund')}</CardTitle>
              <CardDescription>{t('Refund wallet consumption matching the selected time, channels, and models.')}</CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={submitPreview} className='space-y-5'>
                <div className='grid gap-4 sm:grid-cols-2'>
                  <div className='space-y-2'><Label htmlFor='refund-start'>{t('Start time')}</Label><Input id='refund-start' type='datetime-local' {...form.register('startTime')} /></div>
                  <div className='space-y-2'><Label htmlFor='refund-end'>{t('End time')}</Label><Input id='refund-end' type='datetime-local' {...form.register('endTime')} /><p className='text-destructive text-xs'>{form.formState.errors.endTime && t(form.formState.errors.endTime.message ?? '')}</p></div>
                </div>

                <div className='space-y-2'>
                  <div className='flex items-center justify-between'><Label>{t('Channels')}</Label><Button type='button' size='sm' variant='ghost' onClick={() => form.setValue('channelIds', options?.channels.map((channel) => channel.id) ?? [], { shouldValidate: true })}>{t('Select all')}</Button></div>
                  <div className='grid max-h-40 gap-2 overflow-y-auto rounded-lg border p-3 sm:grid-cols-2 lg:grid-cols-3'>
                    {optionsQuery.isLoading && <span className='text-muted-foreground'>{t('Loading...')}</span>}
                    {!optionsQuery.isLoading && options?.channels.length === 0 && <span className='text-muted-foreground'>{t('No eligible channels in this time range.')}</span>}
                    {options?.channels.map((channel) => <Label key={channel.id} className='font-normal'><Checkbox checked={values.channelIds.includes(channel.id)} onCheckedChange={(checked) => toggleNumber('channelIds', channel.id, checked === true)} />{channel.name || `#${channel.id}`}</Label>)}
                  </div>
                  <p className='text-destructive text-xs'>{form.formState.errors.channelIds && t('Select at least one channel.')}</p>
                </div>

                <div className='space-y-2'>
                  <div className='flex items-center justify-between'><Label>{t('Models (optional)')}</Label><Button type='button' size='sm' variant='ghost' onClick={() => form.setValue('modelNames', [])}>{t('Clear')}</Button></div>
                  <div className='grid max-h-40 gap-2 overflow-y-auto rounded-lg border p-3 sm:grid-cols-2 lg:grid-cols-3'>
                    {options?.models.length === 0 && <span className='text-muted-foreground'>{t('No eligible models in this time range.')}</span>}
                    {options?.models.map((model) => <Label key={model} className='font-normal'><Checkbox checked={values.modelNames.includes(model)} onCheckedChange={(checked) => toggleString(model, checked === true)} />{model}</Label>)}
                  </div>
                  <p className='text-muted-foreground text-xs'>{t('Leave all models unselected to include every model on the selected channels.')}</p>
                </div>

                <div className='grid gap-4 sm:grid-cols-2'>
                  <div className='space-y-2'><Label htmlFor='refund-ratio'>{t('Refund ratio')}</Label><div className='flex flex-wrap gap-2'>{[25, 50, 75, 100].map((ratio) => <Button key={ratio} type='button' size='sm' variant={values.ratio === ratio ? 'default' : 'outline'} onClick={() => form.setValue('ratio', ratio, { shouldValidate: true })}>{ratio}%</Button>)}<Input id='refund-ratio' className='w-24' type='number' min={1} max={100} {...form.register('ratio')} /></div><p className='text-destructive text-xs'>{form.formState.errors.ratio && t('Ratio must be between 1 and 100.')}</p></div>
                  <div className='space-y-2'><Label htmlFor='refund-reason'>{t('Refund reason')}</Label><Textarea id='refund-reason' maxLength={500} placeholder={t('Describe why this refund is needed.')} {...form.register('reason')} /><p className='text-destructive text-xs'>{form.formState.errors.reason && t('Refund reason is required.')}</p></div>
                </div>

                <Button type='submit' disabled={previewMutation.isPending || optionsQuery.isLoading}>{previewMutation.isPending ? t('Previewing...') : t('Preview refund')}</Button>
              </form>
            </CardContent>
          </Card>

          {preview && <Card><CardHeader><CardTitle>{t('Preview statistics')}</CardTitle><CardDescription>{t('Review the result carefully before creating the refund batch.')}</CardDescription></CardHeader><CardContent><div className='grid gap-3 sm:grid-cols-4'>{[[t('Matched items'), preview.matched_items], [t('Source quota'), formatQuota(preview.source_quota)], [t('Refund quota'), formatQuota(preview.refund_quota)], [t('Skipped'), preview.skipped]].map(([label, value]) => <div key={label} className='rounded-lg border p-3'><div className='text-muted-foreground text-xs'>{label}</div><div className='mt-1 text-lg font-medium'>{value}</div></div>)}</div><Button className='mt-4' variant='destructive' disabled={preview.matched_items === 0} onClick={() => setConfirmOpen(true)}>{t('Create refund batch')}</Button></CardContent></Card>}

          <RefundHistory batches={batchesQuery.data?.data.items ?? []} loading={batchesQuery.isLoading} selectedBatch={selectedBatch} onSelect={openBatch} onCloseDetails={() => setSelectedBatch(null)} />
        </div>

        <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}><DialogContent><DialogHeader><DialogTitle>{t('Confirm quick refund')}</DialogTitle><DialogDescription>{t('This will immediately credit matching users and cannot be undone.')}</DialogDescription></DialogHeader><div className='space-y-2'><p>{t('{{count}} items will receive {{quota}}.', { count: preview?.matched_items ?? 0, quota: formatQuota(preview?.refund_quota ?? 0) })}</p><p className='font-medium'>{t('Reason')}: {values.reason}</p></div><DialogFooter><Button variant='outline' onClick={() => setConfirmOpen(false)}>{t('Cancel')}</Button><Button variant='destructive' disabled={createMutation.isPending} onClick={() => createMutation.mutate({ ...payload, idempotency_key: crypto.randomUUID() })}>{createMutation.isPending ? t('Refunding...') : t('Confirm refund')}</Button></DialogFooter></DialogContent></Dialog>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
