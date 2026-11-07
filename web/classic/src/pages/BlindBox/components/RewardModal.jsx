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
import { Button, Modal, Typography } from '@douyinfe/semi-ui';
import { Gift } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { renderQuota } from '../../../helpers';
import { formatMultiplier } from './format';

const { Text } = Typography;

const RewardModal = ({ visible, reward, onClose }) => {
  const { t } = useTranslation();

  if (!reward) {
    return null;
  }

  return (
    <Modal
      title={t('开奖结果')}
      visible={visible}
      onCancel={onClose}
      footer={null}
      centered
      maskClosable={false}
      size='small'
    >
      <div className='py-2 text-center'>
        <div
          className='mx-auto mb-3 flex items-center justify-center rounded-full'
          style={{
            width: 56,
            height: 56,
            background: 'var(--semi-color-success-light-default)',
            color: 'var(--semi-color-success)',
          }}
        >
          <Gift size={28} />
        </div>

        <div
          className='text-2xl font-bold'
          style={{ color: 'var(--semi-color-success)' }}
        >
          {t('抽中 {{multiplier}} 倍', {
            multiplier: formatMultiplier(reward.multiplier),
          })}
        </div>

        <div
          className='mt-4 rounded-xl px-4 py-3'
          style={{ background: 'var(--semi-color-fill-0)' }}
        >
          <div
            className='text-xs'
            style={{ color: 'var(--semi-color-text-2)' }}
          >
            {t('获得余额')}
          </div>
          <div className='mt-1 text-3xl font-bold'>
            {renderQuota(reward.rewardQuota)}
          </div>
        </div>

        {reward.boxName ? (
          <Text type='tertiary' size='small'>
            {reward.boxName}
          </Text>
        ) : null}

        <Button
          type='primary'
          theme='solid'
          className='!rounded-lg'
          style={{ width: '100%', marginTop: 20 }}
          onClick={onClose}
        >
          {t('知道了')}
        </Button>
      </div>
    </Modal>
  );
};

export default RewardModal;
