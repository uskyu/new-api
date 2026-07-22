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
import { Field, FieldLabel } from '@/components/ui/field'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { changeUserRole } from '../api'
import { USER_ROLES } from '../constants'
import type { User } from '../types'

interface UserRoleDialogProps {
  open: boolean
  user: User
  targetRoles: number[]
  onOpenChange: (open: boolean) => void
  onSuccess: () => void
}

export function UserRoleDialog(props: UserRoleDialogProps) {
  const { t } = useTranslation()
  const [targetRole, setTargetRole] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (props.open) {
      setTargetRole(String(props.targetRoles[0] ?? ''))
    }
  }, [props.open, props.targetRoles])

  const options = props.targetRoles.map((role) => ({
    value: String(role),
    label: t(USER_ROLES[role as keyof typeof USER_ROLES].labelKey),
  }))

  const handleSubmit = async () => {
    const role = Number(targetRole)
    if (!props.targetRoles.includes(role)) return
    setSubmitting(true)
    try {
      const response = await changeUserRole({
        id: props.user.id,
        action: 'change_role',
        target_role: role,
      })
      if (!response.success) {
        toast.error(response.message || t('Failed to update user role'))
        return
      }
      toast.success(t('User role updated successfully'))
      props.onOpenChange(false)
      props.onSuccess()
    } catch {
      toast.error(t('Failed to update user role'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Promote user')}
      description={`${props.user.display_name || props.user.username} (#${props.user.id})`}
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleSubmit} disabled={!targetRole || submitting}>
            {submitting ? t('Saving...') : t('Confirm')}
          </Button>
        </>
      }
    >
      <Field>
        <FieldLabel htmlFor='user-target-role'>{t('Target role')}</FieldLabel>
        <Select
          items={options}
          value={targetRole}
          onValueChange={(value) => value !== null && setTargetRole(value)}
        >
          <SelectTrigger id='user-target-role' className='w-full'>
            <SelectValue placeholder={t('Select a role')} />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              {options.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </Field>
    </Dialog>
  )
}
