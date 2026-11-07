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

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import {
  Button,
  DatePicker,
  Empty,
  Input,
  Modal,
  Select,
  SideSheet,
  Tabs,
  TabPane,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  API,
  renderQuota,
  renderQuotaWithAmount,
  showError,
  showSuccess,
  timestamp2string,
} from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import CardTable from '../../../common/ui/CardTable';

const { Text } = Typography;

const DEFAULT_PAGE_SIZE = 20;
const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

// 状态映射必须覆盖 success/pending/failed/expired，未知状态如实展示，绝不回退成 pending
const STATUS_CONFIG = {
  success: { color: 'green', label: '成功' },
  pending: { color: 'orange', label: '待支付' },
  failed: { color: 'red', label: '失败' },
  expired: { color: 'grey', label: '已过期' },
};

const PAYMENT_METHOD_LABELS = {
  alipay: '支付宝',
  wxpay: '微信',
  stripe: 'Stripe',
  creem: 'Creem',
  waffo: 'Waffo',
  waffo_pancake: 'Waffo Pancake',
  balance: '余额',
};

const buildQuery = (params) => {
  return Object.entries(params)
    .filter(
      ([, value]) => value !== undefined && value !== null && value !== '',
    )
    .map(([key, value]) => `${key}=${encodeURIComponent(value)}`)
    .join('&');
};

const toSeconds = (value) => {
  if (!value) return null;
  if (value instanceof Date) {
    const ms = value.getTime();
    return Number.isFinite(ms) ? Math.floor(ms / 1000) : null;
  }
  const numeric = Number(value);
  if (Number.isFinite(numeric)) {
    return Math.floor(numeric);
  }
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? Math.floor(parsed / 1000) : null;
};

const isSubscriptionTopup = (record) => {
  const tradeNo = (record?.trade_no || '').toLowerCase();
  return Number(record?.amount || 0) === 0 && tradeNo.startsWith('sub');
};

const renderStatusTag = (status, t) => {
  const config = STATUS_CONFIG[status];
  if (!config) {
    return (
      <Tag color='grey' shape='circle' size='small'>
        {status || '-'}
      </Tag>
    );
  }
  return (
    <Tag color={config.color} shape='circle' size='small'>
      {t(config.label)}
    </Tag>
  );
};

const renderEmpty = (description) => (
  <Empty
    image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
    darkModeImage={
      <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
    }
    description={description}
    style={{ padding: 30 }}
  />
);

