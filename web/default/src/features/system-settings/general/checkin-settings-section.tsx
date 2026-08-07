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
  SettingsFormGrid,
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
  minQuota: z.coerce.number().int().min(0),
  maxQuota: z.coerce.number().int().min(0),
  captchaEnabled: z.boolean(),
  captchaKind: z.string(),
  bonusEnabled: z.boolean(),
  bonusMetric: z.string(),
  bonusTiers: z.string(),
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
  minQuota: 'checkin_setting.min_quota',
  maxQuota: 'checkin_setting.max_quota',
  captchaEnabled: 'checkin_setting.captcha_enabled',
  captchaKind: 'checkin_setting.captcha_kind',
  bonusEnabled: 'checkin_setting.bonus_enabled',
  bonusMetric: 'checkin_setting.bonus_metric',
  bonusTiers: 'checkin_setting.bonus_tiers',
}

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
  const [tiers, setTiers] = useState<CheckinBonusTier[]>(() =>
    parseBonusTiers(defaultValues.bonusTiers)
  )

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
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
          await updateOption.mutateAsync({
            key: optionKey,
            value: serialized,
          })
        }
      },
    })

  const enabled = form.watch('enabled')
  const captchaEnabled = form.watch('captchaEnabled')
  const bonusEnabled = form.watch('bonusEnabled')
  const bonusMetric = form.watch('bonusMetric')

  useEffect(() => {
    setTiers(parseBonusTiers(defaultValues.bonusTiers))
  }, [defaultValues.bonusTiers])

  const updateTiers = (next: CheckinBonusTier[]) => {
    setTiers(next)
    form.setValue('bonusTiers', JSON.stringify(next), {
      shouldDirty: true,
      shouldTouch: true,
    })
  }

  const handleTierChange = (
    index: number,
    field: keyof CheckinBonusTier,
    raw: string | number
  ) => {
    const next = tiers.map((tier, tierIndex) =>
      tierIndex === index ? { ...tier, [field]: Number(raw) || 0 } : tier
    )
    updateTiers(next)
  }

  const handleAddTier = () => {
    updateTiers([
      ...tiers,
      { threshold: 50, min_quota: 2000, max_quota: 20000 },
    ])
  }

  const handleRemoveTier = (index: number) => {
    updateTiers(tiers.filter((_, tierIndex) => tierIndex !== index))
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
            <SettingsFormGrid>
              <FormField
                control={form.control}
                name='minQuota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Minimum check-in quota')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        value={field.value ?? ''}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Minimum quota amount awarded for check-in')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='maxQuota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Maximum check-in quota')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        value={field.value ?? ''}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Maximum quota amount awarded for check-in')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGrid>
          )}

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

              <FormField
                control={form.control}
                name='bonusEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Enable active tier rewards')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Reward users by the matched tier range based on yesterday usage'
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

              {bonusEnabled && (
                <>
                  <SettingsFormGridItem>
                    <FormField
                      control={form.control}
                      name='bonusMetric'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Metric')}</FormLabel>
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
                                <SelectItem value='request_count'>
                                  {t('Yesterday Request Count')}
                                </SelectItem>
                                <SelectItem value='quota_consumed'>
                                  {t('Yesterday Quota Consumed')}
                                </SelectItem>
                              </SelectGroup>
                            </SelectContent>
                          </Select>
                          <FormDescription>{t('Metric')}</FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </SettingsFormGridItem>

                  <SettingsFormGridItem>
                    <div className='space-y-2'>
                      <div className='text-sm font-medium'>
                        {t('Active Tier Rewards')}
                      </div>
                      <div className='text-muted-foreground hidden grid-cols-4 gap-2 text-xs font-medium md:grid'>
                        <div>
                          {bonusMetric === 'quota_consumed'
                            ? t('Yesterday Quota Threshold')
                            : t('Yesterday Call Threshold (calls)')}
                        </div>
                        <div>{t('Minimum Reward (quota)')}</div>
                        <div>{t('Maximum Reward (quota)')}</div>
                        <div>{t('Actions')}</div>
                      </div>
                      {tiers.map((tier, index) => (
                        <div
                          key={index}
                          className='bg-muted/30 grid grid-cols-2 gap-2 rounded-lg border p-3 md:grid-cols-4'
                        >
                          <Input
                            type='number'
                            min={0}
                            value={tier.threshold}
                            placeholder={
                              bonusMetric === 'quota_consumed'
                                ? t('Quota Threshold')
                                : t('Calls Threshold')
                            }
                            onChange={(event) =>
                              handleTierChange(
                                index,
                                'threshold',
                                event.target.valueAsNumber
                              )
                            }
                          />
                          <Input
                            type='number'
                            min={0}
                            value={tier.min_quota}
                            placeholder={t('Minimum (quota)')}
                            onChange={(event) =>
                              handleTierChange(
                                index,
                                'min_quota',
                                event.target.valueAsNumber
                              )
                            }
                          />
                          <Input
                            type='number'
                            min={0}
                            value={tier.max_quota}
                            placeholder={t('Maximum (quota)')}
                            onChange={(event) =>
                              handleTierChange(
                                index,
                                'max_quota',
                                event.target.valueAsNumber
                              )
                            }
                          />
                          <div className='flex items-center justify-end'>
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon'
                              aria-label={t('Delete')}
                              onClick={() => handleRemoveTier(index)}
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
                        onClick={handleAddTier}
                      >
                        <Plus className='size-4' />
                        {t('Add tier')}
                      </Button>
                    </div>
                  </SettingsFormGridItem>
                </>
              )}
            </>
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
