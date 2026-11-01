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
import { Plus, Save } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { StaticRowActions } from '@/components/data-table/static/static-row-actions'
import { Dialog } from '@/components/dialog'
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
import { Label } from '@/components/ui/label'
import { ContactIcon } from '@/features/contact-widget'
import { CONTACT_ICON_PRESETS } from '@/features/contact-widget/constants'
import {
  CONTACT_WIDGET_LIMITS,
  createEmptyContactWidgetConfig,
  parseContactWidgetConfig,
  serializeContactWidgetConfig,
} from '@/features/contact-widget/lib/config'
import {
  CONTACT_WIDGET_OPTION_KEY,
  type ContactWidgetConfig,
  type ContactWidgetContact,
} from '@/features/contact-widget/types'
import { isHttpUrl } from '@/lib/content-format'
import { handleServerError } from '@/lib/handle-server-error'
import { cn } from '@/lib/utils'

import { SettingsSwitchField } from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const CONTACT_FORM_ID = 'contact-form'

type ContactFormValues = {
  name: string
  icon: string
  contactId: string
  link: string
}

type ContactWidgetSectionProps = {
  defaultValue: string
}

export function ContactWidgetSection(props: ContactWidgetSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [config, setConfig] = useState<ContactWidgetConfig>(
    () =>
      parseContactWidgetConfig(props.defaultValue) ??
      createEmptyContactWidgetConfig()
  )
  const [isDirty, setIsDirty] = useState(false)
  const [showDialog, setShowDialog] = useState(false)
  const [editingContact, setEditingContact] =
    useState<ContactWidgetContact | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<ContactWidgetContact | null>(
    null
  )

  const contactSchema = z
    .object({
      name: z
        .string()
        .trim()
        .min(1, { error: () => t('Name is required') })
        .max(CONTACT_WIDGET_LIMITS.name, {
          error: () => t('Name must be at most 30 characters'),
        }),
      icon: z.string().max(CONTACT_WIDGET_LIMITS.icon, {
        error: () => t('Icon must be at most 200 characters'),
      }),
      contactId: z.string().max(CONTACT_WIDGET_LIMITS.contactId, {
        error: () => t('Contact ID must be at most 128 characters'),
      }),
      link: z.string().max(CONTACT_WIDGET_LIMITS.link, {
        error: () => t('Link must be at most 500 characters'),
      }),
    })
    .refine(
      (values) => values.contactId.trim() !== '' || values.link.trim() !== '',
      { error: () => t('Contact ID or link is required'), path: ['contactId'] }
    )
    .refine(
      (values) => values.link.trim() === '' || isHttpUrl(values.link.trim()),
      { error: () => t('Link must be a valid http(s) URL'), path: ['link'] }
    )

  const contactsSchema = z
    .array(contactSchema)
    .max(CONTACT_WIDGET_LIMITS.contacts, {
      error: () => t('At most 20 contacts are allowed'),
    })

  const form = useForm<ContactFormValues>({
    resolver: zodResolver(contactSchema) as Resolver<
      ContactFormValues,
      unknown,
      ContactFormValues
    >,
    defaultValues: {
      name: '',
      icon: '',
      contactId: '',
      link: '',
    },
  })

  useEffect(() => {
    setConfig(
      parseContactWidgetConfig(props.defaultValue) ??
        createEmptyContactWidgetConfig()
    )
    setIsDirty(false)
  }, [props.defaultValue])

  const iconOptions = useMemo(
    () =>
      CONTACT_ICON_PRESETS.map((preset) => ({
        key: preset.key,
        label: t(preset.labelKey),
        Icon: preset.Icon,
      })),
    [t]
  )

  const watchedName = form.watch('name')

  const handleToggleEnabled = (checked: boolean) => {
    setConfig((prev) => ({ ...prev, enabled: checked }))
    setIsDirty(true)
  }

  const handleToggleDefaultOpen = (checked: boolean) => {
    setConfig((prev) => ({ ...prev, defaultOpen: checked }))
    setIsDirty(true)
  }

  const handleAdd = () => {
    setEditingContact(null)
    form.reset({
      name: '',
      icon: '',
      contactId: '',
      link: '',
    })
    setShowDialog(true)
  }

  const handleEdit = (contact: ContactWidgetContact) => {
    setEditingContact(contact)
    form.reset({
      name: contact.name,
      icon: contact.icon,
      contactId: contact.contactId,
      link: contact.link,
    })
    setShowDialog(true)
  }

  const handleSubmitForm = (values: ContactFormValues) => {
    if (editingContact) {
      setConfig((prev) => ({
        ...prev,
        contacts: prev.contacts.map((item) =>
          item.id === editingContact.id ? { ...item, ...values } : item
        ),
      }))
      toast.success(t('Contact updated. Click "Save Settings" to apply.'))
    } else {
      const newId = Math.max(...config.contacts.map((item) => item.id), 0) + 1
      setConfig((prev) => ({
        ...prev,
        contacts: [...prev.contacts, { id: newId, ...values }],
      }))
      toast.success(t('Contact added. Click "Save Settings" to apply.'))
    }
    setIsDirty(true)
    setShowDialog(false)
  }

  const handleDelete = (contact: ContactWidgetContact) => {
    setDeleteTarget(contact)
  }

  const confirmDelete = () => {
    if (deleteTarget) {
      setConfig((prev) => ({
        ...prev,
        contacts: prev.contacts.filter((item) => item.id !== deleteTarget.id),
      }))
      setIsDirty(true)
      toast.success(t('Contact deleted. Click "Save Settings" to apply.'))
    }
    setDeleteTarget(null)
  }

  const handleSave = async () => {
    if (config.enabled && config.title.trim() === '') {
      toast.error(t('Contact widget title is required when enabled'))
      return
    }
    const result = contactsSchema.safeParse(config.contacts)
    if (!result.success) {
      toast.error(
        result.error.issues[0]?.message ?? t('Failed to save contact widget')
      )
      return
    }
    try {
      await updateOption.mutateAsync({
        key: CONTACT_WIDGET_OPTION_KEY,
        value: serializeContactWidgetConfig(config),
      })
      setIsDirty(false)
      toast.success(t('Contact widget saved successfully'))
    } catch (error) {
      handleServerError(error, t('Failed to save contact widget'))
    }
  }

  return (
    <SettingsSection title={t('Contact Widget')}>
      <div className='space-y-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <Button onClick={handleAdd} size='sm'>
              <Plus className='mr-2 h-4 w-4' />
              {t('Add Contact')}
            </Button>
            <Button
              onClick={handleSave}
              size='sm'
              variant='secondary'
              disabled={!isDirty || updateOption.isPending}
            >
              <Save className='mr-2 h-4 w-4' />
              {updateOption.isPending ? t('Saving...') : t('Save Settings')}
            </Button>
          </div>
          <div className='flex flex-wrap items-center gap-x-6 gap-y-2'>
            <SettingsSwitchField
              controlId='contact-widget-enabled'
              checked={config.enabled}
              onCheckedChange={handleToggleEnabled}
              label={t('Enabled')}
              className='py-0'
            />
            <SettingsSwitchField
              controlId='contact-widget-default-open'
              checked={config.defaultOpen}
              onCheckedChange={handleToggleDefaultOpen}
              label={t('Expand by default')}
              className='py-0'
            />
          </div>
        </div>

        <div className='grid gap-4 sm:grid-cols-2'>
          <div className='space-y-2'>
            <Label htmlFor='contact-widget-title'>{t('Widget Title')}</Label>
            <Input
              id='contact-widget-title'
              value={config.title}
              maxLength={CONTACT_WIDGET_LIMITS.title}
              onChange={(event) => {
                setConfig((prev) => ({ ...prev, title: event.target.value }))
                setIsDirty(true)
              }}
              placeholder={t('Contact Us')}
            />
            <p className='text-muted-foreground text-xs'>
              {t(
                'Heading of the floating widget. Required when the widget is enabled.'
              )}
            </p>
          </div>
          <div className='space-y-2'>
            <Label htmlFor='contact-widget-description'>
              {t('Widget Description')}
            </Label>
            <Input
              id='contact-widget-description'
              value={config.description}
              maxLength={CONTACT_WIDGET_LIMITS.description}
              onChange={(event) => {
                setConfig((prev) => ({
                  ...prev,
                  description: event.target.value,
                }))
                setIsDirty(true)
              }}
              placeholder={t('Choose your preferred contact method')}
            />
            <p className='text-muted-foreground text-xs'>
              {t('Optional subtitle shown under the widget heading.')}
            </p>
          </div>
        </div>

        <StaticDataTable
          data={config.contacts}
          getRowKey={(contact) => contact.id}
          emptyContent={t(
            'No contacts yet. Click "Add Contact" to create one.'
          )}
          columns={[
            {
              id: 'icon',
              header: t('Icon'),
              className: 'w-12',
              cell: (contact) => (
                <ContactIcon
                  icon={contact.icon}
                  name={contact.name}
                  size={20}
                />
              ),
            },
            {
              id: 'name',
              header: t('Name'),
              cell: (contact) => contact.name,
            },
            {
              id: 'contact-id',
              header: t('Contact ID'),
              cellClassName: 'max-w-[12rem]',
              cell: (contact) =>
                contact.contactId ? (
                  <span className='font-mono text-xs'>{contact.contactId}</span>
                ) : (
                  <span className='text-muted-foreground'>-</span>
                ),
            },
            {
              id: 'link',
              header: t('Link'),
              cellClassName: 'max-w-xs',
              cell: (contact) =>
                contact.link ? (
                  <span className='text-xs'>{contact.link}</span>
                ) : (
                  <span className='text-muted-foreground'>-</span>
                ),
            },
            {
              id: 'actions',
              header: t('Actions'),
              cell: (contact) => (
                <StaticRowActions
                  editLabel={t('Edit')}
                  deleteLabel={t('Delete')}
                  menuLabel={t('Open menu')}
                  onEdit={() => handleEdit(contact)}
                  onDelete={() => handleDelete(contact)}
                />
              ),
            },
          ]}
        />
      </div>

      <Dialog
        open={showDialog}
        onOpenChange={setShowDialog}
        title={editingContact ? t('Edit Contact') : t('Add Contact')}
        description={t(
          'Contacts are shown in the home page floating contact widget'
        )}
        contentClassName='max-w-2xl'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              onClick={() => setShowDialog(false)}
            >
              {t('Cancel')}
            </Button>
            <Button type='submit' form={CONTACT_FORM_ID}>
              {editingContact ? t('Update') : t('Add')}
            </Button>
          </>
        }
      >
        <Form {...form}>
          <form
            id={CONTACT_FORM_ID}
            onSubmit={form.handleSubmit(handleSubmitForm)}
            className='space-y-4'
          >
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('Contact name')} {...field} />
                  </FormControl>
                  <FormDescription>
                    {t('Displayed name of the contact')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='icon'
              render={({ field }) => (
                <FormItem>
                  <FormLabel htmlFor='contact-icon'>{t('Icon')}</FormLabel>
                  <div className='space-y-2'>
                    <div className='flex items-center gap-2'>
                      <span className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-lg'>
                        <ContactIcon
                          icon={field.value}
                          name={watchedName}
                          size={20}
                        />
                      </span>
                      <Input
                        id='contact-icon'
                        value={field.value}
                        onChange={(event) => field.onChange(event.target.value)}
                        placeholder={t('Custom icon image URL or key')}
                        className='min-w-0'
                      />
                      {field.value !== '' && (
                        <Button
                          type='button'
                          variant='ghost'
                          size='sm'
                          onClick={() => field.onChange('')}
                        >
                          {t('Clear')}
                        </Button>
                      )}
                    </div>
                    <div
                      role='group'
                      aria-label={t('Preset icons')}
                      className='max-h-40 overflow-y-auto rounded-lg border p-2'
                    >
                      <div className='grid grid-cols-6 gap-1 sm:grid-cols-10'>
                        {iconOptions.map((option) => (
                          <button
                            key={option.key}
                            type='button'
                            title={option.label}
                            aria-label={option.label}
                            aria-pressed={field.value === option.key}
                            onClick={() => field.onChange(option.key)}
                            className={cn(
                              'text-muted-foreground hover:bg-muted hover:text-foreground flex size-8 items-center justify-center rounded-md transition-colors',
                              field.value === option.key &&
                                'bg-muted text-foreground ring-ring ring-1'
                            )}
                          >
                            <option.Icon size={18} />
                          </button>
                        ))}
                      </div>
                    </div>
                  </div>
                  <FormDescription>
                    {t(
                      'Optional. Pick a preset icon or enter an image URL / custom key. Values that match nothing show a letter badge.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='contactId'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Contact ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('e.g. support@example.com')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Shown under the contact name. Visitors copy it with one click.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='link'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Link')}</FormLabel>
                  <FormControl>
                    <Input placeholder='https://example.com' {...field} />
                  </FormControl>
                  <FormDescription>
                    {t('Opens in a new tab when a visitor clicks this contact')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>
      </Dialog>

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Are you sure?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('This contact will be removed from the widget.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction variant='destructive' onClick={confirmDelete}>
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
