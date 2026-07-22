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
import { useEffect } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

import {
  useAgentRebateGroups,
  useUpsertAgentProfile,
} from '../hooks/use-agent-data'
import type { AgentProfile } from '../types'

const agentProfileSchema = z.object({
  userId: z
    .string()
    .refine(
      (value) => Number.isInteger(Number(value)) && Number(value) > 0,
      'Enter a valid user ID'
    ),
  status: z.enum(['1', '2']),
  rebateGroupId: z
    .string()
    .refine(
      (value) => Number.isInteger(Number(value)) && Number(value) > 0,
      'Select a rebate group'
    ),
  customRatePercent: z.string().refine((value) => {
    const rate = Number(value)
    return Number.isFinite(rate) && rate >= 0 && rate <= 100
  }, 'Enter a percentage from 0 to 100'),
  remark: z.string().max(255, 'Remark must not exceed 255 characters'),
})

type AgentProfileForm = z.infer<typeof agentProfileSchema>

interface AgentProfileDialogProps {
  open: boolean
  profile?: AgentProfile
  defaultGroupId: number
  onOpenChange: (open: boolean) => void
}

export function AgentProfileDialog(props: AgentProfileDialogProps) {
  const { t } = useTranslation()
  const groups = useAgentRebateGroups(props.open)
  const mutation = useUpsertAgentProfile()
  const form = useForm<AgentProfileForm>({
    resolver: zodResolver(agentProfileSchema),
    defaultValues: {
      userId: '',
      status: '1',
      rebateGroupId: String(props.defaultGroupId || ''),
      customRatePercent: '0',
      remark: '',
    },
  })

  useEffect(() => {
    if (!props.open) return
    form.reset({
      userId: props.profile ? String(props.profile.user_id) : '',
      status: String(props.profile?.status ?? 1) as '1' | '2',
      rebateGroupId: String(
        props.profile?.rebate_group_id || props.defaultGroupId || ''
      ),
      customRatePercent: String((props.profile?.custom_rate ?? 0) / 100),
      remark: props.profile?.remark ?? '',
    })
  }, [form, props.defaultGroupId, props.open, props.profile])

  const handleSubmit = form.handleSubmit(async (values) => {
    try {
      const response = await mutation.mutateAsync({
        userId: Number(values.userId),
        status: Number(values.status),
        rebateGroupId: Number(values.rebateGroupId),
        customRate: Math.round(Number(values.customRatePercent) * 100),
        remark: values.remark.trim(),
      })
      if (!response.success) {
        toast.error(response.message || t('Failed to save agent profile'))
        return
      }
      toast.success(t('Agent profile saved'))
      props.onOpenChange(false)
    } catch {
      toast.error(t('Failed to save agent profile'))
    }
  })

  const groupItems = (groups.data?.data ?? []).map((group) => ({
    value: String(group.id),
    label: `${group.name} (${(group.rebate_rate / 100).toFixed(2)}%)`,
  }))

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.profile ? t('Edit agent') : t('Add agent')}
      description={t('Configure the agent account and rebate policy.')}
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form='agent-profile-form'
            disabled={mutation.isPending || groups.isLoading}
          >
            {mutation.isPending ? t('Saving...') : t('Save')}
          </Button>
        </>
      }
    >
      <form
        id='agent-profile-form'
        className='flex flex-col gap-4'
        onSubmit={handleSubmit}
      >
        <Field data-invalid={!!form.formState.errors.userId || undefined}>
          <FieldLabel htmlFor='agent-profile-user-id'>
            {t('User ID')}
          </FieldLabel>
          <Input
            id='agent-profile-user-id'
            inputMode='numeric'
            disabled={!!props.profile}
            aria-invalid={!!form.formState.errors.userId}
            {...form.register('userId')}
          />
          <FieldError>
            {form.formState.errors.userId?.message
              ? t(form.formState.errors.userId.message)
              : null}
          </FieldError>
        </Field>

        <Controller
          control={form.control}
          name='status'
          render={({ field }) => (
            <Field>
              <FieldLabel htmlFor='agent-profile-status'>
                {t('Status')}
              </FieldLabel>
              <Select
                items={[
                  { value: '1', label: t('Enabled') },
                  { value: '2', label: t('Disabled') },
                ]}
                value={field.value}
                onValueChange={(value) =>
                  value !== null && field.onChange(value)
                }
              >
                <SelectTrigger id='agent-profile-status' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='1'>{t('Enabled')}</SelectItem>
                    <SelectItem value='2'>{t('Disabled')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
          )}
        />

        <Controller
          control={form.control}
          name='rebateGroupId'
          render={({ field }) => (
            <Field
              data-invalid={!!form.formState.errors.rebateGroupId || undefined}
            >
              <FieldLabel htmlFor='agent-profile-group'>
                {t('Rebate group')}
              </FieldLabel>
              <Select
                items={groupItems}
                value={field.value}
                onValueChange={(value) =>
                  value !== null && field.onChange(value)
                }
              >
                <SelectTrigger id='agent-profile-group' className='w-full'>
                  <SelectValue placeholder={t('Select a rebate group')} />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {(groups.data?.data ?? []).map((group) => (
                      <SelectItem key={group.id} value={String(group.id)}>
                        {group.name} ({(group.rebate_rate / 100).toFixed(2)}%)
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
              <FieldError>
                {form.formState.errors.rebateGroupId?.message
                  ? t(form.formState.errors.rebateGroupId.message)
                  : null}
              </FieldError>
            </Field>
          )}
        />

        <Field
          data-invalid={!!form.formState.errors.customRatePercent || undefined}
        >
          <FieldLabel htmlFor='agent-profile-custom-rate'>
            {t('Custom rebate rate')}
          </FieldLabel>
          <Input
            id='agent-profile-custom-rate'
            type='number'
            min='0'
            max='100'
            step='0.01'
            aria-invalid={!!form.formState.errors.customRatePercent}
            {...form.register('customRatePercent')}
          />
          <FieldDescription>
            {t('Use 0 to follow the rebate group')}
          </FieldDescription>
          <FieldError>
            {form.formState.errors.customRatePercent?.message
              ? t(form.formState.errors.customRatePercent.message)
              : null}
          </FieldError>
        </Field>

        <Field data-invalid={!!form.formState.errors.remark || undefined}>
          <FieldLabel htmlFor='agent-profile-remark'>{t('Remark')}</FieldLabel>
          <Textarea
            id='agent-profile-remark'
            maxLength={255}
            aria-invalid={!!form.formState.errors.remark}
            {...form.register('remark')}
          />
          <FieldError>
            {form.formState.errors.remark?.message
              ? t(form.formState.errors.remark.message)
              : null}
          </FieldError>
        </Field>
      </form>
    </Dialog>
  )
}
