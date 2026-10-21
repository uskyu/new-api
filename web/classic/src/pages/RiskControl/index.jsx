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
  Checkbox,
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
import { API, renderQuota, showError, timestamp2string } from '../../helpers';
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
// 排序方式合并为单个下拉，状态形如 `sort_by:order`。
const DEFAULT_SHARED_SORT = 'user_count:desc';
const DEFAULT_INVITER_SORT = 'direct_invite_count:desc';
const SHARED_SORT_FALLBACK = 'user_count';
const INVITER_SORT_FALLBACK = 'direct_invite_count';

// 分类 chip：点击后一次性写入该分类的预设筛选值，预设里没有的项回到页签默认值，
// 因此「全部」等价于改造前的默认行为（不额外传任何新参数）。
const SHARED_CHIP_DEFAULTS = {
  source: 'all',
  minUsers: DEFAULT_MIN_USERS,
  banState: '',
  active24h: false,
};
const SHARED_CHIPS = [
  { key: 'all', label: '全部', preset: {} },
  {
    key: 'high_risk',
    label: '高危',
    preset: { minUsers: 3, banState: 'has_banned' },
  },
  { key: 'multi_user', label: '多人共用', preset: { minUsers: 5 } },
  { key: 'active_24h', label: '24小时活跃', preset: { active24h: true } },
  { key: 'source_login', label: '仅登录来源', preset: { source: 'login' } },
  { key: 'source_token', label: '仅令牌来源', preset: { source: 'token' } },
];

const INVITER_CHIP_DEFAULTS = {
  banState: '',
  suspiciousOnly: false,
  minInvites: 0,
  inviterStatus: '',
  recentInvitesWindow: 0,
  minRecentInvites: 0,
  minSharedIps: 0,
};
const INVITER_CHIPS = [
  { key: 'all', label: '全部', preset: {} },
  { key: 'needs_review', label: '需复核', preset: { suspiciousOnly: true } },
  { key: 'big_inviters', label: '大邀请人', preset: { minInvites: 10 } },
  {
    key: 'has_banned_invitees',
    label: '含已封禁被邀请人',
    preset: { banState: 'has_banned' },
  },
  {
    key: 'disabled_inviters',
    label: '已封禁邀请人',
    preset: { inviterStatus: 'disabled' },
  },
  {
    key: 'recent_bulk_invites',
    label: '短时大量邀请',
    preset: { recentInvitesWindow: 86400, minRecentInvites: 20 },
  },
  {
    key: 'many_shared_ips',
    label: '大量同 IP',
    preset: { minSharedIps: 3 },
  },
];

// 邀请人范围批量操作需要 include_inviter（batchStatus.js 不在本次写作用域，先本地封装）。
const submitInviterScopeStatus = async ({
  action,
  inviterId,
  includeInviter,
}) => {
  const res = await API.post(
    `/api/risk-control/inviters/${inviterId}/batch-status`,
    { action, include_inviter: Boolean(includeInviter) },
    { skipErrorHandler: true },
  );
  const payload = res?.data;
  if (!payload?.success) {
    return { ok: false, message: payload?.message || '' };
  }
  const data = payload.data || {};
  return {
    ok: true,
    updated: Number(data.updated) || 0,
    skipped: Array.isArray(data.skipped) ? data.skipped : [],
  };
};

// 邀请人范围「封禁/解禁全部」确认内容：默认包含邀请人本人，可取消勾选。
const InviterScopeConfirmContent = ({
  t,
  action,
  inviteeCount,
  includeInviterRef,
}) => {
  const [includeInviter, setIncludeInviter] = useState(true);
  const total = inviteeCount + (includeInviter ? 1 : 0);
  return (
    <div>
      <div>
        {action === 'disable'
          ? t('封禁全部被邀请账号')
          : t('解禁全部被邀请账号')}
      </div>
      <div className='mt-1 text-[var(--semi-color-text-2)]'>
        {t('共 {{total}} 个账号', { total })}
        {includeInviter ? `（${t('含邀请人本人')}）` : ''}
      </div>
      <div className='mt-1'>
        {action === 'disable'
          ? t('封禁后这些账号将无法登录和调用 API。')
          : t('解禁后这些账号将恢复访问。')}
      </div>
      <div className='mt-2'>
        <Checkbox
          defaultChecked
          onChange={(e) => {
            const checked = Boolean(e?.target?.checked);
            includeInviterRef.current = checked;
            setIncludeInviter(checked);
          }}
        >
          {action === 'disable'
            ? t('同时封禁邀请人本人')
            : t('同时解禁邀请人本人')}
        </Checkbox>
      </div>
    </div>
  );
};

