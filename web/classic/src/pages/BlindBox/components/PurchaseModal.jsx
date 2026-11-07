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

import React from 'react';
import { Modal, Radio, RadioGroup, Typography } from '@douyinfe/semi-ui';
import { CreditCard } from 'lucide-react';
import { SiAlipay, SiWechat } from 'react-icons/si';
import { useTranslation } from 'react-i18next';
import { renderQuota } from '../../../helpers';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { formatPrice } from './format';

const { Text } = Typography;

const renderMethodIcon = (method) => {
  if (method.type === 'alipay') {
    return <SiAlipay size={18} color='#1677FF' />;
  }
  if (method.type === 'wxpay') {
    return <SiWechat size={18} color='#07C160' />;
  }
  if (method.icon) {
    return (
      <img
        src={method.icon}
        alt={method.name}
        style={{ width: 18, height: 18, objectFit: 'contain' }}
      />
    );
  }
  return (
    <CreditCard size={18} color={method.color || 'var(--semi-color-text-2)'} />
  );
};

const PurchaseModal = ({
  visible,
  box,
  payMethods,
  payMethod,
  onPayMethodChange,
  onConfirm,
  onCancel,
  confirmLoading,
}) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();

  const handlePayMethodChange = (eventOrValue) => {
    const value = eventOrValue?.target?.value ?? eventOrValue;
    if (typeof value === 'string') {
      onPayMethodChange(value);
    }
  };

  return (
    <Modal
      title={t('购买盲盒')}
      visible={visible}
      onOk={onConfirm}
      onCancel={onCancel}
      okText={t('前往支付')}
      cancelText={t('取消')}
      confirmLoading={confirmLoading}
      maskClosable={false}
      closable={!confirmLoading}
      closeOnEsc={!confirmLoading}
      centered
      size={isMobile ? 'full-width' : 'small'}
      okButtonProps={{ disabled: !payMethod }}
      bodyStyle={{ paddingTop: 8 }}
    >
      {box ? (
        <div className='space-y-4'>
          <div
            className='rounded-xl p-4'
            style={{ background: 'var(--semi-color-fill-0)' }}
          >
            <div className='flex items-center justify-between gap-3'>
              <div className='min-w-0'>
                <div
                  className='truncate font-semibold'
                  style={{ color: 'var(--semi-color-text-0)' }}
                >
                  {box.name || '-'}
                </div>
                <div
                  className='mt-1 text-xs'
                  style={{ color: 'var(--semi-color-text-2)' }}
                >
                  {t('售价')} {formatPrice(box.price)}
                </div>
              </div>
              <div className='shrink-0 text-right'>
                <div
                  className='text-xs'
                  style={{ color: 'var(--semi-color-text-2)' }}
                >
                  {t('最高可得')}
                </div>
                <div
                  className='text-lg font-bold'
                  style={{ color: 'var(--semi-color-warning)' }}
                >
                  {renderQuota(box.max_reward ?? box.base_quota ?? 0)}
                </div>
              </div>
            </div>
          </div>

          <div>
            <div
              className='mb-2 text-sm font-semibold'
              style={{ color: 'var(--semi-color-text-0)' }}
            >
              {t('支付方式')}
            </div>
            <RadioGroup
              type='card'
              direction='vertical'
              value={payMethod}
              onChange={handlePayMethodChange}
              style={{ width: '100%' }}
            >
              {payMethods.map((method) => (
                <Radio
                  key={method.type}
                  value={method.type}
                  style={{ width: '100%', marginBottom: 8 }}
                >
                  <span className='flex items-center gap-2'>
                    {renderMethodIcon(method)}
                    <span>{method.name}</span>
                  </span>
                </Radio>
              ))}
            </RadioGroup>
          </div>

          <Text type='tertiary' size='small'>
            {t('支付成功后系统将自动开奖，奖励余额将直接发放到账户。')}
          </Text>
        </div>
      ) : null}
    </Modal>
  );
};

export default PurchaseModal;
