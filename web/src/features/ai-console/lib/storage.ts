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

import type { ContentPart, MessageStatus } from '@/features/playground/types'

import type {
  AiConsoleImage,
  AiConsoleMessage,
  AiConsolePreferences,
  AiConsoleSession,
} from '../types'

const DB_NAME = 'ai_console_db'
const DB_VERSION = 1
const STORE_SESSIONS = 'sessions'
const STORE_MESSAGES = 'messages'
const CURRENT_SESSION_KEY = 'ai_console_current_session'
const PREFERENCES_KEY = 'ai_console_prefs'
const DEFAULT_MODEL = 'gpt-4o'

type StoredRecord = Record<string, unknown>

function asRecord(value: unknown): StoredRecord {
  return value && typeof value === 'object' ? (value as StoredRecord) : {}
}

function normalizeStatus(value: unknown, hasContent: boolean): MessageStatus {
  if (value === 'error') return 'error'
  if (value === 'loading' || value === 'streaming' || value === 'incomplete') {
    return hasContent ? 'complete' : 'error'
  }
  return 'complete'
}

function readLegacyContent(value: unknown): {
  text: string
  images: AiConsoleImage[]
} {
  if (typeof value === 'string') return { text: value, images: [] }
  if (!Array.isArray(value)) return { text: '', images: [] }

  const parts = value.filter((part): part is ContentPart =>
    Boolean(part && typeof part === 'object')
  )
  const text = parts
    .filter((part) => part.type === 'text')
    .map((part) => part.text || '')
    .join('\n')
  const images = parts
    .filter((part) => part.type === 'image_url' && part.image_url?.url)
    .map((part, index) => ({
      id: `legacy-image-${index}-${nanoid()}`,
      dataUrl: part.image_url?.url || '',
      name: `image-${index + 1}`,
    }))

  return { text, images }
}

export function normalizeStoredMessage(value: unknown): AiConsoleMessage {
  const record = asRecord(value)
  const legacyContent = readLegacyContent(record.content)
  const storedVersions = Array.isArray(record.versions) ? record.versions : []
  const versionRecord = asRecord(storedVersions[0])
  const content =
    typeof versionRecord.content === 'string'
      ? versionRecord.content
      : legacyContent.text
  let from: AiConsoleMessage['from'] = 'user'
  if (record.from === 'assistant' || record.role === 'assistant') {
    from = 'assistant'
  } else if (record.from === 'system' || record.role === 'system') {
    from = 'system'
  }
  const reasoningRecord = asRecord(record.reasoning)
  let reasoningContent = ''
  if (typeof reasoningRecord.content === 'string') {
    reasoningContent = reasoningRecord.content
  } else if (typeof record.reasoningContent === 'string') {
    reasoningContent = record.reasoningContent
  }

  let key = nanoid()
  if (typeof record.key === 'string') {
    key = record.key
  } else if (typeof record.id === 'string') {
    key = record.id
  }

  let createdAt = Date.now()
  if (typeof record.createdAt === 'number') {
    createdAt = record.createdAt
  } else if (typeof record.createAt === 'number') {
    createdAt = record.createAt
  }

  const message: AiConsoleMessage = {
    key,
    from,
    versions: [
      {
        id: typeof versionRecord.id === 'string' ? versionRecord.id : nanoid(),
        content,
      },
    ],
    createdAt,
    status: normalizeStatus(record.status, content.trim() !== ''),
  }

  if (typeof record.requestText === 'string') {
    message.requestText = record.requestText
  } else if (from === 'user') {
    message.requestText = legacyContent.text || content
  }
  if (Array.isArray(record.images)) {
    message.images = record.images as AiConsoleImage[]
  } else if (legacyContent.images.length > 0) {
    message.images = legacyContent.images
  }
  if (Array.isArray(record.files)) {
    message.files = record.files as AiConsoleMessage['files']
  } else if (Array.isArray(record.attachments)) {
    message.files = record.attachments as AiConsoleMessage['files']
  }
  if (reasoningContent) {
    message.reasoning = {
      content: reasoningContent,
      duration:
        typeof reasoningRecord.duration === 'number'
          ? reasoningRecord.duration
          : 0,
    }
  }

  return message
}

function requestResult<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.addEventListener('success', () => resolve(request.result), {
      once: true,
    })
    request.addEventListener('error', () => reject(request.error), {
      once: true,
    })
  })
}

function transactionComplete(transaction: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    transaction.addEventListener('complete', () => resolve(), { once: true })
    transaction.addEventListener('error', () => reject(transaction.error), {
      once: true,
    })
    transaction.addEventListener('abort', () => reject(transaction.error), {
      once: true,
    })
  })
}

