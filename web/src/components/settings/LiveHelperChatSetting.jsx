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

import React, { useEffect, useState } from 'react';
import {
  Banner,
  Button,
  Form,
  Input,
  Spin,
  Switch,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../helpers';
import { useTranslation } from 'react-i18next';

const OPTION_KEYS = {
  enabled: 'LiveHelperChatEnabled',
  baseUrl: 'LiveHelperChatBaseUrl',
};

const LiveHelperChatSetting = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [saving, setSaving] = useState(false);
  const [inputs, setInputs] = useState({
    enabled: false,
    baseUrl: '',
    originalBaseUrl: '',
  });

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    API.get('/api/option/')
      .then((res) => {
        if (cancelled) return;
        const options = res?.data?.data || [];
        const enabled = options.find(
          (option) => option.key === OPTION_KEYS.enabled,
        );
        const baseUrl = options.find(
          (option) => option.key === OPTION_KEYS.baseUrl,
        );
        setInputs({
          enabled: enabled?.value === 'true',
          baseUrl: baseUrl?.value || '',
          originalBaseUrl: baseUrl?.value || '',
        });
        setLoadFailed(false);
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const updateOption = async (key, value) => {
    const res = await API.put('/api/option/', { key, value });
    if (!res?.data?.success) {
      throw new Error(res?.data?.message || t('保存失败，请重试'));
    }
  };

  const save = async () => {
    const baseUrl = inputs.baseUrl.trim();
    if (inputs.enabled && !baseUrl) {
      showError(t('请先填写 LiveHelperChat 地址'));
      return;
    }

    setSaving(true);
    try {
      if (!inputs.enabled) {
        // Disable first so the address may be cleared safely.
        await updateOption(OPTION_KEYS.enabled, 'false');
        if (baseUrl !== inputs.originalBaseUrl) {
          await updateOption(OPTION_KEYS.baseUrl, baseUrl);
        }
      } else {
        // Save the address before enabling; the server validates the current
        // stored address when it receives the enable flag.
        await updateOption(OPTION_KEYS.baseUrl, baseUrl);
        await updateOption(OPTION_KEYS.enabled, 'true');
      }
      setInputs((current) => ({
        ...current,
        baseUrl,
        originalBaseUrl: baseUrl,
      }));
      showSuccess(t('在线客服配置已保存，刷新页面后生效'));
    } catch (error) {
      showError(error?.message || t('保存失败，请重试'));
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className='flex justify-center py-10'>
        <Spin />
      </div>
    );
  }

  return (
    <div className='px-2 py-4'>
      <h3 className='text-lg font-semibold mb-4'>{t('在线客服设置')}</h3>
      {loadFailed && (
        <Banner
          type='danger'
          description={t('加载在线客服设置失败')}
          className='mb-4'
        />
      )}
      <Form labelPosition='top'>
        <div className='flex items-center gap-3 mb-4'>
          <Switch
            checked={inputs.enabled}
            onChange={(enabled) =>
              setInputs((current) => ({ ...current, enabled }))
            }
          />
          <span>{t('启用在线客服')}</span>
        </div>

        <div className='mb-2'>{t('LiveHelperChat 地址')}</div>
        <Input
          value={inputs.baseUrl}
          onChange={(baseUrl) =>
            setInputs((current) => ({ ...current, baseUrl }))
          }
          placeholder='https://chat.example.com'
        />
        <Typography.Text
          type='tertiary'
          size='small'
          style={{ display: 'block', marginTop: '8px' }}
        >
          {t(
            '填写 LiveHelperChat 站点根地址，例如 https://chat.example.com；生产环境必须使用 HTTPS。',
          )}
        </Typography.Text>

        <Button
          theme='solid'
          type='primary'
          loading={saving}
          onClick={save}
          style={{ marginTop: '16px' }}
        >
          {t('保存')}
        </Button>
      </Form>
    </div>
  );
};

export default LiveHelperChatSetting;
