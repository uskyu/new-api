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
import { useMutation, useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Check, KeyRound, MonitorSmartphone, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Main } from '@/components/layout'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Spinner } from '@/components/ui/spinner'
import { useAuthStore } from '@/stores/auth-store'

import { approveQuickLogin, getQuickLoginAuthorization } from './api'

export function QuickLoginAuthorizationPage({ code }: { code: string }) {
  const { t } = useTranslation()
  const username = useAuthStore((state) => state.auth.user?.username ?? '')
  const authorizationQuery = useQuery({
    queryKey: ['quick-login-authorization', code],
    queryFn: () => getQuickLoginAuthorization(code),
    enabled: code.length > 0,
    retry: false,
  })
  const authorization = authorizationQuery.data?.data
  const approveMutation = useMutation({
    mutationFn: () => approveQuickLogin(code),
    onSuccess: (response) => {
      if (response.success && response.data?.callback_url) {
        window.location.assign(response.data.callback_url)
      }
    },
  })

  const requestInvalid =
    !code ||
    authorizationQuery.isError ||
    (authorizationQuery.isSuccess &&
      (!authorizationQuery.data.success || !authorization))
  const approvalFailed =
    approveMutation.isError ||
    (approveMutation.isSuccess && !approveMutation.data.success)

  return (
    <Main>
      <div className='flex min-h-full flex-1 items-center justify-center px-3 py-6 sm:px-6'>
        <Card className='w-full max-w-lg rounded-lg'>
          {authorizationQuery.isLoading ? (
            <CardContent className='flex min-h-52 items-center justify-center'>
              <Spinner className='size-6' />
            </CardContent>
          ) : requestInvalid ? (
            <>
              <CardHeader>
                <CardTitle>{t('Quick login')}</CardTitle>
                <CardDescription>
                  {t('Invalid or expired authorization request')}
                </CardDescription>
              </CardHeader>
              <CardFooter className='justify-end'>
                <Button variant='outline' render={<Link to='/dashboard' />}>
                  {t('Return to dashboard')}
                </Button>
              </CardFooter>
            </>
          ) : (
            <>
              <CardHeader className='border-b'>
                <div className='bg-primary/10 text-primary mb-2 flex size-10 items-center justify-center rounded-lg'>
                  <KeyRound className='size-5' />
                </div>
                <CardTitle>{t('Authorize quick login')}</CardTitle>
                <CardDescription>
                  {t('{{client}} is requesting API access', {
                    client: authorization?.client_name,
                  })}
                </CardDescription>
              </CardHeader>
              <CardContent className='space-y-4'>
                <div className='grid gap-3 text-sm'>
                  <div className='flex min-w-0 items-center gap-3'>
                    <div className='bg-muted flex size-8 shrink-0 items-center justify-center rounded-md'>
                      <Check className='size-4' />
                    </div>
                    <div className='min-w-0'>
                      <div className='text-muted-foreground text-xs'>
                        {t('Signed in account')}
                      </div>
                      <div className='truncate font-medium'>{username}</div>
                    </div>
                  </div>
                  <div className='flex min-w-0 items-center gap-3'>
                    <div className='bg-muted flex size-8 shrink-0 items-center justify-center rounded-md'>
                      <MonitorSmartphone className='size-4' />
                    </div>
                    <div className='min-w-0'>
                      <div className='text-muted-foreground text-xs'>
                        {t('Local callback')}
                      </div>
                      <div className='truncate font-mono text-xs'>
                        {authorization?.redirect_uri}
                      </div>
                    </div>
                  </div>
                </div>
                <p className='text-muted-foreground text-sm leading-6'>
                  {t(
                    'A personal API key in your current group will be created for this application.'
                  )}
                </p>
                {approvalFailed && (
                  <p className='text-destructive text-sm' role='alert'>
                    {t('Authorization failed. Please try again.')}
                  </p>
                )}
              </CardContent>
              <CardFooter className='flex-col-reverse gap-2 sm:flex-row sm:justify-end'>
                <Button
                  className='w-full sm:w-auto'
                  variant='outline'
                  disabled={approveMutation.isPending}
                  onClick={() =>
                    window.location.assign(authorization!.cancel_url)
                  }
                >
                  <X data-icon='inline-start' />
                  {t('Cancel')}
                </Button>
                <Button
                  className='w-full sm:w-auto'
                  disabled={approveMutation.isPending}
                  onClick={() => approveMutation.mutate()}
                >
                  {approveMutation.isPending ? (
                    <Spinner data-icon='inline-start' />
                  ) : (
                    <Check data-icon='inline-start' />
                  )}
                  {approveMutation.isPending
                    ? t('Authorizing...')
                    : t('Authorize')}
                </Button>
              </CardFooter>
            </>
          )}
        </Card>
      </div>
    </Main>
  )
}
