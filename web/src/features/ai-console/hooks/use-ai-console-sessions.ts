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
import { useCallback, useEffect, useRef, useState } from 'react'

import type { Message } from '@/features/playground/types'

import {
  createSessionRecord,
  deleteSession,
  getCurrentSessionId,
  loadAllSessions,
  loadSessionMessages,
  putSession,
  readPreferences,
  replaceSessionMessages,
  setCurrentSessionId,
  writePreferences,
} from '../lib/storage'
import type {
  AiConsoleMessage,
  AiConsoleReasoningEffort,
  AiConsoleSession,
} from '../types'

const SAVE_DEBOUNCE_MS = 200
const TITLE_MAX_LENGTH = 24

type MessageUpdater =
  | AiConsoleMessage[]
  | ((messages: AiConsoleMessage[]) => AiConsoleMessage[])

function getMessageText(message: Message): string {
  return message.versions[0]?.content || ''
}

function deriveSessionMetadata(
  session: AiConsoleSession,
  messages: AiConsoleMessage[],
  touched: boolean
): AiConsoleSession {
  const firstUserMessage = messages.find((message) => message.from === 'user')
  const latestVisibleMessage = [...messages]
    .reverse()
    .find((message) => message.status !== 'loading')
  const firstUserText = firstUserMessage
    ? getMessageText(firstUserMessage).trim()
    : ''

  return {
    ...session,
    title:
      session.customTitle || !firstUserText
        ? session.title
        : firstUserText.slice(0, TITLE_MAX_LENGTH),
    updatedAt: touched ? Date.now() : session.updatedAt,
    lastMessagePreview: latestVisibleMessage
      ? getMessageText(latestVisibleMessage).trim().slice(0, 80)
      : '',
  }
}

