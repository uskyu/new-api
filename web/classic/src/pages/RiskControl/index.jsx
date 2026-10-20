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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Button,
  Card,
  Input,
  Modal,
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
import AccountManageModal from './AccountManageModal';
import UserInlineList, {
  getUserRowKey,
  isUserBanned,
  userMatchesKeyword,
} from './UserInlineList';
import { notifyBatchStatus, submitBatchStatus } from './batchStatus';

const { Text, Title } = Typography;
const PAGE_SIZE = 10;
const MIN_USERS_OPTIONS = [2, 3, 5, 10];
const DEFAULT_MIN_USERS = 2;
const DEFAULT_SHARED_SORT = 'user_count';
const DEFAULT_INVITER_SORT = 'direct_invite_count';
const DEFAULT_ORDER = 'desc';

const SourceTag = ({ source, t }) => (
  <Tag color={source === 'login' ? 'blue' : 'cyan'} shape='circle'>
    {source === 'login'
      ? t('登录')
      : source === 'token'
        ? t('令牌')
        : t('全部来源')}
  </Tag>
);

const MatchHint = ({ t }) => (
  <div className='mb-1'>
    <Tag color='red' shape='circle' size='small'>
      {t('命中用户')}
    </Tag>
  </div>
);

const scopeActionKey = (mode, item, action) =>
  mode === 'shared'
    ? `shared:${item?.source}:${item?.ip}:${action}`
    : `inviter:${item?.inviter_id}:${action}`;

