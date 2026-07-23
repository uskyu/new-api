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
import { Search } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'

import { AgentPagination } from './components/agent-pagination'
import { SupportUsersTable } from './components/support-users-table'
import { useSupportUsers } from './hooks/use-agent-data'

const PAGE_SIZE = 10

export function SupportUserManagement() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [input, setInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const users = useSupportUsers({
    page,
    pageSize: PAGE_SIZE,
    keyword,
    enabled: keyword.length > 0,
  })
  const data = users.data?.data

  const handleSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const nextKeyword = input.trim()
    if (!nextKeyword) return
    setPage(1)
    setKeyword(nextKeyword)
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('User Management')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <Card>
          <CardHeader>
            <CardTitle>{t('Search and change upstream agents')}</CardTitle>
          </CardHeader>
          <CardContent className='flex flex-col gap-4'>
            <form className='flex gap-2' onSubmit={handleSearch}>
              <Input
                value={input}
                onChange={(event) => setInput(event.target.value)}
                placeholder={t('Search users by ID or name')}
                aria-label={t('Search users')}
              />
              <Button type='submit' disabled={!input.trim()}>
                <Search data-icon='inline-start' />
                {t('Search')}
              </Button>
            </form>
            <SupportUsersTable
              users={data?.items ?? []}
              loading={users.isLoading}
              searched={keyword.length > 0}
            />
            {keyword && (
              <AgentPagination
                page={page}
                pageSize={PAGE_SIZE}
                total={data?.total ?? 0}
                onPageChange={setPage}
              />
            )}
          </CardContent>
        </Card>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
