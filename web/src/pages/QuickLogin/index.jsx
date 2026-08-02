/*
Copyright (C) 2025 QuantumNous

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

import React, { useContext, useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Button, Card, Spin, Typography } from '@douyinfe/semi-ui';
import { Check, KeyRound, MonitorSmartphone, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import { API } from '../../helpers';

const { Text, Title } = Typography;

const QuickLogin = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [userState] = useContext(UserContext);
  const code = searchParams.get('code')?.trim() || '';
  const [authorization, setAuthorization] = useState(null);
  const [loading, setLoading] = useState(true);
  const [requestInvalid, setRequestInvalid] = useState(false);
  const [approving, setApproving] = useState(false);
  const [approvalFailed, setApprovalFailed] = useState(false);

  useEffect(() => {
    let active = true;

    const loadAuthorization = async () => {
      if (!code) {
        setRequestInvalid(true);
        setLoading(false);
        return;
      }
      try {
        const response = await API.get('/api/quick-login/authorize', {
          params: { code },
          skipErrorHandler: true,
        });
        if (!active) {
          return;
        }
        if (response.data?.success && response.data?.data) {
          setAuthorization(response.data.data);
        } else {
          setRequestInvalid(true);
        }
      } catch {
        if (active) {
          setRequestInvalid(true);
        }
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    };

    loadAuthorization();
    return () => {
      active = false;
    };
  }, [code]);

  const approve = async () => {
    setApproving(true);
    setApprovalFailed(false);
    try {
      const response = await API.post(
        '/api/quick-login/approve',
        { code },
        { skipErrorHandler: true },
      );
      const callbackURL = response.data?.data?.callback_url;
      if (response.data?.success && callbackURL) {
        window.location.assign(callbackURL);
        return;
      }
      setApprovalFailed(true);
    } catch {
      setApprovalFailed(true);
    } finally {
      setApproving(false);
    }
  };

  return (
    <main className='mt-[64px] min-h-[calc(100vh-64px)] flex items-center justify-center px-3 py-8 sm:px-6'>
      <Card className='w-full max-w-lg !rounded-lg'>
        {loading ? (
          <div className='min-h-52 flex items-center justify-center'>
            <Spin size='large' />
          </div>
        ) : requestInvalid ? (
          <div className='p-2 sm:p-4'>
            <Title heading={3}>{t('Quick login')}</Title>
            <Text type='tertiary'>
              {t('Invalid or expired authorization request')}
            </Text>
            <div className='mt-6 flex justify-end'>
              <Button onClick={() => navigate('/console')}>
                {t('Return to dashboard')}
              </Button>
            </div>
          </div>
        ) : (
          <div className='p-2 sm:p-4'>
            <div className='mb-5 border-b border-gray-200 pb-5 dark:border-gray-700'>
              <div className='mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-950 dark:text-blue-300'>
                <KeyRound size={20} aria-hidden='true' />
              </div>
              <Title heading={3}>{t('Authorize quick login')}</Title>
              <Text type='tertiary'>
                {t('{{client}} is requesting API access', {
                  client: authorization?.client_name,
                })}
              </Text>
            </div>

            <div className='space-y-4'>
              <div className='flex min-w-0 items-center gap-3'>
                <div className='flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-gray-100 dark:bg-gray-800'>
                  <Check size={16} aria-hidden='true' />
                </div>
                <div className='min-w-0'>
                  <Text type='tertiary' size='small'>
                    {t('Signed in account')}
                  </Text>
                  <div className='truncate font-medium'>
                    {userState?.user?.username || ''}
                  </div>
                </div>
              </div>

              <div className='flex min-w-0 items-center gap-3'>
                <div className='flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-gray-100 dark:bg-gray-800'>
                  <MonitorSmartphone size={16} aria-hidden='true' />
                </div>
                <div className='min-w-0'>
                  <Text type='tertiary' size='small'>
                    {t('Local callback')}
                  </Text>
                  <div className='break-all font-mono text-xs'>
                    {authorization?.redirect_uri}
                  </div>
                </div>
              </div>

              <Text type='tertiary'>
                {t(
                  'A personal API key in your current group will be created for this application.',
                )}
              </Text>

              {approvalFailed && (
                <Text type='danger' role='alert'>
                  {t('Authorization failed. Please try again.')}
                </Text>
              )}
            </div>

            <div className='mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end'>
              <Button
                className='w-full sm:w-auto'
                icon={<X size={16} aria-hidden='true' />}
                disabled={approving}
                onClick={() => window.location.assign(authorization.cancel_url)}
              >
                {t('Cancel')}
              </Button>
              <Button
                className='w-full sm:w-auto'
                theme='solid'
                type='primary'
                icon={<Check size={16} aria-hidden='true' />}
                loading={approving}
                onClick={approve}
              >
                {approving ? t('Authorizing...') : t('Authorize')}
              </Button>
            </div>
          </div>
        )}
      </Card>
    </main>
  );
};

export default QuickLogin;
