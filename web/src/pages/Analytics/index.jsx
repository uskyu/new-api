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
  Button,
  Card,
  DatePicker,
  Empty,
  Input,
  InputNumber,
  Pagination,
  Select,
  Spin,
  Table,
  TabPane,
  Tabs,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import dayjs from 'dayjs';
import {
  Activity,
  BarChart3,
  CreditCard,
  RefreshCw,
  Repeat2,
  UserCheck,
  Users,
  WalletCards,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, renderNumber, renderQuota, showError } from '../../helpers';
import { CHART_CONFIG } from '../../constants/dashboard.constants';

const { Text, Title } = Typography;

const RANGE_OPTIONS = [
  { key: 'today', label: '今日' },
  { key: 'yesterday', label: '昨日' },
  { key: 'week', label: '本周' },
  { key: 'month', label: '本月' },
  { key: 'custom', label: '自定义' },
];

const INACTIVE_DAY_OPTIONS = [
  { value: 7, label: '7 天' },
  { value: 30, label: '30 天' },
  { value: 60, label: '2 个月' },
  { value: 90, label: '3 个月' },
  { value: 180, label: '6 个月' },
  { value: 'custom', label: '自定义' },
];

const INACTIVE_PAGE_SIZE = 10;

const chartColors = [
  '#1664FF',
  '#3CC780',
  '#FF8A00',
  '#7442D4',
  '#009488',
  '#D64B7D',
  '#304D77',
  '#1AC6FF',
];

