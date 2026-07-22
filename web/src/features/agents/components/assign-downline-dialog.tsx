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
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

import {
  useAssignAgentDownline,
  useSupportUsers,
} from '../hooks/use-agent-data'
import type { AgentProfile } from '../types'

interface AssignDownlineDialogProps {
  profile: AgentProfile
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function AssignDownlineDialog(props: AssignDownlineDialogProps) {
  const { t } = useTranslation()
  const [input, setInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [selectedUserId, setSelectedUserId] = useState<number>()
  const [remark, setRemark] = useState('')
  const users = useSupportUsers({
    page: 1,
    pageSize: 20,
    keyword,
    enabled: props.open && keyword.length > 0,
  })
  const mutation = useAssignAgentDownline()

  const handleSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setSelectedUserId(undefined)
    setKeyword(input.trim())
  }

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      setInput('')
      setKeyword('')
      setSelectedUserId(undefined)
      setRemark('')
    }
    props.onOpenChange(open)
  }

  const handleSubmit = async () => {
    if (!selectedUserId) return
    try {
      const response = await mutation.mutateAsync({
        targetAgentUserId: props.profile.user_id,
        downlineUserId: selectedUserId,
        remark: remark.trim(),
      })
      if (!response.success) {
        toast.error(response.message || t('Failed to assign user'))
        return
      }
      toast.success(t('User assigned to agent'))
      handleOpenChange(false)
    } catch {
      toast.error(t('Failed to assign user'))
    }
  }

  const items = users.data?.data?.items ?? []

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Assign user')}
      description={`${props.profile.display_name || props.profile.username} (#${props.profile.user_id})`}
      contentClassName='sm:max-w-xl'
      footer={
        <>
          <Button variant='outline' onClick={() => handleOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            disabled={!selectedUserId || mutation.isPending}
            onClick={handleSubmit}
          >
            {mutation.isPending ? t('Processing...') : t('Confirm assignment')}
          </Button>
        </>
      }
    >
      <div className='flex flex-col gap-4'>
        <form className='flex gap-2' onSubmit={handleSearch}>
          <Input
            value={input}
            onChange={(event) => setInput(event.target.value)}
            placeholder={t('Search users by ID or name')}
          />
          <Button type='submit' disabled={!input.trim()}>
            <Search />
            {t('Search')}
          </Button>
        </form>
        <ScrollArea className='h-56 rounded-lg border'>
          <div className='flex flex-col gap-1 p-2'>
            {!keyword && (
              <div className='text-muted-foreground p-4 text-center text-sm'>
                {t('Search for an unassigned user')}
              </div>
            )}
            {keyword && users.isLoading && (
              <div className='text-muted-foreground p-4 text-center text-sm'>
                {t('Loading...')}
              </div>
            )}
            {keyword && !users.isLoading && items.length === 0 && (
              <div className='text-muted-foreground p-4 text-center text-sm'>
                {t('No users found')}
              </div>
            )}
            {items.map((user) => {
              const unavailable = user.inviter_id > 0 || user.is_agent
              let assignmentStatus = t('Unassigned')
              if (user.is_agent) {
                assignmentStatus = t('Already an agent')
              } else if (user.inviter_id > 0) {
                assignmentStatus = t('Already assigned')
              }
              return (
                <Button
                  key={user.id}
                  type='button'
                  variant='ghost'
                  disabled={unavailable}
                  className={cn(
                    'h-auto w-full justify-between px-3 py-2 text-left',
                    selectedUserId === user.id && 'bg-accent'
                  )}
                  onClick={() => setSelectedUserId(user.id)}
                >
                  <span className='min-w-0 truncate'>
                    {user.display_name || user.username} (#{user.id})
                  </span>
                  <span className='text-muted-foreground ml-3 shrink-0 text-xs'>
                    {assignmentStatus}
                  </span>
                </Button>
              )
            })}
          </div>
        </ScrollArea>
        <Textarea
          value={remark}
          onChange={(event) => setRemark(event.target.value)}
          placeholder={t('Remark')}
          maxLength={255}
        />
      </div>
    </Dialog>
  )
}
