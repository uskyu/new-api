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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Banner,
  Button,
  Card,
  DatePicker,
  Empty,
  Input,
  InputNumber,
  Modal,
  Select,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconRefresh, IconSearch } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { isRoot, renderQuota, showError, showSuccess } from '../../helpers';
import {
  createRefundBatch,
  getRefundBatch,
  getRefundBatches,
  getRefundOptions,
  previewRefund,
} from './api';
import RefundHistory from './components/RefundHistory';

const { Paragraph, Text, Title } = Typography;
const HISTORY_PAGE_SIZE = 20;
const QUICK_RATIOS = [25, 50, 75, 100];

const initialRange = () => {
  const end = new Date();
  return [new Date(end.getTime() - 60 * 60 * 1000), end];
};

const toTimestamp = (value) => {
  const time =
    value instanceof Date ? value.getTime() : new Date(value).getTime();
  return Number.isFinite(time) ? Math.floor(time / 1000) : 0;
};

const makeIdempotencyKey = () => {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
  return `refund-${Date.now()}-${Math.random().toString(36).slice(2)}`;
};

const payloadFingerprint = (payload) => JSON.stringify(payload);

const StatBlock = ({ label, value, tone = 'default' }) => (
  <div
    className='min-w-0 rounded-md border p-3'
    style={{
      borderColor:
        tone === 'danger'
          ? 'var(--semi-color-danger-light-active)'
          : 'var(--semi-color-border)',
      background:
        tone === 'danger'
          ? 'var(--semi-color-danger-light-default)'
          : 'var(--semi-color-fill-0)',
    }}
  >
    <Text type='tertiary' size='small'>
      {label}
    </Text>
    <div className='mt-1 truncate text-xl font-semibold'>{value}</div>
  </div>
);

const Breakdown = ({ title, data, labels = {} }) => {
  const { t } = useTranslation();
  const entries = Object.entries(data || {}).sort(
    (a, b) => (b[1]?.refund_quota || 0) - (a[1]?.refund_quota || 0),
  );

  if (!entries.length) return null;

  return (
    <div className='min-w-0'>
      <Text strong>{title}</Text>
      <div className='mt-2 divide-y divide-semi-color-border rounded-md border border-semi-color-border'>
        {entries.map(([key, value]) => (
          <div
            key={key}
            className='flex min-w-0 items-center justify-between gap-3 px-3 py-2'
          >
            <div className='min-w-0'>
              <div className='break-all text-sm font-medium'>
                {labels[key] || key}
              </div>
              <div className='text-xs text-semi-color-text-2'>
                {t('记录')} {value.items || 0} · {t('原始额度')}{' '}
                {renderQuota(value.source_quota || 0)}
              </div>
            </div>
            <Text strong>{renderQuota(value.refund_quota || 0)}</Text>
          </div>
        ))}
      </div>
    </div>
  );
};

