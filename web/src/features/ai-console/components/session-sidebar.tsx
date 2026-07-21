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
import { Check, MessageSquarePlus, Pencil, Trash2, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

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
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import type { AiConsoleSession } from '../types'

type SessionSidebarProps = {
  sessions: AiConsoleSession[]
  currentSessionId: string
  disabled?: boolean
  onCreate: () => void
  onSelect: (sessionId: string) => void
  onRename: (sessionId: string, title: string) => void
  onDelete: (sessionId: string) => void
}

export function SessionSidebar(props: SessionSidebarProps) {
  const { t } = useTranslation()
  const [editingId, setEditingId] = useState<string | null>(null)
  const [title, setTitle] = useState('')
  const [deleteId, setDeleteId] = useState<string | null>(null)

  const startRename = (session: AiConsoleSession) => {
    setEditingId(session.id)
    setTitle(session.title)
  }

  const saveRename = () => {
    if (!editingId) return
    props.onRename(editingId, title)
    setEditingId(null)
    setTitle('')
  }

  const resolveTitle = (sessionTitle: string) => {
    if (sessionTitle === 'New conversation' || sessionTitle === '新对话') {
      return t('New conversation')
    }
    return sessionTitle
  }

  return (
    <div className='bg-muted/20 flex size-full min-h-0 flex-col'>
      <div className='border-b p-3'>
        <Button
          className='w-full justify-start gap-2'
          disabled={props.disabled}
          onClick={props.onCreate}
        >
          <MessageSquarePlus className='size-4' aria-hidden='true' />
          {t('New conversation')}
        </Button>
      </div>

      <ScrollArea className='min-h-0 flex-1'>
        <div className='grid gap-1.5 p-2'>
          {props.sessions.map((session) => {
            const isActive = session.id === props.currentSessionId
            const isEditing = session.id === editingId

            return (
              <div
                className={cn(
                  'group min-w-0 rounded-lg border border-transparent p-2 transition-colors',
                  isActive
                    ? 'bg-background border-border'
                    : 'hover:bg-accent/60'
                )}
                key={session.id}
              >
                {isEditing ? (
                  <div className='grid gap-2'>
                    <Input
                      aria-label={t('Conversation name')}
                      autoFocus
                      onChange={(event) => setTitle(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter') saveRename()
                        if (event.key === 'Escape') setEditingId(null)
                      }}
                      value={title}
                    />
                    <div className='flex justify-end gap-1'>
                      <Button
                        aria-label={t('Cancel')}
                        onClick={() => setEditingId(null)}
                        size='icon-sm'
                        variant='ghost'
                      >
                        <X className='size-4' />
                      </Button>
                      <Button
                        aria-label={t('Save')}
                        onClick={saveRename}
                        size='icon-sm'
                      >
                        <Check className='size-4' />
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div className='flex min-w-0 items-start gap-1'>
                    <button
                      className='min-w-0 flex-1 px-1 py-0.5 text-left'
                      disabled={props.disabled}
                      onClick={() => props.onSelect(session.id)}
                      type='button'
                    >
                      <span className='block truncate text-sm font-medium'>
                        {resolveTitle(session.title)}
                      </span>
                      <span className='text-muted-foreground mt-0.5 block truncate text-xs'>
                        {session.lastMessagePreview || t('No messages yet')}
                      </span>
                    </button>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            aria-label={t('Rename')}
                            className='opacity-100 md:opacity-0 md:group-hover:opacity-100'
                            disabled={props.disabled}
                            onClick={() => startRename(session)}
                            size='icon-sm'
                            variant='ghost'
                          />
                        }
                      >
                        <Pencil className='size-3.5' />
                      </TooltipTrigger>
                      <TooltipContent>{t('Rename')}</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            aria-label={t('Delete')}
                            className='text-muted-foreground hover:text-destructive opacity-100 md:opacity-0 md:group-hover:opacity-100'
                            disabled={props.disabled}
                            onClick={() => setDeleteId(session.id)}
                            size='icon-sm'
                            variant='ghost'
                          />
                        }
                      >
                        <Trash2 className='size-3.5' />
                      </TooltipTrigger>
                      <TooltipContent>{t('Delete')}</TooltipContent>
                    </Tooltip>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </ScrollArea>

      <AlertDialog
        open={deleteId !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteId(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete conversation?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('This conversation and its local history will be removed.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={() => {
                if (deleteId) props.onDelete(deleteId)
                setDeleteId(null)
              }}
            >
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
