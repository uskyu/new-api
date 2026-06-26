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
import { useTranslation } from 'react-i18next';
import {
  Input,
  InputNumber,
  Modal,
  Space,
  Typography,
} from '@douyinfe/semi-ui';
import {
  API,
  getCurrencyConfig,
  renderQuota,
  showError,
  showSuccess,
} from '../../../../helpers';
import {
  displayAmountToQuota,
  quotaToDisplayAmount,
} from '../../../../helpers/quota';

const { Text } = Typography;

const text = {
  emptyQuota: '\u8bf7\u8f93\u5165\u8981\u51cf\u5c11\u7684\u989d\u5ea6',
  quotaTooLarge:
    '\u51cf\u5c11\u989d\u5ea6\u4e0d\u80fd\u8d85\u8fc7\u5f53\u524d\u5269\u4f59\u989d\u5ea6',
  success: '\u989d\u5ea6\u5df2\u51cf\u5c11',
  failed: '\u64cd\u4f5c\u5931\u8d25\uff0c\u8bf7\u91cd\u8bd5',
  title: '\u51cf\u5c11\u7528\u6237\u4f59\u989d',
  ok: '\u786e\u8ba4\u51cf\u5c11',
  cancel: '\u53d6\u6d88',
  user: '\u7528\u6237',
  currentQuota: '\u5f53\u524d\u989d\u5ea6',
  amountPlaceholder: '\u8f93\u5165\u51cf\u5c11\u91d1\u989d',
  quotaPlaceholder: '\u8f93\u5165\u51cf\u5c11\u989d\u5ea6',
  afterQuota: '\u51cf\u5c11\u540e\u989d\u5ea6',
  reasonPlaceholder: '\u5907\u6ce8\u539f\u56e0',
};

const DecreaseQuotaModal = ({ visible, user, onCancel, refresh }) => {
  const { t } = useTranslation();
  const [amount, setAmount] = useState('');
  const [quota, setQuota] = useState('');
  const [reason, setReason] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (visible) {
      setAmount('');
      setQuota('');
      setReason('');
    }
  }, [visible]);

  const quotaValue = Number(quota || 0);
  const currentQuota = Number(user?.quota || 0);
  const afterQuota = Math.max(currentQuota - quotaValue, 0);

  const submit = async () => {
    if (!user?.id) return;
    if (!quotaValue || quotaValue <= 0) {
      showError(t(text.emptyQuota));
      return;
    }
    if (quotaValue > currentQuota) {
      showError(t(text.quotaTooLarge));
      return;
    }
    setLoading(true);
    try {
      const res = await API.post(`/api/support/users/${user.id}/quota/decrease`, {
        quota: quotaValue,
        reason,
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t(text.success));
        await refresh?.();
        onCancel?.();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message || t(text.failed));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title={t(text.title)}
      visible={visible}
      onCancel={onCancel}
      onOk={submit}
      confirmLoading={loading}
      okText={t(text.ok)}
      cancelText={t(text.cancel)}
      closable={null}
    >
      <Space vertical align='start' spacing='medium' style={{ width: '100%' }}>
        <Text>
          {t(text.user)}: {user?.username || user?.id}
        </Text>
        <Text>
          {t(text.currentQuota)}: {renderQuota(currentQuota)}
        </Text>
        <InputNumber
          prefix={getCurrencyConfig().symbol}
          placeholder={t(text.amountPlaceholder)}
          value={amount}
          precision={2}
          onChange={(value) => {
            setAmount(value);
            setQuota(value ? displayAmountToQuota(Math.abs(value)) : '');
          }}
          style={{ width: '100%' }}
          showClear
        />
        <InputNumber
          placeholder={t(text.quotaPlaceholder)}
          value={quota}
          onChange={(value) => {
            setQuota(value);
            setAmount(
              value
                ? Number(quotaToDisplayAmount(Math.abs(value)).toFixed(2))
                : '',
            );
          }}
          style={{ width: '100%' }}
          showClear
        />
        <Text>
          {t(text.afterQuota)}: {renderQuota(afterQuota)}
        </Text>
        <Input
          placeholder={t(text.reasonPlaceholder)}
          value={reason}
          onChange={setReason}
          showClear
        />
      </Space>
    </Modal>
  );
};

export default DecreaseQuotaModal;
