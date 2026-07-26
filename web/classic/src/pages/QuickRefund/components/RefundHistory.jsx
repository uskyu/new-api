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
import {
  Banner,
  Button,
  Empty,
  Modal,
  Pagination,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconEyeOpened, IconRefresh } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { renderQuota, timestamp2string } from '../../../helpers';

const { Text, Title } = Typography;
const ITEM_PAGE_SIZE = 20;

const STATUS_META = {
  pending: ['orange', '等待执行'],
  running: ['blue', '执行中'],
  completed: ['green', '已完成'],
  success: ['green', '成功'],
  skipped: ['grey', '已跳过'],
  failed: ['red', '失败'],
};

const StatusTag = ({ status, t }) => {
  const [color, label] = STATUS_META[status] || ['grey', status || '未知'];
  return <Tag color={color}>{t(label)}</Tag>;
};

const DetailRow = ({ label, children }) => (
  <div className='min-w-0'>
    <Text type='tertiary' size='small'>
      {label}
    </Text>
    <div className='mt-1 break-words'>{children ?? '-'}</div>
  </div>
);

const parseList = (value) => {
  if (!value) return [];
  if (Array.isArray(value)) return value;
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return String(value)
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean);
  }
};

const BatchMobileCard = ({ batch, onOpen, t }) => (
  <div className='rounded-md border border-semi-color-border p-3'>
    <div className='flex items-center justify-between gap-2'>
      <Text strong>
        {t('批次')} #{batch.id}
      </Text>
      <StatusTag status={batch.status} t={t} />
    </div>
    <div className='mt-3 grid grid-cols-2 gap-3 text-sm'>
      <DetailRow label={t('退款比例')}>{batch.ratio}%</DetailRow>
      <DetailRow label={t('退款额度')}>
        {renderQuota(batch.refunded_quota || 0)}
      </DetailRow>
      <DetailRow label={t('成功 / 失败')}>
        {batch.success_items || 0} / {batch.failed_items || 0}
      </DetailRow>
      <DetailRow label={t('创建时间')}>
        {timestamp2string(batch.created_at)}
      </DetailRow>
    </div>
    <Button
      block
      theme='outline'
      icon={<IconEyeOpened />}
      onClick={() => onOpen(batch.id)}
      style={{ marginTop: 12 }}
    >
      {t('查看详情')}
    </Button>
  </div>
);

const ItemMobileCard = ({ item, t }) => (
  <div className='rounded-md border border-semi-color-border p-3'>
    <div className='flex items-start justify-between gap-2'>
      <div className='min-w-0'>
        <Text strong className='break-all'>
          {item.model_name || '-'}
        </Text>
        <div className='mt-1 text-xs text-semi-color-text-2'>
          {t('源日志')} #{item.source_log_id} · {t('用户')} #{item.user_id}
        </div>
      </div>
      <StatusTag status={item.status} t={t} />
    </div>
    <div className='mt-3 grid grid-cols-2 gap-3 text-sm'>
      <DetailRow label={t('原始额度')}>
        {renderQuota(item.source_quota || 0)}
      </DetailRow>
      <DetailRow label={t('退款额度')}>
        {renderQuota(item.refund_quota || 0)}
      </DetailRow>
      <DetailRow label={t('渠道 ID')}>{item.channel_id}</DetailRow>
      <DetailRow label={t('处理说明')}>{item.message || '-'}</DetailRow>
    </div>
  </div>
);

