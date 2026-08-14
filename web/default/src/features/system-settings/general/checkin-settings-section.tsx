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
import { useEffect, useState } from 'react'
import * as z from 'zod'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGridItem,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const checkinSchema = z.object({
  enabled: z.boolean(),
  captchaEnabled: z.boolean(),
  captchaKind: z.string(),
  requestCountTiers: z.string(),
  quotaConsumedTiers: z.string(),
})

type CheckinFormValues = z.infer<typeof checkinSchema>

type CheckinBonusTier = {
  threshold: number
  min_quota: number
  max_quota: number
}

type CheckinSettingsSectionProps = {
  defaultValues: CheckinFormValues
}

const OPTION_KEY_MAP: Record<string, string> = {
  enabled: 'checkin_setting.enabled',
  captchaEnabled: 'checkin_setting.captcha_enabled',
  captchaKind: 'checkin_setting.captcha_kind',
  requestCountTiers: 'checkin_setting.request_count_tiers',
  quotaConsumedTiers: 'checkin_setting.quota_consumed_tiers',
}

const TIER_FIELD_BY_METRIC = {
  request_count: 'requestCountTiers',
  quota_consumed: 'quotaConsumedTiers',
} as const

type CheckinMetric = keyof typeof TIER_FIELD_BY_METRIC

function parseBonusTiers(raw: string | undefined): CheckinBonusTier[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    if (Array.isArray(parsed)) return parsed as CheckinBonusTier[]
  } catch {
    // fall through to an empty tier list
  }
  return []
}

