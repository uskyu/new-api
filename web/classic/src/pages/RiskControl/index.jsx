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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Card,
  Input,
  Pagination,
  Select,
  Space,
  Spin,
  Table,
  TabPane,
  Tabs,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { RefreshCw, Search, ShieldAlert, Users } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showError, timestamp2string } from '../../helpers';

const { Text, Title } = Typography;
const PAGE_SIZE = 10;

const SourceTag = ({ source, t }) => (
  <Tag color={source === 'login' ? 'blue' : 'cyan'} shape='circle'>
    {source === 'login'
      ? t('登录')
      : source === 'token'
        ? t('令牌')
        : t('全部来源')}
  </Tag>
);

const UserList = ({ users = [] }) => (
  <div className='flex min-w-0 flex-wrap gap-1'>
    {users.map((user) => (
      <Tag key={`${user.user_id}-${user.token_id}`} color='white'>
        #{user.user_id} {user.username}
      </Tag>
    ))}
  </div>
);

function SharedIPCards({ items, t }) {
  return (
    <div className='grid gap-3 md:hidden'>
      {items.map((item) => (
        <div
          key={`${item.source}-${item.ip}`}
          className='rounded-lg border border-[var(--semi-color-border)] p-3'
        >
          <div className='flex items-start justify-between gap-3'>
            <Text strong copyable={{ content: item.ip }} className='break-all'>
              {item.ip}
            </Text>
            <SourceTag source={item.source} t={t} />
          </div>
          <div className='mt-3 grid grid-cols-3 gap-2 text-center'>
            <div>
              <div className='text-xs text-[var(--semi-color-text-2)]'>
                {t('用户数')}
              </div>
              <div className='mt-1 font-medium'>{item.user_count}</div>
            </div>
            <div>
              <div className='text-xs text-[var(--semi-color-text-2)]'>
                {t('采样次数')}
              </div>
              <div className='mt-1 font-medium'>{item.event_count}</div>
            </div>
            <div>
              <div className='text-xs text-[var(--semi-color-text-2)]'>
                {t('最近出现')}
              </div>
              <div className='mt-1 text-xs'>
                {timestamp2string(item.last_seen_at)}
              </div>
            </div>
          </div>
          <div className='mt-3 border-t border-[var(--semi-color-border)] pt-3'>
            <UserList users={item.users} />
          </div>
        </div>
      ))}
    </div>
  );
}

function InviterCards({ items, t }) {
  return (
    <div className='grid gap-3 md:hidden'>
      {items.map((item) => (
        <div
          key={item.inviter_id}
          className='rounded-lg border border-[var(--semi-color-border)] p-3'
        >
          <div className='flex items-start justify-between gap-3'>
            <div>
              <Text strong>
                #{item.inviter_id} {item.username}
              </Text>
              {item.display_name && (
                <div className='text-xs text-[var(--semi-color-text-2)]'>
                  {item.display_name}
                </div>
              )}
            </div>
            <Tag color={item.suspicious ? 'red' : 'green'} shape='circle'>
              {item.suspicious ? t('需复核') : t('正常')}
            </Tag>
          </div>
          <div className='mt-3 flex flex-wrap gap-2'>
            <Tag>
              {t('直接邀请')}: {item.direct_invite_count}
            </Tag>
            <Tag color={item.shared_ip_count ? 'red' : 'white'}>
              {t('关联同 IP')}: {item.shared_ip_count}
            </Tag>
            <Tag color='white'>
              {t('最近邀请')}:{' '}
              {item.latest_invite_at
                ? timestamp2string(item.latest_invite_at)
                : '-'}
            </Tag>
          </div>
          <div className='mt-3 border-t border-[var(--semi-color-border)] pt-3'>
            <UserList users={item.invitees} />
          </div>
        </div>
      ))}
    </div>
  );
}

