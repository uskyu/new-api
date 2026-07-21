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
import { nanoid } from 'nanoid'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { SSE } from 'sse.js'

import { getFreshAuthHeaders } from '@/lib/api'

import { parseAiConsoleStreamEvent } from '../lib/stream'
import type {
  AiConsoleFile,
  AiConsoleImage,
  AiConsoleMessage,
  AiConsoleReasoningEffort,
  AiConsoleRequest,
} from '../types'

type MessageUpdater =
  | AiConsoleMessage[]
  | ((messages: AiConsoleMessage[]) => AiConsoleMessage[])

type UseAiConsoleChatProps = {
  messages: AiConsoleMessage[]
  model: string
  group: string
  reasoningEffort: AiConsoleReasoningEffort
  updateMessages: (updater: MessageUpdater) => void
  markActivity: () => void
}

type StreamSource = SSE & { readyState?: number; status?: number }

const STREAM_FLUSH_MS = 50

function currentContent(message: AiConsoleMessage): string {
  return message.versions[0]?.content || ''
}

function buildRequestMessages(messages: AiConsoleMessage[]) {
  return messages
    .filter((message) => {
      if (message.status === 'loading' || message.status === 'streaming') {
        return false
      }
      if (message.from === 'assistant' && !currentContent(message).trim()) {
        return false
      }
      if (message.from === 'assistant' && message.status === 'error') {
        return false
      }
      return message.from !== 'system' || currentContent(message).trim() !== ''
    })
    .map((message) => {
      const imageUrls = (message.images || []).map((image) => image.dataUrl)
      if (message.from === 'user' && imageUrls.length > 0) {
        return {
          role: message.from,
          content: [
            {
              type: 'text' as const,
              text: message.requestText || currentContent(message),
            },
            ...imageUrls.map((url) => ({
              type: 'image_url' as const,
              image_url: { url },
            })),
          ],
        }
      }
      return {
        role: message.from,
        content: message.requestText || currentContent(message),
      }
    })
}

function buildFileContext(text: string, files: AiConsoleFile[]): string {
  const fileContext = files
    .map(
      (file, index) =>
        `File ${index + 1}: ${file.filename}\n\n\`\`\`markdown\n${file.markdown}\n\`\`\``
    )
    .join('\n\n')
  if (!fileContext) return text
  return [text, text ? '' : null, 'Uploaded file context:', fileContext]
    .filter((value): value is string => value !== null)
    .join('\n')
}

