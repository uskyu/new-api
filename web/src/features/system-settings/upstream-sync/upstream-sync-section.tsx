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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Play, Save } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { ErrorState } from '@/components/error-state'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { formatTimestampToDate } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { createServerError } from '@/lib/server-error-message'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { SettingsCard } from '../components/settings-card'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import {
  SettingsPageActionsPortal,
  useSuppressSettingsSectionHeader,
} from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { safeNumberFieldProps } from '../utils/numeric-field'
import {
  getUpstreamSyncConfig,
  runUpstreamSync,
  updateUpstreamSyncConfig,
} from './api'
import { SyncLogsCard } from './sync-logs-card'
import { SyncStatusBadge } from './sync-status-badge'
import type { UpstreamSyncConfig } from './types'

const CONFIG_QUERY_KEY = ['upstream-sync', 'config'] as const
const LOGS_QUERY_KEY = ['upstream-sync', 'logs'] as const
const ACTIVE_TASK_POLL_INTERVAL_MS = 5000
const RUN_POLL_WINDOW_MS = 60_000
const DEFAULT_INTERVAL_MINUTES = 5

const configSchema = z.object({
  enabled: z.boolean(),
  interval_minutes: z
    .number()
    .int('Interval must be an integer between 1 and 1440 minutes.')
    .min(1, 'Interval must be an integer between 1 and 1440 minutes.')
    .max(1440, 'Interval must be an integer between 1 and 1440 minutes.'),
})

type ConfigFormValues = z.infer<typeof configSchema>

function normalizeIntervalMinutes(value: number): number {
  return Number.isInteger(value) && value >= 1 && value <= 1440
    ? value
    : DEFAULT_INTERVAL_MINUTES
}

function normalizeConfig(config: UpstreamSyncConfig): ConfigFormValues {
  return {
    enabled: Boolean(config.enabled),
    interval_minutes: normalizeIntervalMinutes(Number(config.interval_minutes)),
  }
}

function StatusTile(props: { label: string; children: React.ReactNode }) {
  return (
    <div className='bg-muted/20 min-w-0 rounded-lg border p-3'>
      <div className='text-muted-foreground text-xs font-medium'>
        {props.label}
      </div>
      <div className='mt-1.5 min-h-5 text-sm font-medium'>{props.children}</div>
    </div>
  )
}

