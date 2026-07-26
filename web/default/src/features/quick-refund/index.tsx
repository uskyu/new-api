import { type ReactNode, useEffect, useMemo, useState } from 'react'
import { type Resolver, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  CheckCircle2,
  Clock,
  Loader2,
  RefreshCw,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { formatQuota } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
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
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { SectionPageLayout } from '@/components/layout'
import {
  createRefundBatch,
  getRefundBatch,
  getRefundBatches,
  getRefundOptions,
  previewRefund,
} from './api'
import { RefundHistory } from './components/refund-history'
import { quickRefundSchema, type QuickRefundFormValues } from './lib/schema'
import type {
  RefundBatch,
  RefundBreakdown,
  RefundPayload,
  RefundPreview,
} from './types'

const HISTORY_PAGE_SIZE = 20
const QUICK_RATIO_OPTIONS = [25, 50, 75, 100] as const

function toLocalInput(date: Date): string {
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

function timestampFromLocal(value: string): number {
  const time = new Date(value).getTime()
  if (!Number.isFinite(time)) return 0
  return Math.floor(time / 1000)
}

function buildPayload(values: QuickRefundFormValues): RefundPayload {
  return {
    start_time: timestampFromLocal(values.startTime),
    end_time: timestampFromLocal(values.endTime),
    channel_ids: values.channelIds,
    model_names: values.modelNames,
    ratio: Number(values.ratio),
    reason: values.reason.trim(),
  }
}

function payloadKey(payload: RefundPayload): string {
  return JSON.stringify(payload)
}

function StatBlock(props: {
  label: string
  value: ReactNode
  muted?: boolean
}) {
  return (
    <div className='bg-muted/30 min-w-0 rounded-md border p-3'>
      <div className='text-muted-foreground truncate text-xs'>
        {props.label}
      </div>
      <div
        className={cn(
          'mt-1 min-w-0 truncate text-xl font-semibold',
          props.muted && 'text-muted-foreground'
        )}
      >
        {props.value}
      </div>
    </div>
  )
}

function BreakdownList(props: {
  title: string
  data: Record<string, RefundBreakdown>
  labels?: Record<string, string>
}) {
  const { t } = useTranslation()
  const entries = Object.entries(props.data ?? {}).sort(
    (a, b) => b[1].refund_quota - a[1].refund_quota
  )
  if (entries.length === 0) return null

  return (
    <div className='flex min-w-0 flex-col gap-2 rounded-md border p-3'>
      <div className='font-medium'>{props.title}</div>
      <div className='flex max-h-64 flex-col gap-2 overflow-y-auto pr-1'>
        {entries.map(([key, value]) => (
          <div
            key={key}
            className='bg-muted/20 flex min-w-0 flex-col gap-1 rounded-md p-2 text-sm sm:flex-row sm:items-center sm:justify-between'
          >
            <div className='min-w-0'>
              <div className='truncate font-medium'>
                {props.labels?.[key] ?? key}
              </div>
              <div className='text-muted-foreground text-xs'>
                {t('{{count}} items', { count: value.items })}
              </div>
            </div>
            <div className='shrink-0 font-mono text-sm'>
              {formatQuota(value.refund_quota)}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function createDefaultValues(): QuickRefundFormValues {
  const now = new Date()
  return {
    startTime: toLocalInput(new Date(now.getTime() - 60 * 60 * 1000)),
    endTime: toLocalInput(now),
    channelIds: [],
    modelNames: [],
    ratio: 100,
    reason: '',
  }
}

export function QuickRefund() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [selectedBatch, setSelectedBatch] = useState<RefundBatch | null>(null)
  const [selectedLoading, setSelectedLoading] = useState(false)
  const [selectedItemPage, setSelectedItemPage] = useState(1)
  const [historyPage, setHistoryPage] = useState(1)
  const [previewPayload, setPreviewPayload] = useState<RefundPayload | null>(
    null
  )
  const currentUserRole = useAuthStore((state) => state.auth.user?.role)
  const canCreateBatch = (currentUserRole ?? ROLE.GUEST) >= ROLE.SUPER_ADMIN

  const form = useForm<QuickRefundFormValues>({
    resolver: zodResolver(quickRefundSchema) as Resolver<QuickRefundFormValues>,
    defaultValues: createDefaultValues(),
  })
  const values = form.watch()
  const currentPayload = useMemo(() => buildPayload(values), [values])
  const currentPayloadKey = useMemo(
    () => payloadKey(currentPayload),
    [currentPayload]
  )
  const previewIsCurrent =
    previewPayload !== null && payloadKey(previewPayload) === currentPayloadKey

  const validRange =
    currentPayload.start_time > 0 &&
    currentPayload.end_time >= currentPayload.start_time

  const optionsQuery = useQuery({
    queryKey: [
      'quick-refund',
      'options',
      currentPayload.start_time,
      currentPayload.end_time,
    ],
    queryFn: () =>
      getRefundOptions(currentPayload.start_time, currentPayload.end_time),
    enabled: validRange,
  })

  const batchesQuery = useQuery({
    queryKey: ['quick-refund', 'batches', historyPage],
    queryFn: () => getRefundBatches(historyPage, HISTORY_PAGE_SIZE),
    refetchInterval: (query) => {
      const batches = query.state.data?.data.items ?? []
      return batches.some((batch) =>
        ['pending', 'running'].includes(batch.status)
      )
        ? 3000
        : false
    },
  })

  const options = optionsQuery.data?.data
  const channelLabels = useMemo(() => {
    const labels: Record<string, string> = {}
    for (const channel of options?.channels ?? []) {
      labels[String(channel.id)] = channel.name
        ? `${channel.name} #${channel.id}`
        : `#${channel.id}`
    }
    return labels
  }, [options?.channels])

  useEffect(() => {
    if (!options) return
    const availableChannels = new Set(
      options.channels.map((channel) => channel.id)
    )
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

  const previewMutation = useMutation({
    mutationFn: previewRefund,
    onSuccess: (result, variables) => {
      if (!result.success) return
      setPreviewPayload(variables)
      toast.success(t('Refund preview updated.'))
    },
  })

  const createMutation = useMutation({
    mutationFn: createRefundBatch,
    onSuccess: async (result) => {
      if (!result.success) return
      setConfirmOpen(false)
      setPreviewPayload(null)
      previewMutation.reset()
      toast.success(t('Refund batch submitted for background processing.'))
      setHistoryPage(1)
      await queryClient.invalidateQueries({ queryKey: ['quick-refund'] })
      setSelectedLoading(true)
      setSelectedItemPage(1)
      try {
        const detail = await getRefundBatch(result.data.id, 1)
        if (detail.success) setSelectedBatch(detail.data)
      } finally {
        setSelectedLoading(false)
      }
    },
  })

  const preview = previewIsCurrent
    ? (previewMutation.data?.data as RefundPreview | undefined)
    : undefined

  const resetPreview = () => {
    setPreviewPayload(null)
    previewMutation.reset()
  }

  const submitPreview = form.handleSubmit((data) => {
    const nextPayload = buildPayload(data)
    previewMutation.mutate(nextPayload)
  })

  const setChannelChecked = (value: number, checked: boolean) => {
    const current = form.getValues('channelIds')
    form.setValue(
      'channelIds',
      checked ? [...current, value] : current.filter((item) => item !== value),
      { shouldDirty: true, shouldValidate: true }
    )
    resetPreview()
  }

  const setModelChecked = (value: string, checked: boolean) => {
    const current = form.getValues('modelNames')
    form.setValue(
      'modelNames',
      checked ? [...current, value] : current.filter((item) => item !== value),
      { shouldDirty: true, shouldValidate: true }
    )
    resetPreview()
  }

  const selectAllChannels = () => {
    form.setValue(
      'channelIds',
      options?.channels.map((channel) => channel.id) ?? [],
      { shouldDirty: true, shouldValidate: true }
    )
    resetPreview()
  }

  const selectAllModels = () => {
    form.setValue('modelNames', options?.models ?? [], {
      shouldDirty: true,
      shouldValidate: true,
    })
    resetPreview()
  }

  const clearModels = () => {
    form.setValue('modelNames', [], { shouldDirty: true })
    resetPreview()
  }

  const openBatch = async (batch: RefundBatch, itemPage = 1) => {
    setSelectedLoading(true)
    setSelectedBatch(batch)
    setSelectedItemPage(itemPage)
    try {
      const result = await getRefundBatch(batch.id, itemPage)
      if (result.success) setSelectedBatch(result.data)
    } finally {
      setSelectedLoading(false)
    }
  }

  useEffect(() => {
    if (
      !selectedBatch?.id ||
      !['pending', 'running'].includes(selectedBatch.status)
    ) {
      return undefined
    }
    const timer = window.setInterval(async () => {
      try {
        const result = await getRefundBatch(selectedBatch.id, selectedItemPage)
        if (result.success) {
          setSelectedBatch(result.data)
          if (!['pending', 'running'].includes(result.data.status)) {
            await queryClient.invalidateQueries({ queryKey: ['quick-refund'] })
          }
        }
      } catch {
        // The next polling interval retries transient request failures.
      }
    }, 3000)
    return () => window.clearInterval(timer)
  }, [queryClient, selectedBatch?.id, selectedBatch?.status, selectedItemPage])

  const confirmRefund = async () => {
    if (!preview || !previewPayload) return
    const idempotencyKey = crypto.randomUUID()
    await createMutation.mutateAsync({
      ...previewPayload,
      idempotency_key: idempotencyKey,
    })
  }

  const previewDisabled =
    previewMutation.isPending || optionsQuery.isLoading || !validRange
  const canConfirm =
    canCreateBatch &&
    !!preview &&
    preview.matched_items > 0 &&
    preview.refund_quota > 0

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Quick Refund')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          onClick={() => batchesQuery.refetch()}
          disabled={batchesQuery.isFetching}
        >
          {batchesQuery.isFetching ? (
            <Loader2 className='animate-spin' />
          ) : (
            <RefreshCw />
          )}
          {t('Refresh history')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <Alert>
            <AlertTriangle />
            <AlertTitle>
              {t('Quick refund is a high-risk wallet operation.')}
            </AlertTitle>
            <AlertDescription>
              {t(
                'Only wallet-billed consume logs are eligible. Subscription or unknown billing sources are skipped, and each source log can only be refunded up to its original cost.'
              )}
              {!canCreateBatch && (
                <span className='mt-1 block'>
                  {t(
                    'Only super administrators can create refund batches. Admins may preview and inspect history.'
                  )}
                </span>
              )}
            </AlertDescription>
          </Alert>

          <Card>
            <CardHeader>
              <CardTitle>{t('Create a quick refund')}</CardTitle>
              <CardDescription>
                {t(
                  'Select a precise time range first, preview the impact, then verify and create the batch.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={submitPreview}>
                <FieldGroup>
                  <div className='grid gap-4 sm:grid-cols-2'>
                    <Field data-invalid={!!form.formState.errors.startTime}>
                      <FieldLabel htmlFor='refund-start'>
                        {t('Start time')}
                      </FieldLabel>
                      <Input
                        id='refund-start'
                        type='datetime-local'
                        aria-invalid={!!form.formState.errors.startTime}
                        {...form.register('startTime', {
                          onChange: resetPreview,
                        })}
                      />
                      <FieldError errors={[form.formState.errors.startTime]} />
                    </Field>
                    <Field data-invalid={!!form.formState.errors.endTime}>
                      <FieldLabel htmlFor='refund-end'>
                        {t('End time')}
                      </FieldLabel>
                      <Input
                        id='refund-end'
                        type='datetime-local'
                        aria-invalid={!!form.formState.errors.endTime}
                        {...form.register('endTime', {
                          onChange: resetPreview,
                        })}
                      />
                      <FieldError errors={[form.formState.errors.endTime]} />
                    </Field>
                  </div>

                  <FieldSet>
                    <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                      <div>
                        <FieldLegend>{t('Channels')}</FieldLegend>
                        <FieldDescription>
                          {t(
                            'Only channels with eligible wallet consume logs in this time range are listed.'
                          )}
                        </FieldDescription>
                      </div>
                      <Button
                        type='button'
                        size='sm'
                        variant='outline'
                        onClick={selectAllChannels}
                        disabled={!options?.channels.length}
                      >
                        {t('Select all')}
                      </Button>
                    </div>
                    <div className='grid max-h-56 gap-2 overflow-y-auto rounded-md border p-3 sm:grid-cols-2 xl:grid-cols-3'>
                      {optionsQuery.isLoading && (
                        <div className='text-muted-foreground flex items-center gap-2 text-sm'>
                          <Loader2 className='animate-spin' />
                          {t('Loading...')}
                        </div>
                      )}
                      {optionsQuery.isError && (
                        <div className='text-destructive text-sm'>
                          {t('Failed to load refund options.')}
                        </div>
                      )}
                      {!optionsQuery.isLoading &&
                        !optionsQuery.isError &&
                        options?.channels.length === 0 && (
                          <Empty className='border-0 p-2'>
                            <EmptyHeader>
                              <EmptyMedia variant='icon'>
                                <Clock />
                              </EmptyMedia>
                              <EmptyTitle>
                                {t('No eligible channels in this time range.')}
                              </EmptyTitle>
                            </EmptyHeader>
                          </Empty>
                        )}
                      {options?.channels.map((channel) => (
                        <label
                          key={channel.id}
                          className='hover:bg-muted/40 flex min-w-0 cursor-pointer items-center gap-2 rounded-md p-2 text-sm'
                        >
                          <Checkbox
                            checked={values.channelIds.includes(channel.id)}
                            onCheckedChange={(checked) =>
                              setChannelChecked(channel.id, checked === true)
                            }
                          />
                          <span className='min-w-0 truncate'>
                            {channel.name || `#${channel.id}`}
                          </span>
                          <span className='text-muted-foreground shrink-0 font-mono text-xs'>
                            #{channel.id}
                          </span>
                        </label>
                      ))}
                    </div>
                    <FieldError
                      errors={
                        form.formState.errors.channelIds
                          ? [{ message: t('Select at least one channel.') }]
                          : []
                      }
                    />
                  </FieldSet>

                  <FieldSet>
                    <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                      <div>
                        <FieldLegend>{t('Models')}</FieldLegend>
                        <FieldDescription>
                          {t(
                            'Leave models empty to include every eligible model on the selected channels.'
                          )}
                        </FieldDescription>
                      </div>
                      <div className='flex gap-2'>
                        <Button
                          type='button'
                          size='sm'
                          variant='outline'
                          onClick={selectAllModels}
                          disabled={!options?.models.length}
                        >
                          {t('Select all')}
                        </Button>
                        <Button
                          type='button'
                          size='sm'
                          variant='outline'
                          onClick={clearModels}
                          disabled={values.modelNames.length === 0}
                        >
                          {t('Clear')}
                        </Button>
                      </div>
                    </div>
                    <div className='grid max-h-56 gap-2 overflow-y-auto rounded-md border p-3 sm:grid-cols-2 xl:grid-cols-3'>
                      {!optionsQuery.isLoading &&
                        options?.models.length === 0 && (
                          <Empty className='border-0 p-2'>
                            <EmptyHeader>
                              <EmptyMedia variant='icon'>
                                <Clock />
                              </EmptyMedia>
                              <EmptyTitle>
                                {t('No eligible models in this time range.')}
                              </EmptyTitle>
                            </EmptyHeader>
                          </Empty>
                        )}
                      {options?.models.map((model) => (
                        <label
                          key={model}
                          className='hover:bg-muted/40 flex min-w-0 cursor-pointer items-center gap-2 rounded-md p-2 text-sm'
                        >
                          <Checkbox
                            checked={values.modelNames.includes(model)}
                            onCheckedChange={(checked) =>
                              setModelChecked(model, checked === true)
                            }
                          />
                          <span className='min-w-0 truncate font-mono text-xs'>
                            {model}
                          </span>
                        </label>
                      ))}
                    </div>
                    <div className='text-muted-foreground flex flex-wrap gap-2 text-xs'>
                      <Badge variant='outline'>
                        {t('{{count}} selected channels', {
                          count: values.channelIds.length,
                        })}
                      </Badge>
                      <Badge variant='outline'>
                        {values.modelNames.length === 0
                          ? t('All eligible models')
                          : t('{{count}} selected models', {
                              count: values.modelNames.length,
                            })}
                      </Badge>
                    </div>
                  </FieldSet>

                  <div className='grid gap-4 sm:grid-cols-2'>
                    <Field data-invalid={!!form.formState.errors.ratio}>
                      <FieldLabel htmlFor='refund-ratio'>
                        {t('Refund ratio')}
                      </FieldLabel>
                      <div className='flex flex-wrap gap-2'>
                        {QUICK_RATIO_OPTIONS.map((ratio) => (
                          <Button
                            key={ratio}
                            type='button'
                            size='sm'
                            variant={
                              values.ratio === ratio ? 'default' : 'outline'
                            }
                            onClick={() => {
                              form.setValue('ratio', ratio, {
                                shouldDirty: true,
                                shouldValidate: true,
                              })
                              resetPreview()
                            }}
                          >
                            {ratio}%
                          </Button>
                        ))}
                        <Input
                          id='refund-ratio'
                          className='w-28'
                          type='number'
                          min={1}
                          max={100}
                          aria-invalid={!!form.formState.errors.ratio}
                          {...form.register('ratio', {
                            onChange: resetPreview,
                          })}
                        />
                      </div>
                      <FieldDescription>
                        {t(
                          'The final refund is capped by each source log remaining refundable quota.'
                        )}
                      </FieldDescription>
                      <FieldError
                        errors={
                          form.formState.errors.ratio
                            ? [
                                {
                                  message: t(
                                    'Ratio must be between 1 and 100.'
                                  ),
                                },
                              ]
                            : []
                        }
                      />
                    </Field>

                    <Field data-invalid={!!form.formState.errors.reason}>
                      <FieldLabel htmlFor='refund-reason'>
                        {t('Refund reason')}
                      </FieldLabel>
                      <Textarea
                        id='refund-reason'
                        maxLength={500}
                        placeholder={t('Describe why this refund is needed.')}
                        aria-invalid={!!form.formState.errors.reason}
                        {...form.register('reason', { onChange: resetPreview })}
                      />
                      <FieldError
                        errors={
                          form.formState.errors.reason
                            ? [{ message: t('Refund reason is required.') }]
                            : []
                        }
                      />
                    </Field>
                  </div>

                  <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
                    <Button type='submit' disabled={previewDisabled}>
                      {previewMutation.isPending ? (
                        <Loader2 className='animate-spin' />
                      ) : (
                        <CheckCircle2 />
                      )}
                      {previewMutation.isPending
                        ? t('Previewing...')
                        : t('Preview refund')}
                    </Button>
                    {previewMutation.data && !previewIsCurrent && (
                      <span className='text-muted-foreground text-sm'>
                        {t('Filters changed. Preview again before confirming.')}
                      </span>
                    )}
                  </div>
                </FieldGroup>
              </form>
            </CardContent>
          </Card>

          {preview && (
            <Card>
              <CardHeader>
                <CardTitle>{t('Preview statistics')}</CardTitle>
                <CardDescription>
                  {t(
                    'Review the result carefully. The execution amount will use the same refundable quota calculation as this preview.'
                  )}
                </CardDescription>
              </CardHeader>
              <CardContent className='flex flex-col gap-4'>
                <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
                  <StatBlock
                    label={t('Matched items')}
                    value={preview.matched_items}
                  />
                  <StatBlock
                    label={t('Source quota')}
                    value={formatQuota(preview.source_quota)}
                  />
                  <StatBlock
                    label={t('Refund quota')}
                    value={formatQuota(preview.refund_quota)}
                  />
                  <StatBlock
                    label={t('Skipped')}
                    value={preview.skipped}
                    muted
                  />
                </div>

                <div className='grid gap-3 lg:grid-cols-2'>
                  <BreakdownList
                    title={t('By channel')}
                    data={preview.by_channel}
                    labels={channelLabels}
                  />
                  <BreakdownList
                    title={t('By model')}
                    data={preview.by_model}
                  />
                </div>

                <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
                  <Button
                    type='button'
                    variant='destructive'
                    disabled={!canConfirm}
                    onClick={() => setConfirmOpen(true)}
                  >
                    <AlertTriangle />
                    {t('Create refund batch')}
                  </Button>
                  {!canConfirm && (
                    <span className='text-muted-foreground text-sm'>
                      {!canCreateBatch
                        ? t(
                            'Super administrator permission is required to create refund batches.'
                          )
                        : t(
                            'No refundable wallet consumption remains for this preview.'
                          )}
                    </span>
                  )}
                </div>
              </CardContent>
            </Card>
          )}

          <RefundHistory
            pageInfo={batchesQuery.data?.data}
            loading={batchesQuery.isLoading}
            fetching={batchesQuery.isFetching}
            selectedBatch={selectedBatch}
            selectedLoading={selectedLoading}
            onSelect={openBatch}
            onCloseDetails={() => {
              setSelectedBatch(null)
              setSelectedLoading(false)
              setSelectedItemPage(1)
            }}
            onDetailPageChange={(page) => {
              if (selectedBatch) void openBatch(selectedBatch, page)
            }}
            onPageChange={setHistoryPage}
            onRefresh={() => batchesQuery.refetch()}
          />
        </div>

        <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('Confirm quick refund')}</DialogTitle>
              <DialogDescription>
                {t(
                  'This will queue a background refund job and cannot be undone.'
                )}
              </DialogDescription>
            </DialogHeader>
            <div className='flex flex-col gap-3'>
              <Alert variant='destructive'>
                <AlertTriangle />
                <AlertTitle>
                  {t('Confirm the amount before continuing.')}
                </AlertTitle>
                <AlertDescription>
                  {t(
                    '{{count}} items will receive {{quota}}. Create smaller batches if the range is large.',
                    {
                      count: preview?.matched_items ?? 0,
                      quota: formatQuota(preview?.refund_quota ?? 0),
                    }
                  )}
                </AlertDescription>
              </Alert>
              <div className='rounded-md border p-3 text-sm'>
                <div className='text-muted-foreground text-xs'>
                  {t('Reason')}
                </div>
                <div className='mt-1 break-words'>{previewPayload?.reason}</div>
              </div>
            </div>
            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => setConfirmOpen(false)}
                disabled={createMutation.isPending}
              >
                {t('Cancel')}
              </Button>
              <Button
                type='button'
                variant='destructive'
                disabled={createMutation.isPending || !canConfirm}
                onClick={confirmRefund}
              >
                {createMutation.isPending && (
                  <Loader2 className='animate-spin' />
                )}
                {createMutation.isPending
                  ? t('Queueing...')
                  : t('Confirm refund')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
