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
import { Menu, Sparkles, Trash2 } from 'lucide-react'
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
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Spinner } from '@/components/ui/spinner'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { ConsoleChat } from './components/console-chat'
import { ConsoleInput } from './components/console-input'
import { SessionSidebar } from './components/session-sidebar'
import { useAiConsoleChat } from './hooks/use-ai-console-chat'
import { useAiConsoleOptions } from './hooks/use-ai-console-options'
import { useAiConsoleSessions } from './hooks/use-ai-console-sessions'

export function AiConsole() {
  const { t } = useTranslation()
  const [mobileSessionsOpen, setMobileSessionsOpen] = useState(false)
  const [clearDialogOpen, setClearDialogOpen] = useState(false)
  const sessions = useAiConsoleSessions()
  const options = useAiConsoleOptions({
    group: sessions.group,
    model: sessions.model,
    onGroupChange: sessions.setGroup,
    onModelChange: sessions.setModel,
  })
  const chat = useAiConsoleChat({
    messages: sessions.messages,
    model: sessions.model,
    group: sessions.group,
    reasoningEffort: sessions.reasoningEffort,
    updateMessages: sessions.updateMessages,
    markActivity: sessions.markActivity,
  })

  const sidebar = (
    <SessionSidebar
      currentSessionId={sessions.currentSessionId}
      disabled={chat.isGenerating}
      onCreate={() => {
        void sessions.createSession()
        setMobileSessionsOpen(false)
      }}
      onDelete={(sessionId) => void sessions.removeSession(sessionId)}
      onRename={(sessionId, title) =>
        void sessions.renameSession(sessionId, title)
      }
      onSelect={(sessionId) => {
        void sessions.switchSession(sessionId)
        setMobileSessionsOpen(false)
      }}
      sessions={sessions.sessions}
    />
  )

  if (!sessions.ready) {
    return (
      <div className='flex size-full min-h-80 items-center justify-center'>
        <Spinner className='size-5' />
      </div>
    )
  }

  if (sessions.storageError === 'initialize') {
    return (
      <div className='flex size-full min-h-80 items-center justify-center px-5'>
        <div className='max-w-md text-center'>
          <Sparkles className='text-muted-foreground mx-auto mb-3 size-8' />
          <h1 className='text-lg font-semibold'>
            {t('AI Console could not start')}
          </h1>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t('Local browser storage is unavailable.')}
          </p>
        </div>
      </div>
    )
  }

  const currentSession = sessions.sessions.find(
    (session) => session.id === sessions.currentSessionId
  )
  const title = currentSession?.title || 'New conversation'
  const displayTitle =
    title === 'New conversation' || title === '新对话'
      ? t('New conversation')
      : title

  return (
    <div className='bg-background flex size-full min-h-0 overflow-hidden border-t'>
      <aside className='hidden w-72 shrink-0 border-r lg:block'>
        {sidebar}
      </aside>

      <section className='flex min-w-0 flex-1 flex-col overflow-hidden'>
        <header className='flex h-12 shrink-0 items-center gap-2 border-b px-2 sm:px-4'>
          <Button
            aria-label={t('Open conversations')}
            className='lg:hidden'
            onClick={() => setMobileSessionsOpen(true)}
            size='icon-sm'
            variant='ghost'
          >
            <Menu className='size-4' />
          </Button>
          <div className='min-w-0 flex-1'>
            <h1 className='truncate text-sm font-semibold'>{displayTitle}</h1>
          </div>
          {sessions.storageError === 'save' ? (
            <span className='text-destructive hidden text-xs sm:inline'>
              {t('History could not be saved')}
            </span>
          ) : null}
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  aria-label={t('Clear conversation')}
                  disabled={chat.isGenerating || sessions.messages.length === 0}
                  onClick={() => setClearDialogOpen(true)}
                  size='icon-sm'
                  variant='ghost'
                />
              }
            >
              <Trash2 className='size-4' />
            </TooltipTrigger>
            <TooltipContent>{t('Clear conversation')}</TooltipContent>
          </Tooltip>
        </header>

        <ConsoleChat
          isGenerating={chat.isGenerating}
          messages={sessions.messages}
          onDelete={chat.remove}
          onRegenerate={chat.regenerate}
        />
        <ConsoleInput
          disabled={chat.isGenerating}
          group={sessions.group}
          groups={options.groups}
          isGenerating={chat.isGenerating}
          isLoadingOptions={options.isLoading}
          model={sessions.model}
          models={options.models}
          onGroupChange={sessions.setGroup}
          onModelChange={sessions.setModel}
          onReasoningEffortChange={sessions.setReasoningEffort}
          onSend={chat.send}
          onStop={chat.stop}
          reasoningEffort={sessions.reasoningEffort}
        />
      </section>

      <Sheet open={mobileSessionsOpen} onOpenChange={setMobileSessionsOpen}>
        <SheetContent className='w-[min(86vw,22rem)] gap-0 p-0' side='left'>
          <SheetHeader className='sr-only'>
            <SheetTitle>{t('Conversations')}</SheetTitle>
            <SheetDescription>{t('Conversation history')}</SheetDescription>
          </SheetHeader>
          {sidebar}
        </SheetContent>
      </Sheet>

      <AlertDialog open={clearDialogOpen} onOpenChange={setClearDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Clear conversation?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('All messages in this conversation will be removed.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={() => {
                sessions.clearMessages()
                setClearDialogOpen(false)
              }}
            >
              {t('Clear')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
