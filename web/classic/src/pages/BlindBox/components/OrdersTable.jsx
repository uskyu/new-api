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

import React, { useMemo } from 'react';
import {
  Badge,
  Empty,
  Pagination,
  Spin,
  Table,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';
import { renderQuota, timestamp2string } from '../../../helpers';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { formatMultiplier, formatPrice } from './format';

const { Text } = Typography;

const STATUS_CONFIG = {
  success: { type: 'success', label: '成功' },
  pending: { type: 'warning', label: '待支付' },
  failed: { type: 'danger', label: '失败' },
  expired: { type: 'tertiary', label: '已过期' },
};

const OrdersTable = ({
  orders,
  total,
  loading,
  page,
  pageSize,
  onPageChange,
  onPageSizeChange,
  emptyText,
}) => {
  const { t } = useTranslation();

  const columns = useMemo(
    () => [
      {
        title: t('时间'),
        dataIndex: 'create_time',
        key: 'create_time',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('盲盒'),
        dataIndex: 'box_name',
        key: 'box_name',
        render: (value) => <Text strong>{value || '-'}</Text>,
      },
      {
        title: t('售价'),
        dataIndex: 'price',
        key: 'price',
        width: 110,
        render: (value) => formatPrice(value),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        key: 'status',
        width: 110,
        render: (status) => {
          const config = STATUS_CONFIG[status] || {
            type: 'primary',
            label: status || '-',
          };
          return (
            <span className='flex items-center gap-2'>
              <Badge dot type={config.type} />
              <span>{t(config.label)}</span>
            </span>
          );
        },
      },
      {
        title: t('抽中倍率'),
        dataIndex: 'drawn_multiplier',
        key: 'drawn_multiplier',
        width: 110,
        render: (value, record) =>
          record.status === 'success' ? `${formatMultiplier(value)}×` : '-',
      },
      {
        title: t('获得余额'),
        dataIndex: 'reward_quota',
        key: 'reward_quota',
        width: 130,
        render: (value, record) =>
          record.status === 'success' ? (
            <Text strong style={{ color: 'var(--semi-color-success)' }}>
              {renderQuota(value || 0)}
            </Text>
          ) : (
            '-'
          ),
      },
    ],
    [t],
  );

  const isMobile = useIsMobile();

  const emptyState = (
    <Empty
      image={<IllustrationNoResult style={{ width: 120, height: 120 }} />}
      darkModeImage={
        <IllustrationNoResultDark style={{ width: 120, height: 120 }} />
      }
      description={emptyText || t('暂无盲盒记录')}
      style={{ padding: 24 }}
    />
  );

  if (isMobile) {
    return (
      <div>
        {loading ? (
          <div className='flex justify-center py-8'>
            <Spin />
          </div>
        ) : orders.length === 0 ? (
          emptyState
        ) : (
          <div className='space-y-3'>
            {orders.map((order) => {
              const config = STATUS_CONFIG[order.status] || {
                type: 'primary',
                label: order.status || '-',
              };
              return (
                <div
                  key={order.id}
                  className='rounded-xl p-4'
                  style={{
                    border: '1px solid var(--semi-color-border)',
                    background: 'var(--semi-color-bg-2)',
                  }}
                >
                  <div className='flex items-center justify-between gap-2'>
                    <Text strong ellipsis>
                      {order.box_name || '-'}
                    </Text>
                    <span className='flex shrink-0 items-center gap-1.5 text-xs'>
                      <Badge dot type={config.type} />
                      {t(config.label)}
                    </span>
                  </div>
                  <div className='mt-3 space-y-1.5 text-sm'>
                    <div className='flex items-center justify-between gap-3'>
                      <span style={{ color: 'var(--semi-color-text-2)' }}>
                        {t('时间')}
                      </span>
                      <span>
                        {order.create_time
                          ? timestamp2string(order.create_time)
                          : '-'}
                      </span>
                    </div>
                    <div className='flex items-center justify-between gap-3'>
                      <span style={{ color: 'var(--semi-color-text-2)' }}>
                        {t('售价')}
                      </span>
                      <span>{formatPrice(order.price)}</span>
                    </div>
                    <div className='flex items-center justify-between gap-3'>
                      <span style={{ color: 'var(--semi-color-text-2)' }}>
                        {t('抽中倍率')}
                      </span>
                      <span>
                        {order.status === 'success'
                          ? `${formatMultiplier(order.drawn_multiplier)}×`
                          : '-'}
                      </span>
                    </div>
                    <div className='flex items-center justify-between gap-3'>
                      <span style={{ color: 'var(--semi-color-text-2)' }}>
                        {t('获得余额')}
                      </span>
                      {order.status === 'success' ? (
                        <Text
                          strong
                          style={{ color: 'var(--semi-color-success)' }}
                        >
                          {renderQuota(order.reward_quota || 0)}
                        </Text>
                      ) : (
                        <span>-</span>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
        {total > 0 ? (
          <div className='mt-4 flex justify-end'>
            <Pagination
              total={total}
              pageSize={pageSize}
              currentPage={page}
              onPageChange={onPageChange}
              showSizeChanger
              pageSizeOpts={[10, 20, 50, 100]}
              onPageSizeChange={onPageSizeChange}
            />
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <Table
      columns={columns}
      dataSource={orders}
      loading={loading}
      rowKey='id'
      size='small'
      scroll={{ x: 'max-content' }}
      pagination={{
        currentPage: page,
        pageSize,
        total,
        showSizeChanger: true,
        pageSizeOpts: [10, 20, 50, 100],
        onPageChange,
        onPageSizeChange,
      }}
      empty={emptyState}
    />
  );
};

export default OrdersTable;
