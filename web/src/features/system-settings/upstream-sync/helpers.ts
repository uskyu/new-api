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
export type SyncStatusTone =
  | 'success'
  | 'danger'
  | 'warning'
  | 'info'
  | 'neutral'

export const SYNC_STATUS_TONE_CLASS: Record<SyncStatusTone, string> = {
  success:
    'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300',
  danger: '',
  warning:
    'bg-amber-50 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300',
  info: 'bg-sky-50 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300',
  neutral: '',
}

export const SYNC_STATUS_DOT_CLASS: Record<SyncStatusTone, string> = {
  success: 'bg-emerald-500',
  danger: 'bg-destructive',
  warning: 'bg-amber-500',
  info: 'bg-sky-500',
  neutral: 'bg-muted-foreground',
}

function normalizeToken(value?: string | null): string {
  return (value ?? '').trim().toLowerCase().replaceAll('_', '-')
}

export function getSyncStatusTone(status?: string | null): SyncStatusTone {
  switch (normalizeToken(status)) {
    case 'success':
    case 'succeeded':
    case 'complete':
    case 'completed':
      return 'success'
    case 'failed':
    case 'error':
      return 'danger'
    case 'pending':
    case 'queued':
    case 'waiting':
      return 'warning'
    case 'partial':
      return 'warning'
    case 'running':
    case 'in-progress':
    case 'processing':
      return 'info'
    default:
      return 'neutral'
  }
}

export function getSyncStatusLabel(status?: string | null): string {
  switch (normalizeToken(status)) {
    case 'success':
    case 'succeeded':
    case 'complete':
    case 'completed':
      return 'Success'
    case 'failed':
    case 'error':
      return 'Failed'
    case 'pending':
    case 'queued':
    case 'waiting':
      return 'Waiting'
    case 'partial':
      return 'Partial'
    case 'running':
    case 'in-progress':
    case 'processing':
      return 'Running'
    default:
      return status?.trim() || '-'
  }
}

export function getSyncTriggerLabel(trigger?: string | null): string {
  switch (normalizeToken(trigger)) {
    case 'manual':
      return 'Manual'
    case 'schedule':
    case 'scheduled':
    case 'cron':
    case 'auto':
      return 'Scheduled'
    default:
      return trigger?.trim() || '-'
  }
}

/**
 * `added_models`, `removed_models`, `price_changes` and `errors` are
 * intentionally typed as unknown because the backend returns JSON arrays or
 * objects depending on the type of change. Count the entries without losing
 * non-array payloads.
 */
export function getJsonEntryCount(value: unknown): number | null {
  if (value === null || value === undefined) return null
  if (Array.isArray(value)) return value.length
  if (typeof value === 'object') return Object.keys(value).length
  if (typeof value === 'number') return Number.isFinite(value) ? value : null
  if (typeof value === 'string') return value.trim() ? 1 : 0
  return 1
}

export function formatDurationMs(value?: number | null): string {
  if (value === null || value === undefined || !Number.isFinite(value)) {
    return '-'
  }
  if (value < 0) return '-'
  if (value < 1000) return `${Math.round(value)} ms`

  const seconds = value / 1000
  if (seconds < 60) {
    return `${seconds.toFixed(seconds < 10 ? 2 : 1)} s`
  }

  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = Math.round(seconds % 60)
  if (minutes < 60) {
    return remainingSeconds
      ? `${minutes} min ${remainingSeconds} s`
      : `${minutes} min`
  }

  const hours = Math.floor(minutes / 60)
  const remainingMinutes = minutes % 60
  return remainingMinutes ? `${hours} h ${remainingMinutes} min` : `${hours} h`
}

export function formatJsonDetail(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) return ''
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2)
    } catch {
      return trimmed
    }
  }
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}
