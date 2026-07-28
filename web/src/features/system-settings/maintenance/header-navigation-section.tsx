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
import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo } from 'react'
import { useFieldArray, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

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
import { Switch } from '@/components/ui/switch'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  isSafeTopNavHref,
  MAX_CUSTOM_TOP_NAV_LINKS,
  MAX_CUSTOM_TOP_NAV_TITLE_LENGTH,
  MAX_CUSTOM_TOP_NAV_URL_LENGTH,
} from '@/lib/nav-modules'

import {
  SettingsControlChildren,
  SettingsForm,
  SettingsSwitchContent,
  SettingsControlGroup,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  HEADER_NAV_DEFAULT,
  type HeaderNavModulesConfig,
  serializeHeaderNavModules,
} from './config'

const customLinkSchema = z.object({
  id: z.string(),
  title: z
    .string()
    .trim()
    .min(1, 'Navigation name is required')
    .max(
      MAX_CUSTOM_TOP_NAV_TITLE_LENGTH,
      'Navigation name must be 24 characters or fewer'
    ),
  url: z
    .string()
    .trim()
    .min(1, 'Navigation URL is required')
    .max(MAX_CUSTOM_TOP_NAV_URL_LENGTH, 'Navigation URL is too long')
    .refine(
      isSafeTopNavHref,
      'Use an internal path starting with / or an HTTP(S) URL'
    ),
  enabled: z.boolean(),
})

const headerNavSchema = z.object({
  home: z.boolean(),
  console: z.boolean(),
  pricingEnabled: z.boolean(),
  pricingRequireAuth: z.boolean(),
  rankingsEnabled: z.boolean(),
  rankingsRequireAuth: z.boolean(),
  docs: z.boolean(),
  docsLink: z
    .string()
    .trim()
    .max(MAX_CUSTOM_TOP_NAV_URL_LENGTH, 'Documentation URL is too long')
    .refine(
      (value) => value === '' || isSafeTopNavHref(value),
      'Use an internal path starting with / or an HTTP(S) URL'
    ),
  about: z.boolean(),
  customLinks: z.array(customLinkSchema).max(MAX_CUSTOM_TOP_NAV_LINKS),
})

type HeaderNavFormValues = z.infer<typeof headerNavSchema>
type SimpleHeaderNavField = 'home' | 'console' | 'about'
type AccessHeaderNavEnabledField = 'pricingEnabled' | 'rankingsEnabled'
type AccessHeaderNavAuthField = 'pricingRequireAuth' | 'rankingsRequireAuth'

type HeaderNavigationSectionProps = {
  config: HeaderNavModulesConfig
  initialSerialized: string
  initialDocsLink: string
}

const toFormValues = (
  config: HeaderNavModulesConfig,
  docsLink: string
): HeaderNavFormValues => ({
  home:
    config.home === undefined ? HEADER_NAV_DEFAULT.home : Boolean(config.home),
  console:
    config.console === undefined
      ? HEADER_NAV_DEFAULT.console
      : Boolean(config.console),
  pricingEnabled:
    config.pricing?.enabled === undefined
      ? HEADER_NAV_DEFAULT.pricing.enabled
      : Boolean(config.pricing.enabled),
  pricingRequireAuth:
    config.pricing?.requireAuth === undefined
      ? HEADER_NAV_DEFAULT.pricing.requireAuth
      : Boolean(config.pricing.requireAuth),
  rankingsEnabled:
    config.rankings?.enabled === undefined
      ? HEADER_NAV_DEFAULT.rankings.enabled
      : Boolean(config.rankings.enabled),
  rankingsRequireAuth:
    config.rankings?.requireAuth === undefined
      ? HEADER_NAV_DEFAULT.rankings.requireAuth
      : Boolean(config.rankings.requireAuth),
  docs:
    config.docs === undefined ? HEADER_NAV_DEFAULT.docs : Boolean(config.docs),
  docsLink,
  about:
    config.about === undefined
      ? HEADER_NAV_DEFAULT.about
      : Boolean(config.about),
  customLinks: config.customLinks.map((link) => ({ ...link })),
})