export function useAiConsoleChat(props: UseAiConsoleChatProps) {
  const { t } = useTranslation()
  const [isGenerating, setIsGenerating] = useState(false)
  const sourceRef = useRef<StreamSource | null>(null)
  const generationRef = useRef(0)
  const pendingRef = useRef({ content: '', reasoning: '' })
  const flushTimerRef = useRef<number | null>(null)

  const closeSource = useCallback(() => {
    sourceRef.current?.close()
    sourceRef.current = null
  }, [])

  const flush = useCallback(
    (generation: number) => {
      if (generation !== generationRef.current) return
      if (flushTimerRef.current !== null) {
        window.clearTimeout(flushTimerRef.current)
        flushTimerRef.current = null
      }
      const pending = pendingRef.current
      if (!pending.content && !pending.reasoning) return
      pendingRef.current = { content: '', reasoning: '' }

      props.updateMessages((messages) => {
        if (generation !== generationRef.current) return messages
        const next = [...messages]
        const index = next.length - 1
        const message = next[index]
        if (!message || message.from !== 'assistant') return messages
        const content = currentContent(message) + pending.content
        const reasoning = (message.reasoning?.content || '') + pending.reasoning
        next[index] = {
          ...message,
          versions: [{ ...message.versions[0], content }],
          reasoning: reasoning
            ? { content: reasoning, duration: message.reasoning?.duration || 0 }
            : undefined,
          status: 'streaming',
          isReasoningStreaming: Boolean(pending.reasoning),
        }
        return next
      })
    },
    [props]
  )

  const scheduleFlush = useCallback(
    (generation: number) => {
      if (flushTimerRef.current !== null) return
      flushTimerRef.current = window.setTimeout(
        () => flush(generation),
        STREAM_FLUSH_MS
      )
    },
    [flush]
  )

  const finishWithError = useCallback(
    (generation: number, message: string, code?: string) => {
      if (generation !== generationRef.current) return
      flush(generation)
      closeSource()
      setIsGenerating(false)
      toast.error(message)
      props.updateMessages((messages) => {
        const next = [...messages]
        const index = next.length - 1
        const assistant = next[index]
        if (!assistant || assistant.from !== 'assistant') return messages
        next[index] = {
          ...assistant,
          versions: [
            {
              ...assistant.versions[0],
              content: currentContent(assistant) || message,
            },
          ],
          status: 'error',
          errorCode: code,
          isReasoningStreaming: false,
        }
        return next
      })
    },
    [closeSource, flush, props]
  )

  const complete = useCallback(
    (generation: number) => {
      if (generation !== generationRef.current) return
      flush(generation)
      closeSource()
      setIsGenerating(false)
      props.updateMessages((messages) => {
        const next = [...messages]
        const index = next.length - 1
        const assistant = next[index]
        if (!assistant || assistant.from !== 'assistant') return messages
        const content = currentContent(assistant)
        if (!content.trim() && !assistant.reasoning?.content.trim()) {
          next[index] = {
            ...assistant,
            versions: [
              { ...assistant.versions[0], content: t('Empty response') },
            ],
            status: 'error',
            isReasoningStreaming: false,
          }
          return next
        }
        next[index] = {
          ...assistant,
          status: 'complete',
          completedAt: Date.now(),
          isContentComplete: true,
          isReasoningComplete: true,
          isReasoningStreaming: false,
        }
        return next
      })
    },
    [closeSource, flush, props, t]
  )

  const startRequest = useCallback(
    async (messages: AiConsoleMessage[]) => {
      const generation = generationRef.current + 1
      generationRef.current = generation
      closeSource()
      pendingRef.current = { content: '', reasoning: '' }
      setIsGenerating(true)

      let headers: Record<string, string>
      try {
        headers = await getFreshAuthHeaders()
      } catch (error) {
        finishWithError(
          generation,
          error instanceof Error ? error.message : t('Request failed')
        )
        return
      }
      if (generation !== generationRef.current) return

      const payload: AiConsoleRequest = {
        model: props.model,
        group: props.group,
        messages: buildRequestMessages(messages),
        stream: true,
      }
      if (props.reasoningEffort) {
        payload.reasoning_effort = props.reasoningEffort
      }

      const source = new SSE('/pg/chat/completions', {
        headers,
        method: 'POST',
        payload: JSON.stringify(payload),
      }) as StreamSource
      sourceRef.current = source

      source.addEventListener('message', (event: Event & { data?: string }) => {
        if (
          generation !== generationRef.current ||
          sourceRef.current !== source
        ) {
          return
        }
        try {
          const events = parseAiConsoleStreamEvent(event.data || '')
          for (const streamEvent of events) {
            if (streamEvent.type === 'heartbeat') continue
            if (streamEvent.type === 'done') {
              complete(generation)
              return
            }
            if (streamEvent.type === 'error') {
              finishWithError(generation, streamEvent.message, streamEvent.code)
              return
            }
            pendingRef.current[streamEvent.type] += streamEvent.chunk
            scheduleFlush(generation)
          }
        } catch {
          finishWithError(generation, t('Error parsing response data'))
        }
      })
      source.addEventListener('error', (event: Event & { data?: string }) => {
        if (
          generation !== generationRef.current ||
          sourceRef.current !== source
        ) {
          return
        }
        const eventData = event.data
        let message =
          eventData || t('Network connection failed or server not responding')
        try {
          const parsed = JSON.parse(eventData || '{}') as {
            error?: { message?: string }
          }
          message = parsed.error?.message || message
        } catch {
          // Keep the transport error text when the response is not JSON.
        }
        finishWithError(generation, message)
      })
      try {
        source.stream()
      } catch {
        finishWithError(generation, t('Error establishing connection'))
      }
    },
    [
      closeSource,
      complete,
      finishWithError,
      props.group,
      props.model,
      props.reasoningEffort,
      scheduleFlush,
      t,
    ]
  )

  useEffect(
    () => () => {
      generationRef.current += 1
      if (flushTimerRef.current !== null) {
        window.clearTimeout(flushTimerRef.current)
      }
      closeSource()
    },
    [closeSource]
  )

  const send = useCallback(
    (text: string, images: AiConsoleImage[], files: AiConsoleFile[]) => {
      const trimmed = text.trim()
      if (!trimmed && images.length === 0 && files.length === 0) return
      const requestText = buildFileContext(trimmed, files)
      const displayText =
        trimmed ||
        files.map((file) => file.filename).join(', ') ||
        t('Image attachment')
      const submittedAt = Date.now()
      const userMessage: AiConsoleMessage = {
        key: nanoid(),
        from: 'user',
        versions: [{ id: nanoid(), content: displayText }],
        createdAt: submittedAt,
        status: 'complete',
        requestText,
        images,
        files: files.map((file) => ({
          filename: file.filename,
          size: file.size,
          warnings: file.warnings,
        })),
      }
      const assistantMessage: AiConsoleMessage = {
        key: nanoid(),
        from: 'assistant',
        versions: [{ id: nanoid(), content: '' }],
        createdAt: submittedAt,
        startedAt: submittedAt,
        status: 'loading',
      }
      const nextMessages = [...props.messages, userMessage, assistantMessage]
      props.markActivity()
      props.updateMessages(nextMessages)
      void startRequest(nextMessages)
    },
    [props, startRequest, t]
  )

  const stop = useCallback(() => {
    const generation = generationRef.current
    flush(generation)
    generationRef.current += 1
    closeSource()
    setIsGenerating(false)
    props.updateMessages((messages) => {
      const next = [...messages]
      const index = next.length - 1
      const assistant = next[index]
      if (!assistant || assistant.from !== 'assistant') return messages
      next[index] = {
        ...assistant,
        status: 'complete',
        completedAt: Date.now(),
        isReasoningStreaming: false,
      }
      return next
    })
  }, [closeSource, flush, props])

  const regenerate = useCallback(
    (message: AiConsoleMessage) => {
      if (isGenerating) return
      const index = props.messages.findIndex((item) => item.key === message.key)
      if (index < 0) return
      const keepCount = message.from === 'user' ? index + 1 : index
      const base = props.messages.slice(0, keepCount)
      const assistant: AiConsoleMessage = {
        key: nanoid(),
        from: 'assistant',
        versions: [{ id: nanoid(), content: '' }],
        createdAt: Date.now(),
        startedAt: Date.now(),
        status: 'loading',
      }
      const next = [...base, assistant]
      props.markActivity()
      props.updateMessages(next)
      void startRequest(next)
    },
    [isGenerating, props, startRequest]
  )

  const remove = useCallback(
    (message: AiConsoleMessage) => {
      if (isGenerating) return
      props.updateMessages((messages) => {
        const index = messages.findIndex((item) => item.key === message.key)
        if (index < 0) return messages
        let deleteCount = 1
        if (
          message.from === 'user' &&
          messages[index + 1]?.from === 'assistant'
        ) {
          deleteCount = 2
        }
        return [
          ...messages.slice(0, index),
          ...messages.slice(index + deleteCount),
        ]
      })
    },
    [isGenerating, props]
  )

  return { isGenerating, send, stop, regenerate, remove }
}
