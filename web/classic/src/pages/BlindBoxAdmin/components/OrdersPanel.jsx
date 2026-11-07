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
import {
  Button,
  DatePicker,
  Empty,
  Input,
  InputNumber,
  Modal,
  Select,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconRefresh, IconSearch } from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';
import {
  renderQuota,
  showError,
  showSuccess,
  timestamp2string,
} from '../../../helpers';
import CardTable from '../../../components/common/ui/CardTable';
import {
  completeBlindBoxOrder,
  formatPrice,
  getAdminBlindBoxOrders,
} from '../api';

const { Text } = Typography;

const ORDER_STATUS_CONFIG = {
  success: { color: 'green', label: '成功' },
  pending: { color: 'orange', label: '待支付' },
  failed: { color: 'red', label: '失败' },
  expired: { color: 'grey', label: '已过期' },
};

const EMPTY_FILTERS = {
  box_id: undefined,
  user_id: undefined,
  status: undefined,
  keyword: '',
  dateRange: undefined,
};

function renderOrderStatus(status, t) {
  const config = ORDER_STATUS_CONFIG[status];
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
}

function formatMultiplier(value) {
  const multiplier = Number(value);
  if (!Number.isFinite(multiplier) || multiplier <= 0) {
    return '-';
  }
  const text = multiplier.toFixed(4).replace(/0+$/, '').replace(/\.$/, '');
  return `×${text}`;
}

function toTimestamp(value) {
  if (!value) return null;
  const timestamp = new Date(value).getTime();
  return Number.isFinite(timestamp) ? Math.floor(timestamp / 1000) : null;
}

