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
import { Modal, Typography, Input, InputNumber, Popover } from '@douyinfe/semi-ui';
import { CreditCard, HelpCircle } from 'lucide-react';

// 后端未下发比例时按默认设置（100%）兜底，避免白屏或误导。
const normalizeRatio = (value, fallback = 100) => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return fallback;
  }
  return Math.min(100, Math.max(0, parsed));
};

const TransferModal = ({
  t,
  openTransfer,
  transfer,
  handleTransferCancel,
  userState,
  renderQuota,
  getQuotaPerUnit,
  transferAmount,
  setTransferAmount,
}) => {
  const [ruleVisible, setRuleVisible] = useState(false);

  // 字段由 GET /api/user/self 新增下发，旧数据缺失时回退到 aff_quota / 0。
  const withdrawableQuota =
    Number(userState?.user?.aff_withdrawable_quota ?? userState?.user?.aff_quota ?? 0) ||
    0;
  const lockedQuota = Number(userState?.user?.aff_locked_quota ?? 0) || 0;
  const withdrawRule = userState?.user?.aff_withdraw_rule;
  const rule =
    withdrawRule && typeof withdrawRule === 'object' ? withdrawRule : {};
  const requireInviteeRecharge =
    rule?.inviter_withdraw_require_invitee_recharge === true;
  const rechargedRatio = normalizeRatio(rule?.recharged_withdraw_ratio, 100);
  const unrechargedRatio = normalizeRatio(rule?.unrecharged_withdraw_ratio, 100);

  // 点击/悬浮展开；点击页面其它区域或关闭弹窗时收起。
  useEffect(() => {
    if (!ruleVisible) {
      return undefined;
    }
    const handleDocumentClick = () => setRuleVisible(false);
    document.addEventListener('click', handleDocumentClick);
    return () => document.removeEventListener('click', handleDocumentClick);
  }, [ruleVisible]);

  useEffect(() => {
    if (!openTransfer) {
      setRuleVisible(false);
    }
  }, [openTransfer]);

  const ruleContent = (
    <div className='max-w-[280px] text-xs leading-5'>
      {requireInviteeRecharge ? (
        <>
          <div>
            {t('被邀请人已充值：可划转 {{ratio}}%', {
              ratio: rechargedRatio,
            })}
          </div>
          <div>
            {t('被邀请人未充值：可划转 {{ratio}}%', {
              ratio: unrechargedRatio,
            })}
          </div>
          <div className='mt-1'>
            {t(
              '邀请好友充值后，剩余额度将自动解锁',
            )}
          </div>
        </>
      ) : (
        <div>
          {t('邀请额度当前全额可划转（{{ratio}}%）', {
            ratio: rechargedRatio,
          })}
        </div>
      )}
    </div>
  );

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
          <div className='mb-2 flex items-center gap-1'>
            <Typography.Text strong>{t('当前可提现')}</Typography.Text>
            <Popover
              trigger='custom'
              visible={ruleVisible}
              content={ruleContent}
              position='top'
              onVisibleChange={setRuleVisible}
            >
              <span
                className='inline-flex cursor-help items-center text-[var(--semi-color-text-2)]'
                role='button'
                tabIndex={0}
                onClick={(e) => {
                  e.stopPropagation();
                  setRuleVisible(true);
                }}
                onMouseEnter={() => setRuleVisible(true)}
                onMouseLeave={() => setRuleVisible(false)}
              >
                <HelpCircle size={14} />
              </span>
            </Popover>
          </div>
          <Input
            value={renderQuota(withdrawableQuota)}
            disabled
            className='!rounded-lg'
          />
          {lockedQuota > 0 && (
            <div className='mt-1 flex items-center justify-between'>
              <Typography.Text type='tertiary' size='small'>
                {t('待解锁')}
              </Typography.Text>
              <Typography.Text type='tertiary' size='small'>
                {renderQuota(lockedQuota)}
              </Typography.Text>
            </div>
          )}
        </div>
        <div>
          <Typography.Text strong className='block mb-2'>
            {t('划转额度')} · {t('最低') + renderQuota(getQuotaPerUnit())}
          </Typography.Text>
          <InputNumber
            min={getQuotaPerUnit()}
            max={withdrawableQuota}
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
