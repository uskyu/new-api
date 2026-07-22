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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'

import { useDecreaseSupportUserQuota } from '../hooks/use-agent-data'
import type { SupportManagedUser } from '../types'

interface SupportQuotaDialogProps {
  user: SupportManagedUser
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SupportQuotaDialog(props: SupportQuotaDialogProps) {
  const { t } = useTranslation()
  const [amount, setAmount] = useState('')
  const [reason, setReason] = useState('')
  const mutation = useDecreaseSupportUserQuota()
  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const quota = parseQuotaFromDollars(Math.abs(Number(amount) || 0))
  const remainingQuota = props.user.quota - quota
  const valid = quota > 0 && remainingQuota >= 0 && reason.trim().length > 0

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      setAmount('')
      setReason('')
    }
    props.onOpenChange(open)
  }

  const handleSubmit = async () => {
    if (!valid) return
    try {
      const response = await mutation.mutateAsync({
        userId: props.user.id,
        quota,
        reason: reason.trim(),
      })
      if (!response.success) {
        toast.error(response.message || t('Failed to decrease user balance'))
        return
      }
      toast.success(t('User balance decreased'))
      handleOpenChange(false)
    } catch {
      toast.error(t('Failed to decrease user balance'))
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Decrease user balance')}
      description={`${props.user.display_name || props.user.username} (#${props.user.id})`}
      footer={
        <>
          <Button variant='outline' onClick={() => handleOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            disabled={!valid || mutation.isPending}
            onClick={handleSubmit}
          >
            {mutation.isPending ? t('Processing...') : t('Confirm decrease')}
          </Button>
        </>
      }
    >
      <div className='flex flex-col gap-4'>
        <div className='bg-muted/50 rounded-lg px-3 py-2 text-sm'>
          <div>
            {t('Current quota')}: {formatQuota(props.user.quota)}
          </div>
          <div>
            {t('Balance after decrease')}:{' '}
            {formatQuota(Math.max(remainingQuota, 0))}
          </div>
        </div>
        <div className='space-y-2'>
          <Label htmlFor='support-quota-amount'>
            {t('Amount')} ({currencyLabel})
          </Label>
          <Input
            id='support-quota-amount'
            type='number'
            min='0'
            step={currencyMeta.kind === 'tokens' ? 1 : 0.01}
            value={amount}
            onChange={(event) => setAmount(event.target.value)}
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='support-quota-reason'>{t('Reason')}</Label>
          <Input
            id='support-quota-reason'
            value={reason}
            maxLength={255}
            onChange={(event) => setReason(event.target.value)}
          />
        </div>
      </div>
    </Dialog>
  )
}