const OrdersPanel = ({ boxes = [], active = false, onCompleted }) => {
  const { t } = useTranslation();

  const [filters, setFilters] = useState(EMPTY_FILTERS);
  const [orders, setOrders] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loading, setLoading] = useState(false);
  const [completingTradeNo, setCompletingTradeNo] = useState(null);

  const loadOrders = async (
    nextPage = 1,
    nextPageSize = pageSize,
    nextFilters = filters,
  ) => {
    setLoading(true);
    try {
      const params = {
        p: nextPage,
        page_size: nextPageSize,
      };

      if (nextFilters.box_id) {
        params.box_id = nextFilters.box_id;
      }
      if (Number(nextFilters.user_id) > 0) {
        params.user_id = Number(nextFilters.user_id);
      }
      if (nextFilters.status) {
        params.status = nextFilters.status;
      }
      const keyword = (nextFilters.keyword || '').trim();
      if (keyword) {
        params.keyword = keyword;
      }
      if (Array.isArray(nextFilters.dateRange)) {
        const [startValue, endValue] = nextFilters.dateRange;
        const startTimestamp = toTimestamp(startValue);
        const endTimestamp = toTimestamp(endValue);
        if (startTimestamp !== null) {
          params.start_timestamp = startTimestamp;
        }
        if (endTimestamp !== null) {
          params.end_timestamp = endTimestamp;
        }
      }

      const res = await getAdminBlindBoxOrders(params);
      if (res.data?.success) {
        const data = res.data.data || {};
        setOrders(Array.isArray(data.items) ? data.items : []);
        setTotal(Number(data.total) || 0);
        setPage(nextPage);
        setPageSize(nextPageSize);
      } else {
        showError(res.data?.message || t('加载订单失败'));
      }
    } catch (e) {
      showError(t('加载订单失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!active) return;
    // 仅在切换到订单记录页时按当前筛选条件加载一次
    loadOrders(1, pageSize, filters);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active]);

  const handleSearch = () => {
    loadOrders(1, pageSize, filters);
  };

  const handleReset = () => {
    const emptyFilters = { ...EMPTY_FILTERS };
    setFilters(emptyFilters);
    loadOrders(1, pageSize, emptyFilters);
  };

  const handlePageChange = (nextPage) => {
    loadOrders(nextPage, pageSize, filters);
  };

  const handlePageSizeChange = (nextPageSize) => {
    loadOrders(1, nextPageSize, filters);
  };

  const confirmComplete = (record) => {
    Modal.confirm({
      title: t('确认手动补单'),
      content: `${t('订单')} ${record.trade_no} ${t(
        '将执行开奖并向用户发放余额，是否继续？',
      )}`,
      centered: true,
      onOk: async () => {
        setCompletingTradeNo(record.trade_no);
        try {
          const res = await completeBlindBoxOrder(record.trade_no);
          if (res.data?.success) {
            showSuccess(t('补单成功'));
            await loadOrders(page, pageSize, filters);
            onCompleted?.();
          } else {
            showError(res.data?.message || t('补单失败'));
          }
        } catch (e) {
          showError(t('补单失败'));
        } finally {
          setCompletingTradeNo(null);
        }
      },
    });
  };

  const boxOptions = (boxes || []).map((box) => ({
    label: `${box.name || `#${box.id}`} (#${box.id})`,
    value: box.id,
  }));

  const statusOptions = [
    { value: 'success', label: t('成功') },
    { value: 'pending', label: t('待支付') },
    { value: 'failed', label: t('失败') },
    { value: 'expired', label: t('已过期') },
  ];

  const columns = [
    {
      title: t('订单号'),
      dataIndex: 'trade_no',
      key: 'trade_no',
      width: 190,
      render: (tradeNo) => <Text copyable>{tradeNo || '-'}</Text>,
    },
    {
      title: t('用户 ID'),
      dataIndex: 'user_id',
      key: 'user_id',
      width: 80,
      render: (userId) => <Text>{userId ?? '-'}</Text>,
    },
    {
      title: t('盲盒'),
      key: 'box',
      width: 160,
      render: (_, record) => (
        <div className='min-w-0'>
          <div className='truncate'>{record.box_name || '-'}</div>
          <div className='text-xs text-gray-500'>{`#${record.box_id ?? '-'}`}</div>
        </div>
      ),
    },
    {
      title: t('售价'),
      dataIndex: 'price',
      key: 'price',
      width: 100,
      render: (price) => <Text type='danger'>{formatPrice(price)}</Text>,
    },
    {
      title: t('状态'),
      key: 'status',
      width: 100,
      render: (_, record) => renderOrderStatus(record.status, t),
    },
    {
      title: t('抽中倍率'),
      key: 'drawn_multiplier',
      width: 110,
      render: (_, record) =>
        record.status === 'success' ? (
          <Text strong>{formatMultiplier(record.drawn_multiplier)}</Text>
        ) : (
          <Text type='tertiary'>-</Text>
        ),
    },
    {
      title: t('获得余额'),
      key: 'reward_quota',
      width: 120,
      render: (_, record) =>
        record.status === 'success' ? (
          <Text strong>{renderQuota(Number(record.reward_quota) || 0)}</Text>
        ) : (
          <Text type='tertiary'>-</Text>
        ),
    },
    {
      title: t('标记'),
      key: 'flags',
      width: 120,
      render: (_, record) => {
        const flags = [];
        if (record.oversold) {
          flags.push(
            <Tag key='oversold' color='red' shape='circle' size='small'>
              {t('超卖')}
            </Tag>,
          );
        }
        if (record.over_limit) {
          flags.push(
            <Tag key='over_limit' color='orange' shape='circle' size='small'>
              {t('超限')}
            </Tag>,
          );
        }
        if (flags.length === 0) {
          return <Text type='tertiary'>-</Text>;
        }
        return <div className='flex flex-wrap gap-1'>{flags}</div>;
      },
    },
    {
      title: t('创建/完成时间'),
      key: 'time',
      width: 190,
      render: (_, record) => (
        <div className='text-xs text-gray-600 flex flex-col gap-1'>
          <div>
            {`${t('创建')}: ${
              record.create_time ? timestamp2string(record.create_time) : '-'
            }`}
          </div>
          <div>
            {`${t('完成')}: ${
              record.complete_time
                ? timestamp2string(record.complete_time)
                : '-'
            }`}
          </div>
        </div>
      ),
    },
    {
      title: t('操作'),
      key: 'action',
      width: 110,
      render: (_, record) => {
        if (record.status !== 'pending' && record.status !== 'expired') {
          return null;
        }
        return (
          <Button
            size='small'
            type='primary'
            theme='outline'
            loading={completingTradeNo === record.trade_no}
            onClick={() => confirmComplete(record)}
          >
            {t('手动补单')}
          </Button>
        );
      },
    },
  ];

  return (
    <div>
      <div className='flex flex-wrap items-center gap-2 mb-3'>
        <Select
          placeholder={t('选择盲盒')}
          optionList={boxOptions}
          value={filters.box_id}
          onChange={(value) =>
            setFilters((prev) => ({ ...prev, box_id: value }))
          }
          showClear
          filter
          style={{ width: 200 }}
        />
        <InputNumber
          placeholder={t('用户 ID')}
          value={filters.user_id}
          min={1}
          precision={0}
          onChange={(value) =>
            setFilters((prev) => ({ ...prev, user_id: value }))
          }
          style={{ width: 120 }}
        />
        <Select
          placeholder={t('订单状态')}
          optionList={statusOptions}
          value={filters.status}
          onChange={(value) =>
            setFilters((prev) => ({ ...prev, status: value }))
          }
          showClear
          style={{ width: 140 }}
        />
        <Input
          prefix={<IconSearch />}
          placeholder={t('订单号')}
          value={filters.keyword}
          onChange={(value) =>
            setFilters((prev) => ({ ...prev, keyword: value }))
          }
          onEnterPress={handleSearch}
          showClear
          style={{ width: 220 }}
        />
        <DatePicker
          type='dateTimeRange'
          value={filters.dateRange}
          onChange={(value) =>
            setFilters((prev) => ({ ...prev, dateRange: value }))
          }
          placeholder={[t('开始时间'), t('结束时间')]}
          showClear
          style={{ width: 320 }}
        />
        <Button type='primary' theme='solid' onClick={handleSearch}>
          {t('查询')}
        </Button>
        <Button icon={<IconRefresh />} onClick={handleReset}>
          {t('重置')}
        </Button>
      </div>

      <CardTable
        columns={columns}
        dataSource={orders}
        rowKey='id'
        loading={loading}
        scroll={{ x: 'max-content' }}
        hidePagination={false}
        pagination={{
          currentPage: page,
          pageSize,
          total,
          pageSizeOpts: [10, 20, 50, 100],
          showSizeChanger: true,
          onPageChange: handlePageChange,
          onPageSizeChange: handlePageSizeChange,
        }}
        empty={
          <Empty
            image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
            darkModeImage={
              <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
            }
            description={t('暂无订单记录')}
            style={{ padding: 30 }}
          />
        }
        size='small'
      />
    </div>
  );
};

export default OrdersPanel;