export default function QuickRefund() {
  const { t } = useTranslation();
  const canCreate = isRoot();
  const [dateRange, setDateRange] = useState(initialRange);
  const [options, setOptions] = useState({ channels: [], models: [] });
  const [channelIds, setChannelIds] = useState([]);
  const [modelNames, setModelNames] = useState([]);
  const [ratio, setRatio] = useState(100);
  const [reason, setReason] = useState('');
  const [optionsLoading, setOptionsLoading] = useState(false);
  const [optionsError, setOptionsError] = useState('');
  const [preview, setPreview] = useState(null);
  const [previewKey, setPreviewKey] = useState('');
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState('');
  const [confirmVisible, setConfirmVisible] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState('');
  const [idempotencyKey, setIdempotencyKey] = useState('');
  const [batches, setBatches] = useState([]);
  const [historyPage, setHistoryPage] = useState(1);
  const [historyTotal, setHistoryTotal] = useState(0);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyError, setHistoryError] = useState('');
  const [detail, setDetail] = useState(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');
  const [detailItemPage, setDetailItemPage] = useState(1);

  const payload = useMemo(
    () => ({
      start_time: toTimestamp(dateRange?.[0]),
      end_time: toTimestamp(dateRange?.[1]),
      channel_ids: channelIds,
      model_names: modelNames,
      ratio: Number(ratio || 0),
      reason: reason.trim(),
    }),
    [dateRange, channelIds, modelNames, ratio, reason],
  );
  const currentPayloadKey = useMemo(
    () => payloadFingerprint(payload),
    [payload],
  );
  const previewIsCurrent = Boolean(preview && previewKey === currentPayloadKey);

  const channelLabels = useMemo(
    () =>
      Object.fromEntries(
        (options.channels || []).map((channel) => [
          String(channel.id),
          channel.name ? `${channel.name} (#${channel.id})` : `#${channel.id}`,
        ]),
      ),
    [options.channels],
  );

  const validatePayload = useCallback(() => {
    if (
      !payload.start_time ||
      !payload.end_time ||
      payload.start_time > payload.end_time
    ) {
      return t('请选择有效的开始和结束时间');
    }
    if (!payload.channel_ids.length) return t('请至少选择一个渠道');
    if (payload.ratio < 1 || payload.ratio > 100) {
      return t('退款比例必须在 1% 到 100% 之间');
    }
    if (!payload.reason) return t('请填写退款原因');
    if (payload.reason.length > 500) return t('退款原因不能超过 500 个字符');
    return '';
  }, [payload, t]);

  const loadOptions = useCallback(async () => {
    const start = toTimestamp(dateRange?.[0]);
    const end = toTimestamp(dateRange?.[1]);
    if (!start || !end || start > end) {
      setOptionsError(t('请选择有效的开始和结束时间'));
      return;
    }
    setOptionsLoading(true);
    setOptionsError('');
    try {
      const data = await getRefundOptions(start, end);
      const next = data || { channels: [], models: [] };
      setOptions(next);
      const validChannelIds = new Set(
        (next.channels || []).map((item) => item.id),
      );
      const validModels = new Set(next.models || []);
      setChannelIds((current) =>
        current.filter((id) => validChannelIds.has(id)),
      );
      setModelNames((current) =>
        current.filter((name) => validModels.has(name)),
      );
    } catch (error) {
      setOptions({ channels: [], models: [] });
      setOptionsError(error.message);
    } finally {
      setOptionsLoading(false);
    }
  }, [dateRange, t]);

  const loadHistory = useCallback(
    async (page = historyPage) => {
      setHistoryLoading(true);
      setHistoryError('');
      try {
        const data = await getRefundBatches(page, HISTORY_PAGE_SIZE);
        setBatches(data?.items || []);
        setHistoryTotal(data?.total || 0);
        setHistoryPage(data?.page || page);
      } catch (error) {
        setHistoryError(error.message);
      } finally {
        setHistoryLoading(false);
      }
    },
    [historyPage],
  );

  useEffect(() => {
    loadOptions();
    loadHistory(1);
  }, []);

  const hasActiveBatches = batches.some((batch) =>
    ['pending', 'running'].includes(batch.status),
  );

  useEffect(() => {
    if (!hasActiveBatches) return undefined;
    const timer = window.setInterval(() => loadHistory(historyPage), 3000);
    return () => window.clearInterval(timer);
  }, [hasActiveBatches, historyPage, loadHistory]);

  useEffect(() => {
    if (
      !detailVisible ||
      !detail?.id ||
      !['pending', 'running'].includes(detail.status)
    ) {
      return undefined;
    }
    const timer = window.setInterval(async () => {
      try {
        const next = await getRefundBatch(detail.id, detailItemPage);
        setDetail(next);
        if (!['pending', 'running'].includes(next.status)) {
          await loadHistory(historyPage);
        }
      } catch (error) {
        setDetailError(error.message);
      }
    }, 3000);
    return () => window.clearInterval(timer);
  }, [
    detail?.id,
    detail?.status,
    detailItemPage,
    detailVisible,
    historyPage,
    loadHistory,
  ]);

  const handlePreview = async () => {
    const validationError = validatePayload();
    if (validationError) {
      setPreviewError(validationError);
      showError(validationError);
      return;
    }
    setPreviewLoading(true);
    setPreviewError('');
    setCreateError('');
    try {
      const data = await previewRefund(payload);
      setPreview(data);
      setPreviewKey(payloadFingerprint(payload));
      setIdempotencyKey(makeIdempotencyKey());
      showSuccess(t('退款预览已更新'));
    } catch (error) {
      setPreview(null);
      setPreviewKey('');
      setIdempotencyKey('');
      setPreviewError(error.message);
    } finally {
      setPreviewLoading(false);
    }
  };

  const openConfirm = () => {
    if (!canCreate) {
      setCreateError(t('仅超级管理员可以创建退款批次'));
      return;
    }
    if (!previewIsCurrent) {
      setCreateError(t('筛选条件已变化，请重新预览后再创建退款'));
      return;
    }
    if (!preview?.matched_items || !preview?.refund_quota) {
      setCreateError(t('当前预览没有可退款记录'));
      return;
    }
    setCreateError('');
    setConfirmVisible(true);
  };

  const handleCreate = async () => {
    if (!canCreate || !previewIsCurrent || !idempotencyKey) return;
    setCreateLoading(true);
    setCreateError('');
    try {
      const batch = await createRefundBatch({
        ...payload,
        idempotency_key: idempotencyKey,
      });
      setConfirmVisible(false);
      setPreview(null);
      setPreviewKey('');
      setIdempotencyKey('');
      showSuccess(t('退款批次已提交后台处理'));
      await loadHistory(1);
      if (batch?.id) await openDetail(batch.id);
    } catch (error) {
      setCreateError(error.message);
      showError(error.message);
    } finally {
      setCreateLoading(false);
    }
  };

  const openDetail = async (id, itemPage = 1) => {
    setDetailVisible(true);
    setDetailLoading(true);
    setDetailError('');
    setDetailItemPage(itemPage);
    setDetail(null);
    try {
      setDetail(await getRefundBatch(id, itemPage));
    } catch (error) {
      setDetailError(error.message);
    } finally {
      setDetailLoading(false);
    }
  };

  const hasOptions = (options.channels || []).length > 0;

  return (
    <div className='mx-auto mt-[60px] w-full max-w-[1440px] space-y-3 px-2 pb-6 sm:px-4'>
      <Card>
        <div className='flex flex-col justify-between gap-3 sm:flex-row sm:items-start'>
          <div>
            <Title heading={3} style={{ margin: 0 }}>
              {t('快捷退款')}
            </Title>
            <Paragraph type='tertiary' style={{ margin: '6px 0 0' }}>
              {t('按消费日志筛选钱包扣费记录，预览确认后批量返还额度。')}
            </Paragraph>
          </div>
          <Tag color={canCreate ? 'red' : 'blue'}>
            {canCreate ? t('超级管理员操作') : t('管理员只读')}
          </Tag>
        </div>
        {!canCreate ? (
          <Banner
            type='warning'
            closeIcon={null}
            description={t(
              '当前账号可以预览和查看历史，但只有超级管理员可以发起退款。',
            )}
            style={{ marginTop: 16 }}
          />
        ) : null}
      </Card>

      <Card>
        <div className='mb-4 flex flex-wrap items-center justify-between gap-2'>
          <div>
            <Title heading={5} style={{ margin: 0 }}>
              {t('退款条件')}
            </Title>
            <Text type='tertiary'>{t('先加载时间范围内可用的渠道和模型')}</Text>
          </div>
          <Button
            theme='outline'
            icon={<IconRefresh />}
            loading={optionsLoading}
            onClick={loadOptions}
          >
            {t('加载选项')}
          </Button>
        </div>

        {optionsError ? (
          <Banner
            type='danger'
            closeIcon={null}
            description={optionsError}
            style={{ marginBottom: 12 }}
          />
        ) : null}

        <Spin spinning={optionsLoading}>
          <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
            <div className='lg:col-span-2'>
              <Text strong>{t('时间范围')}</Text>
              <DatePicker
                type='dateTimeRange'
                value={dateRange}
                onChange={(value) => setDateRange(value || [])}
                placeholder={[t('开始时间'), t('结束时间')]}
                style={{ width: '100%', marginTop: 8 }}
                showClear={false}
              />
            </div>

            <div className='min-w-0'>
              <div className='mb-2 flex items-center justify-between gap-2'>
                <Text strong>{t('渠道')}</Text>
                <div className='flex gap-1'>
                  <Button
                    size='small'
                    theme='borderless'
                    disabled={!hasOptions}
                    onClick={() =>
                      setChannelIds(
                        (options.channels || []).map((item) => item.id),
                      )
                    }
                  >
                    {t('全选')}
                  </Button>
                  <Button
                    size='small'
                    theme='borderless'
                    disabled={!channelIds.length}
                    onClick={() => setChannelIds([])}
                  >
                    {t('清空')}
                  </Button>
                </div>
              </div>
              <Select
                multiple
                filter
                value={channelIds}
                optionList={(options.channels || []).map((channel) => ({
                  value: channel.id,
                  label: channel.name
                    ? `${channel.name} (#${channel.id})`
                    : `#${channel.id}`,
                }))}
                onChange={(value) => setChannelIds(value || [])}
                placeholder={t('选择一个或多个渠道')}
                prefix={<IconSearch />}
                style={{ width: '100%' }}
                maxTagCount={3}
              />
            </div>

            <div className='min-w-0'>
              <div className='mb-2 flex items-center justify-between gap-2'>
                <Text strong>{t('模型')}</Text>
                <div className='flex gap-1'>
                  <Button
                    size='small'
                    theme='borderless'
                    disabled={!options.models?.length}
                    onClick={() => setModelNames(options.models || [])}
                  >
                    {t('全选')}
                  </Button>
                  <Button
                    size='small'
                    theme='borderless'
                    disabled={!modelNames.length}
                    onClick={() => setModelNames([])}
                  >
                    {t('全部模型')}
                  </Button>
                </div>
              </div>
              <Select
                multiple
                filter
                value={modelNames}
                optionList={(options.models || []).map((model) => ({
                  value: model,
                  label: model,
                }))}
                onChange={(value) => setModelNames(value || [])}
                placeholder={t('不选择表示该渠道的全部模型')}
                prefix={<IconSearch />}
                style={{ width: '100%' }}
                maxTagCount={3}
              />
            </div>

            <div className='min-w-0'>
              <Text strong>{t('退款比例')}</Text>
              <div className='mt-2 flex flex-wrap items-center gap-2'>
                {QUICK_RATIOS.map((value) => (
                  <Button
                    key={value}
                    theme={ratio === value ? 'solid' : 'outline'}
                    type={ratio === value ? 'primary' : 'tertiary'}
                    onClick={() => setRatio(value)}
                  >
                    {value}%
                  </Button>
                ))}
                <InputNumber
                  min={1}
                  max={100}
                  value={ratio}
                  suffix='%'
                  onChange={(value) => setRatio(Number(value || 0))}
                  style={{ width: 120 }}
                />
              </div>
            </div>

            <div className='min-w-0'>
              <Text strong>{t('退款原因')}</Text>
              <Input
                value={reason}
                maxLength={500}
                showClear
                onChange={setReason}
                placeholder={t('填写本次退款的原因，最多 500 字')}
                style={{ marginTop: 8 }}
              />
              <div className='mt-1 text-right text-xs text-semi-color-text-2'>
                {reason.length}/500
              </div>
            </div>
          </div>

          <div className='mt-4 flex flex-col gap-2 sm:flex-row sm:items-center'>
            <Button
              type='primary'
              icon={<IconSearch />}
              loading={previewLoading}
              onClick={handlePreview}
            >
              {t('预览退款')}
            </Button>
            <Text type='tertiary' size='small'>
              {t('未选择模型时，将匹配所选渠道下的全部模型。')}
            </Text>
          </div>
        </Spin>
      </Card>

      <Card>
        <div className='mb-3'>
          <Title heading={5} style={{ margin: 0 }}>
            {t('退款预览')}
          </Title>
          <Text type='tertiary'>{t('实际退款前请核对命中数量和返还额度')}</Text>
        </div>

        {previewError ? (
          <Banner
            type='danger'
            closeIcon={null}
            description={previewError}
            style={{ marginBottom: 12 }}
          />
        ) : null}
        {createError ? (
          <Banner
            type='danger'
            closeIcon={null}
            description={createError}
            style={{ marginBottom: 12 }}
          />
        ) : null}

        <Spin spinning={previewLoading}>
          {previewIsCurrent ? (
            <div className='space-y-4'>
              <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
                <StatBlock
                  label={t('命中记录')}
                  value={preview.matched_items || 0}
                />
                <StatBlock
                  label={t('原始消费额度')}
                  value={renderQuota(preview.source_quota || 0)}
                />
                <StatBlock
                  label={t('预计退款额度')}
                  value={renderQuota(preview.refund_quota || 0)}
                />
                <StatBlock
                  label={t('跳过记录')}
                  value={preview.skipped || 0}
                  tone={preview.skipped ? 'danger' : 'default'}
                />
              </div>
              <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
                <Breakdown
                  title={t('按渠道汇总')}
                  data={preview.by_channel}
                  labels={channelLabels}
                />
                <Breakdown title={t('按模型汇总')} data={preview.by_model} />
              </div>
              <div className='flex flex-col justify-between gap-3 border-t border-semi-color-border pt-4 sm:flex-row sm:items-center'>
                <Text type='tertiary' size='small'>
                  {t(
                    '创建前会再次显示明确确认窗口。退款完成后不可在此页面撤销。',
                  )}
                </Text>
                <Button
                  type='danger'
                  theme='solid'
                  disabled={
                    !canCreate || !preview.refund_quota || createLoading
                  }
                  onClick={openConfirm}
                >
                  {t('确认创建退款')}
                </Button>
              </div>
            </div>
          ) : (
            <Empty description={t('请先设置筛选条件并生成退款预览')} />
          )}
        </Spin>
      </Card>

      <Card>
        <RefundHistory
          batches={batches}
          pageInfo={{
            page: historyPage,
            pageSize: HISTORY_PAGE_SIZE,
            total: historyTotal,
          }}
          loading={historyLoading}
          error={historyError}
          onRefresh={() => loadHistory(historyPage)}
          onPageChange={(page) => loadHistory(page)}
          detail={detail}
          detailVisible={detailVisible}
          detailLoading={detailLoading}
          detailError={detailError}
          onOpenDetail={openDetail}
          onDetailPageChange={(page) => {
            if (detail?.id) openDetail(detail.id, page);
          }}
          onCloseDetail={() => {
            setDetailVisible(false);
            setDetailItemPage(1);
          }}
        />
      </Card>

      <Modal
        title={t('确认执行快捷退款')}
        visible={confirmVisible}
        onCancel={() => {
          if (!createLoading) setConfirmVisible(false);
        }}
        closeOnEsc={!createLoading}
        width={560}
        style={{ maxWidth: 'calc(100vw - 24px)' }}
        footer={
          <div className='flex flex-wrap justify-end gap-2'>
            <Button
              disabled={createLoading}
              onClick={() => setConfirmVisible(false)}
            >
              {t('取消')}
            </Button>
            <Button
              type='danger'
              theme='solid'
              loading={createLoading}
              disabled={!canCreate || !previewIsCurrent}
              onClick={handleCreate}
            >
              {t('确认退款')}
            </Button>
          </div>
        }
      >
        <Banner
          type='warning'
          closeIcon={null}
          description={t(
            '此操作会直接增加用户钱包余额，执行后不能通过本页面撤销。',
          )}
        />
        <div className='mt-4 grid grid-cols-2 gap-3 rounded-md bg-semi-color-fill-0 p-3'>
          <StatBlock
            label={t('命中记录')}
            value={preview?.matched_items || 0}
          />
          <StatBlock
            label={t('退款额度')}
            value={renderQuota(preview?.refund_quota || 0)}
          />
          <div className='col-span-2'>
            <Text type='tertiary' size='small'>
              {t('退款原因')}
            </Text>
            <div className='mt-1 break-words'>{payload.reason || '-'}</div>
          </div>
        </div>
      </Modal>
    </div>
  );
}
