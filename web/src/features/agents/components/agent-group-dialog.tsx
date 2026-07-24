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
import { AlertTriangle } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Field, FieldError, FieldLabel } from '@/components/ui/field'
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

import { useUpsertAgentRebateGroup } from '../hooks/use-agent-data'
import { formatAgentRate } from '../lib/format'
import type { AgentRateConflict, AgentRebateGroup } from '../types'

const agentGroupSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'Group name is required')
    .max(64, 'Group name must not exceed 64 characters'),
  rebateRatePercent: z.string().refine((value) => {
    const rate = Number(value)
    return Number.isFinite(rate) && rate >= 0 && rate <= 100
  }, 'Enter a percentage from 0 to 100'),
  status: z.enum(['1', '0']),
  remark: z.string().max(255, 'Remark must not exceed 255 characters'),
})

type AgentGroupForm = z.infer<typeof agentGroupSchema>

interface AgentGroupDialogProps {
  open: boolean
  group?: AgentRebateGroup
  onOpenChange: (open: boolean) => void
}

export function AgentGroupDialog(props: AgentGroupDialogProps) {
  const { t } = useTranslation()
  const mutation = useUpsertAgentRebateGroup()
  const [conflicts, setConflicts] = useState<AgentRateConflict[]>([])
  const form = useForm<AgentGroupForm>({
    resolver: zodResolver(agentGroupSchema),
    defaultValues: {
      name: '',
      rebateRatePercent: '0',
      status: '1',
      remark: '',
    },
  })

  useEffect(() => {
    if (!props.open) return
    setConflicts([])
    form.reset({
      name: props.group?.name ?? '',
      rebateRatePercent: String((props.group?.rebate_rate ?? 0) / 100),
      status: String(props.group?.status ?? 1) as '1' | '0',
      remark: props.group?.remark ?? '',
    })
  }, [form, props.group, props.open])

  const handleSubmit = form.handleSubmit(async (values) => {
    setConflicts([])
    try {
      const response = await mutation.mutateAsync({
        id: props.group?.id ?? 0,
        name: values.name.trim(),
        rebateRate: Math.round(Number(values.rebateRatePercent) * 100),
        status: Number(values.status),
        remark: values.remark.trim(),
      })
      if (!response.success) {
        setConflicts(response.data?.conflicts ?? [])
        toast.error(response.message || t('Failed to save rebate group'))
        return
      }
      toast.success(t('Rebate group saved'))
      props.onOpenChange(false)
    } catch {
      toast.error(t('Failed to save rebate group'))
    }
  })

  const selectedStatus = form.watch('status')
  const disablingUsedGroup =
    selectedStatus === '0' && (props.group?.agent_count ?? 0) > 0

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.group ? t('Edit rebate group') : t('Add rebate group')}
      description={t('Configure the default rebate rate for assigned agents.')}
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form='agent-group-form'
            disabled={mutation.isPending}
          >
            {mutation.isPending ? t('Saving...') : t('Save')}
          </Button>
        </>
      }
    >
      <form
        id='agent-group-form'
        className='flex flex-col gap-4'
        onSubmit={handleSubmit}
      >
        {disablingUsedGroup && (
          <Alert variant='destructive'>
            <AlertTriangle />
            <AlertTitle>{t('This group is currently in use')}</AlertTitle>
            <AlertDescription>
              {t(
                'Existing assignments keep their current rate, but this group cannot be selected for new assignments.'
              )}
            </AlertDescription>
          </Alert>
        )}

        {conflicts.length > 0 && (
          <Alert variant='destructive'>
            <AlertTriangle />
            <AlertTitle>{t('Agent hierarchy rate conflict')}</AlertTitle>
            <AlertDescription>
              <ul className='list-disc space-y-1 pl-4'>
                {conflicts.slice(0, 5).map((conflict) => (
                  <li
                    key={`${conflict.agent_user_id}-${conflict.conflict_type}`}
                  >
                    {conflict.agent_username || `#${conflict.agent_user_id}`}:{' '}
                    {formatAgentRate(conflict.agent_rate)} /{' '}
                    {formatAgentRate(conflict.parent_allowed_rate)}
                  </li>
                ))}
              </ul>
            </AlertDescription>
          </Alert>
        )}

        <Field data-invalid={!!form.formState.errors.name || undefined}>
          <FieldLabel htmlFor='agent-group-name'>{t('Group name')}</FieldLabel>
          <Input
            id='agent-group-name'
            maxLength={64}
            aria-invalid={!!form.formState.errors.name}
            {...form.register('name')}
          />
          <FieldError>
            {form.formState.errors.name?.message
              ? t(form.formState.errors.name.message)
              : null}
          </FieldError>
        </Field>

        <Field
          data-invalid={!!form.formState.errors.rebateRatePercent || undefined}
        >
          <FieldLabel htmlFor='agent-group-rate'>
            {t('Default rebate rate')}
          </FieldLabel>
          <Input
            id='agent-group-rate'
            type='number'
            min='0'
            max='100'
            step='0.01'
            aria-invalid={!!form.formState.errors.rebateRatePercent}
            {...form.register('rebateRatePercent')}
          />
          <FieldError>
            {form.formState.errors.rebateRatePercent?.message
              ? t(form.formState.errors.rebateRatePercent.message)
              : null}
          </FieldError>
        </Field>

        <Controller
          control={form.control}
          name='status'
          render={({ field }) => (
            <Field>
              <FieldLabel htmlFor='agent-group-status'>
                {t('Status')}
              </FieldLabel>
              <Select
                items={[
                  { value: '1', label: t('Enabled') },
                  { value: '0', label: t('Disabled') },
                ]}
                value={field.value}
                onValueChange={(value) =>
                  value !== null && field.onChange(value)
                }
                disabled={props.group?.is_default}
              >
                <SelectTrigger id='agent-group-status' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='1'>{t('Enabled')}</SelectItem>
                    <SelectItem value='0'>{t('Disabled')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
          )}
        />

        <Field data-invalid={!!form.formState.errors.remark || undefined}>
          <FieldLabel htmlFor='agent-group-remark'>{t('Remark')}</FieldLabel>
          <Textarea
            id='agent-group-remark'
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