function openDatabase(): Promise<IDBDatabase> {
  if (!window.indexedDB) {
    return Promise.reject(new Error('IndexedDB is not available'))
  }

  return new Promise((resolve, reject) => {
    const request = window.indexedDB.open(DB_NAME, DB_VERSION)
    request.addEventListener('upgradeneeded', () => {
      const database = request.result
      if (!database.objectStoreNames.contains(STORE_SESSIONS)) {
        database.createObjectStore(STORE_SESSIONS, { keyPath: 'id' })
      }
      if (!database.objectStoreNames.contains(STORE_MESSAGES)) {
        const store = database.createObjectStore(STORE_MESSAGES, {
          keyPath: 'id',
        })
        store.createIndex('sessionId', 'sessionId', { unique: false })
        store.createIndex('createdAt', 'createdAt', { unique: false })
        store.createIndex('sortIndex', 'sortIndex', { unique: false })
      }
    })
    request.addEventListener('success', () => resolve(request.result), {
      once: true,
    })
    request.addEventListener('error', () => reject(request.error), {
      once: true,
    })
  })
}

export function createSessionRecord(
  preferences: AiConsolePreferences = {}
): AiConsoleSession {
  return {
    id: `session-${Date.now()}-${nanoid()}`,
    title: 'New conversation',
    model: preferences.selectedModel || DEFAULT_MODEL,
    group: preferences.selectedGroup || '',
    reasoningEffort: preferences.reasoningEffort || '',
    customTitle: false,
    updatedAt: Date.now(),
    lastMessagePreview: '',
  }
}

export async function loadAllSessions(): Promise<AiConsoleSession[]> {
  const database = await openDatabase()
  const transaction = database.transaction(STORE_SESSIONS, 'readonly')
  const sessions = await requestResult(
    transaction.objectStore(STORE_SESSIONS).getAll()
  )
  database.close()
  return (sessions as AiConsoleSession[]).sort(
    (left, right) => right.updatedAt - left.updatedAt
  )
}

export async function putSession(session: AiConsoleSession): Promise<void> {
  const database = await openDatabase()
  const transaction = database.transaction(STORE_SESSIONS, 'readwrite')
  transaction.objectStore(STORE_SESSIONS).put(session)
  await transactionComplete(transaction)
  database.close()
}

export async function loadSessionMessages(
  sessionId: string
): Promise<AiConsoleMessage[]> {
  const database = await openDatabase()
  const transaction = database.transaction(STORE_MESSAGES, 'readonly')
  const messages = await requestResult(
    transaction
      .objectStore(STORE_MESSAGES)
      .index('sessionId')
      .getAll(IDBKeyRange.only(sessionId))
  )
  database.close()

  return (messages as StoredRecord[])
    .sort((left, right) => Number(left.sortIndex) - Number(right.sortIndex))
    .map(normalizeStoredMessage)
}

export async function replaceSessionMessages(
  sessionId: string,
  messages: AiConsoleMessage[]
): Promise<void> {
  const database = await openDatabase()
  const clearTransaction = database.transaction(STORE_MESSAGES, 'readwrite')
  const clearStore = clearTransaction.objectStore(STORE_MESSAGES)
  const cursorRequest = clearStore
    .index('sessionId')
    .openKeyCursor(IDBKeyRange.only(sessionId))

  cursorRequest.addEventListener('success', () => {
    const cursor = cursorRequest.result
    if (cursor) {
      clearStore.delete(cursor.primaryKey)
      cursor.continue()
    }
  })
  await transactionComplete(clearTransaction)

  const writeTransaction = database.transaction(STORE_MESSAGES, 'readwrite')
  const writeStore = writeTransaction.objectStore(STORE_MESSAGES)
  messages.forEach((message, sortIndex) => {
    writeStore.put({
      ...message,
      id: message.key,
      sessionId,
      sortIndex,
    })
  })
  await transactionComplete(writeTransaction)
  database.close()
}

export async function deleteSession(sessionId: string): Promise<void> {
  const database = await openDatabase()
  const transaction = database.transaction(
    [STORE_SESSIONS, STORE_MESSAGES],
    'readwrite'
  )
  transaction.objectStore(STORE_SESSIONS).delete(sessionId)

  const store = transaction.objectStore(STORE_MESSAGES)
  const cursorRequest = store
    .index('sessionId')
    .openKeyCursor(IDBKeyRange.only(sessionId))
  cursorRequest.addEventListener('success', () => {
    const cursor = cursorRequest.result
    if (!cursor) return
    store.delete(cursor.primaryKey)
    cursor.continue()
  })

  await transactionComplete(transaction)
  database.close()
}

export function getCurrentSessionId(): string | null {
  return localStorage.getItem(CURRENT_SESSION_KEY)
}

export function setCurrentSessionId(sessionId: string): void {
  localStorage.setItem(CURRENT_SESSION_KEY, sessionId)
}

export function readPreferences(): AiConsolePreferences {
  try {
    const raw = localStorage.getItem(PREFERENCES_KEY)
    return raw ? (JSON.parse(raw) as AiConsolePreferences) : {}
  } catch {
    return {}
  }
}

export function writePreferences(preferences: AiConsolePreferences): void {
  try {
    localStorage.setItem(PREFERENCES_KEY, JSON.stringify(preferences))
  } catch {
    // IndexedDB remains the source of truth when lightweight prefs cannot save.
  }
}