// 同 IP 页签的关键词匹配：email 为后端新增的 search_type，其余沿用通用匹配。
const matchesSharedUserKeyword = (user, keyword, searchType) => {
  if (!keyword) return false;
  if (searchType === 'email') {
    return String(user?.email || '')
      .toLowerCase()
      .includes(String(keyword).trim().toLowerCase());
  }
  return userMatchesKeyword(user, keyword, searchType);
};

// 网段疑似标记：同网段账号数大于本行账号数时才展示。
const hasSuspiciousSubnet = (item) =>
  Boolean(item?.subnet) &&
  (Number(item?.subnet_user_count) || 0) > (Number(item?.user_count) || 0);

const parseSortValue = (value, fallbackSortBy) => {
  const [sortBy, order] = String(value || '').split(':');
  return [sortBy || fallbackSortBy, order === 'asc' ? 'asc' : 'desc'];
};

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

const SubnetTag = ({ item, t }) => {
  if (!hasSuspiciousSubnet(item)) {
    return <Text type='tertiary'>-</Text>;
  }
  return (
    <Tag color='orange' shape='circle' size='small'>
      {t('网段 {{subnet}} · {{count}} 账号', {
        subnet: item.subnet,
        count: Number(item.subnet_user_count),
      })}
    </Tag>
  );
};

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
        const matchedUsers = highlightKeyword
          ? (item.users || []).filter((user) =>
              matchesSharedUserKeyword(user, highlightKeyword, searchType),
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
            {hasSuspiciousSubnet(item) && (
              <div className='mt-2'>
                <SubnetTag item={item} t={t} />
              </div>
            )}
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
                {t('同 IP 组')}: {Number(item.shared_ip_count) || 0}
              </Tag>
              <Tag>
                {t('额度')}:{' '}
                {typeof item.quota === 'number' ? renderQuota(item.quota) : '-'}
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

  // 封禁状态筛选（两个页签共用同一个状态，切换页签时保持各自语义）：
  // '' 不筛选 / has_banned 有已封禁账号 / all_banned 全部已封禁。
  const [banState, setBanState] = useState('');

  // 同 IP 用户排序/筛选
  const [sharedSort, setSharedSort] = useState(DEFAULT_SHARED_SORT);
  const [minUsers, setMinUsers] = useState(DEFAULT_MIN_USERS);
  // 邀请关系排序
  const [inviterSort, setInviterSort] = useState(DEFAULT_INVITER_SORT);

  // 以下为分类 chip 对应的隐藏筛选参数（没有独立下拉框，点 chip 直接生效）。
  // 默认值即「不筛选」，保证「全部」与改造前行为一致。
  const [active24h, setActive24h] = useState(false);
  const [inviterSearchType, setInviterSearchType] = useState('any');
  const [suspiciousOnly, setSuspiciousOnly] = useState(false);
  const [minInvites, setMinInvites] = useState(0);
  const [inviterStatus, setInviterStatus] = useState('');
  // E.3 新增隐藏参数：短时大量邀请（窗口 + 人数）、大量同 IP。
  const [recentInvitesWindow, setRecentInvitesWindow] = useState(0);
  const [minRecentInvites, setMinRecentInvites] = useState(0);
  const [minSharedIps, setMinSharedIps] = useState(0);

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
      if (banState) {
        params.ban_state = banState;
      }
      if (tab === 'shared') {
        if (source !== 'all') {
          params.source = source;
        }
        params.search_type = searchType;
        params.min_users = minUsers;
        if (active24h) {
          params.active_24h = 'true';
        }
        const [sortBy, order] = parseSortValue(
          sharedSort,
          SHARED_SORT_FALLBACK,
        );
        params.sort_by = sortBy;
        params.order = order;
      } else {
        if (inviterSearchType === 'ip') {
          params.search_type = inviterSearchType;
        }
        if (suspiciousOnly) {
          params.suspicious_only = 'true';
        }
        if (minInvites > 1) {
          params.min_invites = minInvites;
        }
        if (inviterStatus) {
          params.inviter_status = inviterStatus;
        }
        if (recentInvitesWindow > 0 && minRecentInvites > 1) {
          params.recent_invites_window = recentInvitesWindow;
          params.min_recent_invites = minRecentInvites;
        }
        if (minSharedIps >= 1) {
          params.min_shared_ips = minSharedIps;
        }
        const [sortBy, order] = parseSortValue(
          inviterSort,
          INVITER_SORT_FALLBACK,
        );
        params.sort_by = sortBy;
        params.order = order;
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
    sharedSort,
    minUsers,
    inviterSort,
    banState,
    active24h,
    inviterSearchType,
    suspiciousOnly,
    minInvites,
    inviterStatus,
    recentInvitesWindow,
    minRecentInvites,
    minSharedIps,
  ]);

  const openAccountModal = useCallback((mode, item) => {
    setAccountModal({ visible: true, mode, item });
  }, []);

  const closeAccountModal = useCallback(() => {
    setAccountModal((prev) => ({ ...prev, visible: false }));
  }, []);

  const executeScopeAction = async (
    mode,
    item,
    action,
    includeInviter = false,
  ) => {
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
          : await submitInviterScopeStatus({
              action,
              inviterId: item?.inviter_id,
              includeInviter,
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

  const buildSharedScopeConfirmContent = (item, action) => {
    const scopeText =
      action === 'disable'
        ? t('封禁该 IP 下全部账号')
        : t('解禁该 IP 下全部账号');
    const scopeTotal = Number(item?.user_count) || 0;
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
        content: buildSharedScopeConfirmContent(item, action),
        centered: true,
        okText: action === 'disable' ? t('封禁') : t('解禁'),
        cancelText: t('取消'),
        okType: action === 'disable' ? 'danger' : 'primary',
        onOk: () => executeScopeAction(mode, item, action),
      });
      return;
    }
    const includeInviterRef = { current: true };
    Modal.confirm({
      title:
        action === 'disable'
          ? t('确认封禁该邀请人的全部被邀请账号')
          : t('确认解禁该邀请人的全部被邀请账号'),
      content: (
        <InviterScopeConfirmContent
          t={t}
          action={action}
          inviteeCount={Number(item?.direct_invite_count) || 0}
          includeInviterRef={includeInviterRef}
        />
      ),
      centered: true,
      okText: action === 'disable' ? t('封禁') : t('解禁'),
      cancelText: t('取消'),
      okType: action === 'disable' ? 'danger' : 'primary',
      onOk: () =>
        executeScopeAction(mode, item, action, includeInviterRef.current),
    });
  };

  const appliedKeyword = search.trim();
  const sharedHighlightActive =
    tab === 'shared' &&
    (searchType === 'username' ||
      searchType === 'user_id' ||
      searchType === 'email') &&
    appliedKeyword !== '';
  const inviterHighlightActive = tab === 'inviters' && appliedKeyword !== '';

  // chip 高亮由当前筛选值反推：手工调整下拉后若不再精确匹配任何预设，
  // 自动取消高亮（「预设」与「自定义」二选一里选取消高亮）。
  const sharedChipValues = { source, minUsers, banState, active24h };
  const inviterChipValues = {
    banState,
    suspiciousOnly,
    minInvites,
    inviterStatus,
    recentInvitesWindow,
    minRecentInvites,
    minSharedIps,
  };
  const activeSharedChip = SHARED_CHIPS.find((chip) => {
    const expected = { ...SHARED_CHIP_DEFAULTS, ...chip.preset };
    return Object.keys(expected).every(
      (key) => sharedChipValues[key] === expected[key],
    );
  });
  const activeInviterChip = INVITER_CHIPS.find((chip) => {
    const expected = { ...INVITER_CHIP_DEFAULTS, ...chip.preset };
    return Object.keys(expected).every(
      (key) => inviterChipValues[key] === expected[key],
    );
  });
  const activeChip = tab === 'shared' ? activeSharedChip : activeInviterChip;
  const currentChips = tab === 'shared' ? SHARED_CHIPS : INVITER_CHIPS;

  // 点击分类 chip：写入预设值，未在预设里出现的项回到该页签默认值。
  const applyChip = (chip) => {
    if (tab === 'shared') {
      const values = { ...SHARED_CHIP_DEFAULTS, ...chip.preset };
      setSource(values.source);
      setMinUsers(values.minUsers);
      setBanState(values.banState);
      setActive24h(values.active24h);
    } else {
      const values = { ...INVITER_CHIP_DEFAULTS, ...chip.preset };
      setBanState(values.banState);
      setSuspiciousOnly(values.suspiciousOnly);
      setMinInvites(values.minInvites);
      setInviterStatus(values.inviterStatus);
      setRecentInvitesWindow(values.recentInvitesWindow);
      setMinRecentInvites(values.minRecentInvites);
      setMinSharedIps(values.minSharedIps);
    }
    setPage(1);
  };

  // 重置筛选：清空关键词/筛选/排序与 chip 隐藏参数，分类回到「全部」。
  const resetFilters = () => {
    setKeyword('');
    setSearch('');
    setSource('all');
    setMinUsers(DEFAULT_MIN_USERS);
    setSharedSort(DEFAULT_SHARED_SORT);
    setInviterSort(DEFAULT_INVITER_SORT);
    setBanState('');
    setActive24h(false);
    setSuspiciousOnly(false);
    setMinInvites(0);
    setInviterStatus('');
    setRecentInvitesWindow(0);
    setMinRecentInvites(0);
    setMinSharedIps(0);
    setPage(1);
  };

  const applySearch = () => {
    setSearch(keyword.trim());
    setPage(1);
  };

  const keywordPlaceholder =
    tab === 'shared'
      ? searchType === 'username'
        ? t('搜索用户名/昵称')
        : searchType === 'user_id'
          ? t('搜索用户ID')
          : searchType === 'email'
            ? t('搜索邮箱')
            : t('搜索 IP')
      : inviterSearchType === 'ip'
        ? t('搜索 IP')
        : t('搜索用户 ID 或名称');

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
              matchesSharedUserKeyword(user, appliedKeyword, searchType),
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
    {
      title: t('网段'),
      dataIndex: 'subnet',
      width: 200,
      render: (_, item) => <SubnetTag item={item} t={t} />,
    },
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
    {
      title: t('同 IP 组'),
      dataIndex: 'shared_ip_count',
      width: 120,
      render: (value) => (
        <span className='tabular-nums'>
          {t('同 IP 组')}: {Number(value) || 0}
        </span>
      ),
    },
    {
      title: t('额度'),
      dataIndex: 'quota',
      width: 130,
      render: (value) => (
        <span className='tabular-nums'>
          {typeof value === 'number' ? renderQuota(value) : '-'}
        </span>
      ),
    },
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

          {/* 分类 chip：移动端单行横向滚动，桌面端正常换行；高亮由当前筛选值反推。 */}
          <div
            role='group'
            className='-mx-1 mt-3 flex gap-2 overflow-x-auto px-1 pb-1 scrollbar-hide md:flex-wrap md:overflow-visible'
          >
            {currentChips.map((chip) => {
              const active = activeChip?.key === chip.key;
              return (
                <Button
                  key={chip.key}
                  size='small'
                  type={active ? 'primary' : 'tertiary'}
                  theme={active ? 'solid' : 'light'}
                  className='shrink-0 !rounded-full whitespace-nowrap'
                  onClick={() => applyChip(chip)}
                >
                  {t(chip.label)}
                </Button>
              );
            })}
          </div>

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
              {tab === 'shared' ? (
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
                  <Select.Option value='email'>{t('搜索邮箱')}</Select.Option>
                </Select>
              ) : (
                <Select
                  value={inviterSearchType}
                  onChange={(value) => {
                    setInviterSearchType(value);
                    setPage(1);
                  }}
                  className='w-full sm:w-40'
                >
                  <Select.Option value='any'>{t('综合搜索')}</Select.Option>
                  <Select.Option value='ip'>{t('搜索 IP')}</Select.Option>
                </Select>
              )}
              <Input
                value={keyword}
                onChange={setKeyword}
                prefix={<Search size={16} />}
                className='w-full sm:w-64'
                placeholder={keywordPlaceholder}
                onEnterPress={applySearch}
              />
              <div className='flex w-full gap-2 sm:w-auto'>
                <Button className='flex-1 sm:flex-none' onClick={applySearch}>
                  {t('查询')}
                </Button>
                <Button className='flex-1 sm:flex-none' onClick={resetFilters}>
                  {t('重置筛选')}
                </Button>
              </div>
            </div>

            <div className='flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center'>
              <Select
                value={banState}
                onChange={(value) => {
                  setBanState(value || '');
                  setPage(1);
                }}
                className='w-full sm:w-56'
                // 用字符串前缀而非 JSX：Semi 只给字符串前缀加 semi-select-prefix-text
                // （含 12px 间距），JSX 前缀缺少该类名会与选中值贴在一起。
                prefix={t('封禁状态')}
              >
                <Select.Option value=''>{t('全部状态')}</Select.Option>
                <Select.Option value='has_banned'>{t('有已封禁账号')}</Select.Option>
                <Select.Option value='all_banned'>{t('全部已封禁')}</Select.Option>
              </Select>
              {tab === 'shared' ? (
                <Select
                  value={sharedSort}
                  onChange={(value) => {
                    setSharedSort(value);
                    setPage(1);
                  }}
                  className='w-full sm:w-72'
                  prefix={t('排序方式')}
                >
                  <Select.Option value='user_count:desc'>
                    {t('用户数从多到少')}
                  </Select.Option>
                  <Select.Option value='user_count:asc'>
                    {t('用户数从少到多')}
                  </Select.Option>
                  <Select.Option value='last_seen_at:desc'>
                    {t('最近出现优先')}
                  </Select.Option>
                  <Select.Option value='first_seen_at:asc'>
                    {t('最早出现优先')}
                  </Select.Option>
                  <Select.Option value='event_count:desc'>
                    {t('采样次数从多到少')}
                  </Select.Option>
                  <Select.Option value='event_count:asc'>
                    {t('采样次数从少到多')}
                  </Select.Option>
                  <Select.Option value='ip:asc'>
                    {t('按 IP 升序')}
                  </Select.Option>
                </Select>
              ) : (
                <Select
                  value={inviterSort}
                  onChange={(value) => {
                    setInviterSort(value);
                    setPage(1);
                  }}
                  className='w-full sm:w-72'
                  prefix={t('排序方式')}
                >
                  <Select.Option value='direct_invite_count:desc'>
                    {t('邀请人数从多到少')}
                  </Select.Option>
                  <Select.Option value='direct_invite_count:asc'>
                    {t('邀请人数从少到多')}
                  </Select.Option>
                  <Select.Option value='shared_ip_count:desc'>
                    {t('同 IP 邀请从多到少')}
                  </Select.Option>
                  <Select.Option value='aff_quota:desc'>
                    {t('可提现额度从多到少')}
                  </Select.Option>
                  <Select.Option value='inviter_id:asc'>
                    {t('邀请人 ID 升序')}
                  </Select.Option>
                  <Select.Option value='created_at:desc'>
                    {t('邀请人注册时间从新到旧')}
                  </Select.Option>
                </Select>
              )}
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
