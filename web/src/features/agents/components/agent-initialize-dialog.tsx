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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

import { useInitializeAgentModule } from '../hooks/use-agent-data'

interface AgentInitializeDialogProps {
  open: boolean
  defaultRate: number
  onOpenChange: (open: boolean) => void
}

export function AgentInitializeDialog(props: AgentInitializeDialogProps) {
  const { t } = useTranslation()
  const [rate, setRate] = useState('10')
  const mutation = useInitializeAgentModule()

  useEffect(() => {
    if (props.open) {
      setRate(String(props.defaultRate > 0 ? props.defaultRate / 100 : 10))
    }
  }, [props.defaultRate, props.open])

  const rateValue = Number(rate)
  const validRate =
    Number.isFinite(rateValue) && rateValue >= 0 && rateValue <= 100

  const handleSubmit = async () => {
    if (!validRate) return
    try {
      const response = await mutation.mutateAsync(Math.round(rateValue * 100))
      if (!response.success) {
        toast.error(response.message || t('Failed to initialize agent module'))
        return
      }
      toast.success(t('Agent module initialized'))
      props.onOpenChange(false)
    } catch {
      toast.error(t('Failed to initialize agent module'))
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Initialize agent module')}
      description={t(
        'Create the agent data structure and default rebate group.'
      )}
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={!validRate || mutation.isPending}
          >
            {mutation.isPending ? t('Initializing...') : t('Initialize')}
          </Button>
        </>
      }
    >
      <Field data-invalid={!validRate || undefined}>
        <FieldLabel htmlFor='agent-default-rate'>
          {t('Default rebate rate')}
        </FieldLabel>
        <Input
          id='agent-default-rate'
          type='number'
          min='0'
          max='100'
          step='0.01'
          value={rate}
          aria-invalid={!validRate}
          onChange={(event) => setRate(event.target.value)}
        />
        <FieldDescription>{t('Percentage from 0 to 100')}</FieldDescription>
      </Field>
    </Dialog>
  )
}