export default function RefundHistory({
  batches,
  pageInfo,
  loading,
  error,
  onRefresh,
  onPageChange,
  detail,
  detailVisible,
  detailLoading,
  detailError,
  onOpenDetail,
  onDetailPageChange,
  onCloseDetail,
}) {
  const { t } = useTranslation();
  const detailItems = detail?.items || [];
  const detailItemPage = detail?.item_page || 1;
  const detailItemPageSize = detail?.item_page_size || ITEM_PAGE_SIZE;
  const detailItemTotal = detail?.item_total || 0;

  const columns = [
    { title: t('批次'), dataIndex: 'id', width: 90 },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 100,
      render: (value) => <StatusTag status={value} t={t} />,
    },
    {
      title: t('比例'),
      dataIndex: 'ratio',
      width: 80,
      render: (value) => `${value}%`,
    },
    {
      title: t('成功 / 跳过 / 失败'),
      render: (_, row) =>
        `${row.success_items || 0} / ${row.skipped_items || 0} / ${row.failed_items || 0}`,
    },
    {
      title: t('已退款额度'),
      dataIndex: 'refunded_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('创建时间'),
      dataIndex: 'created_at',
      render: timestamp2string,
    },
    {
      title: t('操作'),
      width: 100,
      render: (_, row) => (
        <Button
          theme='borderless'
          icon={<IconEyeOpened />}
          onClick={() => onOpenDetail(row.id)}
        >
          {t('详情')}
        </Button>
      ),
    },
  ];

  const itemColumns = [
    { title: t('源日志'), dataIndex: 'source_log_id', width: 90 },
    { title: t('用户'), dataIndex: 'user_id', width: 80 },
    { title: t('渠道'), dataIndex: 'channel_id', width: 80 },
    {
      title: t('模型'),
      dataIndex: 'model_name',
      render: (value) => <span className='break-all'>{value || '-'}</span>,
    },
    {
      title: t('原始额度'),
      dataIndex: 'source_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('退款额度'),
      dataIndex: 'refund_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (value) => <StatusTag status={value} t={t} />,
    },
    { title: t('说明'), dataIndex: 'message', render: (value) => value || '-' },
  ];

  return (
    <>
      <div className='mb-3 flex flex-wrap items-center justify-between gap-2'>
        <div>
          <Title heading={5} style={{ margin: 0 }}>
            {t('退款批次历史')}
          </Title>
          <Text type='tertiary'>{t('查看每次退款的执行结果与明细')}</Text>
        </div>
        <Button
          theme='outline'
          icon={<IconRefresh />}
          loading={loading}
          onClick={onRefresh}
        >
          {t('刷新')}
        </Button>
      </div>

      {error ? (
        <Banner type='danger' closeIcon={null} description={error} />
      ) : null}

      <Spin spinning={loading}>
        {batches.length === 0 && !loading ? (
          <Empty description={t('暂无退款批次')} />
        ) : (
          <>
            <div className='hidden md:block'>
              <Table
                rowKey='id'
                columns={columns}
                dataSource={batches}
                pagination={false}
              />
            </div>
            <div className='grid gap-3 md:hidden'>
              {batches.map((batch) => (
                <BatchMobileCard
                  key={batch.id}
                  batch={batch}
                  onOpen={onOpenDetail}
                  t={t}
                />
              ))}
            </div>
          </>
        )}
      </Spin>

      {pageInfo.total > pageInfo.pageSize ? (
        <div className='mt-4 flex justify-end'>
          <Pagination
            currentPage={pageInfo.page}
            pageSize={pageInfo.pageSize}
            total={pageInfo.total}
            showSizeChanger={false}
            onPageChange={onPageChange}
          />
        </div>
      ) : null}

      <Modal
        title={detail ? `${t('退款批次')} #${detail.id}` : t('退款批次详情')}
        visible={detailVisible}
        onCancel={onCloseDetail}
        footer={<Button onClick={onCloseDetail}>{t('关闭')}</Button>}
        width={960}
        style={{ maxWidth: 'calc(100vw - 24px)' }}
        bodyStyle={{ maxHeight: '76vh', overflowY: 'auto' }}
      >
        <Spin spinning={detailLoading}>
          {detailError ? (
            <Banner type='danger' closeIcon={null} description={detailError} />
          ) : null}
          {detail ? (
            <div className='space-y-4'>
              <div className='grid grid-cols-2 gap-3 rounded-md bg-semi-color-fill-0 p-3 sm:grid-cols-4'>
                <DetailRow label={t('状态')}>
                  <StatusTag status={detail.status} t={t} />
                </DetailRow>
                <DetailRow label={t('退款比例')}>{detail.ratio}%</DetailRow>
                <DetailRow label={t('命中记录')}>
                  {detail.total_items || 0}
                </DetailRow>
                <DetailRow label={t('已退款额度')}>
                  {renderQuota(detail.refunded_quota || 0)}
                </DetailRow>
                <DetailRow label={t('时间范围')}>
                  {timestamp2string(detail.start_time)} -{' '}
                  {timestamp2string(detail.end_time)}
                </DetailRow>
                <DetailRow label={t('渠道 ID')}>
                  {parseList(detail.channel_ids).join(', ') || '-'}
                </DetailRow>
                <DetailRow label={t('模型')}>
                  {parseList(detail.model_names).join(', ') || t('全部模型')}
                </DetailRow>
                <DetailRow label={t('原因')}>{detail.reason || '-'}</DetailRow>
              </div>
              {detail.error ? (
                <Banner
                  type='danger'
                  closeIcon={null}
                  description={detail.error}
                />
              ) : null}
              <div className='hidden md:block'>
                <Table
                  rowKey='id'
                  columns={itemColumns}
                  dataSource={detail.items || []}
                  pagination={false}
                />
              </div>
              <div className='grid gap-3 md:hidden'>
                {detailItems.length ? (
                  detailItems.map((item) => (
                    <ItemMobileCard key={item.id} item={item} t={t} />
                  ))
                ) : (
                  <Empty description={t('暂无退款明细')} />
                )}
              </div>
              {detailItemTotal > detailItemPageSize ? (
                <div className='flex justify-end'>
                  <Pagination
                    currentPage={detailItemPage}
                    pageSize={detailItemPageSize}
                    total={detailItemTotal}
                    showSizeChanger={false}
                    onPageChange={onDetailPageChange}
                  />
                </div>
              ) : null}
            </div>
          ) : null}
        </Spin>
      </Modal>
    </>
  );
}