const formatMoney = (value) =>
  `¥${Number(value || 0).toLocaleString(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;

const formatPercent = (value) => `${(Number(value || 0) * 100).toFixed(1)}%`;

const formatSeconds = (value) => `${Number(value || 0).toFixed(2)}s`;

const formatRangePoint = (timestamp) => {
  if (!timestamp) return '';
  return dayjs.unix(timestamp).format('MM-DD HH:mm');
};

const metricValue = (metrics, key) => Number(metrics?.[key] || 0);

const Analytics = () => {
  const { t } = useTranslation();
  const [rangeKey, setRangeKey] = useState('today');
  const [customRange, setCustomRange] = useState([]);
  const [loading, setLoading] = useState(false);
  const [overview, setOverview] = useState({});
  const [distributionType, setDistributionType] = useState('models');
  const [rankingType, setRankingType] = useState('consumption');
  const [usageType, setUsageType] = useState('models');
  const [inactiveRange, setInactiveRange] = useState(30);
  const [inactiveCustomDays, setInactiveCustomDays] = useState(30);
  const [inactiveAccountType, setInactiveAccountType] = useState('all');
  const [inactiveKeyword, setInactiveKeyword] = useState('');
  const [inactivePage, setInactivePage] = useState(1);
  const [inactiveLoading, setInactiveLoading] = useState(false);
  const [inactiveResult, setInactiveResult] = useState(null);

  useEffect(() => {
    initVChartSemiTheme({ isWatchingThemeSwitch: true });
  }, []);

  const loadOverview = useCallback(async () => {
    if (rangeKey === 'custom' && customRange?.length !== 2) {
      return;
    }
    setLoading(true);
    try {
      const params = { range: rangeKey, limit: 10 };
      if (rangeKey === 'custom' && customRange?.length === 2) {
        params.start = dayjs(customRange[0]).startOf('day').unix();
        params.end = dayjs(customRange[1]).endOf('day').unix();
      }
      const res = await API.get('/api/admin/analytics/overview', { params });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('加载数据分析失败'));
        return;
      }
      setOverview(data || {});
    } catch (error) {
      showError(
        error?.response?.data?.message ||
          error?.message ||
          t('加载数据分析失败'),
      );
    } finally {
      setLoading(false);
    }
  }, [customRange, rangeKey, t]);

  useEffect(() => {
    loadOverview();
  }, [loadOverview]);

  const loadInactiveUsers = useCallback(
    async (page = inactivePage) => {
      const days =
        inactiveRange === 'custom'
          ? Number(inactiveCustomDays || 0)
          : Number(inactiveRange);
      if (days < 1 || days > 3650) {
        showError(t('未调用天数必须在 1 到 3650 之间'));
        return;
      }
      setInactiveLoading(true);
      try {
        const res = await API.get('/api/admin/analytics/inactive-users', {
          params: {
            days,
            account_type: inactiveAccountType,
            keyword: inactiveKeyword.trim(),
            p: page,
            page_size: INACTIVE_PAGE_SIZE,
          },
        });
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message || t('加载僵尸用户失败'));
          return;
        }
        setInactivePage(page);
        setInactiveResult(data || null);
      } catch (error) {
        showError(
          error?.response?.data?.message ||
            error?.message ||
            t('加载僵尸用户失败'),
        );
      } finally {
        setInactiveLoading(false);
      }
    },
    [
      inactiveAccountType,
      inactiveCustomDays,
      inactiveKeyword,
      inactivePage,
      inactiveRange,
      t,
    ],
  );

  useEffect(() => {
    loadInactiveUsers(1);
  }, [inactiveRange, inactiveAccountType]);

  const metrics = overview.metrics || {};
  const distributions = overview.distributions || {};
  const rankings = overview.rankings || {};
  const trends = overview.trends || [];

  const metricCards = useMemo(
    () => [
      {
        key: 'topup',
        title: t('成功充值金额'),
        value: formatMoney(metricValue(metrics, 'successful_topup_amount')),
        sub: `${t('订单')} ${renderNumber(metricValue(metrics, 'successful_topup_count'))}`,
        icon: <CreditCard size={16} />,
        color: '#1664FF',
      },
      {
        key: 'repurchase',
        title: t('复购率'),
        value: formatPercent(metricValue(metrics, 'repurchase_rate')),
        sub: `${t('复购用户')} ${renderNumber(metricValue(metrics, 'repeat_topup_user_count'))}`,
        icon: <Repeat2 size={16} />,
        color: '#7442D4',
      },
      {
        key: 'consume',
        title: t('消耗额度'),
        value: renderQuota(metricValue(metrics, 'consume_quota'), 4),
        sub: `${t('调用')} ${renderNumber(metricValue(metrics, 'call_count'))}`,
        icon: <Activity size={16} />,
        color: '#3CC780',
      },
      {
        key: 'active',
        title: t('活跃用户'),
        value: renderNumber(metricValue(metrics, 'active_user_count')),
        sub: t('周期内产生消费的用户'),
        icon: <UserCheck size={16} />,
        color: '#009488',
      },
      {
        key: 'balance',
        title: t('剩余额度池'),
        value: renderQuota(metricValue(metrics, 'user_balance_quota'), 4),
        sub: `${t('累计已用')} ${renderQuota(metricValue(metrics, 'user_used_quota'), 4)}`,
        icon: <WalletCards size={16} />,
        color: '#FF8A00',
      },
      {
        key: 'users',
        title: t('用户总数'),
        value: renderNumber(metricValue(metrics, 'user_count')),
        sub: t('当前平台账号规模'),
        icon: <Users size={16} />,
        color: '#304D77',
      },
    ],
    [metrics, t],
  );

  const trendSpec = useMemo(() => {
    const values = trends.flatMap((item) => {
      const time = formatRangePoint(item.start);
      return [
        { time, type: t('充值金额'), value: Number(item.topup_amount || 0) },
        { time, type: t('消耗额度'), value: Number(item.consume_quota || 0) },
        { time, type: t('调用次数'), value: Number(item.call_count || 0) },
        {
          time,
          type: t('活跃用户'),
          value: Number(item.active_user_count || 0),
        },
      ];
    });
    return {
      type: 'line',
      data: [{ id: 'analyticsTrend', values }],
      xField: 'time',
      yField: 'value',
      seriesField: 'type',
      legends: { visible: true, orient: 'top' },
      color: chartColors,
      line: { style: { curveType: 'monotone' } },
      point: { visible: false },
      axes: [
        { orient: 'bottom', label: { autoHide: true, autoRotate: false } },
        {
          orient: 'left',
          label: { formatMethod: (value) => renderNumber(value) },
        },
      ],
      tooltip: {
        dimension: {
          content: [
            {
              key: (datum) => datum.type,
              value: (datum) => renderNumber(datum.value),
            },
          ],
        },
      },
    };
  }, [trends, t]);

  const modelDistribution = useMemo(
    () =>
      (distributions.model_consumption || []).map((item) => ({
        name: item.model_name || t('未知模型'),
        value: Number(item.quota || 0),
        count: Number(item.call_count || 0),
      })),
    [distributions.model_consumption, t],
  );

  const paymentDistribution = useMemo(
    () =>
      (distributions.payment_method || []).map((item) => ({
        name: item.payment_method || t('未知方式'),
        value: Number(item.topup_amount || 0),
        count: Number(item.topup_count || 0),
      })),
    [distributions.payment_method, t],
  );

  const channelDistribution = useMemo(
    () =>
      (distributions.channel_usage || []).map((item) => ({
        name: item.channel_name || t('未知渠道'),
        value: Number(item.quota || 0),
        count: Number(item.call_count || 0),
      })),
    [distributions.channel_usage, t],
  );

  const balanceDistribution = useMemo(
    () =>
      (distributions.balance_buckets || []).map((item) => ({
        name: item.label,
        value: Number(item.user_count || 0),
      })),
    [distributions.balance_buckets],
  );

  const distributionMap = {
    models: modelDistribution,
    channels: channelDistribution,
    payments: paymentDistribution,
    balances: balanceDistribution,
  };
  const activeDistribution = distributionMap[distributionType] || [];

  const pieSpec = useMemo(() => {
    const values = activeDistribution.length
      ? activeDistribution
      : [{ name: t('暂无数据'), value: 0 }];
    return {
      type: 'pie',
      data: [{ id: 'analyticsPie', values }],
      outerRadius: 0.82,
      innerRadius: 0.52,
      padAngle: 0.6,
      valueField: 'value',
      categoryField: 'name',
      color: chartColors,
      legends: { visible: true, orient: 'right' },
      label: { visible: true },
      tooltip: {
        mark: {
          content: [
            {
              key: (datum) => datum.name,
              value: (datum) => renderNumber(datum.value),
            },
          ],
        },
      },
    };
  }, [activeDistribution, t]);

  const barSpec = useMemo(() => {
    const values = activeDistribution.length
      ? activeDistribution.slice(0, 12)
      : [{ name: t('暂无数据'), value: 0 }];
    return {
      type: 'bar',
      data: [{ id: 'analyticsBar', values }],
      xField: 'name',
      yField: 'value',
      seriesField: 'name',
      color: chartColors,
      legends: { visible: false },
      axes: [
        { orient: 'bottom', label: { autoHide: true, autoRotate: true } },
        {
          orient: 'left',
          label: { formatMethod: (value) => renderNumber(value) },
        },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: (datum) => datum.name,
              value: (datum) => renderNumber(datum.value),
            },
          ],
        },
      },
    };
  }, [activeDistribution, t]);

  const rankingData = useMemo(() => {
    if (rankingType === 'topups') {
      return (rankings.user_topups || []).map((item, index) => ({
        rank: index + 1,
        id: item.user_id,
        name: item.display_name || item.username || `#${item.user_id}`,
        primary: formatMoney(item.topup_amount),
        secondary: `${t('订单')} ${renderNumber(item.topup_count)}`,
      }));
    }
    if (rankingType === 'agents') {
      return (rankings.agent_contribution || []).map((item, index) => ({
        rank: index + 1,
        id: item.agent_user_id,
        name: item.display_name || item.username || `#${item.agent_user_id}`,
        primary: formatMoney(Number(item.pay_amount || 0) / 100),
        secondary: `${t('返佣')} ${formatMoney(Number(item.rebate_amount || 0) / 100)}`,
      }));
    }
    return (rankings.user_consumptions || []).map((item, index) => ({
      rank: index + 1,
      id: item.user_id,
      name: item.display_name || item.username || `#${item.user_id}`,
      primary: renderQuota(item.consume_quota || 0, 4),
      secondary: `${t('调用')} ${renderNumber(item.call_count)}`,
    }));
  }, [rankingType, rankings, t]);

  const rankingColumns = useMemo(
    () => [
      {
        title: t('排名'),
        dataIndex: 'rank',
        width: 72,
        render: (value) => (
          <Tag color={value <= 3 ? 'blue' : 'grey'}>#{value}</Tag>
        ),
      },
      {
        title: t('对象'),
        dataIndex: 'name',
        render: (value, record) => (
          <div className='flex flex-col'>
            <Text strong>{value}</Text>
            <Text type='tertiary' size='small'>
              ID: {record.id}
            </Text>
          </div>
        ),
      },
      {
        title: t('核心值'),
        dataIndex: 'primary',
        align: 'right',
        render: (value) => <Text strong>{value}</Text>,
      },
      {
        title: t('补充指标'),
        dataIndex: 'secondary',
        align: 'right',
      },
    ],
    [t],
  );

  const usageData = useMemo(() => {
    if (usageType === 'channels') {
      return (distributions.channel_usage || []).map((item) => ({
        id: item.channel_id || 0,
        name:
          item.channel_name ||
          (item.channel_id ? `#${item.channel_id}` : t('未知渠道')),
        callCount: item.call_count || 0,
        consumeQuota: item.quota || 0,
        activeUsers: item.active_user_count || 0,
        totalTokens: null,
        averageQuota: item.average_quota || 0,
        averageUseTime: item.average_use_time || 0,
      }));
    }
    return (distributions.model_consumption || []).map((item) => ({
      id: item.model_name || 'unknown',
      name: item.model_name || t('未知模型'),
      callCount: item.call_count || 0,
      consumeQuota: item.quota || 0,
      activeUsers: item.active_user_count || 0,
      totalTokens: item.total_tokens || 0,
      promptTokens: item.prompt_tokens || 0,
      completionTokens: item.completion_tokens || 0,
      averageQuota: item.average_quota || 0,
      averageUseTime: item.average_use_time || 0,
    }));
  }, [
    distributions.channel_usage,
    distributions.model_consumption,
    t,
    usageType,
  ]);

  const usageColumns = useMemo(
    () => [
      {
        title: usageType === 'channels' ? t('渠道') : t('模型'),
        dataIndex: 'name',
        width: 220,
        render: (value) => <Text strong>{value}</Text>,
      },
      {
        title: t('调用量'),
        dataIndex: 'callCount',
        align: 'right',
        render: (value) => renderNumber(value),
      },
      {
        title: t('消耗额度'),
        dataIndex: 'consumeQuota',
        align: 'right',
        render: (value) => <Text strong>{renderQuota(value || 0, 4)}</Text>,
      },
      {
        title: t('活跃用户'),
        dataIndex: 'activeUsers',
        align: 'right',
        render: (value) => renderNumber(value),
      },
      {
        title: t('Token 总量'),
        dataIndex: 'totalTokens',
        align: 'right',
        render: (value, record) =>
          value === null ? (
            <Text type='tertiary'>-</Text>
          ) : (
            <div className='flex flex-col items-end'>
              <Text>{renderNumber(value)}</Text>
              <Text type='tertiary' size='small'>
                {t('输入')} {renderNumber(record.promptTokens)} / {t('输出')}{' '}
                {renderNumber(record.completionTokens)}
              </Text>
            </div>
          ),
      },
      {
        title: t('均次消耗'),
        dataIndex: 'averageQuota',
        align: 'right',
        render: (value) => renderQuota(value || 0, 4),
      },
      {
        title: t('平均耗时'),
        dataIndex: 'averageUseTime',
        align: 'right',
        render: (value) => formatSeconds(value),
      },
    ],
    [t, usageType],
  );

  const inactiveColumns = useMemo(
    () => [
      {
        title: t('用户'),
        dataIndex: 'username',
        width: 200,
        render: (value, record) => (
          <div className='flex flex-col'>
            <Text strong>{record.display_name || value}</Text>
            <Text type='tertiary' size='small'>
              ID: {record.id} · {value}
            </Text>
          </div>
        ),
      },
      {
        title: t('账户类型'),
        dataIndex: 'is_agent',
        width: 100,
        render: (value) => (
          <Tag color={value ? 'blue' : 'grey'}>
            {value ? t('代理') : t('普通用户')}
          </Tag>
        ),
      },
      {
        title: t('沉淀余额'),
        dataIndex: 'quota',
        width: 150,
        render: (value) => <Text strong>{renderQuota(value || 0, 4)}</Text>,
      },
      {
        title: t('最后有效调用'),
        dataIndex: 'last_api_activity_at',
        width: 170,
        render: (value) =>
          value > 0
            ? dayjs.unix(value).format('YYYY-MM-DD HH:mm')
            : t('从未调用'),
      },
      {
        title: t('未调用天数'),
        dataIndex: 'inactive_days',
        width: 120,
        render: (value) => t('{{count}} 天', { count: value || 0 }),
      },
      {
        title: t('上级代理'),
        dataIndex: 'inviter_username',
        width: 150,
        render: (value, record) =>
          value || (record.inviter_id ? `#${record.inviter_id}` : '-'),
      },
    ],
    [t],
  );

  const inactiveSummary = inactiveResult?.summary || {};

  const rangeText =
    overview.range?.start && overview.range?.end
      ? `${dayjs.unix(overview.range.start).format('YYYY-MM-DD HH:mm')} - ${dayjs
          .unix(overview.range.end)
          .format('YYYY-MM-DD HH:mm')}`
      : '';

  return (
    <div className='mt-[60px] px-2 pb-6'>
      <div className='mb-4 flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between'>
        <div>
          <Title heading={4} className='!mb-1'>
            {t('数据分析')}
          </Title>
          <Text type='tertiary'>{rangeText}</Text>
        </div>
        <div className='flex flex-col gap-2 md:flex-row md:items-center'>
          <div className='flex flex-wrap gap-2'>
            {RANGE_OPTIONS.map((option) => (
              <Button
                key={option.key}
                size='small'
                type={rangeKey === option.key ? 'primary' : 'tertiary'}
                theme={rangeKey === option.key ? 'solid' : 'light'}
                onClick={() => setRangeKey(option.key)}
              >
                {t(option.label)}
              </Button>
            ))}
          </div>
          {rangeKey === 'custom' ? (
            <DatePicker
              type='dateRange'
              size='small'
              value={customRange}
              onChange={(value) =>
                setCustomRange(Array.isArray(value) ? value : [])
              }
              placeholder={[t('开始日期'), t('结束日期')]}
            />
          ) : null}
          <Button
            size='small'
            type='primary'
            theme='light'
            icon={<RefreshCw size={14} />}
            loading={loading}
            onClick={loadOverview}
          >
            {t('刷新')}
          </Button>
        </div>
      </div>

      <Spin spinning={loading}>
        <div className='grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-6'>
          {metricCards.map((item) => (
            <Card
              key={item.key}
              bordered
              headerLine={false}
              bodyStyle={{ padding: 16 }}
            >
              <div className='mb-3 flex items-center justify-between gap-2'>
                <Text type='tertiary' size='small'>
                  {item.title}
                </Text>
                <span style={{ color: item.color }}>{item.icon}</span>
              </div>
              <div className='truncate text-2xl font-semibold text-[var(--semi-color-text-0)]'>
                {item.value}
              </div>
              <div className='mt-2 truncate text-xs text-[var(--semi-color-text-2)]'>
                {item.sub}
              </div>
            </Card>
          ))}
        </div>

        <div className='mt-3 grid grid-cols-1 gap-3 xl:grid-cols-3'>
          <Card
            className='xl:col-span-2'
            bordered
            headerLine
            title={
              <div className='flex items-center gap-2'>
                <Activity size={16} />
                {t('经营趋势')}
              </div>
            }
            bodyStyle={{ padding: 12 }}
          >
            {trends.length ? (
              <div className='h-[360px]'>
                <VChart spec={trendSpec} option={CHART_CONFIG} />
              </div>
            ) : (
              <Empty className='py-16' title={t('暂无数据')} />
            )}
          </Card>

          <Card
            bordered
            headerLine
            title={
              <div className='flex flex-col gap-2'>
                <div className='flex items-center gap-2'>
                  <BarChart3 size={16} />
                  {t('结构占比')}
                </div>
                <Tabs
                  type='button'
                  size='small'
                  activeKey={distributionType}
                  onChange={setDistributionType}
                >
                  <TabPane tab={t('模型')} itemKey='models' />
                  <TabPane tab={t('渠道')} itemKey='channels' />
                  <TabPane tab={t('支付')} itemKey='payments' />
                  <TabPane tab={t('余额')} itemKey='balances' />
                </Tabs>
              </div>
            }
            bodyStyle={{ padding: 12 }}
          >
            <div className='h-[340px]'>
              <VChart spec={pieSpec} option={CHART_CONFIG} />
            </div>
          </Card>
        </div>

        <Card
          className='mt-3'
          bordered
          headerLine
          title={
            <div className='flex flex-col gap-2 md:flex-row md:items-center md:justify-between'>
              <div className='flex items-center gap-2'>
                <BarChart3 size={16} />
                {t('使用日志细分')}
              </div>
              <Tabs
                type='button'
                size='small'
                activeKey={usageType}
                onChange={setUsageType}
              >
                <TabPane tab={t('模型明细')} itemKey='models' />
                <TabPane tab={t('渠道明细')} itemKey='channels' />
              </Tabs>
            </div>
          }
          bodyStyle={{ padding: 0 }}
        >
          <Table
            size='small'
            rowKey={(record) => `${usageType}-${record.id}`}
            columns={usageColumns}
            dataSource={usageData}
            pagination={false}
            scroll={{ y: 360, x: 920 }}
            empty={<Empty title={t('暂无数据')} />}
          />
        </Card>

        <div className='mt-3 grid grid-cols-1 gap-3 xl:grid-cols-2'>
          <Card
            bordered
            headerLine
            title={
              <div className='flex items-center gap-2'>
                <BarChart3 size={16} />
                {t('分布排行')}
              </div>
            }
            bodyStyle={{ padding: 12 }}
          >
            <div className='h-[340px]'>
              <VChart spec={barSpec} option={CHART_CONFIG} />
            </div>
          </Card>

          <Card
            bordered
            headerLine
            title={
              <div className='flex flex-col gap-2 md:flex-row md:items-center md:justify-between'>
                <div className='flex items-center gap-2'>
                  <Users size={16} />
                  {t('关键排行')}
                </div>
                <Tabs
                  type='button'
                  size='small'
                  activeKey={rankingType}
                  onChange={setRankingType}
                >
                  <TabPane tab={t('消耗用户')} itemKey='consumption' />
                  <TabPane tab={t('充值用户')} itemKey='topups' />
                  <TabPane tab={t('代理贡献')} itemKey='agents' />
                </Tabs>
              </div>
            }
            bodyStyle={{ padding: 0 }}
          >
            <Table
              size='small'
              rowKey={(record) => `${rankingType}-${record.id}-${record.rank}`}
              columns={rankingColumns}
              dataSource={rankingData}
              pagination={false}
              scroll={{ y: 300, x: 560 }}
              empty={<Empty title={t('暂无数据')} />}
            />
          </Card>
        </div>

        <Card
          className='mt-3'
          bordered
          headerLine
          title={
            <div className='flex flex-col gap-1'>
              <div className='flex items-center gap-2'>
                <Users size={16} />
                {t('僵尸用户分析')}
              </div>
              <Text type='tertiary' size='small'>
                {t('按最后一次有效 API 调用时间统计')}
              </Text>
            </div>
          }
          bodyStyle={{ padding: 16 }}
        >
          <div className='mb-4 flex flex-col gap-2 xl:flex-row xl:items-center'>
            <Select
              value={inactiveRange}
              optionList={INACTIVE_DAY_OPTIONS.map((option) => ({
                ...option,
                label: t(option.label),
              }))}
              onChange={(value) => {
                setInactiveRange(value);
                setInactivePage(1);
              }}
              style={{ width: 150 }}
            />
            {inactiveRange === 'custom' ? (
              <InputNumber
                value={inactiveCustomDays}
                min={1}
                max={3650}
                suffix={t('天')}
                onChange={setInactiveCustomDays}
                style={{ width: 150 }}
              />
            ) : null}
            <Select
              value={inactiveAccountType}
              optionList={[
                { value: 'all', label: t('全部账户') },
                { value: 'users', label: t('普通用户') },
                { value: 'agents', label: t('代理账户') },
              ]}
              onChange={(value) => {
                setInactiveAccountType(value);
                setInactivePage(1);
              }}
              style={{ width: 140 }}
            />
            <Input
              value={inactiveKeyword}
              onChange={setInactiveKeyword}
              onEnterPress={() => loadInactiveUsers(1)}
              placeholder={t('搜索用户 ID、用户名或显示名称')}
              showClear
              className='xl:max-w-[320px]'
            />
            <Button
              type='primary'
              loading={inactiveLoading}
              onClick={() => loadInactiveUsers(1)}
            >
              {t('查询')}
            </Button>
          </div>

          <div className='mb-4 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4'>
            {[
              {
                label: t('僵尸用户数量'),
                value: renderNumber(inactiveSummary.inactive_user_count || 0),
              },
              {
                label: t('沉默资金金额'),
                value: renderQuota(
                  inactiveSummary.inactive_balance_quota || 0,
                  4,
                ),
              },
              {
                label: t('从未调用用户'),
                value: renderNumber(inactiveSummary.never_called_count || 0),
              },
              {
                label: t('僵尸用户占比'),
                value: formatPercent(inactiveSummary.inactive_ratio || 0),
              },
            ].map((item) => (
              <div
                key={item.label}
                className='border border-solid border-[var(--semi-color-border)] p-3'
              >
                <Text type='secondary' size='small'>
                  {item.label}
                </Text>
                <div className='mt-2 text-xl font-semibold'>{item.value}</div>
              </div>
            ))}
          </div>

          <Spin spinning={inactiveLoading}>
            <Table
              size='small'
              rowKey='id'
              columns={inactiveColumns}
              dataSource={inactiveResult?.items || []}
              pagination={false}
              scroll={{ x: 1000 }}
              empty={<Empty title={t('未找到僵尸用户')} />}
            />
            <div className='mt-3 flex justify-end'>
              <Pagination
                currentPage={inactivePage}
                pageSize={INACTIVE_PAGE_SIZE}
                total={Number(inactiveResult?.total || 0)}
                onPageChange={(page) => loadInactiveUsers(page)}
                showTotal
              />
            </div>
          </Spin>
        </Card>
      </Spin>
    </div>
  );
};

export default Analytics;
