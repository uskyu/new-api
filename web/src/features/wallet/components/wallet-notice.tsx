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
import { Bell } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { RichContent } from '@/components/rich-content'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'

interface WalletNoticeProps {
  content?: string
}

export function WalletNotice(props: WalletNoticeProps) {
  const { t } = useTranslation()
  const content = props.content?.trim()

  if (!content) {
    return null
  }

  return (
    <Card
      id='wallet-topup-help'
      data-card-hover='false'
      className='scroll-mt-4 gap-0 overflow-hidden py-0'
    >
      <CardHeader className='border-b px-3 py-3 sm:px-5 sm:py-4'>
        <CardTitle className='flex min-w-0 items-center gap-2 text-sm sm:text-base'>
          <IconBadge tone='warning' size='xs'>
            <Bell />
          </IconBadge>
          <span className='truncate'>{t('Wallet notice')}</span>
        </CardTitle>
      </CardHeader>
      <CardContent className='min-w-0 overflow-hidden p-3 sm:p-5'>
        <RichContent
          mode='html'
          content={content}
          className='text-sm leading-6 break-words [&_a]:break-all [&_img]:h-auto [&_img]:max-w-full [&_pre]:max-w-full [&_pre]:overflow-x-auto [&_table]:block [&_table]:max-w-full [&_table]:overflow-x-auto [&_video]:h-auto [&_video]:max-w-full'
        />
      </CardContent>
    </Card>
  )
}
