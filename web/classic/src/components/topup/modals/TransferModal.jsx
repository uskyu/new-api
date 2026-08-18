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
import { Modal, Typography, Input, InputNumber } from '@douyinfe/semi-ui';
import { CreditCard } from 'lucide-react';
import { quotaToDisplayAmount } from '../../../helpers/quota';
import { getCurrencyConfig } from '../../../helpers/render';

function getDisplayPrecision(isTokens) {
  if (isTokens) return 0;
  const unitDisplay = quotaToDisplayAmount(1);
  if (!Number.isFinite(unitDisplay) || unitDisplay <= 0) return 2;
  return Math.min(100, Math.max(2, Math.ceil(-Math.log10(unitDisplay))));
}

const TransferModal = ({
  t,
  openTransfer,
  transfer,
  handleTransferCancel,
  userState,
  renderQuota,
  transferAmount,
  setTransferAmount,
}) => {
  const affQuota = userState?.user?.aff_quota || 0;
  const affQuotaDisplay = quotaToDisplayAmount(affQuota);
  const isTokens = getCurrencyConfig().type === 'TOKENS';

  const precision = getDisplayPrecision(isTokens);
  const minDisplayAmount = isTokens ? 1 : quotaToDisplayAmount(1);

  return (
    <Modal
      title={
        <div className='flex items-center'>
          <CreditCard className='mr-2' size={18} />
          {t('划转邀请额度')}
        </div>
      }
      visible={openTransfer}
      onOk={transfer}
      onCancel={handleTransferCancel}
      maskClosable={false}
      centered
    >
      <div className='space-y-4'>
        <div>
          <Typography.Text strong className='block mb-2'>
            {t('可用邀请额度')}
          </Typography.Text>
          <Input
            value={renderQuota(affQuota, precision)}
            disabled
            className='!rounded-lg'
          />
        </div>
        <div>
          <Typography.Text strong className='block mb-2'>
            {t('划转额度')}
          </Typography.Text>
          <InputNumber
            min={minDisplayAmount}
            max={affQuotaDisplay}
            precision={precision}
            value={transferAmount}
            onChange={(value) => setTransferAmount(value)}
            className='w-full !rounded-lg'
          />
        </div>
      </div>
    </Modal>
  );
};

export default TransferModal;