export function CheckinSettingsSection({
  defaultValues,
}: CheckinSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [tiersByMetric, setTiersByMetric] = useState<
    Record<CheckinMetric, CheckinBonusTier[]>
  >(() => ({
    request_count: parseBonusTiers(defaultValues.requestCountTiers),
    quota_consumed: parseBonusTiers(defaultValues.quotaConsumedTiers),
  }))

  const { form, handleSubmit, handleReset: resetForm, isDirty, isSubmitting } =
    useSettingsForm<CheckinFormValues>({
      resolver: zodResolver(checkinSchema) as Resolver<
        CheckinFormValues,
        unknown,
        CheckinFormValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          const optionKey = OPTION_KEY_MAP[key]
          if (!optionKey || value === undefined || value === null) continue
          let serialized = value as string | boolean | number
          if (
            typeof serialized === 'boolean' ||
            typeof serialized === 'number'
          ) {
            serialized = String(serialized)
          }
          const result = await updateOption.mutateAsync({
            key: optionKey,
            value: serialized,
          })
          if (!result.success) {
            throw new Error(result.message || t('Failed to update setting'))
          }
        }
      },
    })

  const enabled = form.watch('enabled') ?? defaultValues.enabled
  const captchaEnabled =
    form.watch('captchaEnabled') ?? defaultValues.captchaEnabled

  useEffect(() => {
    setTiersByMetric({
      request_count: parseBonusTiers(defaultValues.requestCountTiers),
      quota_consumed: parseBonusTiers(defaultValues.quotaConsumedTiers),
    })
  }, [defaultValues.requestCountTiers, defaultValues.quotaConsumedTiers])

  const handleReset = () => {
    resetForm()
    setTiersByMetric({
      request_count: parseBonusTiers(defaultValues.requestCountTiers),
      quota_consumed: parseBonusTiers(defaultValues.quotaConsumedTiers),
    })
  }

  const updateTiers = (metric: CheckinMetric, next: CheckinBonusTier[]) => {
    const sorted = [...next].sort((a, b) => a.threshold - b.threshold)
    setTiersByMetric((current) => ({ ...current, [metric]: next }))
    form.setValue(TIER_FIELD_BY_METRIC[metric], JSON.stringify(sorted), {
      shouldDirty: true,
      shouldTouch: true,
    })
  }

  const handleTierChange = (
    metric: CheckinMetric,
    index: number,
    field: keyof CheckinBonusTier,
    raw: string | number
  ) => {
    const next = tiersByMetric[metric].map((tier, tierIndex) =>
      tierIndex === index ? { ...tier, [field]: Number(raw) || 0 } : tier
    )
    updateTiers(metric, next)
  }

  const handleAddTier = (metric: CheckinMetric) => {
    const tiers = tiersByMetric[metric]
    const maxThreshold = tiers.reduce(
      (max, tier) => Math.max(max, tier.threshold),
      0
    )
    const step = metric === 'quota_consumed' ? 50000 : 50
    updateTiers(metric, [
      ...tiers,
      {
        threshold: tiers.length === 0 ? step : maxThreshold + step,
        min_quota: 2000,
        max_quota: 20000,
      },
    ])
  }

  const handleRemoveTier = (metric: CheckinMetric, index: number) => {
    updateTiers(
      metric,
      tiersByMetric[metric].filter((_, tierIndex) => tierIndex !== index)
    )
  }

  return (
    <SettingsSection title={t('Check-in Settings')}>
      <FormNavigationGuard when={isDirty} />

      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            onReset={handleReset}
            isSaving={updateOption.isPending || isSubmitting}
            isResetDisabled={!isDirty}
          />
          <FormDirtyIndicator isDirty={isDirty} />

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable check-in feature')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Allow users to check in daily for random quota rewards'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={updateOption.isPending || isSubmitting}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          {enabled && (
            <>
              <FormField
                control={form.control}
                name='captchaEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Enable check-in captcha')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Require users to solve an image captcha before checking in'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending || isSubmitting}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />

              {captchaEnabled && (
                <SettingsFormGridItem>
                  <FormField
                    control={form.control}
                    name='captchaKind'
                    defaultValue={defaultValues.captchaKind}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Captcha Type')}</FormLabel>
                        <Select
                          value={field.value}
                          onValueChange={field.onChange}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectGroup>
                              <SelectItem value='math'>
                                {t('Math Expression')}
                              </SelectItem>
                              <SelectItem value='digit'>
                                {t('Digits')}
                              </SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormDescription>{t('Captcha Type')}</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SettingsFormGridItem>
              )}

              <SettingsFormGridItem>
                <FormDescription>
                  {t(
                    'Both request count and quota consumption are evaluated. Only the matched tier with the higher maximum reward is used, and rewards are not stacked.'
                  )}
                </FormDescription>
              </SettingsFormGridItem>

              {(
                [
                  {
                    metric: 'request_count',
                    title: t('Yesterday Request Count'),
                    thresholdLabel: t('Yesterday Call Threshold (calls)'),
                    thresholdPlaceholder: t('Calls Threshold'),
                  },
                  {
                    metric: 'quota_consumed',
                    title: t('Yesterday Quota Consumed'),
                    thresholdLabel: t('Yesterday Quota Threshold'),
                    thresholdPlaceholder: t('Quota Threshold'),
                  },
                ] as const
              ).map((config) => (
                <SettingsFormGridItem key={config.metric}>
                  <div className='space-y-2 rounded-lg border p-4'>
                    <div className='text-sm font-medium'>{config.title}</div>
                    <div className='text-muted-foreground hidden grid-cols-4 gap-2 text-xs font-medium md:grid'>
                      <div>{config.thresholdLabel}</div>
                      <div>{t('Minimum Reward (quota)')}</div>
                      <div>{t('Maximum Reward (quota)')}</div>
                      <div>{t('Actions')}</div>
                    </div>
                    {tiersByMetric[config.metric].map((tier, index) => (
                      <div
                        key={`${config.metric}-${index}`}
                        className='bg-muted/30 grid grid-cols-2 gap-2 rounded-lg border p-3 md:grid-cols-4'
                      >
                        <div className='space-y-1'>
                          <div className='text-muted-foreground text-xs md:hidden'>
                            {config.thresholdLabel}
                          </div>
                          <Input
                            type='number'
                            min={0}
                            value={tier.threshold}
                            placeholder={config.thresholdPlaceholder}
                            onChange={(event) =>
                              handleTierChange(
                                config.metric,
                                index,
                                'threshold',
                                event.target.valueAsNumber
                              )
                            }
                          />
                        </div>
                        <div className='space-y-1'>
                          <div className='text-muted-foreground text-xs md:hidden'>
                            {t('Minimum Reward (quota)')}
                          </div>
                          <Input
                            type='number'
                            min={0}
                            value={tier.min_quota}
                            placeholder={t('Minimum (quota)')}
                            onChange={(event) =>
                              handleTierChange(
                                config.metric,
                                index,
                                'min_quota',
                                event.target.valueAsNumber
                              )
                            }
                          />
                        </div>
                        <div className='space-y-1'>
                          <div className='text-muted-foreground text-xs md:hidden'>
                            {t('Maximum Reward (quota)')}
                          </div>
                          <Input
                            type='number'
                            min={0}
                            value={tier.max_quota}
                            placeholder={t('Maximum (quota)')}
                            onChange={(event) =>
                              handleTierChange(
                                config.metric,
                                index,
                                'max_quota',
                                event.target.valueAsNumber
                              )
                            }
                          />
                        </div>
                        <div className='col-span-2 flex items-center justify-end md:col-span-1'>
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon'
                            aria-label={t('Delete')}
                            onClick={() =>
                              handleRemoveTier(config.metric, index)
                            }
                          >
                            <Trash2 className='size-4' />
                          </Button>
                        </div>
                      </div>
                    ))}
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => handleAddTier(config.metric)}
                    >
                      <Plus className='size-4' />
                      {t('Add tier')}
                    </Button>
                  </div>
                </SettingsFormGridItem>
              ))}
            </>
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
