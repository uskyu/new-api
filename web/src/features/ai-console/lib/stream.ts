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
export type AiConsoleStreamEvent =
  | { type: 'heartbeat' }
  | { type: 'done' }
  | { type: 'content'; chunk: string }
  | { type: 'reasoning'; chunk: string }
  | { type: 'error'; message: string; code?: string }

type StreamPayload = {
  type?: string
  event?: string
  error?: { message?: string; code?: string }
  choices?: Array<{
    delta?: {
      content?: unknown
      reasoning_content?: unknown
      reasoning?: unknown
    }
  }>
}

const HEARTBEAT_VALUES = new Set([
  '',
  'ping',
  'heartbeat',
  'keepalive',
  '[PING]',
  '[HEARTBEAT]',
  '[KEEPALIVE]',
])

export function parseAiConsoleStreamEvent(
  data: string
): AiConsoleStreamEvent[] {
  const normalized = data.trim()
  if (HEARTBEAT_VALUES.has(normalized) || normalized.startsWith(':')) {
    return [{ type: 'heartbeat' }]
  }
  if (normalized === '[DONE]') {
    return [{ type: 'done' }]
  }

  const payload = JSON.parse(normalized) as StreamPayload
  if (
    payload.type === 'ping' ||
    payload.type === 'heartbeat' ||
    payload.event === 'ping' ||
    payload.event === 'heartbeat'
  ) {
    return [{ type: 'heartbeat' }]
  }
  if (payload.error) {
    return [
      {
        type: 'error',
        message: payload.error.message || 'Request failed',
        code: payload.error.code,
      },
    ]
  }

  const delta = payload.choices?.[0]?.delta
  if (!delta) return []

  const events: AiConsoleStreamEvent[] = []
  const reasoning = delta.reasoning_content ?? delta.reasoning
  if (typeof reasoning === 'string' && reasoning) {
    events.push({ type: 'reasoning', chunk: reasoning })
  }
  if (typeof delta.content === 'string' && delta.content) {
    events.push({ type: 'content', chunk: delta.content })
  }
  return events
}
