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
import { Search } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

import {
  useAgentProfiles,
  useChangeUserAgent,
  useTransferAgentDownline,
} from '../hooks/use-agent-data'
import type { ChangeableAgentUser } from '../types'

interface ChangeAgentDialogProps {
  user: ChangeableAgentUser
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess?: () => void
  sourceAgentUserId?: number
  userIsAgent?: boolean
}

export function ChangeAgentDialog(props: ChangeAgentDialogProps) {
  const { t } = useTranslation()
  const [keyword, setKeyword] = useState('')
  const [selectedAgentId, setSelectedAgentId] = useState<number>()
  const [remark, setRemark] = useState('')
  const profiles = useAgentProfiles({
    page: 1,
    pageSize: 20,
    keyword,
    enabled: props.open,
  })
  const mutation = useChangeUserAgent()
  const transferMutation = useTransferAgentDownline()
  const items = (profiles.data?.data?.items ?? []).filter(
    (profile) =>
      profile.user_id !== props.user.id &&
      profile.user_id !== props.sourceAgentUserId
  )
  const currentAgentName =
    props.user.inviter_display_name || props.user.inviter_username
  let agentListContent: ReactNode
  if (profiles.isLoading) {
    agentListContent = (
      <div className='text-muted-foreground p-4 text-center text-sm'>
        {t('Loading...')}
      </div>
    )
  } else if (items.length === 0) {
    agentListContent = (
      <div className='text-muted-foreground p-4 text-center text-sm'>
        {t('No agents found')}
      </div>
    )
  } else {
    agentListContent = items.map((profile) => (
      <Button
        key={profile.user_id}
        type='button'
        variant='ghost'
        className={cn(
          'h-auto w-full justify-between px-3 py-2 text-left',
          selectedAgentId === profile.user_id && 'bg-accent'
        )}
        onClick={() => setSelectedAgentId(profile.user_id)}
      >
        <span className='min-w-0 truncate'>
          {profile.display_name || profile.username}
        </span>
        <span className='text-muted-foreground ml-3 shrink-0'>
          #{profile.user_id}
        </span>
      </Button>
    ))
  }

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      setKeyword('')
      setSelectedAgentId(undefined)
      setRemark('')
    }
    props.onOpenChange(open)
  }

  const handleSubmit = async () => {
    if (!selectedAgentId || selectedAgentId === props.user.inviter_id) return

    try {
      const response =
        props.sourceAgentUserId && !props.userIsAgent
          ? await transferMutation.mutateAsync({
              sourceAgentUserId: props.sourceAgentUserId,
              downlineUserId: props.user.id,
              targetAgentUserId: selectedAgentId,
              remark: remark.trim(),
            })
          : await mutation.mutateAsync({
              downlineUserId: props.user.id,
              targetAgentUserId: selectedAgentId,
              remark: remark.trim(),
            })
      if (!response.success) {
        toast.error(response.message || t('Failed to change upstream agent'))
        return
      }
      toast.success(t('Upstream agent changed'))
      props.onSuccess?.()
      handleOpenChange(false)
    } catch {
      toast.error(t('Failed to change upstream agent'))
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={
        props.userIsAgent
          ? t('Change agent parent')
          : t('Change upstream agent')
      }
      description={`${props.user.display_name || props.user.username} (#${props.user.id})`}
      contentClassName='sm:max-w-xl'
      footer={
        <>
          <Button variant='outline' onClick={() => handleOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={
              mutation.isPending ||
              transferMutation.isPending ||
              !selectedAgentId ||
              selectedAgentId === props.user.inviter_id
            }
          >
            {mutation.isPending || transferMutation.isPending
              ? t('Processing...')
              : t('Confirm change')}
          </Button>
        </>
      }
    >
      <div className='flex flex-col gap-4'>
        <div className='bg-muted/50 rounded-lg px-3 py-2 text-sm'>
          <span className='text-muted-foreground'>
            {t('Current upstream')}:
          </span>{' '}
          <span className='font-medium'>
            {(props.user.inviter_id ?? 0) > 0
              ? `${currentAgentName || t('Agent')} (#${props.user.inviter_id})`
              : t('No upstream agent')}
          </span>
        </div>
        <Field>
          <FieldLabel htmlFor='agent-search'>{t('Target agent')}</FieldLabel>
          <div className='relative'>
            <Search className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2' />
            <Input
              id='agent-search'
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              placeholder={t('Search agents by ID or name')}
              className='pl-9'
            />
          </div>
        </Field>
        <ScrollArea className='h-48 rounded-lg border'>
          <div className='flex flex-col gap-1 p-2'>{agentListContent}</div>
        </ScrollArea>
        <Field>
          <FieldLabel htmlFor='agent-change-remark'>{t('Remark')}</FieldLabel>
          <Textarea
            id='agent-change-remark'
            value={remark}
            onChange={(event) => setRemark(event.target.value)}
            maxLength={255}
          />
        </Field>
      </div>
    </Dialog>
  )
}