function SharedIPCards({
  items,
  t,
  searchType,
  highlightKeyword,
  actionLoading,
  onManage,
  onScopeAction,
}) {
  if (!items.length) {
    return (
      <div className='py-8 text-center text-sm text-[var(--semi-color-text-2)] md:hidden'>
        {t('暂无数据')}
      </div>
    );
  }
  return (
    <div className='grid gap-3 md:hidden'>
      {items.map((item) => {
        const matchedUsers =
          highlightKeyword &&
          (searchType === 'username' || searchType === 'user_id')
            ? (item.users || []).filter((user) =>
                userMatchesKeyword(user, highlightKeyword, searchType),
              )
            : [];
        return (
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
              <div className='mb-1 text-xs text-[var(--semi-color-text-2)]'>
                {t('关联用户')}
              </div>
              {matchedUsers.length > 0 && <MatchHint t={t} />}
              <UserInlineList
                users={item.users}
                max={2}
                isHighlighted={(user) => matchedUsers.includes(user)}
                onShowMore={() => onManage(item)}
              />
            </div>
            <div className='mt-3 flex gap-2'>
              <Button className='flex-1' onClick={() => onManage(item)}>
                {t('管理账号')}
              </Button>
              <Button
                className='flex-1'
                type='danger'
                theme='light'
                loading={
                  actionLoading ===
                  scopeActionKey('shared', item, 'disable')
                }
                onClick={() => onScopeAction(item, 'disable')}
              >
                {t('封禁全部')}
              </Button>
              <Button
                className='flex-1'
                type='primary'
                theme='light'
                loading={
                  actionLoading === scopeActionKey('shared', item, 'enable')
                }
                onClick={() => onScopeAction(item, 'enable')}
              >
                {t('解禁全部')}
              </Button>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function InviterCards({
  items,
  t,
  highlightKeyword,
  actionLoading,
  onManage,
  onScopeAction,
}) {
  if (!items.length) {
    return (
      <div className='py-8 text-center text-sm text-[var(--semi-color-text-2)] md:hidden'>
        {t('暂无数据')}
      </div>
    );
  }
  return (
    <div className='grid gap-3 md:hidden'>
      {items.map((item) => {
        const matchedInvitees =
          highlightKeyword && Array.isArray(item.matched_invitees)
            ? item.matched_invitees
            : [];
        const matchedIds = new Set(
          matchedInvitees.map((user) => getUserRowKey(user)),
        );
        const selfMatched =
          highlightKeyword &&
          userMatchesKeyword(
            {
              user_id: item.inviter_id,
              username: item.username,
              display_name: item.display_name,
            },
            highlightKeyword,
            'any',
          );
        return (
          <div
            key={item.inviter_id}
            className='rounded-lg border border-[var(--semi-color-border)] p-3'
          >
            <div className='flex items-start justify-between gap-3'>
              <div className='min-w-0'>
                <div className='flex flex-wrap items-center gap-1'>
                  <Text strong type={selfMatched ? 'danger' : undefined}>
                    #{item.inviter_id} {item.username}
                  </Text>
                  {selfMatched && (
                    <Tag color='red' shape='circle' size='small'>
                      {t('命中用户')}
                    </Tag>
                  )}
                </div>
                {item.display_name && (
                  <div className='truncate text-xs text-[var(--semi-color-text-2)]'>
                    {item.display_name}
                  </div>
                )}
                <div className='truncate text-xs text-[var(--semi-color-text-2)]'>
                  {item.email || t('暂无邮箱')}
                </div>
              </div>
              <div className='flex shrink-0 flex-col items-end gap-1'>
                <Tag
                  color={isUserBanned(item) ? 'red' : 'green'}
                  shape='circle'
                  size='small'
                >
                  {isUserBanned(item) ? t('已封禁') : t('正常')}
                </Tag>
                {item.suspicious && (
                  <Tag color='orange' shape='circle' size='small'>
                    {t('需复核')}
                  </Tag>
                )}
              </div>
            </div>
            <div className='mt-3 flex flex-wrap gap-2'>
              <Tag>
                {t('直接邀请人数')}: {item.direct_invite_count}
              </Tag>
              <Tag color={item.shared_ip_count ? 'red' : 'white'}>
                {t('关联同 IP')}: {item.shared_ip_count}
              </Tag>
            </div>
            <div className='mt-3 border-t border-[var(--semi-color-border)] pt-3'>
              <div className='mb-1 text-xs text-[var(--semi-color-text-2)]'>
                {t('被邀请用户')}
              </div>
              {matchedInvitees.length > 0 && (
                <div className='mb-1 flex flex-wrap items-center gap-1'>
                  <Tag color='red' shape='circle' size='small'>
                    {t('命中用户')}
                  </Tag>
                  <UserInlineList
                    users={matchedInvitees}
                    max={2}
                    isHighlighted={() => true}
                    onShowMore={() => onManage(item)}
                  />
                </div>
              )}
              <UserInlineList
                users={item.invitees}
                max={2}
                isHighlighted={(user) => matchedIds.has(getUserRowKey(user))}
                onShowMore={() => onManage(item)}
              />
            </div>
            <div className='mt-3 flex gap-2'>
              <Button className='flex-1' onClick={() => onManage(item)}>
                {t('管理账号')}
              </Button>
              <Button
                className='flex-1'
                type='danger'
                theme='light'
                loading={actionLoading === scopeActionKey('inviter', item, 'disable')}
                onClick={() => onScopeAction(item, 'disable')}
              >
                {t('封禁全部')}
              </Button>
              <Button
                className='flex-1'
                type='primary'
                theme='light'
                loading={actionLoading === scopeActionKey('inviter', item, 'enable')}
                onClick={() => onScopeAction(item, 'enable')}
              >
                {t('解禁全部')}
              </Button>
            </div>
          </div>
        );
      })}
    </div>
  );
}

export default function RiskControl() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [overview, setOverview] = useState({});
  const [tab, setTab] = useState('shared');
  const [source, setSource] = useState('all');
  const [searchType, setSearchType] = useState('ip');
  const [keyword, setKeyword] = useState('');
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);

  // 同 IP 用户排序/筛选
  const [sharedSortBy, setSharedSortBy] = useState(DEFAULT_SHARED_SORT);
  const [sharedOrder, setSharedOrder] = useState(DEFAULT_ORDER);
  const [minUsers, setMinUsers] = useState(DEFAULT_MIN_USERS);
  // 邀请关系排序
  const [inviterSortBy, setInviterSortBy] = useState(DEFAULT_INVITER_SORT);
  const [inviterOrder, setInviterOrder] = useState(DEFAULT_ORDER);

  const [accountModal, setAccountModal] = useState({
    visible: false,
    mode: 'shared',
    item: null,
  });
  const [scopeActionLoading, setScopeActionLoading] = useState('');
  // 丢弃过期的列表响应，避免快速切换筛选/排序时旧响应覆盖新结果。
  const loadSeqRef = useRef(0);

  const loadData = async () => {
    const seq = ++loadSeqRef.current;
    setLoading(true);
    try {
      const endpoint =
        tab === 'shared'
          ? '/api/risk-control/shared-ips'
          : '/api/risk-control/inviters';
      const params = { p: page, page_size: PAGE_SIZE, keyword: search };
      if (tab === 'shared') {
        if (source !== 'all') {
          params.source = source;
        }
        params.search_type = searchType;
        params.min_users = minUsers;
        params.sort_by = sharedSortBy;
        params.order = sharedOrder;
      } else {
        params.sort_by = inviterSortBy;
        params.order = inviterOrder;
      }
      const [overviewRes, listRes] = await Promise.all([
        API.get('/api/risk-control/overview'),
        API.get(endpoint, { params }),
      ]);
      if (seq !== loadSeqRef.current) {
        return;
      }
      if (!overviewRes.data.success) {
        throw new Error(overviewRes.data.message);
      }
      if (!listRes.data.success) {
        throw new Error(listRes.data.message);
      }
      setOverview(overviewRes.data.data || {});
      setItems(listRes.data.data?.items || []);
      setTotal(listRes.data.data?.total || 0);
    } catch (error) {
      if (seq !== loadSeqRef.current) {
        return;
      }
      showError(error);
    } finally {
      if (seq === loadSeqRef.current) {
        setLoading(false);
      }
    }
  };

  useEffect(() => {
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    tab,
    source,
    searchType,
    search,
    page,
    sharedSortBy,
    sharedOrder,
    minUsers,
    inviterSortBy,
    inviterOrder,
  ]);

  const openAccountModal = useCallback((mode, item) => {
    setAccountModal({ visible: true, mode, item });
  }, []);

  const closeAccountModal = useCallback(() => {
    setAccountModal((prev) => ({ ...prev, visible: false }));
  }, []);

  const executeScopeAction = async (mode, item, action) => {
    setScopeActionLoading(scopeActionKey(mode, item, action));
    try {
      const result =
        mode === 'shared'
          ? await submitBatchStatus({
              action,
              scope: 'shared_ip',
              ip: item?.ip,
              source: item?.source,
            })
          : await submitBatchStatus({
              action,
              scope: 'inviter',
              inviterId: item?.inviter_id,
            });
      if (!result.ok) {
        showError(result.message || t('操作失败'));
        return;
      }
      notifyBatchStatus(action, result, t);
      await loadData();
    } catch (error) {
      showError(error);
    } finally {
      setScopeActionLoading('');
    }
  };

  const buildScopeConfirmContent = (mode, item, action) => {
    const scopeText =
      mode === 'inviter'
        ? action === 'disable'
          ? t('封禁全部被邀请账号')
          : t('解禁全部被邀请账号')
        : action === 'disable'
          ? t('封禁该 IP 下全部账号')
          : t('解禁该 IP 下全部账号');
    const scopeTotal =
      mode === 'inviter'
        ? Number(item?.direct_invite_count) || 0
        : Number(item?.user_count) || 0;
    return (
      <div>
        <div>{scopeText}</div>
        <div className='mt-1 text-[var(--semi-color-text-2)]'>
          {t('共 {{total}} 个账号', { total: scopeTotal })}
        </div>
        <div className='mt-1'>
          {action === 'disable'
            ? t('封禁后这些账号将无法登录和调用 API。')
            : t('解禁后这些账号将恢复访问。')}
        </div>
      </div>
    );
  };

  const handleScopeAction = (mode, item, action) => {
    if (mode === 'shared') {
      Modal.confirm({
        title:
          action === 'disable'
            ? t('确认封禁该 IP 下全部账号')
            : t('确认解禁该 IP 下全部账号'),
        content: buildScopeConfirmContent(mode, item, action),
        centered: true,
        okText: action === 'disable' ? t('封禁') : t('解禁'),
        cancelText: t('取消'),
        okType: action === 'disable' ? 'danger' : 'primary',
        onOk: () => executeScopeAction(mode, item, action),
      });
      return;
    }
    Modal.confirm({
      title:
        action === 'disable'
          ? t('确认封禁该邀请人的全部被邀请账号')
          : t('确认解禁该邀请人的全部被邀请账号'),
      content: buildScopeConfirmContent(mode, item, action),
      centered: true,
      okText: action === 'disable' ? t('封禁') : t('解禁'),
      cancelText: t('取消'),
      okType: action === 'disable' ? 'danger' : 'primary',
      onOk: () => executeScopeAction(mode, item, action),
    });
  };

  const appliedKeyword = search.trim();
  const sharedHighlightActive =
    tab === 'shared' &&
    (searchType === 'username' || searchType === 'user_id') &&
    appliedKeyword !== '';
  const inviterHighlightActive =
    tab === 'inviters' && appliedKeyword !== '';

  const sharedColumns = [
    {
      title: t('IP 地址'),
      dataIndex: 'ip',
      width: 180,
      render: (value) => (
        <Text copyable={{ content: value }} className='break-all'>
          {value}
        </Text>
      ),
    },
    {
      title: t('来源'),
      dataIndex: 'source',
      width: 100,
      render: (value) => <SourceTag source={value} t={t} />,
    },
    {
      title: t('关联用户'),
      dataIndex: 'users',
      width: 340,
      render: (users, item) => {
        const matchedUsers = sharedHighlightActive
          ? (users || []).filter((user) =>
              userMatchesKeyword(user, appliedKeyword, searchType),
            )
          : [];
        return (
          <div className='min-w-0'>
            {matchedUsers.length > 0 && <MatchHint t={t} />}
            <UserInlineList
              users={users}
              max={3}
              isHighlighted={(user) => matchedUsers.includes(user)}
              onShowMore={() => openAccountModal('shared', item)}
            />
          </div>
        );
      },
    },
    { title: t('用户数'), dataIndex: 'user_count', width: 90 },
    { title: t('采样次数'), dataIndex: 'event_count', width: 100 },
    {
      title: t('最近出现'),
      dataIndex: 'last_seen_at',
      width: 180,
      render: timestamp2string,
    },
    {
      title: t('操作'),
      width: 290,
      render: (_, item) => (
        <Space wrap>
          <Button
            size='small'
            onClick={() => openAccountModal('shared', item)}
          >
            {t('管理账号')}
          </Button>
          <Button
            size='small'
            type='danger'
            theme='light'
            loading={
              scopeActionLoading === scopeActionKey('shared', item, 'disable')
            }
            onClick={() => handleScopeAction('shared', item, 'disable')}
          >
            {t('封禁全部')}
          </Button>
          <Button
            size='small'
            type='primary'
            theme='light'
            loading={
              scopeActionLoading === scopeActionKey('shared', item, 'enable')
            }
            onClick={() => handleScopeAction('shared', item, 'enable')}
          >
            {t('解禁全部')}
          </Button>
        </Space>
      ),
    },
  ];

  const inviterColumns = [
    {
      title: t('邀请人'),
      width: 220,
      render: (_, item) => {
        const selfMatched =
          inviterHighlightActive &&
          userMatchesKeyword(
            {
              user_id: item.inviter_id,
              username: item.username,
              display_name: item.display_name,
            },
            appliedKeyword,
            'any',
          );
        return (
          <div className='min-w-0'>
            <div className='flex flex-wrap items-center gap-1'>
              <Text strong type={selfMatched ? 'danger' : undefined}>
                #{item.inviter_id} {item.username}
              </Text>
              {selfMatched && (
                <Tag color='red' shape='circle' size='small'>
                  {t('命中用户')}
                </Tag>
              )}
            </div>
            <div className='truncate text-xs text-[var(--semi-color-text-2)]'>
              {item.display_name || '-'}
            </div>
            <div className='truncate text-xs text-[var(--semi-color-text-2)]'>
              {item.email || t('暂无邮箱')}
            </div>
          </div>
        );
      },
    },
    { title: t('直接邀请人数'), dataIndex: 'direct_invite_count', width: 120 },
    { title: t('关联同 IP'), dataIndex: 'shared_ip_count', width: 110 },
    {
      title: t('被邀请用户'),
      dataIndex: 'invitees',
      width: 360,
      render: (users, item) => {
        const matchedInvitees =
          inviterHighlightActive && Array.isArray(item.matched_invitees)
            ? item.matched_invitees
            : [];
        const matchedIds = new Set(
          matchedInvitees.map((user) => getUserRowKey(user)),
        );
        return (
          <div className='min-w-0'>
            {matchedInvitees.length > 0 && (
              <div className='mb-1 flex flex-wrap items-center gap-1'>
                <Tag color='red' shape='circle' size='small'>
                  {t('命中用户')}
                </Tag>
                <UserInlineList
                  users={matchedInvitees}
                  max={3}
                  isHighlighted={() => true}
                  onShowMore={() => openAccountModal('inviter', item)}
                />
              </div>
            )}
            <UserInlineList
              users={users}
              max={3}
              isHighlighted={(user) => matchedIds.has(getUserRowKey(user))}
              onShowMore={() => openAccountModal('inviter', item)}
            />
          </div>
        );
      },
    },
    {
      title: t('状态'),
      width: 130,
      render: (_, item) => (
        <Space wrap>
          <Tag
            color={isUserBanned(item) ? 'red' : 'green'}
            shape='circle'
            size='small'
          >
            {isUserBanned(item) ? t('已封禁') : t('正常')}
          </Tag>
          {item.suspicious && (
            <Tag color='orange' shape='circle' size='small'>
              {t('需复核')}
            </Tag>
          )}
        </Space>
      ),
    },
    {
      title: t('操作'),
      width: 290,
      render: (_, item) => (
        <Space wrap>
          <Button
            size='small'
            onClick={() => openAccountModal('inviter', item)}
          >
            {t('管理账号')}
          </Button>
          <Button
            size='small'
            type='danger'
            theme='light'
            loading={
              scopeActionLoading === scopeActionKey('inviter', item, 'disable')
            }
            onClick={() => handleScopeAction('inviter', item, 'disable')}
          >
            {t('封禁全部')}
          </Button>
          <Button
            size='small'
            type='primary'
            theme='light'
            loading={
              scopeActionLoading === scopeActionKey('inviter', item, 'enable')
            }
            onClick={() => handleScopeAction('inviter', item, 'enable')}
          >
            {t('解禁全部')}
          </Button>
        </Space>
      ),
    },
  ];

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
              {t('核对共享 IP 与邀请关联，可手动处置高风险账号')}
            </Text>
          </div>
          <Button
            icon={<RefreshCw size={16} />}
            onClick={loadData}
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

          <div className='my-4 flex flex-col gap-3'>
            <div className='flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center'>
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
                  className='w-full sm:w-40'
                >
                  <Select.Option value='ip'>{t('搜索 IP')}</Select.Option>
                  <Select.Option value='username'>
                    {t('搜索用户名/昵称')}
                  </Select.Option>
                  <Select.Option value='user_id'>
                    {t('搜索用户ID')}
                  </Select.Option>
                </Select>
              )}
              <Input
                value={keyword}
                onChange={setKeyword}
                prefix={<Search size={16} />}
                className='w-full sm:w-64'
                placeholder={
                  tab === 'shared'
                    ? searchType === 'username'
                      ? t('搜索用户名/昵称')
                      : searchType === 'user_id'
                        ? t('搜索用户ID')
                        : t('搜索 IP')
                    : t('搜索用户 ID 或名称')
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

            <div className='flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center'>
              {tab === 'shared' ? (
                <Select
                  value={sharedSortBy}
                  onChange={(value) => {
                    setSharedSortBy(value);
                    setPage(1);
                  }}
                  className='w-full sm:w-48'
                  prefix={t('排序')}
                >
                  <Select.Option value='user_count'>
                    {t('按用户数')}
                  </Select.Option>
                  <Select.Option value='event_count'>
                    {t('按采样次数')}
                  </Select.Option>
                  <Select.Option value='last_seen_at'>
                    {t('按最近出现')}
                  </Select.Option>
                  <Select.Option value='ip'>{t('按 IP')}</Select.Option>
                </Select>
              ) : (
                <Select
                  value={inviterSortBy}
                  onChange={(value) => {
                    setInviterSortBy(value);
                    setPage(1);
                  }}
                  className='w-full sm:w-48'
                  prefix={t('排序')}
                >
                  <Select.Option value='direct_invite_count'>
                    {t('按邀请人数')}
                  </Select.Option>
                  <Select.Option value='inviter_id'>
                    {t('按邀请人 ID')}
                  </Select.Option>
                </Select>
              )}
              <Select
                value={tab === 'shared' ? sharedOrder : inviterOrder}
                onChange={(value) => {
                  if (tab === 'shared') {
                    setSharedOrder(value);
                  } else {
                    setInviterOrder(value);
                  }
                  setPage(1);
                }}
                className='w-full sm:w-28'
              >
                <Select.Option value='desc'>{t('降序')}</Select.Option>
                <Select.Option value='asc'>{t('升序')}</Select.Option>
              </Select>
              {tab === 'shared' && (
                <Select
                  value={minUsers}
                  onChange={(value) => {
                    setMinUsers(value);
                    setPage(1);
                  }}
                  className='w-full sm:w-56'
                  prefix={t('最少关联用户数')}
                >
                  {MIN_USERS_OPTIONS.map((value) => (
                    <Select.Option key={value} value={value}>
                      {value}
                    </Select.Option>
                  ))}
                </Select>
              )}
            </div>
          </div>

          <Spin spinning={loading}>
            {tab === 'shared' ? (
              <SharedIPCards
                items={items}
                t={t}
                searchType={searchType}
                highlightKeyword={
                  sharedHighlightActive ? appliedKeyword : ''
                }
                actionLoading={scopeActionLoading}
                onManage={(item) => openAccountModal('shared', item)}
                onScopeAction={(item, action) =>
                  handleScopeAction('shared', item, action)
                }
              />
            ) : (
              <InviterCards
                items={items}
                t={t}
                highlightKeyword={
                  inviterHighlightActive ? appliedKeyword : ''
                }
                actionLoading={scopeActionLoading}
                onManage={(item) => openAccountModal('inviter', item)}
                onScopeAction={(item, action) =>
                  handleScopeAction('inviter', item, action)
                }
              />
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
                empty={t('暂无数据')}
                scroll={{ x: 'max-content' }}
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

      <AccountManageModal
        visible={accountModal.visible}
        mode={accountModal.mode}
        item={accountModal.item}
        t={t}
        onClose={closeAccountModal}
        onSuccess={loadData}
      />
    </div>
  );
}