export default function RiskControl() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [overview, setOverview] = useState({});
  const [tab, setTab] = useState('shared');
  const [source, setSource] = useState('all');
  const [riskStatus, setRiskStatus] = useState('all');
  const [searchType, setSearchType] = useState('ip');
  const [keyword, setKeyword] = useState('');
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const requestSeq = useRef(0);
  const loadingSeq = useRef(0);

  const loadData = async ({ silent = false, listOnly = false } = {}) => {
    const seq = ++requestSeq.current;
    if (!silent) {
      loadingSeq.current = seq;
      setLoading(true);
    }
    try {
      const endpoint =
        tab === 'shared'
          ? '/api/risk-control/shared-ips'
          : '/api/risk-control/inviters';
      const params = { p: page, page_size: PAGE_SIZE, keyword: search };
      if (tab === 'shared' && source !== 'all') params.source = source;
      if (tab === 'shared') params.min_users = 2;
      if (tab === 'shared') params.search_type = searchType;
      if (tab === 'inviters' && riskStatus !== 'all')
        params.risk_status = riskStatus;
      const requests = [
        API.get(endpoint, { params }),
        ...(listOnly ? [] : [API.get('/api/risk-control/overview')]),
      ];
      const [listRes, overviewRes] = await Promise.all(requests);
      if (seq !== requestSeq.current) return;
      if (!listRes.data.success) throw new Error(listRes.data.message);
      setItems(listRes.data.data?.items || []);
      setTotal(listRes.data.data?.total || 0);
      if (overviewRes) {
        if (!overviewRes.data.success)
          throw new Error(overviewRes.data.message);
        setOverview(overviewRes.data.data || {});
      }
    } catch (error) {
      if (seq !== requestSeq.current) return;
      if (silent) return;
      showError(error);
    } finally {
      if (!silent && loadingSeq.current === seq) setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab, source, searchType, search, page, riskStatus]);

  // Poll the inviter list every 30s on the first page, unfiltered only.
  useEffect(() => {
    if (tab !== 'inviters' || page !== 1 || riskStatus !== 'all')
      return undefined;
    const timer = setInterval(
      () => loadData({ silent: true, listOnly: true }),
      30000,
    );
    return () => clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab, page, riskStatus, source, searchType, search]);

  const sharedColumns = useMemo(
    () => [
      {
        title: t('IP 地址'),
        dataIndex: 'ip',
        render: (value) => <Text copyable={{ content: value }}>{value}</Text>,
      },
      {
        title: t('来源'),
        dataIndex: 'source',
        render: (value) => <SourceTag source={value} t={t} />,
      },
      {
        title: t('关联用户'),
        dataIndex: 'users',
        render: (value) => <UserList users={value} />,
      },
      { title: t('用户数'), dataIndex: 'user_count', width: 90 },
      { title: t('采样次数'), dataIndex: 'event_count', width: 100 },
      {
        title: t('最近出现'),
        dataIndex: 'last_seen_at',
        render: timestamp2string,
        width: 180,
      },
    ],
    [t],
  );

  const inviterColumns = useMemo(
    () => [
      {
        title: t('邀请人'),
        render: (_, item) => (
          <Text strong>
            #{item.inviter_id} {item.username}
          </Text>
        ),
      },
      {
        title: t('直接邀请人数'),
        dataIndex: 'direct_invite_count',
        width: 120,
      },
      { title: t('关联同 IP'), dataIndex: 'shared_ip_count', width: 110 },
      {
        title: t('最近邀请时间'),
        dataIndex: 'latest_invite_at',
        render: (value) => (value ? timestamp2string(value) : '-'),
        width: 180,
      },
      {
        title: t('被邀请用户'),
        dataIndex: 'invitees',
        render: (value) => <UserList users={value} />,
      },
      {
        title: t('状态'),
        render: (_, item) => (
          <Tag color={item.suspicious ? 'red' : 'green'}>
            {item.suspicious ? t('需复核') : t('正常')}
          </Tag>
        ),
        width: 90,
      },
    ],
    [t],
  );

  const stats = [
    [t('已记录用户'), overview.tracked_users || 0],
    [t('同 IP 组'), overview.shared_ip_groups || 0],
    [t('邀请人'), overview.inviter_count || 0],
    [t('24 小时活跃记录'), overview.active_records_24h || 0],
  ];

  return (
    <div className='mt-[60px] px-2 pb-8 md:px-4'>
      <div className='mx-auto max-w-[1500px]'>
        <div className='mb-4 flex flex-wrap items-center justify-between gap-3'>
          <div>
            <Title heading={3} className='!mb-1 flex items-center gap-2'>
              <ShieldAlert size={22} />
              {t('风控管理')}
            </Title>
            <Text type='secondary'>
              {t('核对共享 IP 与邀请关联，不自动处置用户')}
            </Text>
          </div>
          <Button
            icon={<RefreshCw size={16} />}
            onClick={() => loadData()}
            loading={loading}
          >
            {t('刷新')}
          </Button>
        </div>

        <div className='mb-4 grid grid-cols-2 gap-3 lg:grid-cols-4'>
          {stats.map(([label, value]) => (
            <Card key={label} bodyStyle={{ padding: 14 }}>
              <Text type='secondary' size='small'>
                {label}
              </Text>
              <div className='mt-1 text-xl font-semibold'>{value}</div>
            </Card>
          ))}
        </div>

        <Card bodyStyle={{ padding: 16 }}>
          <Tabs
            activeKey={tab}
            onChange={(value) => {
              setTab(value);
              setPage(1);
              setItems([]);
            }}
          >
            <TabPane
              tab={
                <span className='flex items-center gap-2'>
                  <ShieldAlert size={16} />
                  {t('同 IP 用户')}
                </span>
              }
              itemKey='shared'
            />
            <TabPane
              tab={
                <span className='flex items-center gap-2'>
                  <Users size={16} />
                  {t('邀请关系')}
                </span>
              }
              itemKey='inviters'
            />
          </Tabs>

          <div className='my-4 flex flex-col gap-3 sm:flex-row'>
            {tab === 'shared' && (
              <Select
                value={source}
                onChange={(value) => {
                  setSource(value);
                  setPage(1);
                }}
                className='w-full sm:w-36'
              >
                <Select.Option value='all'>{t('全部来源')}</Select.Option>
                <Select.Option value='login'>{t('登录')}</Select.Option>
                <Select.Option value='token'>{t('令牌')}</Select.Option>
              </Select>
            )}
            {tab === 'shared' && (
              <Select
                value={searchType}
                onChange={(value) => {
                  setSearchType(value);
                  setPage(1);
                }}
                className='w-full sm:w-32'
              >
                <Select.Option value='ip'>{t('搜索 IP')}</Select.Option>
                <Select.Option value='username'>
                  {t('搜索用户名/昵称')}
                </Select.Option>
                <Select.Option value='user_id'>{t('搜索用户ID')}</Select.Option>
              </Select>
            )}
            {tab === 'inviters' && (
              <Select
                value={riskStatus}
                onChange={(value) => {
                  setRiskStatus(value);
                  setPage(1);
                }}
                className='w-full sm:w-36'
              >
                <Select.Option value='all'>{t('全部')}</Select.Option>
                <Select.Option value='review'>{t('需复核')}</Select.Option>
                <Select.Option value='normal'>{t('未发现异常')}</Select.Option>
              </Select>
            )}
            <Input
              value={keyword}
              onChange={setKeyword}
              prefix={<Search size={16} />}
              placeholder={
                tab === 'shared'
                  ? searchType === 'username'
                    ? t('搜索用户名/昵称')
                    : searchType === 'user_id'
                      ? t('搜索用户ID')
                      : t('搜索 IP')
                  : t('搜索邀请人')
              }
              onEnterPress={() => {
                setSearch(keyword.trim());
                setPage(1);
              }}
            />
            <Button
              onClick={() => {
                setSearch(keyword.trim());
                setPage(1);
              }}
            >
              {t('查询')}
            </Button>
          </div>

          <Spin spinning={loading}>
            {tab === 'shared' ? (
              <SharedIPCards items={items} t={t} />
            ) : (
              <InviterCards items={items} t={t} />
            )}
            <div className='hidden overflow-x-auto md:block'>
              <Table
                columns={tab === 'shared' ? sharedColumns : inviterColumns}
                dataSource={items}
                pagination={false}
                rowKey={
                  tab === 'shared'
                    ? (item) => `${item.source}-${item.ip}`
                    : 'inviter_id'
                }
                empty='-'
              />
            </div>
            <div className='mt-4 flex justify-end'>
              <Pagination
                currentPage={page}
                pageSize={PAGE_SIZE}
                total={total}
                onPageChange={setPage}
                showTotal
              />
            </div>
          </Spin>
        </Card>
      </div>
    </div>
  );
}
