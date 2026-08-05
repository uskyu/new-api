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
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Empty,
  Input,
  InputNumber,
  Modal,
  Pagination,
  Space,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  API,
  copy,
  renderQuota,
  renderQuotaWithAmount,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import { Trophy } from 'lucide-react';

const { Title, Text } = Typography;
const DEFAULT_PAGE_SIZE = 10;
const DAILY_METRIC_PAGE_SIZE = 10;

function getLocalDateString(offsetDays = 0) {
  const date = new Date();
  date.setDate(date.getDate() + offsetDays);
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, '0');
  const day = `${date.getDate()}`.padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function formatAmount(amount) {
  return renderQuotaWithAmount(Number(amount || 0) / 100);
}

function formatRate(rate) {
  return `${(Number(rate || 0) / 100).toFixed(2)}%`;
}

function formatRatio(ratio) {
  return `${(Number(ratio || 0) * 100).toFixed(1)}%`;
}

function MetricStat({ label, value }) {
  return (
    <div
      style={{
        border: '1px solid var(--semi-color-border)',
        borderRadius: 6,
        padding: 12,
        background: 'var(--semi-color-bg-0)',
      }}
    >
      <Text type='secondary'>{label}</Text>
      <Title heading={4} style={{ marginTop: 6, marginBottom: 0 }}>
        {value}
      </Title>
    </div>
  );
}

function getRebateSourceTag(sourceType, t) {
  if (sourceType === 'redemption') {
    return <Tag color='purple'>{t('兑换码')}</Tag>;
  }
  if (sourceType === 'epay') {
    return <Tag color='green'>{t('在线充值')}</Tag>;
  }
  if (sourceType === 'manual') {
    return <Tag color='blue'>{t('手工补单')}</Tag>;
  }
  return <Tag>{sourceType || '-'}</Tag>;
}

function renderRebateBase(record, t) {
  if (
    record?.source_type === 'redemption' &&
    Number(record?.redeem_quota || 0) > 0
  ) {
    return (
      <Space vertical align='start' spacing={0}>
        <Text>{formatAmount(record.pay_amount)}</Text>
        <Text type='secondary' size='small'>
          {t('兑换额度')} {renderQuota(record.redeem_quota)}
        </Text>
      </Space>
    );
  }
  return formatAmount(record?.pay_amount);
}

function getWithdrawStatusTag(status, t) {
  if (status === 'pending') {
    return <Tag color='orange'>{t('待处理')}</Tag>;
  }
  if (status === 'exported') {
    return <Tag color='blue'>{t('已导出')}</Tag>;
  }
  if (status === 'paid') {
    return <Tag color='green'>{t('已打款')}</Tag>;
  }
  return <Tag>{status || '-'}</Tag>;
}