export function HeaderNavigationSection({
  config,
  initialSerialized,
  initialDocsLink,
}: HeaderNavigationSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const formDefaults = useMemo(
    () => toFormValues(config, initialDocsLink),
    [config, initialDocsLink]
  )

  const form = useForm<HeaderNavFormValues>({
    resolver: zodResolver(headerNavSchema),
    defaultValues: formDefaults,
  })
  const customLinks = useFieldArray({
    control: form.control,
    name: 'customLinks',
    keyName: 'fieldKey',
  })

  useEffect(() => {
    form.reset(formDefaults)
  }, [formDefaults, form])

  const onSubmit = async (values: HeaderNavFormValues) => {
    const payload: HeaderNavModulesConfig = {
      ...config,
      home: values.home,
      console: values.console,
      docs: values.docs,
      about: values.about,
      customLinks: values.customLinks.map((link) => ({
        ...link,
        title: link.title.trim(),
        url: link.url.trim(),
      })),
      pricing: {
        ...(config.pricing ?? HEADER_NAV_DEFAULT.pricing),
        enabled: values.pricingEnabled,
        requireAuth: values.pricingRequireAuth,
      },
      rankings: {
        ...(config.rankings ?? HEADER_NAV_DEFAULT.rankings),
        enabled: values.rankingsEnabled,
        requireAuth: values.rankingsRequireAuth,
      },
    }

    const serialized = serializeHeaderNavModules(payload)
    const docsLink = values.docsLink.trim()

    if (serialized !== initialSerialized) {
      await updateOption.mutateAsync({
        key: 'HeaderNavModules',
        value: serialized,
      })
    }

    if (docsLink !== initialDocsLink) {
      await updateOption.mutateAsync({
        key: 'general_setting.docs_link',
        value: docsLink,
      })
    }
  }

  const resetToDefault = () => {
    form.reset(toFormValues(HEADER_NAV_DEFAULT, form.getValues('docsLink')))
  }

  const simpleModules: Array<{
    key: SimpleHeaderNavField
    title: string
    description: string
  }> = [
    {
      key: 'home',
      title: t('Home'),
      description: t('Landing page with system overview.'),
    },
    {
      key: 'console',
      title: t('Console'),
      description: t('User dashboard and quota controls.'),
    },
    {
      key: 'about',
      title: t('About'),
      description: t('Static page describing the platform.'),
    },
  ]

  const accessModules: Array<{
    enabledKey: AccessHeaderNavEnabledField
    requireAuthKey: AccessHeaderNavAuthField
    requireAuthDependsOn: AccessHeaderNavEnabledField
    title: string
    description: string
    requireAuthTitle: string
    requireAuthDescription: string
  }> = [
    {
      enabledKey: 'pricingEnabled',
      requireAuthKey: 'pricingRequireAuth',
      requireAuthDependsOn: 'pricingEnabled',
      title: t('Model Square'),
      description: t('Public model catalog and pricing page.'),
      requireAuthTitle: t('Require login to view models'),
      requireAuthDescription: t(
        'Visitors must authenticate before accessing the pricing directory.'
      ),
    },
    {
      enabledKey: 'rankingsEnabled',
      requireAuthKey: 'rankingsRequireAuth',
      requireAuthDependsOn: 'rankingsEnabled',
      title: t('Rankings'),
      description: t('Public rankings page based on live usage data.'),
      requireAuthTitle: t('Require login to view rankings'),
      requireAuthDescription: t(
        'Visitors must authenticate before accessing the rankings page.'
      ),
    },
  ]

  return (
    <SettingsSection title={t('Header navigation')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            onReset={resetToDefault}
            isSaving={updateOption.isPending || form.formState.isSubmitting}
            resetLabel='Reset to default'
            saveLabel='Save navigation'
          />
          <div className='grid gap-4 md:grid-cols-2'>
            {simpleModules.map((module) => (
              <FormField
                key={module.key}
                control={form.control}
                name={module.key}
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{module.title}</FormLabel>
                      <FormDescription>{module.description}</FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                    <FormMessage />
                  </SettingsSwitchItem>
                )}
              />
            ))}
          </div>

          <SettingsControlGroup>
            <FormField
              control={form.control}
              name='docs'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Docs')}</FormLabel>
                    <FormDescription>
                      {t('Documentation or external knowledge base.')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                  <FormMessage />
                </SettingsSwitchItem>
              )}
            />

            <FormField
              control={form.control}
              name='docsLink'
              render={({ field }) => (
                <SettingsControlChildren>
                  <FormItem>
                    <FormLabel>{t('Documentation Link')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('https://docs.example.com')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Use an internal path or an external HTTP(S) documentation URL.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                </SettingsControlChildren>
              )}
            />
          </SettingsControlGroup>

          <div className='grid gap-4 lg:grid-cols-2'>
            {accessModules.map((module) => (
              <SettingsControlGroup key={module.enabledKey}>
                <FormField
                  control={form.control}
                  name={module.enabledKey}
                  render={({ field }) => (
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>{module.title}</FormLabel>
                        <FormDescription>{module.description}</FormDescription>
                      </SettingsSwitchContent>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                      <FormMessage />
                    </SettingsSwitchItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name={module.requireAuthKey}
                  render={({ field }) => (
                    <SettingsControlChildren>
                      <SettingsSwitchItem className='py-2'>
                        <SettingsSwitchContent>
                          <FormLabel>{module.requireAuthTitle}</FormLabel>
                          <FormDescription>
                            {module.requireAuthDescription}
                          </FormDescription>
                        </SettingsSwitchContent>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                            disabled={!form.watch(module.requireAuthDependsOn)}
                          />
                        </FormControl>
                        <FormMessage />
                      </SettingsSwitchItem>
                    </SettingsControlChildren>
                  )}
                />
              </SettingsControlGroup>
            ))}
          </div>

          <div data-settings-form-span='full' className='space-y-3'>
            <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
              <div className='min-w-0 space-y-1'>
                <h3 className='text-sm font-medium'>
                  {t('Custom navigation links')}
                </h3>
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'Add links that appear in the top navigation. Disabled links remain saved but are hidden.'
                  )}
                </p>
              </div>
              <Button
                type='button'
                variant='outline'
                className='w-full sm:w-auto'
                disabled={customLinks.fields.length >= MAX_CUSTOM_TOP_NAV_LINKS}
                onClick={() => {
                  const id = globalThis.crypto?.randomUUID?.()
                  customLinks.append({
                    id: id ?? `custom-link-${Date.now()}`,
                    title: '',
                    url: '',
                    enabled: true,
                  })
                }}
              >
                <Plus aria-hidden='true' />
                {t('Add top navigation link')}
              </Button>
            </div>

            <p className='text-muted-foreground text-xs'>
              {t('You can add up to {{count}} custom navigation links.', {
                count: MAX_CUSTOM_TOP_NAV_LINKS,
              })}
            </p>

            {customLinks.fields.length === 0 ? (
              <div className='text-muted-foreground rounded-md border border-dashed px-4 py-8 text-center text-sm'>
                {t('No custom navigation links have been added.')}
              </div>
            ) : (
              <div className='space-y-3'>
                {customLinks.fields.map((link, index) => (
                  <div
                    key={link.fieldKey}
                    className='grid min-w-0 gap-3 rounded-md border p-3 md:grid-cols-[auto_minmax(0,0.8fr)_minmax(0,1.4fr)_auto] md:items-start'
                  >
                    <FormField
                      control={form.control}
                      name={`customLinks.${index}.enabled`}
                      render={({ field }) => (
                        <FormItem className='flex items-center gap-2 md:pt-8'>
                          <FormControl>
                            <Switch
                              checked={field.value}
                              onCheckedChange={field.onChange}
                            />
                          </FormControl>
                          <FormLabel className='md:sr-only'>
                            {t('Enabled')}
                          </FormLabel>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name={`customLinks.${index}.title`}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Navigation name')}</FormLabel>
                          <FormControl>
                            <Input
                              placeholder={t('e.g. Support')}
                              maxLength={MAX_CUSTOM_TOP_NAV_TITLE_LENGTH}
                              {...field}
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name={`customLinks.${index}.url`}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Destination URL')}</FormLabel>
                          <FormControl>
                            <Input
                              placeholder={t('/support or https://example.com')}
                              maxLength={MAX_CUSTOM_TOP_NAV_URL_LENGTH}
                              {...field}
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <TooltipProvider delay={200}>
                      <div className='flex items-center justify-end gap-1 md:pt-7'>
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <Button
                                type='button'
                                variant='ghost'
                                size='icon-sm'
                                disabled={index === 0}
                                onClick={() =>
                                  customLinks.move(index, index - 1)
                                }
                                aria-label={t('Move link up')}
                              />
                            }
                          >
                            <ArrowUp aria-hidden='true' />
                          </TooltipTrigger>
                          <TooltipContent>{t('Move link up')}</TooltipContent>
                        </Tooltip>
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <Button
                                type='button'
                                variant='ghost'
                                size='icon-sm'
                                disabled={
                                  index === customLinks.fields.length - 1
                                }
                                onClick={() =>
                                  customLinks.move(index, index + 1)
                                }
                                aria-label={t('Move link down')}
                              />
                            }
                          >
                            <ArrowDown aria-hidden='true' />
                          </TooltipTrigger>
                          <TooltipContent>{t('Move link down')}</TooltipContent>
                        </Tooltip>
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <Button
                                type='button'
                                variant='ghost'
                                size='icon-sm'
                                className='text-destructive hover:text-destructive'
                                onClick={() => customLinks.remove(index)}
                                aria-label={t('Remove link')}
                              />
                            }
                          >
                            <Trash2 aria-hidden='true' />
                          </TooltipTrigger>
                          <TooltipContent>{t('Remove link')}</TooltipContent>
                        </Tooltip>
                      </div>
                    </TooltipProvider>
                  </div>
                ))}
              </div>
            )}
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