export function UpstreamSyncSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const suppressHeader = useSuppressSettingsSectionHeader()
  const [runStartedAt, setRunStartedAt] = useState(0)
  const runStartedAtRef = useRef(0)
  const [triggeredTaskId, setTriggeredTaskId] = useState('')
  const previousActiveTaskIdRef = useRef('')
  const previousLastRunAtRef = useRef<number | null>(null)
  const loadedConfigRef = useRef('')

  const configQuery = useQuery({
    queryKey: CONFIG_QUERY_KEY,
    queryFn: async () => {
      const res = await getUpstreamSyncConfig()
      if (!res.success || !res.data) {
        throw createServerError(
          res,
          t('Failed to load upstream sync configuration')
        )
      }
      return res.data
    },
    retry: false,
    refetchInterval: (query) => {
      if (query.state.data?.active_task_id) {
        return ACTIVE_TASK_POLL_INTERVAL_MS
      }
      if (Date.now() < runStartedAtRef.current + RUN_POLL_WINDOW_MS) {
        return ACTIVE_TASK_POLL_INTERVAL_MS
      }
      return false
    },
  })

  const activeTaskId = configQuery.data?.active_task_id ?? ''
  const config = configQuery.data

  const configForm = useForm<ConfigFormValues>({
    resolver: zodResolver(configSchema),
    defaultValues: {
      enabled: false,
      interval_minutes: DEFAULT_INTERVAL_MINUTES,
    },
    mode: 'onChange',
  })

  useEffect(() => {
    if (!config) return
    const normalized = normalizeConfig(config)
    const serialized = JSON.stringify(normalized)
    if (serialized === loadedConfigRef.current) return
    if (configForm.formState.isDirty) return

    configForm.reset(normalized)
    loadedConfigRef.current = serialized
  }, [config, configForm, configForm.formState.isDirty])

  useEffect(() => {
    if (!runStartedAt) return
    const timeout = window.setTimeout(() => {
      runStartedAtRef.current = 0
      setRunStartedAt(0)
    }, RUN_POLL_WINDOW_MS)
    return () => window.clearTimeout(timeout)
  }, [runStartedAt])

  useEffect(() => {
    const previousActiveTaskId = previousActiveTaskIdRef.current
    if (previousActiveTaskId && !activeTaskId) {
      runStartedAtRef.current = 0
      setRunStartedAt(0)
      void queryClient.invalidateQueries({ queryKey: LOGS_QUERY_KEY })
    }
    previousActiveTaskIdRef.current = activeTaskId
  }, [activeTaskId, queryClient])

  const lastRunAt = config?.last_run_at ?? 0

  useEffect(() => {
    const previousLastRunAt = previousLastRunAtRef.current
    previousLastRunAtRef.current = lastRunAt
    if (!runStartedAt || lastRunAt === 0) return
    if (previousLastRunAt !== null && lastRunAt !== previousLastRunAt) {
      // The run can finish before the first poll ever observes an active task.
      // Once config reports a new completion, stop the short run poll window.
      runStartedAtRef.current = 0
      setRunStartedAt(0)
    }
  }, [lastRunAt, runStartedAt])

  const saveMutation = useMutation({
    mutationFn: async (values: ConfigFormValues) => {
      const res = await updateUpstreamSyncConfig(values)
      if (!res.success) {
        throw createServerError(
          res,
          t('Failed to save upstream sync configuration')
        )
      }
      return res
    },
    onSuccess: async (res, values) => {
      const normalized = res.data ? normalizeConfig(res.data) : { ...values }
      configForm.reset(normalized)
      loadedConfigRef.current = JSON.stringify(normalized)
      await queryClient.invalidateQueries({ queryKey: CONFIG_QUERY_KEY })
      toast.success(t('Upstream sync configuration saved'))
    },
    onError: (error) => {
      handleServerError(error, t('Failed to save upstream sync configuration'))
    },
  })

  const runMutation = useMutation({
    mutationFn: async () => {
      const res = await runUpstreamSync()
      if (!res.success || !res.data?.task_id) {
        throw createServerError(res, t('Failed to trigger upstream sync'))
      }
      return res.data.task_id
    },
    onSuccess: (taskId) => {
      const now = Date.now()
      runStartedAtRef.current = now
      setRunStartedAt(now)
      setTriggeredTaskId(taskId)
      toast.success(
        t('Upstream sync started: {{taskId}}', {
          taskId,
        })
      )
      void queryClient.invalidateQueries({ queryKey: CONFIG_QUERY_KEY })
      void queryClient.invalidateQueries({ queryKey: LOGS_QUERY_KEY })
    },
    onError: (error) => {
      handleServerError(error, t('Failed to trigger upstream sync'))
    },
  })

  const isRunActive =
    runMutation.isPending || Boolean(activeTaskId) || runStartedAt > 0
  const isSaveDisabled =
    configQuery.isLoading ||
    !config ||
    !configForm.formState.isDirty ||
    saveMutation.isPending ||
    runMutation.isPending

  return (
    <SettingsSection
      title={t('Upstream Sync')}
      className={suppressHeader ? undefined : 'min-h-0 flex-1'}
    >
      <div className='space-y-4'>
        {configQuery.isLoading && (
          <SettingsCard
            title={t('Sync Configuration')}
            description={t(
              'Configure how often upstream models, groups and prices are synchronized.'
            )}
          >
            <div className='space-y-3'>
              <Skeleton className='h-9 w-full rounded-md' />
              <Skeleton className='h-9 w-full rounded-md' />
              <Skeleton className='h-24 w-full rounded-md' />
            </div>
          </SettingsCard>
        )}

        {!configQuery.isLoading && (configQuery.isError || !config) && (
          <SettingsCard
            title={t('Sync Configuration')}
            description={t(
              'Configure how often upstream models, groups and prices are synchronized.'
            )}
          >
            <ErrorState
              title={t('Failed to load upstream sync configuration')}
              description={
                configQuery.error instanceof Error
                  ? configQuery.error.message
                  : undefined
              }
              onRetry={() => void configQuery.refetch()}
              className='min-h-[180px]'
            />
          </SettingsCard>
        )}

        {config && !configQuery.isLoading && (
          <>
            <FormDirtyIndicator isDirty={configForm.formState.isDirty} />
            <SettingsPageActionsPortal>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => runMutation.mutate()}
                disabled={isRunActive || saveMutation.isPending}
              >
                <Play data-icon='inline-start' />
                {runMutation.isPending ? t('Starting...') : t('Run Now')}
              </Button>
              <Button
                type='button'
                size='sm'
                onClick={configForm.handleSubmit((values) =>
                  saveMutation.mutate(values)
                )}
                disabled={isSaveDisabled}
              >
                <Save data-icon='inline-start' />
                {saveMutation.isPending ? t('Saving...') : t('Save Changes')}
              </Button>
            </SettingsPageActionsPortal>

            <SettingsCard
              title={t('Sync Configuration')}
              description={t(
                'Configure how often upstream models, groups and prices are synchronized.'
              )}
            >
              <Form {...configForm}>
                <SettingsForm
                  onSubmit={configForm.handleSubmit((values) =>
                    saveMutation.mutate(values)
                  )}
                >
                  <FormField
                    control={configForm.control}
                    name='enabled'
                    render={({ field }) => (
                      <SettingsSwitchItem>
                        <SettingsSwitchContent>
                          <FormLabel>{t('Enable upstream sync')}</FormLabel>
                          <FormDescription>
                            {t(
                              'When disabled, scheduled synchronization stops. Manual runs remain available.'
                            )}
                          </FormDescription>
                        </SettingsSwitchContent>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                          />
                        </FormControl>
                      </SettingsSwitchItem>
                    )}
                  />

                  <FormField
                    control={configForm.control}
                    name='interval_minutes'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Interval (minutes)')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={1}
                            max={1440}
                            step={1}
                            aria-label={t('Interval (minutes)')}
                            {...safeNumberFieldProps(field)}
                          />
                        </FormControl>
                        <FormDescription>
                          {t('Allowed range: 1–1440 minutes. Default: 5.')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <div className='grid min-w-0 gap-3 border-t pt-4 sm:grid-cols-2 xl:grid-cols-3'>
                    <StatusTile label={t('Last sync')}>
                      {config.last_run_at
                        ? formatTimestampToDate(config.last_run_at)
                        : t('Never synced')}
                    </StatusTile>
                    <StatusTile label={t('Last status')}>
                      {config.last_status ? (
                        <SyncStatusBadge status={config.last_status} />
                      ) : (
                        <span className='text-muted-foreground'>-</span>
                      )}
                    </StatusTile>
                    <StatusTile label={t('Active task')}>
                      {activeTaskId ? (
                        <span className='font-mono text-xs break-all'>
                          {activeTaskId}
                        </span>
                      ) : (
                        <span className='text-muted-foreground'>
                          {t('No active task')}
                        </span>
                      )}
                    </StatusTile>
                  </div>

                  {triggeredTaskId && (
                    <div className='flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs'>
                      <span className='text-muted-foreground'>
                        {t('Last triggered task')}:
                      </span>
                      <span className='font-mono break-all'>
                        {triggeredTaskId}
                      </span>
                    </div>
                  )}
                </SettingsForm>
              </Form>
            </SettingsCard>
          </>
        )}

        <SyncLogsCard activeTaskId={activeTaskId} runStartedAt={runStartedAt} />
      </div>
    </SettingsSection>
  )
}
