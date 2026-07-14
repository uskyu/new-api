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
import React, { useState, useEffect, useMemo } from 'react';
import {
  Modal,
  Table,
  Badge,
  Typography,
  Toast,
  Empty,
  Button,
  Input,
  Tag,
  Tabs,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { Coins } from 'lucide-react';
import { IconSearch } from '@douyinfe/semi-icons';
import { API, renderQuota, timestamp2string } from '../../../helpers';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
const { Text } = Typography;

export const TOPUP_HISTORY_SCOPE = Object.freeze({
  SELF: 'self',
  USER: 'user',
  ALL: 'all',
});

const BILL_TYPE = Object.freeze({
  ONLINE: 'online',
  REDEMPTION: 'redemption',
});

// 状态映射配置
const STATUS_CONFIG = {
  success: { type: 'success', key: '成功' },
  pending: { type: 'warning', key: '待支付' },
  failed: { type: 'danger', key: '失败' },
  expired: { type: 'danger', key: '已过期' },
};

// 支付方式映射
const PAYMENT_METHOD_MAP = {
  stripe: 'Stripe',
  creem: 'Creem',
  waffo: 'Waffo',
  alipay: '支付宝',
  wxpay: '微信',
};

const TopupHistoryModal = ({
  visible,
  onCancel,
  t,
  scope = TOPUP_HISTORY_SCOPE.SELF,
  userId,
}) => {
  const [loading, setLoading] = useState(false);
  const [records, setRecords] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [keyword, setKeyword] = useState('');
  const [billType, setBillType] = useState(BILL_TYPE.ONLINE);
  const isMobile = useIsMobile();

  const loadBills = async (currentPage, currentPageSize) => {
    if (scope === TOPUP_HISTORY_SCOPE.USER && !userId) {
      setRecords([]);
      setTotal(0);
      return;
    }
    setLoading(true);
    try {
      const isRedemption = billType === BILL_TYPE.REDEMPTION;
      const base = isRedemption
        ? scope === TOPUP_HISTORY_SCOPE.ALL
          ? '/api/user/redemption'
          : scope === TOPUP_HISTORY_SCOPE.USER
            ? `/api/user/${userId}/redemptions`
            : '/api/user/redemption/self'
        : scope === TOPUP_HISTORY_SCOPE.ALL
          ? '/api/user/topup'
          : scope === TOPUP_HISTORY_SCOPE.USER
            ? `/api/user/${userId}/topups`
            : '/api/user/topup/self';
      const qs =
        `p=${currentPage}&page_size=${currentPageSize}` +
        (keyword ? `&keyword=${encodeURIComponent(keyword)}` : '');
      const endpoint = `${base}?${qs}`;
      const res = await API.get(endpoint);
      const { success, message, data } = res.data;
      if (success) {
        setRecords(data.items || []);
        setTotal(data.total || 0);
      } else {
        Toast.error({ content: message || t('加载失败') });
      }
    } catch (error) {
      Toast.error({ content: t('加载账单失败') });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (visible) {
      loadBills(page, pageSize);
    }
  }, [visible, page, pageSize, keyword, scope, userId, billType]);

  const handlePageChange = (currentPage) => {
    setPage(currentPage);
  };

  const handlePageSizeChange = (currentPageSize) => {
    setPageSize(currentPageSize);
    setPage(1);
  };

  const handleKeywordChange = (value) => {
    setKeyword(value);
    setPage(1);
  };

  const handleBillTypeChange = (nextBillType) => {
    setBillType(nextBillType);
    setKeyword('');
    setPage(1);
    setRecords([]);
    setTotal(0);
  };

  // 管理员补单
  const handleAdminComplete = async (tradeNo) => {
    try {
      const res = await API.post('/api/user/topup/complete', {
        trade_no: tradeNo,
      });
      const { success, message } = res.data;
      if (success) {
        Toast.success({ content: t('补单成功') });
        await loadBills(page, pageSize);
      } else {
        Toast.error({ content: message || t('补单失败') });
      }
    } catch (e) {
      Toast.error({ content: t('补单失败') });
    }
  };

  const confirmAdminComplete = (tradeNo) => {
    Modal.confirm({
      title: t('确认补单'),
      content: t('是否将该订单标记为成功并为用户入账？'),
      onOk: () => handleAdminComplete(tradeNo),
    });
  };

  // 渲染状态徽章
  const renderStatusBadge = (status) => {
    const config = STATUS_CONFIG[status] || { type: 'primary', key: status };
    return (
      <span className='flex items-center gap-2'>
        <Badge dot type={config.type} />
        <span>{t(config.key)}</span>
      </span>
    );
  };

  // 渲染支付方式
  const renderPaymentMethod = (pm) => {
    const displayName = PAYMENT_METHOD_MAP[pm];
    return <Text>{displayName ? t(displayName) : pm || '-'}</Text>;
  };

  const isSubscriptionTopup = (record) => {
    const tradeNo = (record?.trade_no || '').toLowerCase();
    return Number(record?.amount || 0) === 0 && tradeNo.startsWith('sub');
  };

  const showUserId = scope === TOPUP_HISTORY_SCOPE.ALL;
  const allowAdminActions = scope === TOPUP_HISTORY_SCOPE.ALL;

  const onlineColumns = useMemo(() => {
    const baseColumns = [
      ...(showUserId
        ? [
            {
              title: t('用户ID'),
              dataIndex: 'user_id',
              key: 'user_id',
              render: (userId) => <Text>{userId ?? '-'}</Text>,
            },
          ]
        : []),
      {
        title: t('订单号'),
        dataIndex: 'trade_no',
        key: 'trade_no',
        render: (text) => <Text copyable>{text}</Text>,
      },
      {
        title: t('支付方式'),
        dataIndex: 'payment_method',
        key: 'payment_method',
        render: renderPaymentMethod,
      },
      {
        title: t('充值额度'),
        dataIndex: 'amount',
        key: 'amount',
        render: (amount, record) => {
          if (isSubscriptionTopup(record)) {
            return (
              <Tag color='purple' shape='circle' size='small'>
                {t('订阅套餐')}
              </Tag>
            );
          }
          return (
            <span className='flex items-center gap-1'>
              <Coins size={16} />
              <Text>{amount}</Text>
            </span>
          );
        },
      },
      {
        title: t('支付金额'),
        dataIndex: 'money',
        key: 'money',
        render: (money) => <Text type='danger'>¥{money.toFixed(2)}</Text>,
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        key: 'status',
        render: renderStatusBadge,
      },
    ];

    // 管理员才显示操作列
    if (allowAdminActions) {
      baseColumns.push({
        title: t('操作'),
        key: 'action',
        render: (_, record) => {
          const actions = [];
          if (record.status === 'pending') {
            actions.push(
              <Button
                key="complete"
                size='small'
                type='primary'
                theme='outline'
                onClick={() => confirmAdminComplete(record.trade_no)}
              >
                {t('补单')}
              </Button>
            );
          }
          return actions.length > 0 ? <>{actions}</> : null;
        },
      });
    }

    baseColumns.push({
      title: t('创建时间'),
      dataIndex: 'create_time',
      key: 'create_time',
      render: (time) => timestamp2string(time),
    });

    return baseColumns;
  }, [t, showUserId, allowAdminActions]);

  const redemptionColumns = useMemo(
    () => [
      ...(showUserId
        ? [
            {
              title: t('用户ID'),
              dataIndex: 'user_id',
              key: 'user_id',
              render: (recordUserId) => <Text>{recordUserId ?? '-'}</Text>,
            },
          ]
        : []),
      {
        title: t('ID'),
        dataIndex: 'id',
        key: 'id',
        render: (id) => <Text>{id}</Text>,
      },
      {
        title: t('名称'),
        dataIndex: 'name',
        key: 'name',
        render: (name) => <Text>{name || '-'}</Text>,
      },
      {
        title: t('兑换码'),
        dataIndex: 'code',
        key: 'code',
        render: (code) => <Text>{code || '-'}</Text>,
      },
      {
        title: t('兑换额度'),
        dataIndex: 'quota',
        key: 'quota',
        render: (quota) => <Text>{renderQuota(Number(quota) || 0)}</Text>,
      },
      {
        title: t('状态'),
        key: 'status',
        render: () => (
          <span className='flex items-center gap-2'>
            <Badge dot type='success' />
            <span>{t('成功')}</span>
          </span>
        ),
      },
      {
        title: t('兑换时间'),
        dataIndex: 'redeemed_time',
        key: 'redeemed_time',
        render: (time) => timestamp2string(time),
      },
    ],
    [t, showUserId],
  );

  const columns =
    billType === BILL_TYPE.REDEMPTION
      ? redemptionColumns
      : onlineColumns;

  return (
    <Modal
      title={t('充值账单')}
      visible={visible}
      onCancel={onCancel}
      footer={null}
      size={isMobile ? 'full-width' : 'large'}
    >
      <Tabs
        activeKey={billType}
        onChange={handleBillTypeChange}
        type='button'
        className='mb-3'
      >
        <Tabs.TabPane tab={t('在线充值')} itemKey={BILL_TYPE.ONLINE} />
        <Tabs.TabPane tab={t('卡密充值')} itemKey={BILL_TYPE.REDEMPTION} />
      </Tabs>
      <div className='mb-3'>
        <Input
          prefix={<IconSearch />}
          placeholder={
            billType === BILL_TYPE.REDEMPTION
              ? t('关键字(ID、名称或兑换码)')
              : t('订单号')
          }
          value={keyword}
          onChange={handleKeywordChange}
          showClear
        />
      </div>
      <Table
        columns={columns}
        dataSource={records}
        loading={loading}
        rowKey='id'
        pagination={{
          currentPage: page,
          pageSize: pageSize,
          total: total,
          showSizeChanger: true,
          pageSizeOpts: [10, 20, 50, 100],
          onPageChange: handlePageChange,
          onPageSizeChange: handlePageSizeChange,
        }}
        size='small'
        empty={
          <Empty
            image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
            darkModeImage={
              <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
            }
            description={t('暂无充值记录')}
            style={{ padding: 30 }}
          />
        }
      />
    </Modal>
  );
};

export default TopupHistoryModal;