const UserBillingModal = ({ visible, onCancel, user, t, onSuccess }) => {
  const isMobile = useIsMobile();

  const [activeTab, setActiveTab] = useState('topup');
  const lastUserIdRef = useRef(null);

  // 充值订单
  const [topupPage, setTopupPage] = useState(1);
  const [topupPageSize, setTopupPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [topupStatus, setTopupStatus] = useState('');
  const [topupKeyword, setTopupKeyword] = useState('');
  const [topupKeywordInput, setTopupKeywordInput] = useState('');
  const [topupRange, setTopupRange] = useState([]);
  const [topupItems, setTopupItems] = useState([]);
  const [topupTotal, setTopupTotal] = useState(0);
  const [topupLoading, setTopupLoading] = useState(false);
  const [topupRefreshKey, setTopupRefreshKey] = useState(0);

  // 消费明细
  const [consumePage, setConsumePage] = useState(1);
  const [consumePageSize, setConsumePageSize] = useState(DEFAULT_PAGE_SIZE);
  const [consumeRange, setConsumeRange] = useState([]);
  const [consumeItems, setConsumeItems] = useState([]);
  const [consumeTotal, setConsumeTotal] = useState(0);
  const [consumeLoading, setConsumeLoading] = useState(false);

  // 盲盒订单
  const [blindBoxPage, setBlindBoxPage] = useState(1);
  const [blindBoxPageSize, setBlindBoxPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [blindBoxItems, setBlindBoxItems] = useState([]);
  const [blindBoxTotal, setBlindBoxTotal] = useState(0);
  const [blindBoxLoading, setBlindBoxLoading] = useState(false);

  // 切换用户时重置所有 Tab 的独立分页与筛选，避免串用户数据
  useEffect(() => {
    if (!visible || !user?.id) return;
    if (lastUserIdRef.current === user.id) return;
    lastUserIdRef.current = user.id;

    setActiveTab('topup');

    setTopupPage(1);
    setTopupPageSize(DEFAULT_PAGE_SIZE);
    setTopupStatus('');
    setTopupKeyword('');
    setTopupKeywordInput('');
    setTopupRange([]);
    setTopupItems([]);
    setTopupTotal(0);

    setConsumePage(1);
    setConsumePageSize(DEFAULT_PAGE_SIZE);
    setConsumeRange([]);
    setConsumeItems([]);
    setConsumeTotal(0);

    setBlindBoxPage(1);
    setBlindBoxPageSize(DEFAULT_PAGE_SIZE);
    setBlindBoxItems([]);
    setBlindBoxTotal(0);
  }, [visible, user?.id]);

  // 充值订单列表
  useEffect(() => {
    if (!visible || !user?.id || activeTab !== 'topup') return;
    let ignore = false;

    const load = async () => {
      setTopupLoading(true);
      try {
        const qs = buildQuery({
          user_id: user.id,
          p: topupPage,
          page_size: topupPageSize,
          status: topupStatus,
          start_timestamp: toSeconds(topupRange?.[0]),
          end_timestamp: toSeconds(topupRange?.[1]),
          keyword: topupKeyword,
        });
        const res = await API.get(`/api/user/topup?${qs}`);
        if (ignore) return;
        const { success, message, data } = res.data || {};
        if (success) {
          setTopupItems(data?.items || []);
          setTopupTotal(Number(data?.total) || 0);
        } else {
          showError(message || t('加载充值订单失败'));
          setTopupItems([]);
          setTopupTotal(0);
        }
      } catch (error) {
        if (ignore) return;
        showError(t('加载充值订单失败'));
        setTopupItems([]);
        setTopupTotal(0);
      } finally {
        if (!ignore) setTopupLoading(false);
      }
    };

    load();
    return () => {
      ignore = true;
    };
  }, [
    visible,
    user?.id,
    activeTab,
    topupPage,
    topupPageSize,
    topupStatus,
    topupRange,
    topupKeyword,
    topupRefreshKey,
    t,
  ]);

  // 消费明细列表
  useEffect(() => {
    if (!visible || !user?.id || activeTab !== 'consume') return;
    let ignore = false;

    const load = async () => {
      setConsumeLoading(true);
      try {
        const qs = buildQuery({
          user_id: user.id,
          p: consumePage,
          page_size: consumePageSize,
          type: 2,
          start_timestamp: toSeconds(consumeRange?.[0]),
          end_timestamp: toSeconds(consumeRange?.[1]),
        });
        const res = await API.get(`/api/log?${qs}`);
        if (ignore) return;
        const { success, message, data } = res.data || {};
        if (success) {
          setConsumeItems(data?.items || []);
          setConsumeTotal(Number(data?.total) || 0);
        } else {
          showError(message || t('加载消费明细失败'));
          setConsumeItems([]);
          setConsumeTotal(0);
        }
      } catch (error) {
        if (ignore) return;
        showError(t('加载消费明细失败'));
        setConsumeItems([]);
        setConsumeTotal(0);
      } finally {
        if (!ignore) setConsumeLoading(false);
      }
    };

    load();
    return () => {
      ignore = true;
    };
  }, [
    visible,
    user?.id,
    activeTab,
    consumePage,
    consumePageSize,
    consumeRange,
    t,
  ]);

  // 盲盒订单列表
  useEffect(() => {
    if (!visible || !user?.id || activeTab !== 'blind_box') return;
    let ignore = false;

    const load = async () => {
      setBlindBoxLoading(true);
      try {
        const qs = buildQuery({
          user_id: user.id,
          p: blindBoxPage,
          page_size: blindBoxPageSize,
        });
        const res = await API.get(`/api/blind-box/admin/orders?${qs}`);
        if (ignore) return;
        const { success, message, data } = res.data || {};
        if (success) {
          setBlindBoxItems(data?.items || []);
          setBlindBoxTotal(Number(data?.total) || 0);
        } else {
          showError(message || t('加载盲盒订单失败'));
          setBlindBoxItems([]);
          setBlindBoxTotal(0);
        }
      } catch (error) {
        if (ignore) return;
        showError(t('加载盲盒订单失败'));
        setBlindBoxItems([]);
        setBlindBoxTotal(0);
      } finally {
        if (!ignore) setBlindBoxLoading(false);
      }
    };

    load();
    return () => {
      ignore = true;
    };
  }, [visible, user?.id, activeTab, blindBoxPage, blindBoxPageSize, t]);

  const applyTopupKeyword = useCallback(() => {
    setTopupKeyword((topupKeywordInput || '').trim());
    setTopupPage(1);
  }, [topupKeywordInput]);

  const handleTopupKeywordInput = (value) => {
    setTopupKeywordInput(value);
    if (!value) {
      setTopupKeyword('');
      setTopupPage(1);
    }
  };

  const resetTopupFilters = () => {
    setTopupStatus('');
    setTopupKeyword('');
    setTopupKeywordInput('');
    setTopupRange([]);
    setTopupPage(1);
  };

  const resetConsumeFilters = () => {
    setConsumeRange([]);
    setConsumePage(1);
  };

  // 管理员补单
  const completeTopup = useCallback(
    async (tradeNo) => {
      try {
        const res = await API.post('/api/user/topup/complete', {
          trade_no: tradeNo,
        });
        const { success, message } = res.data || {};
        if (success) {
          showSuccess(t('补单成功'));
          setTopupRefreshKey((key) => key + 1);
          onSuccess?.();
        } else {
          showError(message || t('补单失败'));
        }
      } catch (error) {
        showError(t('补单失败'));
      }
    },
    [onSuccess, t],
  );

  const confirmCompleteTopup = useCallback(
    (tradeNo) => {
      Modal.confirm({
        title: t('确认补单'),
        content: t('是否将该订单标记为成功并为用户入账？'),
        centered: true,
        onOk: () => completeTopup(tradeNo),
      });
    },
    [completeTopup, t],
  );

  const topupColumns = useMemo(
    () => [
      {
        title: t('订单号'),
        dataIndex: 'trade_no',
        key: 'trade_no',
        width: 220,
        render: (text) =>
          text ? <Text copyable>{text}</Text> : <Text type='tertiary'>-</Text>,
      },
      {
        title: t('创建时间'),
        dataIndex: 'create_time',
        key: 'create_time',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('完成时间'),
        dataIndex: 'complete_time',
        key: 'complete_time',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('支付方式'),
        dataIndex: 'payment_method',
        key: 'payment_method',
        width: 110,
        render: (value) => (
          <Text>
            {PAYMENT_METHOD_LABELS[value]
              ? t(PAYMENT_METHOD_LABELS[value])
              : value || '-'}
          </Text>
        ),
      },
      {
        title: t('充值金额'),
        dataIndex: 'amount',
        key: 'amount',
        width: 140,
        render: (amount, record) => {
          if (isSubscriptionTopup(record)) {
            return (
              <Tag color='purple' shape='circle' size='small'>
                {t('订阅套餐')}
              </Tag>
            );
          }
          return <Text>{renderQuotaWithAmount(Number(amount) || 0)}</Text>;
        },
      },
      {
        title: t('实付金额'),
        dataIndex: 'money',
        key: 'money',
        width: 120,
        render: (money) => (
          <Text type='danger'>¥{(Number(money) || 0).toFixed(2)}</Text>
        ),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        key: 'status',
        width: 110,
        render: (status) => renderStatusTag(status, t),
      },
      {
        title: t('操作'),
        key: 'action',
        width: 100,
        fixed: 'right',
        render: (_, record) =>
          record?.status === 'pending' ? (
            <Button
              size='small'
              type='primary'
              theme='outline'
              onClick={() => confirmCompleteTopup(record.trade_no)}
            >
              {t('补单')}
            </Button>
          ) : null,
      },
    ],
    [confirmCompleteTopup, t],
  );

  const consumeColumns = useMemo(
    () => [
      {
        title: t('时间'),
        dataIndex: 'created_at',
        key: 'created_at',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('模型'),
        dataIndex: 'model_name',
        key: 'model_name',
        width: 200,
        render: (text) =>
          text ? (
            <Tag color='blue' shape='circle' size='small'>
              {text}
            </Tag>
          ) : (
            '-'
          ),
      },
      {
        title: t('令牌'),
        dataIndex: 'token_name',
        key: 'token_name',
        width: 160,
        render: (text) =>
          text ? (
            <Tag color='grey' shape='circle' size='small'>
              {text}
            </Tag>
          ) : (
            '-'
          ),
      },
      {
        title: t('消耗额度'),
        dataIndex: 'quota',
        key: 'quota',
        width: 140,
        render: (quota) => <Text>{renderQuota(Number(quota) || 0, 6)}</Text>,
      },
      {
        title: t('请求 ID'),
        dataIndex: 'request_id',
        key: 'request_id',
        width: 260,
        render: (text) =>
          text ? <Text copyable>{text}</Text> : <Text type='tertiary'>-</Text>,
      },
    ],
    [t],
  );

  const blindBoxColumns = useMemo(
    () => [
      {
        title: t('盲盒'),
        dataIndex: 'box_name',
        key: 'box_name',
        width: 160,
        render: (text, record) =>
          text || (record?.box_id ? `#${record.box_id}` : '-'),
      },
      {
        title: t('售价'),
        dataIndex: 'price',
        key: 'price',
        width: 120,
        render: (price) => (
          <Text type='danger'>¥{(Number(price) || 0).toFixed(2)}</Text>
        ),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        key: 'status',
        width: 110,
        render: (status) => renderStatusTag(status, t),
      },
      {
        title: t('抽中倍率'),
        dataIndex: 'drawn_multiplier',
        key: 'drawn_multiplier',
        width: 110,
        render: (multiplier) => {
          const value = Number(multiplier);
          return value > 0 ? `${value.toFixed(2)}x` : '-';
        },
      },
      {
        title: t('获得余额'),
        dataIndex: 'reward_quota',
        key: 'reward_quota',
        width: 130,
        render: (quota, record) =>
          record?.status === 'success' ? renderQuota(Number(quota) || 0) : '-',
      },
      {
        title: t('时间'),
        dataIndex: 'create_time',
        key: 'create_time',
        width: 190,
        render: (value, record) => (
          <div className='text-xs text-gray-600'>
            <div>{value ? timestamp2string(value) : '-'}</div>
            {record?.complete_time ? (
              <div>
                {t('完成')}: {timestamp2string(record.complete_time)}
              </div>
            ) : null}
          </div>
        ),
      },
    ],
    [t],
  );

  const paginationProps = (page, pageSize, total, setPage, setPageSize) => ({
    currentPage: page,
    pageSize,
    total,
    pageSizeOpts: PAGE_SIZE_OPTIONS,
    showSizeChanger: true,
    onPageChange: setPage,
    onPageSizeChange: (size) => {
      setPageSize(size);
      setPage(1);
    },
  });

  const balance = Number(user?.quota);
  const usedQuota = Number(user?.used_quota);

  return (
    <SideSheet
      visible={visible}
      placement='right'
      width={isMobile ? '100%' : 1080}
      bodyStyle={{ padding: 0 }}
      onCancel={onCancel}
      title={
        <div className='min-w-0'>
          <div className='flex items-center gap-2 flex-wrap'>
            <Tag color='blue' shape='circle' size='small'>
              {t('账单')}
            </Tag>
            <span className='font-semibold text-base'>
              {user?.username || '-'}
            </span>
            <span
              className='text-xs'
              style={{ color: 'var(--semi-color-text-2)' }}
            >
              ID: {user?.id ?? '-'}
            </span>
          </div>
          <div
            className='mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs'
            style={{ color: 'var(--semi-color-text-2)' }}
          >
            <span>
              {t('当前余额')}：
              <Text strong>
                {renderQuota(Number.isFinite(balance) ? balance : 0)}
              </Text>
            </span>
            <span>
              {t('已用额度')}：
              <Text strong>
                {renderQuota(Number.isFinite(usedQuota) ? usedQuota : 0)}
              </Text>
            </span>
          </div>
        </div>
      }
    >
      <div className='p-4'>
        <Tabs
          activeKey={activeTab}
          onChange={(key) => setActiveTab(key)}
          type='line'
        >
          <TabPane tab={t('充值订单')} itemKey='topup' />
          <TabPane tab={t('消费明细')} itemKey='consume' />
          <TabPane tab={t('盲盒订单')} itemKey='blind_box' />
        </Tabs>

        {activeTab === 'topup' && (
          <div className='pt-3'>
            <div className='flex flex-col xl:flex-row xl:items-center gap-2 mb-3'>
              <DatePicker
                type='dateTimeRange'
                value={topupRange}
                onChange={(value) => {
                  setTopupRange(Array.isArray(value) ? value : []);
                  setTopupPage(1);
                }}
                placeholder={[t('开始时间'), t('结束时间')]}
                showClear
                style={{ width: isMobile ? '100%' : 320 }}
              />
              <Select
                value={topupStatus || undefined}
                onChange={(value) => {
                  setTopupStatus(value || '');
                  setTopupPage(1);
                }}
                placeholder={t('全部状态')}
                optionList={[
                  { value: 'pending', label: t('待支付') },
                  { value: 'success', label: t('成功') },
                  { value: 'failed', label: t('失败') },
                  { value: 'expired', label: t('已过期') },
                ]}
                showClear
                style={{ width: isMobile ? '100%' : 140 }}
              />
              <div className='flex flex-wrap items-center gap-2'>
                <Input
                  prefix={<IconSearch />}
                  value={topupKeywordInput}
                  onChange={handleTopupKeywordInput}
                  placeholder={t('订单号关键词')}
                  showClear
                  onEnterPress={applyTopupKeyword}
                  style={{ width: isMobile ? '100%' : 200 }}
                />
                <Button type='tertiary' onClick={applyTopupKeyword}>
                  {t('查询')}
                </Button>
                <Button type='tertiary' onClick={resetTopupFilters}>
                  {t('重置')}
                </Button>
              </div>
            </div>

            <CardTable
              columns={topupColumns}
              dataSource={topupItems}
              rowKey='id'
              loading={topupLoading}
              scroll={{ x: 'max-content' }}
              pagination={paginationProps(
                topupPage,
                topupPageSize,
                topupTotal,
                setTopupPage,
                setTopupPageSize,
              )}
              empty={renderEmpty(t('暂无充值记录'))}
              size='middle'
            />
          </div>
        )}

        {activeTab === 'consume' && (
          <div className='pt-3'>
            <div className='flex flex-col xl:flex-row xl:items-center gap-2 mb-3'>
              <DatePicker
                type='dateTimeRange'
                value={consumeRange}
                onChange={(value) => {
                  setConsumeRange(Array.isArray(value) ? value : []);
                  setConsumePage(1);
                }}
                placeholder={[t('开始时间'), t('结束时间')]}
                showClear
                style={{ width: isMobile ? '100%' : 320 }}
              />
              <Button type='tertiary' onClick={resetConsumeFilters}>
                {t('重置')}
              </Button>
            </div>

            <CardTable
              columns={consumeColumns}
              dataSource={consumeItems}
              rowKey='id'
              loading={consumeLoading}
              scroll={{ x: 'max-content' }}
              pagination={paginationProps(
                consumePage,
                consumePageSize,
                consumeTotal,
                setConsumePage,
                setConsumePageSize,
              )}
              empty={renderEmpty(t('暂无消费记录'))}
              size='middle'
            />
          </div>
        )}

        {activeTab === 'blind_box' && (
          <div className='pt-3'>
            <CardTable
              columns={blindBoxColumns}
              dataSource={blindBoxItems}
              rowKey='id'
              loading={blindBoxLoading}
              scroll={{ x: 'max-content' }}
              pagination={paginationProps(
                blindBoxPage,
                blindBoxPageSize,
                blindBoxTotal,
                setBlindBoxPage,
                setBlindBoxPageSize,
              )}
              empty={renderEmpty(t('暂无盲盒订单'))}
              size='middle'
            />
          </div>
        )}
      </div>
    </SideSheet>
  );
};

export default UserBillingModal;
