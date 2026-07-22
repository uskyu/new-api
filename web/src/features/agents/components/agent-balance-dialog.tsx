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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

import { useAdjustAgentBalance } from '../hooks/use-agent-data'
import { formatAgentAmount } from '../lib/format'
import type { AgentProfile } from '../types'

interface AgentBalanceDialogProps {
  profile: AgentProfile
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function AgentBalanceDialog(props: AgentBalanceDialogProps) {
  const { t } = useTranslation()
  const [amount, setAmount] = useState('')
  const [reason, setReason] = useState('')
  const mutation = useAdjustAgentBalance()
  const currentBalance =
    props.profile.rebate_balance_amount === undefined
      ? ''
      : ` / ${formatAgentAmount(props.profile.rebate_balance_amount)}`

  const handleSubmit = async () => {
    if (!amount.trim() || Number(amount) === 0 || !reason.trim()) return

    try {
      const response = await mutation.mutateAsync({
        agentUserId: props.profile.user_id,
        amount,
        reason: reason.trim(),
      })
      if (!response.success) {
        toast.error(response.message || t('Failed to adjust agent balance'))
        return
      }
      toast.success(t('Agent balance adjusted'))
      setAmount('')
      setReason('')
      props.onOpenChange(false)
    } catch {
      toast.error(t('Failed to adjust agent balance'))
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Adjust agent balance')}
      description={`${props.profile.display_name || props.profile.username} (#${props.profile.user_id})${currentBalance}`}
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={
              mutation.isPending ||
              !amount.trim() ||
              Number(amount) === 0 ||
              !reason.trim()
            }
          >
            {mutation.isPending ? t('Processing...') : t('Confirm')}
          </Button>
        </>
      }
    >
      <div className='flex flex-col gap-4'>
        <Field>
          <FieldLabel htmlFor='agent-adjust-amount'>
            {t('Adjustment amount')}
          </FieldLabel>
          <Input
            id='agent-adjust-amount'
            type='number'
            step='0.01'
            value={amount}
            onChange={(event) => setAmount(event.target.value)}
            placeholder={t('Use a negative value to decrease balance')}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='agent-adjust-reason'>
            {t('Adjustment reason')}
          </FieldLabel>
          <Textarea
            id='agent-adjust-reason'
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            maxLength={255}
          />
        </Field>
      </div>
    </Dialog>
  )
}
