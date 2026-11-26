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
import { ScrollText } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  sideDrawerContentClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { useIsMobile } from '@/hooks/use-mobile'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  formatDurationMs,
  formatJsonDetail,
  getJsonEntryCount,
  getSyncTriggerLabel,
} from './helpers'
import { SyncStatusBadge } from './sync-status-badge'
import type { UpstreamSyncLog } from './types'

type SyncLogDetailSheetProps = {
  log: UpstreamSyncLog | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

function DetailField(props: {
  label: string
  value: React.ReactNode
  className?: string
}) {
  return (
    <div className={cn('min-w-0 space-y-1', props.className)}>
      <div className='text-muted-foreground text-xs font-medium'>
        {props.label}
      </div>
      <div className='text-sm font-medium break-words'>{props.value}</div>
    </div>
  )
}

function JsonBlock(props: {
  title: string
  value: unknown
  emptyLabel: string
}) {
  const content = formatJsonDetail(props.value)
  const count = getJsonEntryCount(props.value)

  return (
    <section className='space-y-2'>
      <div className='flex items-center justify-between gap-3'>
        <h3 className='text-sm font-semibold'>{props.title}</h3>
        {count !== null && (
          <span className='text-muted-foreground text-xs tabular-nums'>
            {count}
          </span>
        )}
      </div>
      {content ? (
        <pre className='bg-muted/30 text-muted-foreground m-0 max-w-full overflow-x-auto rounded-md border p-3 text-xs leading-relaxed whitespace-pre-wrap'>
          {content}
        </pre>
      ) : (
        <p className='text-muted-foreground text-xs'>{props.emptyLabel}</p>
      )}
    </section>
  )
}

export function SyncLogDetailSheet({
  log,
  open,
  onOpenChange,
}: SyncLogDetailSheetProps) {
  const { t } = useTranslation()
  const isMobile = useIsMobile()

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side={isMobile && log ? 'bottom' : 'right'}
        className={
          isMobile
            ? sideDrawerContentClassName('h-auto max-h-[85dvh] rounded-t-xl')
            : sideDrawerContentClassName('sm:max-w-2xl lg:max-w-3xl')
        }
      >
        {log && (
          <>
            <SheetHeader className={sideDrawerHeaderClassName('sm:px-5')}>
              <SheetTitle className='flex items-center gap-2 pr-10'>
                <ScrollText className='text-muted-foreground size-4' />
                {t('Log Detail')}
                <span className='text-muted-foreground font-mono text-xs font-normal'>
                  #{log.id}
                </span>
              </SheetTitle>
              <SheetDescription className='pr-10'>
                {formatTimestampToDate(log.started_at)} ·{' '}
                {t(getSyncTriggerLabel(log.trigger))}
              </SheetDescription>
            </SheetHeader>

            <div className={sideDrawerFormClassName('gap-5 sm:px-5')}>
              <div className='grid grid-cols-2 gap-x-4 gap-y-5 sm:grid-cols-3'>
                <DetailField
                  label={t('Status')}
                  value={<SyncStatusBadge status={log.status} />}
                />
                <DetailField
                  label={t('Start Time')}
                  value={formatTimestampToDate(log.started_at)}
                />
                <DetailField
                  label={t('End Time')}
                  value={
                    log.finished_at
                      ? formatTimestampToDate(log.finished_at)
                      : '-'
                  }
                />
                <DetailField
                  label={t('Duration')}
                  value={formatDurationMs(log.duration_ms)}
                />
                <DetailField
                  label={t('Groups Checked')}
                  value={log.groups_checked ?? '-'}
                />
                <DetailField
                  label={t('Channels Changed')}
                  value={log.channels_changed ?? '-'}
                />
                <DetailField
                  label={t('Base URL')}
                  value={
                    <span className='font-mono text-xs break-all'>
                      {log.base_url || '-'}
                    </span>
                  }
                  className='col-span-2'
                />
                <DetailField
                  label={t('Pricing Version')}
                  value={
                    <span className='font-mono text-xs break-all'>
                      {log.pricing_version || '-'}
                    </span>
                  }
                />
              </div>

              {log.message && (
                <section className='space-y-2'>
                  <h3 className='text-sm font-semibold'>{t('Message')}</h3>
                  <p className='text-muted-foreground text-sm leading-relaxed break-words'>
                    {log.message}
                  </p>
                </section>
              )}

              <JsonBlock
                title={t('Added Models')}
                value={log.added_models}
                emptyLabel={t('No data')}
              />
              <JsonBlock
                title={t('Removed Models')}
                value={log.removed_models}
                emptyLabel={t('No data')}
              />
              <JsonBlock
                title={t('Price Changes')}
                value={log.price_changes}
                emptyLabel={t('No data')}
              />
              <JsonBlock
                title={t('Errors')}
                value={log.errors}
                emptyLabel={t('No data')}
              />

              <section className='space-y-2'>
                <h3 className='text-sm font-semibold'>{t('Raw JSON')}</h3>
                <pre className='bg-muted/30 text-muted-foreground m-0 max-w-full overflow-x-auto rounded-md border p-3 text-xs leading-relaxed whitespace-pre-wrap'>
                  {formatJsonDetail(log)}
                </pre>
              </section>
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  )
}