export function useAiConsoleSessions() {
  const initialPreferences = useRef(readPreferences())
  const [ready, setReady] = useState(false)
  const [storageError, setStorageError] = useState<string | null>(null)
  const [sessions, setSessions] = useState<AiConsoleSession[]>([])
  const [currentSessionId, setActiveSessionId] = useState('')
  const [messages, setMessages] = useState<AiConsoleMessage[]>([])
  const [model, setModelState] = useState(
    initialPreferences.current.selectedModel || 'gpt-4o'
  )
  const [group, setGroupState] = useState(
    initialPreferences.current.selectedGroup || ''
  )
  const [reasoningEffort, setReasoningEffortState] =
    useState<AiConsoleReasoningEffort>(
      initialPreferences.current.reasoningEffort || ''
    )

  const sessionsRef = useRef(sessions)
  const messagesRef = useRef(messages)
  const currentSessionIdRef = useRef(currentSessionId)
  const modelRef = useRef(model)
  const groupRef = useRef(group)
  const reasoningEffortRef = useRef(reasoningEffort)
  const saveTimerRef = useRef<number | null>(null)
  const touchedRef = useRef(false)
  const saveChainRef = useRef(Promise.resolve())

  useEffect(() => {
    sessionsRef.current = sessions
  }, [sessions])
  useEffect(() => {
    messagesRef.current = messages
  }, [messages])
  useEffect(() => {
    currentSessionIdRef.current = currentSessionId
  }, [currentSessionId])
  useEffect(() => {
    modelRef.current = model
  }, [model])
  useEffect(() => {
    groupRef.current = group
  }, [group])
  useEffect(() => {
    reasoningEffortRef.current = reasoningEffort
  }, [reasoningEffort])

  const persistCurrent = useCallback(() => {
    if (saveTimerRef.current !== null) {
      window.clearTimeout(saveTimerRef.current)
      saveTimerRef.current = null
    }

    const sessionId = currentSessionIdRef.current
    const session = sessionsRef.current.find((item) => item.id === sessionId)
    if (!session) return saveChainRef.current

    const nextSession = deriveSessionMetadata(
      {
        ...session,
        model: modelRef.current,
        group: groupRef.current,
        reasoningEffort: reasoningEffortRef.current,
      },
      messagesRef.current,
      touchedRef.current
    )
    const messagesToSave = messagesRef.current
    touchedRef.current = false
    sessionsRef.current = [
      nextSession,
      ...sessionsRef.current.filter((item) => item.id !== sessionId),
    ].sort((left, right) => right.updatedAt - left.updatedAt)
    setSessions(sessionsRef.current)

    saveChainRef.current = saveChainRef.current
      .catch(() => undefined)
      .then(async () => {
        await putSession(nextSession)
        await replaceSessionMessages(sessionId, messagesToSave)
      })
    return saveChainRef.current
  }, [])

  const scheduleSave = useCallback(() => {
    if (!ready || !currentSessionIdRef.current) return
    if (saveTimerRef.current !== null) {
      window.clearTimeout(saveTimerRef.current)
    }
    saveTimerRef.current = window.setTimeout(() => {
      void persistCurrent().catch(() => setStorageError('save'))
    }, SAVE_DEBOUNCE_MS)
  }, [persistCurrent, ready])

  useEffect(() => {
    let cancelled = false

    const initialize = async () => {
      try {
        let loadedSessions = await loadAllSessions()
        if (loadedSessions.length === 0) {
          const firstSession = createSessionRecord(initialPreferences.current)
          await putSession(firstSession)
          loadedSessions = [firstSession]
        }

        const storedId = getCurrentSessionId()
        const activeSession =
          loadedSessions.find((session) => session.id === storedId) ||
          loadedSessions[0]
        const loadedMessages = await loadSessionMessages(activeSession.id)
        if (cancelled) return

        sessionsRef.current = loadedSessions
        messagesRef.current = loadedMessages
        currentSessionIdRef.current = activeSession.id
        setSessions(loadedSessions)
        setMessages(loadedMessages)
        setActiveSessionId(activeSession.id)
        setCurrentSessionId(activeSession.id)
        setModelState(activeSession.model || modelRef.current)
        setGroupState(activeSession.group || groupRef.current)
        setReasoningEffortState(activeSession.reasoningEffort || '')
      } catch {
        if (!cancelled) setStorageError('initialize')
      } finally {
        if (!cancelled) setReady(true)
      }
    }

    void initialize()
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(
    () => () => {
      if (saveTimerRef.current !== null) {
        window.clearTimeout(saveTimerRef.current)
        void persistCurrent()
      }
    },
    [persistCurrent]
  )

  const updateMessages = useCallback(
    (updater: MessageUpdater) => {
      const next =
        typeof updater === 'function' ? updater(messagesRef.current) : updater
      messagesRef.current = next
      setMessages(next)
      scheduleSave()
    },
    [scheduleSave]
  )

  const markActivity = useCallback(() => {
    touchedRef.current = true
  }, [])

  const setModel = useCallback(
    (value: string) => {
      modelRef.current = value
      setModelState(value)
      writePreferences({
        selectedModel: value,
        selectedGroup: groupRef.current,
        reasoningEffort: reasoningEffortRef.current,
      })
      scheduleSave()
    },
    [scheduleSave]
  )

  const setGroup = useCallback(
    (value: string) => {
      groupRef.current = value
      setGroupState(value)
      writePreferences({
        selectedModel: modelRef.current,
        selectedGroup: value,
        reasoningEffort: reasoningEffortRef.current,
      })
      scheduleSave()
    },
    [scheduleSave]
  )

  const setReasoningEffort = useCallback(
    (value: AiConsoleReasoningEffort) => {
      reasoningEffortRef.current = value
      setReasoningEffortState(value)
      writePreferences({
        selectedModel: modelRef.current,
        selectedGroup: groupRef.current,
        reasoningEffort: value,
      })
      scheduleSave()
    },
    [scheduleSave]
  )

  const createSession = useCallback(async () => {
    await persistCurrent()
    const session = createSessionRecord({
      selectedModel: modelRef.current,
      selectedGroup: groupRef.current,
      reasoningEffort: reasoningEffortRef.current,
    })
    await putSession(session)
    sessionsRef.current = [session, ...sessionsRef.current]
    messagesRef.current = []
    currentSessionIdRef.current = session.id
    setSessions(sessionsRef.current)
    setMessages([])
    setActiveSessionId(session.id)
    setCurrentSessionId(session.id)
  }, [persistCurrent])

  const switchSession = useCallback(
    async (sessionId: string) => {
      if (!sessionId || sessionId === currentSessionIdRef.current) return
      await persistCurrent()
      const session = sessionsRef.current.find((item) => item.id === sessionId)
      if (!session) return
      const loadedMessages = await loadSessionMessages(sessionId)

      currentSessionIdRef.current = sessionId
      messagesRef.current = loadedMessages
      modelRef.current = session.model
      groupRef.current = session.group
      reasoningEffortRef.current = session.reasoningEffort || ''
      setActiveSessionId(sessionId)
      setCurrentSessionId(sessionId)
      setMessages(loadedMessages)
      setModelState(session.model)
      setGroupState(session.group)
      setReasoningEffortState(session.reasoningEffort || '')
    },
    [persistCurrent]
  )

  const renameSession = useCallback(
    async (sessionId: string, title: string) => {
      const target = sessionsRef.current.find((item) => item.id === sessionId)
      if (!target) return
      const nextSession = {
        ...target,
        title: title.trim() || 'New conversation',
        customTitle: true,
        updatedAt: Date.now(),
      }
      await putSession(nextSession)
      sessionsRef.current = [
        nextSession,
        ...sessionsRef.current.filter((item) => item.id !== sessionId),
      ]
      setSessions(sessionsRef.current)
    },
    []
  )

  const removeSession = useCallback(
    async (sessionId: string) => {
      await deleteSession(sessionId)
      const remaining = sessionsRef.current.filter(
        (item) => item.id !== sessionId
      )
      sessionsRef.current = remaining
      setSessions(remaining)
      if (sessionId !== currentSessionIdRef.current) return
      if (remaining.length === 0) {
        await createSession()
        return
      }
      currentSessionIdRef.current = ''
      await switchSession(remaining[0].id)
    },
    [createSession, switchSession]
  )

  const clearMessages = useCallback(() => {
    touchedRef.current = true
    updateMessages([])
  }, [updateMessages])

  return {
    ready,
    storageError,
    sessions,
    currentSessionId,
    messages,
    model,
    group,
    reasoningEffort,
    updateMessages,
    markActivity,
    setModel,
    setGroup,
    setReasoningEffort,
    createSession,
    switchSession,
    renameSession,
    removeSession,
    clearMessages,
  }
}
