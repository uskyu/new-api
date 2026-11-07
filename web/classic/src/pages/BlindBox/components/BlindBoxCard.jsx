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
import { Button, Card, Tag, Typography } from '@douyinfe/semi-ui';
import { Package, Users } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { renderQuota } from '../../../helpers';
import { formatPrice } from './format';

const { Text } = Typography;

const toNumber = (value, fallback = 0) => {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
};

const BlindBoxCard = ({ box, activityEnabled, onBuy }) => {
  const { t } = useTranslation();

  const price = toNumber(box?.price);
  const stockTotal = toNumber(box?.stock_total);
  const soldCount = toNumber(box?.sold_count);
  const remainingValue = Number(box?.remaining);
  const remaining = Number.isFinite(remainingValue)
    ? remainingValue
    : stockTotal > 0
      ? Math.max(stockTotal - soldCount, 0)
      : -1;

  const perUserLimit = toNumber(box?.per_user_limit);
  const userBought = toNumber(box?.user_bought);
  const soldOut = remaining === 0;
  const limitReached = perUserLimit > 0 && userBought >= perUserLimit;

  const minRewardValue = Number(box?.min_reward);
  const maxRewardValue = Number(box?.max_reward);
  const baseReward = toNumber(box?.base_quota);
  const minReward =
    Number.isFinite(minRewardValue) && minRewardValue > 0
      ? minRewardValue
      : baseReward;
  const maxReward =
    Number.isFinite(maxRewardValue) && maxRewardValue > 0
      ? maxRewardValue
      : baseReward;
  const rewardMin = Math.min(minReward, maxReward);
  const rewardMax = Math.max(minReward, maxReward);

  const stockText =
    remaining < 0
      ? t('不限量')
      : soldOut
        ? t('已售罄')
        : t('剩余 {{remaining}} 个', { remaining });
  const limitText =
    perUserLimit > 0
      ? t('每人限购 {{limit}} 个（已购 {{bought}}）', {
          limit: perUserLimit,
          bought: userBought,
        })
      : t('不限购');

  const disabledReason = !activityEnabled
    ? t('盲盒活动未开启')
    : soldOut
      ? t('已售罄')
      : limitReached
        ? t('已达限购上限')
        : '';

  return (
    <Card
      className='!rounded-2xl shadow-sm border-0'
      style={{ height: '100%' }}
      bodyStyle={{ height: '100%', padding: 20 }}
    >
      <div className='flex h-full flex-col'>
        <div className='flex items-start justify-between gap-3'>
          <div className='min-w-0'>
            <div
              className='truncate text-base font-semibold'
              style={{ color: 'var(--semi-color-text-0)' }}
              title={box?.name}
            >
              {box?.name || '-'}
            </div>
            <div className='mt-1 flex items-baseline gap-1'>
              <Text type='tertiary' size='small'>
                {t('售价')}
              </Text>
              <Text strong style={{ color: 'var(--semi-color-danger)' }}>
                {formatPrice(price)}
              </Text>
            </div>
          </div>
          <div className='shrink-0'>
            {soldOut ? (
              <Tag color='danger' shape='circle' size='small'>
                {t('已售罄')}
              </Tag>
            ) : limitReached ? (
              <Tag color='orange' shape='circle' size='small'>
                {t('已达限购')}
              </Tag>
            ) : null}
          </div>
        </div>

        <div
          className='my-4 rounded-xl px-4 py-5 text-center'
          style={{
            background:
              'linear-gradient(135deg, var(--semi-color-warning-light-default) 0%, var(--semi-color-primary-light-default) 100%)',
            border: '1px solid var(--semi-color-warning-light-active)',
          }}
        >
          <Text type='tertiary' size='small'>
            {t('最高可得')}
          </Text>
          <div
            className='mt-1 text-3xl font-bold sm:text-4xl'
            style={{ color: 'var(--semi-color-warning)' }}
          >
            {renderQuota(rewardMax)}
          </div>
          <div
            className='mt-2 text-xs'
            style={{ color: 'var(--semi-color-text-2)' }}
          >
            {t('可得余额')} {renderQuota(rewardMin)} ~ {renderQuota(rewardMax)}
          </div>
        </div>

        <div className='space-y-2'>
          <div className='flex items-center justify-between gap-3 text-sm'>
            <span
              className='flex items-center gap-1.5'
              style={{ color: 'var(--semi-color-text-2)' }}
            >
              <Package size={14} />
              {t('库存')}
            </span>
            <span
              className='min-w-0 truncate text-right'
              style={{
                color: soldOut
                  ? 'var(--semi-color-danger)'
                  : 'var(--semi-color-text-0)',
              }}
              title={stockText}
            >
              {stockText}
            </span>
          </div>
          <div className='flex items-center justify-between gap-3 text-sm'>
            <span
              className='flex items-center gap-1.5'
              style={{ color: 'var(--semi-color-text-2)' }}
            >
              <Users size={14} />
              {t('限购')}
            </span>
            <span
              className='min-w-0 truncate text-right'
              style={{
                color: limitReached
                  ? 'var(--semi-color-danger)'
                  : 'var(--semi-color-text-0)',
              }}
              title={limitText}
            >
              {limitText}
            </span>
          </div>
        </div>

        <div className='mt-auto pt-5'>
          <Button
            type='primary'
            theme='solid'
            className='!rounded-lg'
            style={{ width: '100%' }}
            disabled={Boolean(disabledReason)}
            onClick={() => onBuy(box)}
          >
            {disabledReason || t('立即购买')}
          </Button>
        </div>
      </div>
    </Card>
  );
};

export default BlindBoxCard;
