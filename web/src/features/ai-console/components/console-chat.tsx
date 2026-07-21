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
import {
  Copy,
  FileText,
  MessageSquarePlus,
  RotateCcw,
  Trash2,
} from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  Conversation,
  ConversationContent,
  ConversationEmptyState,
  ConversationScrollButton,
} from '@/components/ai-elements/conversation'
import { Loader } from '@/components/ai-elements/loader'
import { Message, MessageContent } from '@/components/ai-elements/message'
import {
  Reasoning,
  ReasoningContent,
  ReasoningTrigger,
} from '@/components/ai-elements/reasoning'
import { Response } from '@/components/ai-elements/response'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import type { AiConsoleMessage } from '../types'

const MAX_RENDERED_MESSAGES = 60

type ConsoleChatProps = {
  messages: AiConsoleMessage[]
  isGenerating: boolean
  onRegenerate: (message: AiConsoleMessage) => void
  onDelete: (message: AiConsoleMessage) => void
}

export function ConsoleChat(props: ConsoleChatProps) {
  const { t } = useTranslation()
  const visibleMessages = props.messages.slice(-MAX_RENDERED_MESSAGES)

  const copyMessage = async (message: AiConsoleMessage) => {
    const content = message.versions[0]?.content || ''
    if (!content) return
    await navigator.clipboard.writeText(content)
    toast.success(t('Copied!'))
  }

  return (
    <Conversation>
      <ConversationContent className='p-0'>
        <div className='mx-auto grid w-full max-w-4xl gap-2 px-3 py-5 sm:px-5'>
          {visibleMessages.length === 0 ? (
            <ConversationEmptyState
              className='min-h-[min(520px,calc(100svh-16rem))]'
              description={t('How can I help?')}
              icon={<MessageSquarePlus className='size-7' />}
              title={t('Personal AI Console')}
            />
          ) : null}

          {visibleMessages.map((message) => {
            const content = message.versions[0]?.content || ''
            const isPending =
              message.status === 'loading' || message.status === 'streaming'
            const isAssistant = message.from === 'assistant'
            let messageBody: ReactNode = null
            if (content) {
              messageBody = <Response final={!isPending}>{content}</Response>
            } else if (isPending) {
              messageBody = <Loader />
            }

            return (
              <Message className='py-2' from={message.from} key={message.key}>
                <div className='max-w-full min-w-0 flex-1'>
                  <MessageContent
                    className={cn(
                      'leading-6',
                      isAssistant && 'w-full',
                      message.status === 'error' &&
                        'border-destructive/30 bg-destructive/5 border px-3 py-2'
                    )}
                    variant='flat'
                  >
                    {message.images && message.images.length > 0 ? (
                      <div className='mb-2 flex max-w-full flex-wrap gap-2'>
                        {message.images.map((image) => (
                          <img
                            alt={image.name}
                            className='h-28 w-28 rounded-lg border object-cover sm:h-36 sm:w-36'
                            key={image.id}
                            src={image.dataUrl}
                          />
                        ))}
                      </div>
                    ) : null}

                    {message.files && message.files.length > 0 ? (
                      <div className='mb-2 flex flex-wrap gap-2'>
                        {message.files.map((file) => (
                          <span
                            className='bg-muted text-muted-foreground inline-flex max-w-full items-center gap-1.5 rounded-md border px-2 py-1 text-xs'
                            key={`${message.key}-${file.filename}`}
                          >
                            <FileText className='size-3.5 shrink-0' />
                            <span className='truncate'>{file.filename}</span>
                          </span>
                        ))}
                      </div>
                    ) : null}

                    {message.reasoning?.content ? (
                      <Reasoning
                        isStreaming={Boolean(message.isReasoningStreaming)}
                      >
                        <ReasoningTrigger />
                        <ReasoningContent>
                          {message.reasoning.content}
                        </ReasoningContent>
                      </Reasoning>
                    ) : null}

                    {messageBody}
                  </MessageContent>

                  {!isPending ? (
                    <div
                      className={cn(
                        'mt-1 flex items-center gap-0.5 opacity-100 md:opacity-0 md:group-hover:opacity-100',
                        message.from === 'user'
                          ? 'justify-end'
                          : 'justify-start'
                      )}
                    >
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Button
                              aria-label={t('Copy')}
                              onClick={() => void copyMessage(message)}
                              size='icon-sm'
                              variant='ghost'
                            />
                          }
                        >
                          <Copy className='size-3.5' />
                        </TooltipTrigger>
                        <TooltipContent>{t('Copy')}</TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Button
                              aria-label={t('Regenerate')}
                              disabled={props.isGenerating}
                              onClick={() => props.onRegenerate(message)}
                              size='icon-sm'
                              variant='ghost'
                            />
                          }
                        >
                          <RotateCcw className='size-3.5' />
                        </TooltipTrigger>
                        <TooltipContent>{t('Regenerate')}</TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Button
                              aria-label={t('Delete')}
                              className='text-muted-foreground hover:text-destructive'
                              disabled={props.isGenerating}
                              onClick={() => props.onDelete(message)}
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
                  ) : null}
                </div>
              </Message>
            )
          })}
        </div>
      </ConversationContent>
      <ConversationScrollButton />
    </Conversation>
  )
}
