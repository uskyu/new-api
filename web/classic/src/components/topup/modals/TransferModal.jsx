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
import { Modal, Typography, Input, InputNumber, Popover, Button } from '@douyinfe/semi-ui';
import { CreditCard, HelpCircle } from 'lucide-react';
import {
  displayAmountToQuota,
  quotaToDisplayAmount,
} from '../../../helpers/quota';
import { getCurrencyConfig } from '../../../helpers/render';

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
  // 货币配置快照：getCurrencyConfig 依赖 localStorage，弹窗打开或跨标签页切换
  // 币种时刷新该 state 触发重渲染，使 min/max/value 按新币种重新换算。
  const [currencyConfig, setCurrencyConfig] = useState(() =>
    getCurrencyConfig(),
  );

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

  // 显示金额：复用 helpers/quota.js 与当前货币配置换算，不再展示原始额度整数。
  const isTokens = currencyConfig?.type === 'TOKENS';
  const displayPrecision = isTokens ? 0 : 2;

  // 统一按输入精度取整，避免 min/max 为长小数时被 InputNumber 判定越界。
  const toDisplayAmount = (quota) => {
    const amount = Number(quotaToDisplayAmount(quota)) || 0;
    if (isTokens) {
      return Math.round(amount);
    }
    const factor = 10 ** displayPrecision;
    return Math.round(amount * factor) / factor;
  };

  const displayValue = toDisplayAmount(transferAmount);
  const displayMax = toDisplayAmount(withdrawableQuota);
  // 最低划转门槛：0.1 元（QuotaPerUnit/10）。
  const displayMinThreshold = toDisplayAmount(getQuotaPerUnit() / 10);
  // 余额低于最低门槛时允许一次性全提（后端全提特例），输入框下限收敛到可划转额度。
  const displayMin =
    displayMax > 0 && displayMax < displayMinThreshold
      ? displayMax
      : displayMinThreshold;
  const canFillAll = withdrawableQuota > 0 && displayMax > 0;
  // TOKENS 模式展示精确额度，货币模式沿用 renderQuota 的货币格式。
  const minHintAmount = isTokens
    ? String(displayMinThreshold)
    : renderQuota(getQuotaPerUnit() / 10);

  // 输入的是显示金额，写回父组件的仍是原始额度；受可划转额度上限约束。
  const handleAmountChange = (value) => {
    const nextQuota = displayAmountToQuota(value);
    const boundedQuota = Math.min(
      Math.max(Number.isFinite(nextQuota) ? nextQuota : 0, 0),
      withdrawableQuota,
    );
    setTransferAmount(boundedQuota);
  };

  const handleFillAll = () => {
    if (withdrawableQuota > 0) {
      setTransferAmount(withdrawableQuota);
    }
  };

  // 点击/悬浮展开；点击页面其它区域或关闭弹窗时收起。
  useEffect(() => {
    if (!ruleVisible) {
      return undefined;
    }
    const handleDocumentClick = () => setRuleVisible(false);
    document.addEventListener('click', handleDocumentClick);
    return () => document.removeEventListener('click', handleDocumentClick);
  }, [ruleVisible]);

  // 打开弹窗时重新读取货币配置，避免沿用旧币种的 min/max/value。
  useEffect(() => {
    setCurrencyConfig(getCurrencyConfig());
    if (!openTransfer) {
      setRuleVisible(false);
      return;
    }
    // 余额低于最低门槛时，默认输入额度可能超过可划转额度；打开时收敛到可全提额度。
    if (withdrawableQuota > 0 && transferAmount > withdrawableQuota) {
      setTransferAmount(withdrawableQuota);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [openTransfer]);

  // 跨标签页切换显示币种/汇率时同步刷新。
  useEffect(() => {
    const handleStorage = (event) => {
      if (
        event.key &&
        !['quota_display_type', 'quota_per_unit', 'status'].includes(event.key)
      ) {
        return;
      }
      setCurrencyConfig(getCurrencyConfig());
    };
    window.addEventListener('storage', handleStorage);
    return () => window.removeEventListener('storage', handleStorage);
  }, []);

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
              position='rightTop'
              showArrow
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
            {t('划转额度')}
          </Typography.Text>
          <div className='flex items-start gap-2'>
            <div className='min-w-0 flex-1'>
              <InputNumber
                min={displayMin}
                max={displayMax}
                value={displayValue}
                precision={displayPrecision}
                step={isTokens ? 1 : 0.01}
                prefix={isTokens ? undefined : currencyConfig?.symbol}
                onChange={handleAmountChange}
                className='!rounded-lg'
                style={{ width: '100%' }}
              />
              <div className='mt-1 text-xs text-[var(--semi-color-text-2)]'>
                {isTokens
                  ? t('按额度输入，最低可划转 {{amount}}', {
                      amount: minHintAmount,
                    })
                  : t('按当前显示币种输入，最低可划转 {{amount}}', {
                      amount: minHintAmount,
                    })}
              </div>
            </div>
            <Button
              type='primary'
              theme='light'
              disabled={!canFillAll}
              onClick={handleFillAll}
            >
              {t('全部划转')}
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  );
};

export default TransferModal;