export default function AgentCenter() {
  const { t } = useTranslation();
  const [summary, setSummary] = useState(null);
  const [summaryLoading, setSummaryLoading] = useState(false);
  const [dailyMetrics, setDailyMetrics] = useState(null);
  const [dailyMetricsLoading, setDailyMetricsLoading] = useState(false);
  const [dailyMetricStartDate, setDailyMetricStartDate] = useState(() =>
    getLocalDateString(-6),
  );
  const [dailyMetricEndDate, setDailyMetricEndDate] = useState(() =>
    getLocalDateString(0),
  );
  const [dailyMetricPage, setDailyMetricPage] = useState(1);
  const [leaderboard, setLeaderboard] = useState(null);
  const [leaderboardPage, setLeaderboardPage] = useState(1);
  const [leaderboardLoading, setLeaderboardLoading] = useState(false);
  const [rebates, setRebates] = useState([]);
  const [rebatesTotal, setRebatesTotal] = useState(0);
  const [rebatesPage, setRebatesPage] = useState(1);
  const [rebatesLoading, setRebatesLoading] = useState(false);
  const [adjustments, setAdjustments] = useState([]);
  const [adjustmentsTotal, setAdjustmentsTotal] = useState(0);
  const [adjustmentsPage, setAdjustmentsPage] = useState(1);
  const [adjustmentsLoading, setAdjustmentsLoading] = useState(false);
  const [promoLinks, setPromoLinks] = useState([]);
  const [promoLinksLoading, setPromoLinksLoading] = useState(false);
  const [promoLinkStats, setPromoLinkStats] = useState([]);
  const [promoLinkStatsLoading, setPromoLinkStatsLoading] = useState(false);
  const [promoLinkModalVisible, setPromoLinkModalVisible] = useState(false);
  const [promoLinkSubmitting, setPromoLinkSubmitting] = useState(false);
  const [promoLinkForm, setPromoLinkForm] = useState({
    name: '',
    code: '',
    remark: '',
  });
  const [downlines, setDownlines] = useState([]);
  const [downlinesTotal, setDownlinesTotal] = useState(0);
  const [downlinesPage, setDownlinesPage] = useState(1);
  const [downlineKeyword, setDownlineKeyword] = useState('');
  const [downlineSearchKeyword, setDownlineSearchKeyword] = useState('');
  const [downlinesLoading, setDownlinesLoading] = useState(false);
  const [upgradeModalVisible, setUpgradeModalVisible] = useState(false);
  const [upgradeSubmitting, setUpgradeSubmitting] = useState(false);
  const [upgradeForm, setUpgradeForm] = useState({
    targetUserId: 0,
    username: '',
    targetRatePercent: 0,
    remark: '',
  });
  const [withdrawRequests, setWithdrawRequests] = useState([]);
  const [withdrawRequestsTotal, setWithdrawRequestsTotal] = useState(0);
  const [withdrawRequestsPage, setWithdrawRequestsPage] = useState(1);
  const [withdrawRequestsLoading, setWithdrawRequestsLoading] = useState(false);
  const [withdrawModalVisible, setWithdrawModalVisible] = useState(false);
  const [withdrawSubmitting, setWithdrawSubmitting] = useState(false);
  const [withdrawForm, setWithdrawForm] = useState({
    accountName: '',
    accountNo: '',
    amount: '',
    remark: '',
  });

  const loadSummary = useCallback(async () => {
    setSummaryLoading(true);
    try {
      const res = await API.get('/api/agent/self');
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      setSummary(res.data.data);
    } catch (error) {
      showError(error.message || t('获取合作代理信息失败'));
    } finally {
      setSummaryLoading(false);
    }
  }, [t]);

  const loadDailyMetrics = useCallback(async () => {
    if (!summary?.is_agent) {
      setDailyMetrics(null);
      return;
    }
    setDailyMetricsLoading(true);
    setDailyMetricPage(1);
    try {
      const res = await API.get('/api/agent/self/daily-metrics', {
        params: {
          start_date: dailyMetricStartDate,
          end_date: dailyMetricEndDate,
        },
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      setDailyMetrics(res.data.data);
    } catch (error) {
      showError(error.message || t('获取每日数据失败'));
    } finally {
      setDailyMetricsLoading(false);
    }
  }, [dailyMetricEndDate, dailyMetricStartDate, summary?.is_agent, t]);

  const loadLeaderboard = useCallback(
    async (page = 1) => {
      if (!summary?.is_agent) {
        setLeaderboard(null);
        return;
      }
      setLeaderboardLoading(true);
      try {
        const res = await API.get('/api/agent/self/leaderboard', {
          params: {
            p: page,
            page_size: DEFAULT_PAGE_SIZE,
          },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        setLeaderboard(res.data.data);
        setLeaderboardPage(page);
      } catch (error) {
        showError(error.message || t('获取代理竞争排名失败'));
      } finally {
        setLeaderboardLoading(false);
      }
    },
    [summary?.is_agent, t],
  );

  const loadRebates = useCallback(
    async (page = rebatesPage) => {
      if (!summary?.is_agent) {
        setRebates([]);
        setRebatesTotal(0);
        return;
      }
      setRebatesLoading(true);
      try {
        const res = await API.get('/api/agent/self/rebates', {
          params: {
            p: page,
            page_size: DEFAULT_PAGE_SIZE,
          },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        setRebates(res.data.data?.items || []);
        setRebatesTotal(res.data.data?.total || 0);
      } catch (error) {
        showError(error.message || t('获取返利流水失败'));
      } finally {
        setRebatesLoading(false);
      }
    },
    [rebatesPage, summary?.is_agent, t],
  );

  const loadAdjustments = useCallback(
    async (page = adjustmentsPage) => {
      if (!summary?.is_agent) {
        setAdjustments([]);
        setAdjustmentsTotal(0);
        return;
      }
      setAdjustmentsLoading(true);
      try {
        const res = await API.get('/api/agent/self/adjustments', {
          params: {
            p: page,
            page_size: DEFAULT_PAGE_SIZE,
          },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        setAdjustments(res.data.data?.items || []);
        setAdjustmentsTotal(res.data.data?.total || 0);
      } catch (error) {
        showError(error.message || t('获取调账记录失败'));
      } finally {
        setAdjustmentsLoading(false);
      }
    },
    [adjustmentsPage, summary?.is_agent, t],
  );

  const loadPromoLinks = useCallback(async () => {
    if (!summary?.is_agent) {
      setPromoLinks([]);
      return;
    }
    setPromoLinksLoading(true);
    try {
      const res = await API.get('/api/agent/self/promo-links', {
        params: {
          p: 1,
          page_size: 20,
        },
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      setPromoLinks(res.data.data?.items || []);
    } catch (error) {
      showError(error.message || t('获取推广链接失败'));
    } finally {
      setPromoLinksLoading(false);
    }
  }, [summary?.is_agent, t]);

  const loadPromoLinkStats = useCallback(async () => {
    if (!summary?.is_agent) {
      setPromoLinkStats([]);
      return;
    }
    setPromoLinkStatsLoading(true);
    try {
      const res = await API.get('/api/agent/self/promo-link-stats');
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      setPromoLinkStats(res.data.data || []);
    } catch (error) {
      showError(error.message || t('获取渠道统计失败'));
    } finally {
      setPromoLinkStatsLoading(false);
    }
  }, [summary?.is_agent, t]);

  const loadDownlines = useCallback(
    async (page = 1, keyword = downlineSearchKeyword) => {
      if (!summary?.is_agent) {
        setDownlines([]);
        setDownlinesTotal(0);
        return;
      }
      setDownlinesLoading(true);
      try {
        const res = await API.get('/api/agent/self/downlines', {
          params: {
            p: page,
            page_size: DEFAULT_PAGE_SIZE,
            keyword: keyword.trim(),
          },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        setDownlines(res.data.data?.items || []);
        setDownlinesTotal(res.data.data?.total || 0);
        setDownlinesPage(page);
      } catch (error) {
        showError(error.message || t('获取直属下级失败'));
      } finally {
        setDownlinesLoading(false);
      }
    },
    [downlineSearchKeyword, summary?.is_agent, t],
  );

  const loadWithdrawRequests = useCallback(
    async (page = withdrawRequestsPage) => {
      if (!summary?.is_agent) {
        setWithdrawRequests([]);
        setWithdrawRequestsTotal(0);
        return;
      }
      setWithdrawRequestsLoading(true);
      try {
        const res = await API.get('/api/agent/self/withdraw-requests', {
          params: {
            p: page,
            page_size: DEFAULT_PAGE_SIZE,
          },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        setWithdrawRequests(res.data.data?.items || []);
        setWithdrawRequestsTotal(res.data.data?.total || 0);
      } catch (error) {
        showError(error.message || t('获取提现记录失败'));
      } finally {
        setWithdrawRequestsLoading(false);
      }
    },
    [summary?.is_agent, t, withdrawRequestsPage],
  );

  useEffect(() => {
    loadSummary();
  }, [loadSummary]);

  useEffect(() => {
    if (summary?.is_agent) {
      loadLeaderboard();
      loadDailyMetrics();
      loadRebates(1);
      loadAdjustments(1);
      loadPromoLinks();
      loadPromoLinkStats();
      loadDownlines();
      loadWithdrawRequests(1);
    }
  }, [summary?.is_agent]);

  const refreshAll = useCallback(async () => {
    await loadSummary();
    if (summary?.is_agent) {
      await loadLeaderboard();
      await loadDailyMetrics();
      await loadRebates(1);
      await loadAdjustments(1);
      await loadPromoLinks();
      await loadPromoLinkStats();
      await loadDownlines();
      await loadWithdrawRequests(1);
    }
  }, [
    loadAdjustments,
    loadDailyMetrics,
    loadDownlines,
    loadLeaderboard,
    loadPromoLinkStats,
    loadPromoLinks,
    loadRebates,
    loadSummary,
    loadWithdrawRequests,
    summary?.is_agent,
  ]);

  const handleSearchDownlines = useCallback(() => {
    const keyword = downlineKeyword.trim();
    setDownlineSearchKeyword(keyword);
    loadDownlines(1, keyword);
  }, [downlineKeyword, loadDownlines]);

  const handleResetDownlines = useCallback(() => {
    setDownlineKeyword('');
    setDownlineSearchKeyword('');
    loadDownlines(1, '');
  }, [loadDownlines]);

  const handleCopyPromoLink = async (record) => {
    const landingPath = record.landing_page || '/';
    const url = `${window.location.origin}${landingPath.startsWith('/') ? landingPath : `/${landingPath}`}?aff=${record.code}`;
    const ok = await copy(url);
    if (ok) {
      showSuccess(t('推广链接已复制到剪贴板'));
      return;
    }
    showError(t('复制失败，请手动复制'));
  };

  const handleCreatePromoLink = async () => {
    setPromoLinkSubmitting(true);
    try {
      const res = await API.post('/api/agent/self/promo-link', {
        name: promoLinkForm.name,
        code: promoLinkForm.code,
        status: 1,
        landing_page: '/',
        remark: promoLinkForm.remark,
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('推广链接已创建'));
      setPromoLinkModalVisible(false);
      setPromoLinkForm({ name: '', code: '', remark: '' });
      await loadPromoLinks();
      await loadPromoLinkStats();
    } catch (error) {
      showError(error.message || t('创建推广链接失败'));
    } finally {
      setPromoLinkSubmitting(false);
    }
  };

  const handleDeletePromoLink = (record) => {
    Modal.confirm({
      title: t('确认删除推广链接？'),
      content: `${record.name} (${record.code})`,
      onOk: async () => {
        const res = await API.delete(`/api/agent/self/promo-link/${record.id}`);
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        showSuccess(t('推广链接已删除'));
        await loadPromoLinks();
        await loadPromoLinkStats();
      },
    });
  };

  const handleOpenUpgrade = (record) => {
    setUpgradeForm({
      targetUserId: record.user_id,
      username: record.username,
      targetRatePercent: 0,
      remark: '',
    });
    setUpgradeModalVisible(true);
  };

  const handleSubmitUpgrade = async () => {
    setUpgradeSubmitting(true);
    try {
      const res = await API.post('/api/agent/self/upgrade-request', {
        target_user_id: upgradeForm.targetUserId,
        target_rate: Math.round(
          Number(upgradeForm.targetRatePercent || 0) * 100,
        ),
        remark: upgradeForm.remark,
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('已成功开通代理'));
      setUpgradeModalVisible(false);
      await loadDownlines();
    } catch (error) {
      showError(error.message || t('提交代理开通申请失败'));
    } finally {
      setUpgradeSubmitting(false);
    }
  };

  const handleOpenWithdraw = () => {
    setWithdrawForm({
      accountName: summary?.withdraw_account?.account_name || '',
      accountNo: summary?.withdraw_account?.account_no || '',
      amount: '',
      remark: '',
    });
    setWithdrawModalVisible(true);
  };

  const handleSubmitWithdraw = async () => {
    setWithdrawSubmitting(true);
    try {
      const res = await API.post('/api/agent/self/withdraw-request', {
        account_name: withdrawForm.accountName,
        account_no: withdrawForm.accountNo,
        amount: withdrawForm.amount,
        remark: withdrawForm.remark,
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('提现申请已提交'));
      setWithdrawModalVisible(false);
      await loadSummary();
      await loadWithdrawRequests(1);
    } catch (error) {
      showError(error.message || t('提交提现申请失败'));
    } finally {
      setWithdrawSubmitting(false);
    }
  };

  const dailyMetricItems = dailyMetrics?.items || [];
  const pagedDailyMetricItems = useMemo(
    () =>
      dailyMetricItems.slice(
        (dailyMetricPage - 1) * DAILY_METRIC_PAGE_SIZE,
        dailyMetricPage * DAILY_METRIC_PAGE_SIZE,
      ),
    [dailyMetricItems, dailyMetricPage],
  );

  const dailyMetricColumns = useMemo(
    () => [
      { title: t('日期'), dataIndex: 'date' },
      { title: t('新增用户'), dataIndex: 'new_user_count' },
      { title: t('充值笔数'), dataIndex: 'topup_count' },
      {
        title: t('充值金额'),
        dataIndex: 'topup_amount',
        render: (_, record) => formatAmount(record.topup_amount),
      },
      {
        title: t('返利合计'),
        dataIndex: 'total_rebate_amount',
        render: (_, record) => formatAmount(record.total_rebate_amount),
      },
    ],
    [t],
  );

  const leaderboardColumns = useMemo(
    () => [
      {
        title: t('排名'),
        dataIndex: 'rank',
        width: 80,
        render: (rank) =>
          rank <= 3 ? (
            <Tag color={rank === 1 ? 'amber' : rank === 2 ? 'grey' : 'orange'}>
              #{rank}
            </Tag>
          ) : (
            `#${rank}`
          ),
      },
      {
        title: t('代理'),
        dataIndex: 'agent_label',
        render: (_, record) => (
          <Space spacing={6}>
            <Text>{record.is_self ? t('我的代理') : record.agent_label}</Text>
            {record.is_self ? <Tag color='blue'>{t('本人')}</Tag> : null}
          </Space>
        ),
      },
      { title: t('今日新增'), dataIndex: 'day_new_user_count', width: 110 },
      { title: t('本月新增'), dataIndex: 'month_new_user_count', width: 110 },
      {
        title: t('今日在线充值'),
        dataIndex: 'day_topup_amount',
        width: 150,
        render: (value) => formatAmount(value),
      },
      {
        title: t('本月二次充值率'),
        dataIndex: 'month_repurchase_rate',
        width: 150,
        render: (value) => formatRatio(value),
      },
    ],
    [t],
  );

  const rebateColumns = useMemo(
    () => [
      { title: t('订单号'), dataIndex: 'trade_no' },
      {
        title: t('来源'),
        dataIndex: 'source_type',
        render: (_, record) => getRebateSourceTag(record.source_type, t),
      },
      { title: t('下级用户ID'), dataIndex: 'invitee_user_id' },
      {
        title: t('结算基数'),
        dataIndex: 'pay_amount',
        render: (_, record) => renderRebateBase(record, t),
      },
      {
        title: t('返利比例'),
        dataIndex: 'rebate_rate',
        render: (_, record) => formatRate(record.rebate_rate),
      },
      {
        title: t('返利金额'),
        dataIndex: 'rebate_amount',
        render: (_, record) => formatAmount(record.rebate_amount),
      },
      {
        title: t('结算时间'),
        dataIndex: 'settled_at',
        render: (_, record) =>
          timestamp2string(record.settled_at || record.created_at),
      },
    ],
    [t],
  );

  const adjustmentColumns = useMemo(
    () => [
      {
        title: t('类型'),
        dataIndex: 'change_type',
        render: (_, record) =>
          record.change_type === 'increase' ? (
            <Tag color='green'>{t('增加')}</Tag>
          ) : (
            <Tag color='red'>{t('减少')}</Tag>
          ),
      },
      {
        title: t('调整金额'),
        dataIndex: 'delta_amount',
        render: (_, record) => formatAmount(record.delta_amount),
      },
      {
        title: t('调整后余额'),
        dataIndex: 'balance_after',
        render: (_, record) => formatAmount(record.balance_after),
      },
      { title: t('原因'), dataIndex: 'reason' },
      {
        title: t('时间'),
        dataIndex: 'created_at',
        render: (_, record) => timestamp2string(record.created_at),
      },
    ],
    [t],
  );

  const promoLinkColumns = useMemo(
    () => [
      { title: t('渠道名称'), dataIndex: 'name' },
      { title: t('推广码'), dataIndex: 'code' },
      {
        title: t('落地页'),
        dataIndex: 'landing_page',
        render: (_, record) => record.landing_page || '/',
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (_, record) =>
          record.status === 1 ? (
            <Tag color='green'>{t('启用')}</Tag>
          ) : (
            <Tag>{t('禁用')}</Tag>
          ),
      },
      {
        title: t('操作'),
        dataIndex: 'operate',
        render: (_, record) => (
          <Space>
            <a onClick={() => handleCopyPromoLink(record)}>{t('复制链接')}</a>
            <a onClick={() => handleDeletePromoLink(record)}>{t('删除')}</a>
          </Space>
        ),
      },
    ],
    [t],
  );

  const promoLinkStatColumns = useMemo(
    () => [
      { title: t('渠道名称'), dataIndex: 'name' },
      { title: t('注册人数'), dataIndex: 'invitee_count' },
      { title: t('充值笔数'), dataIndex: 'topup_count' },
      {
        title: t('充值金额'),
        dataIndex: 'topup_amount',
        render: (_, record) => formatAmount(record.topup_amount),
      },
      {
        title: t('返利金额(含兑换码)'),
        dataIndex: 'rebate_amount',
        render: (_, record) => formatAmount(record.rebate_amount),
      },
    ],
    [t],
  );

  const downlineColumns = useMemo(
    () => [
      { title: t('用户ID'), dataIndex: 'user_id' },
      { title: t('用户名'), dataIndex: 'username' },
      { title: t('显示名称'), dataIndex: 'display_name' },
      { title: t('来源渠道'), dataIndex: 'promo_link_name' },
      {
        title: t('充值金额'),
        dataIndex: 'topup_amount',
        render: (_, record) => formatAmount(record.topup_amount),
      },
      {
        title: t('当前余额'),
        dataIndex: 'balance_quota',
        render: (_, record) => renderQuota(record.balance_quota || 0),
      },
      {
        title: t('是否代理'),
        dataIndex: 'is_agent',
        render: (_, record) =>
          record.is_agent ? (
            <Tag color='blue'>{t('是')}</Tag>
          ) : (
            <Tag>{t('否')}</Tag>
          ),
      },
      {
        title: t('操作'),
        dataIndex: 'operate',
        render: (_, record) =>
          record.is_agent ? (
            <Text type='secondary'>{t('已是代理')}</Text>
          ) : (
            <a onClick={() => handleOpenUpgrade(record)}>{t('立即开通代理')}</a>
          ),
      },
    ],
    [t],
  );

  const withdrawColumns = useMemo(
    () => [
      { title: t('申请单ID'), dataIndex: 'id' },
      {
        title: t('金额'),
        dataIndex: 'amount',
        render: (_, record) => formatAmount(record.amount),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (_, record) => getWithdrawStatusTag(record.status, t),
      },
      { title: t('支付宝账号'), dataIndex: 'account_no_snapshot' },
      { title: t('姓名'), dataIndex: 'account_name_snapshot' },
      { title: t('批次号'), dataIndex: 'export_batch_no' },
      { title: t('打款订单号'), dataIndex: 'external_order_no' },
      {
        title: t('申请时间'),
        dataIndex: 'created_at',
        render: (_, record) => timestamp2string(record.created_at),
      },
    ],
    [t],
  );

  const leaderboardItems = leaderboard?.items || [];
  const selfOutsideLeaderboard =
    leaderboard?.self && !leaderboardItems.some((item) => item.is_self)
      ? leaderboard.self
      : null;

  return (
    <div className='mt-[60px] px-2'>
      <Space vertical align='start' style={{ width: '100%' }} spacing={16}>
        <Card style={{ width: '100%' }} loading={summaryLoading}>
          <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
            <div className='flex flex-col md:flex-row md:justify-between md:items-center w-full gap-3'>
              <div>
                <Title heading={4} style={{ margin: 0 }}>
                  {t('合作代理')}
                </Title>
                <Text type='secondary'>
                  {t(
                    '这里展示独立的代理返利余额与流水，后续提现也将基于此账本。',
                  )}
                </Text>
              </div>
              <Button onClick={refreshAll}>{t('刷新')}</Button>
            </div>

            {!summary?.agent_initialized ? (
              <Empty
                title={t('代理功能尚未启用')}
                description={t(
                  '管理员完成初始化后，您才可以使用合作代理中心。',
                )}
              />
            ) : !summary?.is_agent ? (
              <Empty
                title={t('您当前还不是代理')}
                description={t(
                  '管理员为您开通代理资料后，这里会显示返利余额和流水。',
                )}
              />
            ) : (
              <div className='grid grid-cols-1 md:grid-cols-5 gap-4 w-full'>
                <Card>
                  <Text type='secondary'>{t('返利余额')}</Text>
                  <Title heading={3} style={{ margin: '8px 0 0' }}>
                    {formatAmount(summary?.profile?.rebate_balance_amount)}
                  </Title>
                  <Button
                    type='primary'
                    style={{ marginTop: 12 }}
                    onClick={handleOpenWithdraw}
                  >
                    {t('提现')}
                  </Button>
                </Card>
                <Card>
                  <Text type='secondary'>{t('冻结金额')}</Text>
                  <Title heading={3} style={{ margin: '8px 0 0' }}>
                    {formatAmount(summary?.profile?.rebate_frozen_amount)}
                  </Title>
                </Card>
                <Card>
                  <Text type='secondary'>{t('累计返利')}</Text>
                  <Title heading={3} style={{ margin: '8px 0 0' }}>
                    {formatAmount(summary?.profile?.rebate_total_amount)}
                  </Title>
                </Card>
                <Card>
                  <Text type='secondary'>{t('当前比例')}</Text>
                  <Title heading={3} style={{ margin: '8px 0 0' }}>
                    {formatRate(summary?.profile?.effective_rate)}
                  </Title>
                </Card>
                <Card>
                  <Text type='secondary'>{t('返利笔数')}</Text>
                  <Title heading={3} style={{ margin: '8px 0 0' }}>
                    {summary?.recent_rebate_count || 0}
                  </Title>
                </Card>
              </div>
            )}
          </Space>
        </Card>

        {summary?.is_agent && (
          <>
            <Card style={{ width: '100%' }} loading={leaderboardLoading}>
              <Space
                vertical
                align='start'
                style={{ width: '100%' }}
                spacing={12}
              >
                <div className='flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between w-full'>
                  <div className='flex items-center gap-2'>
                    <Trophy size={18} />
                    <Title heading={5} style={{ margin: 0 }}>
                      {t('代理竞争排名')}
                    </Title>
                  </div>
                  <Text type='secondary' size='small'>
                    {t('按本月新增用户优先排名，代理身份已脱敏')}
                  </Text>
                </div>
                {selfOutsideLeaderboard ? (
                  <div className='grid w-full grid-cols-2 gap-3 border border-solid border-[var(--semi-color-primary-light-default)] bg-[var(--semi-color-primary-light-default)] p-3 md:grid-cols-5'>
                    <MetricStat
                      label={t('我的排名')}
                      value={`#${selfOutsideLeaderboard.rank}`}
                    />
                    <MetricStat
                      label={t('今日新增')}
                      value={selfOutsideLeaderboard.day_new_user_count || 0}
                    />
                    <MetricStat
                      label={t('本月新增')}
                      value={selfOutsideLeaderboard.month_new_user_count || 0}
                    />
                    <MetricStat
                      label={t('今日在线充值')}
                      value={formatAmount(
                        selfOutsideLeaderboard.day_topup_amount,
                      )}
                    />
                    <MetricStat
                      label={t('本月二次充值率')}
                      value={formatRatio(
                        selfOutsideLeaderboard.month_repurchase_rate,
                      )}
                    />
                  </div>
                ) : null}
                <Table
                  rowKey='rank'
                  columns={leaderboardColumns}
                  dataSource={leaderboardItems}
                  pagination={false}
                  scroll={{ x: 860 }}
                  empty={<Empty title={t('暂无代理排名数据')} />}
                />
                <Pagination
                  currentPage={leaderboardPage}
                  pageSize={DEFAULT_PAGE_SIZE}
                  total={Number(leaderboard?.total || 0)}
                  onPageChange={(page) => loadLeaderboard(page)}
                />
                <Text type='tertiary' size='small'>
                  {t(
                    '今日充值和本月二次充值率仅统计已结算的在线充值，不包含卡密兑换。',
                  )}
                </Text>
              </Space>
            </Card>

            <Card style={{ width: '100%' }} loading={dailyMetricsLoading}>
              <Space
                vertical
                align='start'
                style={{ width: '100%' }}
                spacing={12}
              >
                <div className='flex flex-col md:flex-row md:justify-between md:items-center w-full gap-3'>
                  <Title heading={5} style={{ margin: 0 }}>
                    {t('每日邀请与充值')}
                  </Title>
                  <Space wrap>
                    <Input
                      placeholder={t('开始日期')}
                      value={dailyMetricStartDate}
                      onChange={setDailyMetricStartDate}
                      style={{ width: 140 }}
                    />
                    <Input
                      placeholder={t('结束日期')}
                      value={dailyMetricEndDate}
                      onChange={setDailyMetricEndDate}
                      style={{ width: 140 }}
                    />
                    <Button onClick={loadDailyMetrics}>{t('查询')}</Button>
                  </Space>
                </div>
                <div className='grid grid-cols-1 md:grid-cols-4 gap-4 w-full'>
                  <MetricStat
                    label={t('区间新增用户')}
                    value={dailyMetrics?.summary?.new_user_count || 0}
                  />
                  <MetricStat
                    label={t('区间充值笔数')}
                    value={dailyMetrics?.summary?.topup_count || 0}
                  />
                  <MetricStat
                    label={t('区间充值金额')}
                    value={formatAmount(dailyMetrics?.summary?.topup_amount)}
                  />
                  <MetricStat
                    label={t('区间返利合计')}
                    value={formatAmount(
                      dailyMetrics?.summary?.total_rebate_amount,
                    )}
                  />
                </div>
                <Table
                  rowKey={(record) => `${record.date}-${record.agent_user_id}`}
                  columns={dailyMetricColumns}
                  dataSource={pagedDailyMetricItems}
                  pagination={false}
                  empty={<Empty title={t('暂无每日数据')} />}
                />
                <Pagination
                  currentPage={dailyMetricPage}
                  pageSize={DAILY_METRIC_PAGE_SIZE}
                  total={dailyMetricItems.length}
                  onPageChange={setDailyMetricPage}
                  showTotal
                />
              </Space>
            </Card>

            <Card style={{ width: '100%' }}>
              <Space
                vertical
                align='start'
                style={{ width: '100%' }}
                spacing={12}
              >
                <div className='flex flex-col md:flex-row md:justify-between md:items-center w-full gap-3'>
                  <Title heading={5} style={{ margin: 0 }}>
                    {t('我的推广链接')}
                  </Title>
                  <Button
                    type='primary'
                    onClick={() => setPromoLinkModalVisible(true)}
                  >
                    {t('新增推广链接')}
                  </Button>
                </div>
                <Table
                  rowKey='id'
                  columns={promoLinkColumns}
                  dataSource={promoLinks}
                  loading={promoLinksLoading}
                  pagination={false}
                  empty={
                    <Empty
                      title={t('暂无推广链接')}
                      description={t(
                        '管理员为您创建推广链接后，会显示在这里。',
                      )}
                    />
                  }
                />
              </Space>
            </Card>

            <Card style={{ width: '100%' }}>
              <Space
                vertical
                align='start'
                style={{ width: '100%' }}
                spacing={12}
              >
                <Title heading={5} style={{ margin: 0 }}>
                  {t('渠道统计')}
                </Title>
                <Table
                  rowKey='promo_link_id'
                  columns={promoLinkStatColumns}
                  dataSource={promoLinkStats}
                  loading={promoLinkStatsLoading}
                  pagination={false}
                  empty={<Empty title={t('暂无渠道统计')} />}
                />
              </Space>
            </Card>

            <Card style={{ width: '100%' }}>
              <Space
                vertical
                align='start'
                style={{ width: '100%' }}
                spacing={12}
              >
                <div className='flex w-full flex-col gap-2 md:flex-row md:items-center md:justify-between'>
                  <Title heading={5} style={{ margin: 0 }}>
                    {t('我的直属下级')}
                  </Title>
                  <div className='flex w-full flex-col gap-2 sm:flex-row md:w-auto'>
                    <Input
                      value={downlineKeyword}
                      placeholder={t('搜索用户 ID、用户名或显示名称')}
                      onChange={setDownlineKeyword}
                      onEnterPress={handleSearchDownlines}
                      showClear
                      style={{ width: 260, maxWidth: '100%' }}
                    />
                    <Button type='primary' onClick={handleSearchDownlines}>
                      {t('查询')}
                    </Button>
                    <Button theme='light' onClick={handleResetDownlines}>
                      {t('重置')}
                    </Button>
                  </div>
                </div>
                <Table
                  rowKey='user_id'
                  columns={downlineColumns}
                  dataSource={downlines}
                  loading={downlinesLoading}
                  pagination={false}
                  scroll={{ x: 980 }}
                  empty={<Empty title={t('暂无直属下级')} />}
                />
                <Pagination
                  currentPage={downlinesPage}
                  pageSize={DEFAULT_PAGE_SIZE}
                  total={downlinesTotal}
                  onPageChange={(page) => loadDownlines(page)}
                  showTotal
                />
              </Space>
            </Card>

            <Card style={{ width: '100%' }}>
              <Space
                vertical
                align='start'
                style={{ width: '100%' }}
                spacing={12}
              >
                <Title heading={5} style={{ margin: 0 }}>
                  {t('返利流水')}
                </Title>
                <Table
                  rowKey='record_key'
                  columns={rebateColumns}
                  dataSource={rebates}
                  loading={rebatesLoading}
                  pagination={false}
                  empty={<Empty title={t('暂无返利流水')} />}
                />
                <Pagination
                  currentPage={rebatesPage}
                  pageSize={DEFAULT_PAGE_SIZE}
                  total={rebatesTotal}
                  onPageChange={(page) => {
                    setRebatesPage(page);
                    loadRebates(page);
                  }}
                  showTotal
                />
              </Space>
            </Card>

            <Card style={{ width: '100%' }}>
              <Space
                vertical
                align='start'
                style={{ width: '100%' }}
                spacing={12}
              >
                <Title heading={5} style={{ margin: 0 }}>
                  {t('人工调账记录')}
                </Title>
                <Table
                  rowKey='id'
                  columns={adjustmentColumns}
                  dataSource={adjustments}
                  loading={adjustmentsLoading}
                  pagination={false}
                  empty={<Empty title={t('暂无调账记录')} />}
                />
                <Pagination
                  currentPage={adjustmentsPage}
                  pageSize={DEFAULT_PAGE_SIZE}
                  total={adjustmentsTotal}
                  onPageChange={(page) => {
                    setAdjustmentsPage(page);
                    loadAdjustments(page);
                  }}
                  showTotal
                />
              </Space>
            </Card>

            <Card style={{ width: '100%' }}>
              <Space
                vertical
                align='start'
                style={{ width: '100%' }}
                spacing={12}
              >
                <Title heading={5} style={{ margin: 0 }}>
                  {t('提现记录')}
                </Title>
                <Table
                  rowKey='id'
                  columns={withdrawColumns}
                  dataSource={withdrawRequests}
                  loading={withdrawRequestsLoading}
                  pagination={false}
                  empty={<Empty title={t('暂无提现记录')} />}
                />
                <Pagination
                  currentPage={withdrawRequestsPage}
                  pageSize={DEFAULT_PAGE_SIZE}
                  total={withdrawRequestsTotal}
                  onPageChange={(page) => {
                    setWithdrawRequestsPage(page);
                    loadWithdrawRequests(page);
                  }}
                  showTotal
                />
              </Space>
            </Card>
          </>
        )}
      </Space>

      <Modal
        title={t('新增推广链接')}
        visible={promoLinkModalVisible}
        onCancel={() => setPromoLinkModalVisible(false)}
        onOk={handleCreatePromoLink}
        confirmLoading={promoLinkSubmitting}
      >
        <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
          <Input
            placeholder={t('渠道名称')}
            value={promoLinkForm.name}
            onChange={(value) =>
              setPromoLinkForm((prev) => ({ ...prev, name: value }))
            }
          />
          <Input
            placeholder={t('推广码，留空自动生成')}
            value={promoLinkForm.code}
            onChange={(value) =>
              setPromoLinkForm((prev) => ({ ...prev, code: value }))
            }
          />
          <Input
            placeholder={t('备注')}
            value={promoLinkForm.remark}
            onChange={(value) =>
              setPromoLinkForm((prev) => ({ ...prev, remark: value }))
            }
          />
        </Space>
      </Modal>

      <Modal
        title={t('申请提现')}
        visible={withdrawModalVisible}
        onCancel={() => setWithdrawModalVisible(false)}
        onOk={handleSubmitWithdraw}
        confirmLoading={withdrawSubmitting}
      >
        <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
          <Input
            placeholder={t('姓名')}
            value={withdrawForm.accountName}
            onChange={(value) =>
              setWithdrawForm((prev) => ({ ...prev, accountName: value }))
            }
          />
          <Input
            placeholder={t('支付宝账号')}
            value={withdrawForm.accountNo}
            onChange={(value) =>
              setWithdrawForm((prev) => ({ ...prev, accountNo: value }))
            }
          />
          <Input
            placeholder={t('提现金额')}
            value={withdrawForm.amount}
            onChange={(value) =>
              setWithdrawForm((prev) => ({ ...prev, amount: value }))
            }
          />
          <Input
            placeholder={t('备注')}
            value={withdrawForm.remark}
            onChange={(value) =>
              setWithdrawForm((prev) => ({ ...prev, remark: value }))
            }
          />
        </Space>
      </Modal>

      <Modal
        title={t('立即开通代理')}
        visible={upgradeModalVisible}
        onCancel={() => setUpgradeModalVisible(false)}
        onOk={handleSubmitUpgrade}
        confirmLoading={upgradeSubmitting}
      >
        <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
          <Text>
            {t('目标用户：')}
            {upgradeForm.username}
          </Text>
          <InputNumber
            min={0}
            step={0.1}
            suffix='%'
            value={upgradeForm.targetRatePercent}
            onChange={(value) =>
              setUpgradeForm((prev) => ({
                ...prev,
                targetRatePercent: value || 0,
              }))
            }
            style={{ width: '100%' }}
          />
          <Input
            placeholder={t('备注')}
            value={upgradeForm.remark}
            onChange={(value) =>
              setUpgradeForm((prev) => ({ ...prev, remark: value }))
            }
          />
        </Space>
      </Modal>
    </div>
  );
}
