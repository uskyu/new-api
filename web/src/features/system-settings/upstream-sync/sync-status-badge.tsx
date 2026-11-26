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
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

import {
  SYNC_STATUS_DOT_CLASS,
  SYNC_STATUS_TONE_CLASS,
  getSyncStatusLabel,
  getSyncStatusTone,
} from './helpers'

type SyncStatusBadgeProps = {
  status?: string | null
  className?: string
}

export function SyncStatusBadge({ status, className }: SyncStatusBadgeProps) {
  const { t } = useTranslation()
  const tone = getSyncStatusTone(status)
  const label = getSyncStatusLabel(status)

  return (
    <Badge
      variant={tone === 'danger' ? 'destructive' : 'secondary'}
      className={cn('gap-1.5', SYNC_STATUS_TONE_CLASS[tone], className)}
    >
      <span
        className={cn('size-1.5 rounded-full', SYNC_STATUS_DOT_CLASS[tone])}
        aria-hidden='true'
      />
      {t(label, { skipInterpolation: true })}
    </Badge>
  )
}
