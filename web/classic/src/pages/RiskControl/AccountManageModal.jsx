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
  Checkbox,
  Modal,
  Radio,
  RadioGroup,
  Select,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { API, renderQuota, showError, timestamp2string } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { getUserRowKey, isUserBanned } from './UserInlineList';
import { notifyBatchStatus, submitBatchStatus } from './batchStatus';

const { Text } = Typography;
const PAGE_SIZE = 20;
// 角色 >= 管理员（10）的账号前端置灰不可勾选，服务端仍会二次校验并返回 skipped。
const SELECTABLE_ROLE_LIMIT = 10;

// inviter 弹窗视图：ip / time 为分组卡片视图，flat 为原有平铺分页列表。
const VIEW_IP = 'ip';
const VIEW_TIME = 'time';
const VIEW_FLAT = 'flat';
const DEFAULT_TIME_WINDOW = 3600;
const GROUPS_LIMIT = 200;
const EMPTY_GROUPS_META = {
  total_users: 0,
  loaded_users: 0,
  truncated: false,
  groups_truncated: false,
  group_count: 0,
  time_window: DEFAULT_TIME_WINDOW,
};

const canSelectUser = (user) => Number(user?.role) < SELECTABLE_ROLE_LIMIT;

const renderRoleTag = (role, t) => {
  switch (Number(role)) {
    case 100:
      return (
        <Tag color='orange' shape='circle' size='small'>
          {t('超级管理员')}
        </Tag>
      );
    case 10:
      return (
        <Tag color='yellow' shape='circle' size='small'>
          {t('管理员')}
        </Tag>
      );
    case 1:
      return (
        <Tag color='blue' shape='circle' size='small'>
          {t('普通用户')}
        </Tag>
      );
    default:
      return (
        <Tag color='grey' shape='circle' size='small'>
          {t('访客')}
        </Tag>
      );
  }
};

const renderStatusTag = (user, t) =>
  isUserBanned(user) ? (
    <Tag color='red' shape='circle' size='small'>
      {t('已封禁')}
    </Tag>
  ) : (
    <Tag color='green' shape='circle' size='small'>
      {t('正常')}
    </Tag>
  );

const formatLastSeen = (user) =>
  user?.last_seen_at ? timestamp2string(user.last_seen_at) : '-';

/**
 * 关联账号管理弹窗。
 * - shared 模式：数据来自同 IP 行内 item.users（全量），前端分页，不发请求。
 * - inviter 模式：按页请求 GET /api/risk-control/inviters/:id/invitees，跨页保留选中。
 */
export default function AccountManageModal({
  visible,
  mode,
  item,
  t,
  onClose,
  onSuccess,
}) {
  const isMobile = useIsMobile();
  const isInviter = mode === 'inviter';
  const inviterId = item?.inviter_id;

  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState({});
  const [actionLoading, setActionLoading] = useState('');
  // 用于丢弃过期的分页响应，避免快速关闭/切换弹窗时旧请求覆盖新数据。
  const loadSeqRef = useRef(0);

  // inviter 模式分组视图状态（shared 模式不使用）。
  const [viewMode, setViewMode] = useState(VIEW_IP);
  const [timeWindow, setTimeWindow] = useState(DEFAULT_TIME_WINDOW);
  const [groupsLoading, setGroupsLoading] = useState(false);
  const [groups, setGroups] = useState([]);
  const [groupsError, setGroupsError] = useState(false);
  const [groupsMeta, setGroupsMeta] = useState(EMPTY_GROUPS_META);
  const [expandedGroups, setExpandedGroups] = useState({});
  const [groupActionLoading, setGroupActionLoading] = useState('');
  // 丢弃过期的分组响应，避免快速切换视图/时间窗口时旧响应覆盖新数据。
  const groupsSeqRef = useRef(0);

  // 分组视图（仅 inviter）与「封禁全部」使用的账号总数。
  const isGroupedView = isInviter && viewMode !== VIEW_FLAT;
  const effectiveTotal = isGroupedView
    ? Number(groupsMeta.total_users) || 0
    : total;

  const loadInvitees = async (targetPage) => {
    if (!inviterId) {
      setItems([]);
      setTotal(0);
      return;
    }
    const seq = ++loadSeqRef.current;
    setLoading(true);
    try {
      const res = await API.get(
        `/api/risk-control/inviters/${inviterId}/invitees`,
        {
          params: { p: targetPage, page_size: PAGE_SIZE },
          skipErrorHandler: true,
        },
      );
      if (seq !== loadSeqRef.current) {
        return;
      }
      const payload = res?.data;
      if (!payload?.success) {
        showError(payload?.message || t('加载失败'));
        setItems([]);
        setTotal(0);
        return;
      }
      const data = payload.data || {};
      setItems(Array.isArray(data.items) ? data.items : []);
      setTotal(Number(data.total) || 0);
      setPage(Number(data.page) || targetPage);
    } catch (error) {
      if (seq !== loadSeqRef.current) {
        return;
      }
      showError(error);
      setItems([]);
      setTotal(0);
    } finally {
      if (seq === loadSeqRef.current) {
        setLoading(false);
      }
    }
  };

  // 分组视图：GET /api/risk-control/inviters/:id/invitee-groups
  // groupBy: 'ip' | 'time'；time 模式需传 time_window。
  const loadGroups = async (groupBy, windowSeconds) => {
    if (!inviterId) {
      setGroups([]);
      setGroupsError(false);
      setGroupsMeta(EMPTY_GROUPS_META);
      return;
    }
    const seq = ++groupsSeqRef.current;
    setGroupsLoading(true);
    setGroupsError(false);
    try {
      const params = { group_by: groupBy, limit_groups: GROUPS_LIMIT };
      if (groupBy === VIEW_TIME) {
        params.time_window = windowSeconds || DEFAULT_TIME_WINDOW;
      }
      const res = await API.get(
        `/api/risk-control/inviters/${inviterId}/invitee-groups`,
        { params, skipErrorHandler: true },
      );
      if (seq !== groupsSeqRef.current) {
        return;
      }
      const payload = res?.data;
      if (!payload?.success) {
        showError(payload?.message || t('加载失败'));
        setGroups([]);
        setGroupsError(true);
        setGroupsMeta(EMPTY_GROUPS_META);
        return;
      }
      const data = payload.data || {};
      setGroups(Array.isArray(data.groups) ? data.groups : []);
      setGroupsMeta({
        total_users: Number(data.total_users) || 0,
        loaded_users: Number(data.loaded_users) || 0,
        truncated: Boolean(data.truncated),
        groups_truncated: Boolean(data.groups_truncated),
        group_count: Number(data.group_count) || 0,
        time_window: Number(data.time_window) || DEFAULT_TIME_WINDOW,
      });
    } catch (error) {
      if (seq !== groupsSeqRef.current) {
        return;
      }
      showError(error);
      setGroups([]);
      setGroupsError(true);
      setGroupsMeta(EMPTY_GROUPS_META);
    } finally {
      if (seq === groupsSeqRef.current) {
        setGroupsLoading(false);
      }
    }
  };

  useEffect(() => {
    if (!visible) {
      return;
    }
    setSelected({});
    setPage(1);
    setActionLoading('');
    setGroupActionLoading('');
    setExpandedGroups({});
    setGroups([]);
    setGroupsError(false);
    setGroupsMeta(EMPTY_GROUPS_META);
    if (mode === 'inviter') {
      // 默认进入「按 IP 分组」视图；平铺列表在用户切换时再加载。
      loadSeqRef.current += 1;
      setLoading(false);
      setItems([]);
      setTotal(0);
      setViewMode(VIEW_IP);
      setTimeWindow(DEFAULT_TIME_WINDOW);
      loadGroups(VIEW_IP, DEFAULT_TIME_WINDOW);
    } else {
      // 使邀请人模式下仍在途的请求失效。
      loadSeqRef.current += 1;
      groupsSeqRef.current += 1;
      setGroupsLoading(false);
      setLoading(false);
      const users = Array.isArray(item?.users) ? item.users : [];
      setItems(users);
      setTotal(users.length);
    }
    // item 只按身份字段变化触发重置，避免父列表刷新时打断当前弹窗操作。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible, mode, inviterId, item?.ip, item?.source]);

  const pagedItems = useMemo(() => {
    if (isInviter) {
      return items;
    }
    const start = (page - 1) * PAGE_SIZE;
    return items.slice(start, start + PAGE_SIZE);
  }, [isInviter, items, page]);

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(total / PAGE_SIZE)),
    [total],
  );

  const pageSelectable = useMemo(
    () => pagedItems.filter(canSelectUser),
    [pagedItems],
  );

  const selectedCount = Object.keys(selected).length;
  const pageSelectedCount = pageSelectable.filter(
    (user) => selected[String(getUserRowKey(user))],
  ).length;
  const allPageSelected =
    pageSelectable.length > 0 && pageSelectedCount === pageSelectable.length;
  const somePageSelected = pageSelectedCount > 0 && !allPageSelected;

  const setUserSelected = (user, checked) => {
    const key = String(getUserRowKey(user));
    setSelected((prev) => {
      const next = { ...prev };
      if (checked) {
        next[key] = user;
      } else {
        delete next[key];
      }
      return next;
    });
  };

  const setPageSelected = (checked) => {
    setSelected((prev) => {
      const next = { ...prev };
      pageSelectable.forEach((user) => {
        const key = String(getUserRowKey(user));
        if (checked) {
          next[key] = user;
        } else {
          delete next[key];
        }
      });
      return next;
    });
  };

  // 分组视图：全选/取消全选某一组内可勾选的用户。
  const setUsersSelected = (users, checked) => {
    setSelected((prev) => {
      const next = { ...prev };
      (Array.isArray(users) ? users : []).forEach((user) => {
        if (!canSelectUser(user)) {
          return;
        }
        const key = String(getUserRowKey(user));
        if (checked) {
          next[key] = user;
        } else {
          delete next[key];
        }
      });
      return next;
    });
  };

  const goToPage = (targetPage) => {
    const nextPage = Math.min(Math.max(1, targetPage), totalPages);
    if (nextPage === page) {
      return;
    }
    setPage(nextPage);
    if (isInviter) {
      loadInvitees(nextPage);
    }
  };

  const runBatch = async (action, scope) => {
    setActionLoading(`${scope}-${action}`);
    try {
      let result;
      if (scope === 'selected') {
        const userIds = Object.values(selected)
          .map((user) => getUserRowKey(user))
          .filter((userId) => Number(userId) > 0);
        if (userIds.length === 0) {
          return false;
        }
        result = await submitBatchStatus({
          action,
          scope: 'users',
          userIds,
        });
      } else if (isInviter) {
        result = await submitBatchStatus({
          action,
          scope: 'inviter',
          inviterId,
        });
      } else {
        result = await submitBatchStatus({
          action,
          scope: 'shared_ip',
          ip: item?.ip,
          source: item?.source,
        });
      }
      if (!result.ok) {
        showError(result.message || t('操作失败'));
        return false;
      }
      notifyBatchStatus(action, result, t);
      if (typeof onSuccess === 'function') {
        onSuccess();
      }
      if (typeof onClose === 'function') {
        onClose();
      }
      return true;
    } catch (error) {
      showError(error);
      return false;
    } finally {
      setActionLoading('');
    }
  };

  const confirmBatch = (title, content, action, scope) => {
    Modal.confirm({
      title,
      content,
      centered: true,
      okText: action === 'disable' ? t('封禁') : t('解禁'),
      cancelText: t('取消'),
      okType: action === 'disable' ? 'danger' : 'primary',
      onOk: () => runBatch(action, scope),
    });
  };

  const buildSelectedConfirmContent = (action) => (
    <div>
      <div className='text-[var(--semi-color-text-2)]'>
        {t('已选 {{count}} 项', { count: selectedCount })}
      </div>
      <div className='mt-1'>
        {action === 'disable'
          ? t('封禁后这些账号将无法登录和调用 API。')
          : t('解禁后这些账号将恢复访问。')}
      </div>
    </div>
  );

  const buildAllConfirmContent = (action) => {
    const scopeText = isInviter
      ? action === 'disable'
        ? t('封禁全部被邀请账号')
        : t('解禁全部被邀请账号')
      : action === 'disable'
        ? t('封禁该 IP 下全部账号')
        : t('解禁该 IP 下全部账号');
    return (
      <div>
        <div>{scopeText}</div>
        <div className='mt-1 text-[var(--semi-color-text-2)]'>
          {t('共 {{total}} 个账号', { total: effectiveTotal })}
        </div>
        <div className='mt-1'>
          {action === 'disable'
            ? t('封禁后这些账号将无法登录和调用 API。')
            : t('解禁后这些账号将恢复访问。')}
        </div>
      </div>
    );
  };

  const confirmSelected = (action) => {
    if (action === 'disable') {
      confirmBatch(
        t('确认封禁'),
        buildSelectedConfirmContent(action),
        action,
        'selected',
      );
      return;
    }
    confirmBatch(
      t('确认解禁'),
      buildSelectedConfirmContent(action),
      action,
      'selected',
    );
  };

  const confirmAll = (action) => {
    if (isInviter) {
      confirmBatch(
        action === 'disable'
          ? t('确认封禁该邀请人的全部被邀请账号')
          : t('确认解禁该邀请人的全部被邀请账号'),
        buildAllConfirmContent(action),
        action,
        'all',
      );
      return;
    }
    confirmBatch(
      action === 'disable'
        ? t('确认封禁该 IP 下全部账号')
        : t('确认解禁该 IP 下全部账号'),
      buildAllConfirmContent(action),
      action,
      'all',
    );
  };

  // 分组卡片标题：无 IP / 时间段 / 网段（或 IP）。
  const formatGroupTitle = (group) => {
    if (!group) {
      return '-';
    }
    if (group.kind === 'no_ip') {
      return t('无 IP 记录');
    }
    if (group.kind === 'time') {
      const start = Number(group.bucket_start) || 0;
      if (!start) {
        return '-';
      }
      const windowSeconds =
        Number(groupsMeta.time_window) ||
        Number(timeWindow) ||
        DEFAULT_TIME_WINDOW;
      const end = start + Math.max(windowSeconds - 1, 0);
      return `${timestamp2string(start)} ~ ${timestamp2string(end)}`;
    }
    const label = String(group.label || '').trim();
    return label || group.key || '-';
  };

  const toggleGroup = (key) => {
    if (!key) {
      return;
    }
    setExpandedGroups((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  const handleChangeView = (nextView) => {
    if (!nextView || nextView === viewMode) {
      return;
    }
    setViewMode(nextView);
    setSelected({});
    setExpandedGroups({});
    setGroupActionLoading('');
    if (nextView === VIEW_FLAT) {
      setPage(1);
      loadInvitees(1);
      return;
    }
    setGroups([]);
    setGroupsError(false);
    setGroupsMeta(EMPTY_GROUPS_META);
    loadGroups(nextView, timeWindow);
  };

  const handleChangeTimeWindow = (value) => {
    const nextWindow = Number(value) || DEFAULT_TIME_WINDOW;
    if (nextWindow === timeWindow) {
      return;
    }
    setTimeWindow(nextWindow);
    setSelected({});
    setExpandedGroups({});
    setGroups([]);
    setGroupsError(false);
    setGroupsMeta(EMPTY_GROUPS_META);
    loadGroups(VIEW_TIME, nextWindow);
  };

  // 分组批量封禁/解禁：直接复用 users 范围批量接口。
  const runGroupBatch = async (group, action) => {
    const userIds = (Array.isArray(group?.user_ids) ? group.user_ids : [])
      .map((userId) => Number(userId))
      .filter((userId) => userId > 0);
    if (!group || group.user_ids_truncated || userIds.length === 0) {
      return false;
    }
    setGroupActionLoading(`${group.key}:${action}`);
    try {
      const result = await submitBatchStatus({
        action,
        scope: 'users',
        userIds,
      });
      if (!result.ok) {
        showError(result.message || t('操作失败'));
        return false;
      }
      notifyBatchStatus(action, result, t);
      if (typeof onSuccess === 'function') {
        onSuccess();
      }
      await loadGroups(
        viewMode === VIEW_TIME ? VIEW_TIME : VIEW_IP,
        viewMode === VIEW_TIME ? timeWindow : DEFAULT_TIME_WINDOW,
      );
      return true;
    } catch (error) {
      showError(error);
      return false;
    } finally {
      setGroupActionLoading('');
    }
  };

  const confirmGroupBatch = (group, action) => {
    const groupUserCount =
      Number(group?.user_count) ||
      (Array.isArray(group?.user_ids) ? group.user_ids.length : 0);
    Modal.confirm({
      title:
        action === 'disable'
          ? t('确认封禁该组账号')
          : t('确认解禁该组账号'),
      content: (
        <div>
          <div>{formatGroupTitle(group)}</div>
          <div className='mt-1 text-[var(--semi-color-text-2)]'>
            {t('共 {{total}} 个账号', { total: groupUserCount })}
          </div>
          <div className='mt-1'>
            {action === 'disable'
              ? t('封禁后这些账号将无法登录和调用 API。')
              : t('解禁后这些账号将恢复访问。')}
          </div>
        </div>
      ),
      centered: true,
      okText: action === 'disable' ? t('封禁') : t('解禁'),
      cancelText: t('取消'),
      okType: action === 'disable' ? 'danger' : 'primary',
      onOk: () => runGroupBatch(group, action),
    });
  };

  const buildGroupRowSelection = (groupUsers) => ({
    selectedRowKeys: Object.keys(selected).map((key) => Number(key)),
    getCheckboxProps: (record) => ({ disabled: !canSelectUser(record) }),
    onSelect: (record, checked) => setUserSelected(record, checked),
    onSelectAll: (checked) => setUsersSelected(groupUsers, checked),
  });

  const columns = useMemo(
    () => [
      {
        title: t('用户ID'),
        dataIndex: 'user_id',
        width: 80,
        render: (value, record) => value ?? getUserRowKey(record) ?? '-',
      },
      {
        title: t('用户名'),
        dataIndex: 'username',
        width: 150,
        render: (value, record) => (
          <div className='min-w-0'>
            <div className='truncate font-medium'>{value || '-'}</div>
            {record?.display_name ? (
              <div className='truncate text-xs text-[var(--semi-color-text-2)]'>
                {record.display_name}
              </div>
            ) : null}
          </div>
        ),
      },
      {
        title: t('邮箱'),
        dataIndex: 'email',
        render: (value) => <span className='break-all'>{value || '-'}</span>,
      },
      {
        title: t('状态'),
        width: 100,
        render: (_, record) => renderStatusTag(record, t),
      },
      {
        title: t('额度'),
        dataIndex: 'quota',
        width: 110,
        render: (value) => (
          <span className='tabular-nums'>
            {typeof value === 'number' ? renderQuota(value) : '-'}
          </span>
        ),
      },
      {
        title: t('角色'),
        dataIndex: 'role',
        width: 110,
        render: (value) => renderRoleTag(value, t),
      },
      {
        title: t('最近出现'),
        dataIndex: 'last_seen_at',
        width: 160,
        render: (_, record) => formatLastSeen(record),
      },
    ],
    [t],
  );

  const rowSelection = {
    selectedRowKeys: Object.keys(selected).map((key) => Number(key)),
    getCheckboxProps: (record) => ({ disabled: !canSelectUser(record) }),
    onSelect: (record, checked) => setUserSelected(record, checked),
    onSelectAll: (checked) => setPageSelected(checked),
  };

  const sourceText =
    item?.source === 'login'
      ? t('登录')
      : item?.source === 'token'
        ? t('令牌')
        : t('全部来源');
  const subtitle = isInviter
    ? `#${item?.inviter_id ?? '-'} ${item?.username || ''}`.trim()
    : `${item?.ip || ''} · ${sourceText}`;

  // 分组视图：可折叠卡片 + 展开后的成员列表（桌面表格 / 手机卡片）。
  const renderGroupCard = (group) => {
    const groupKey = group?.key || '';
    const expanded = Boolean(expandedGroups[groupKey]);
    const groupUsers = Array.isArray(group?.users) ? group.users : [];
    const userCount = Number(group?.user_count) || groupUsers.length;
    const bannedCount = Number(group?.banned_count) || 0;
    const userIdsCount = Array.isArray(group?.user_ids)
      ? group.user_ids.length
      : 0;
    const truncated = Boolean(group?.user_ids_truncated);
    const groupBusy = actionLoading !== '' || groupActionLoading !== '';
    return (
      <div
        key={groupKey}
        className='rounded-lg border border-[var(--semi-color-border)]'
      >
        <div className='flex flex-col gap-2 p-3 md:flex-row md:items-center md:justify-between'>
          <button
            type='button'
            className='flex min-w-0 flex-1 items-start gap-2 text-left'
            onClick={() => toggleGroup(groupKey)}
          >
            <span className='mt-0.5 shrink-0 text-[var(--semi-color-text-2)]'>
              {expanded ? (
                <ChevronDown size={16} />
              ) : (
                <ChevronRight size={16} />
              )}
            </span>
            <span className='min-w-0'>
              <span className='flex flex-wrap items-center gap-2'>
                <Text strong className='break-all'>
                  {formatGroupTitle(group)}
                </Text>
                <Tag color='white' shape='circle' size='small'>
                  {t('{{count}} 人', { count: userCount })}
                </Tag>
                <Tag
                  color={bannedCount > 0 ? 'red' : 'white'}
                  shape='circle'
                  size='small'
                >
                  {t('已封禁 {{count}} 人', { count: bannedCount })}
                </Tag>
              </span>
              {truncated && (
                <span className='mt-1 block text-xs text-[var(--semi-color-warning)]'>
                  {t('本组人数过多，请用平铺列表处理')}
                </span>
              )}
            </span>
          </button>
          <div className='flex shrink-0 flex-wrap gap-2'>
            <Button
              size='small'
              type='danger'
              theme='light'
              loading={groupActionLoading === `${groupKey}:disable`}
              disabled={
                truncated || userCount === 0 || userIdsCount === 0 || groupBusy
              }
              onClick={() => confirmGroupBatch(group, 'disable')}
            >
              {t('封禁本组')}
            </Button>
            <Button
              size='small'
              type='primary'
              theme='light'
              loading={groupActionLoading === `${groupKey}:enable`}
              disabled={
                truncated || userCount === 0 || userIdsCount === 0 || groupBusy
              }
              onClick={() => confirmGroupBatch(group, 'enable')}
            >
              {t('解禁本组')}
            </Button>
          </div>
        </div>
        {expanded && (
          <div className='border-t border-[var(--semi-color-border)] p-3'>
            {userCount > groupUsers.length && (
              <div className='mb-2 text-xs text-[var(--semi-color-text-2)]'>
                {t('仅显示前 {{count}} 人', { count: groupUsers.length })}
              </div>
            )}
            <div className='hidden md:block'>
              <Table
                columns={columns}
                dataSource={groupUsers}
                rowKey={(record) => getUserRowKey(record)}
                rowSelection={buildGroupRowSelection(groupUsers)}
                pagination={false}
                size='small'
                scroll={{ x: 'max-content' }}
                empty={t('暂无数据')}
              />
            </div>
            <div className='space-y-2 md:hidden'>
              {groupUsers.length === 0 ? (
                <div className='py-4 text-center text-sm text-[var(--semi-color-text-2)]'>
                  {t('暂无数据')}
                </div>
              ) : (
                groupUsers.map((user) => {
                  const key = getUserRowKey(user);
                  return (
                    <div
                      key={key}
                      className='flex items-start gap-3 rounded-lg border border-[var(--semi-color-border)] p-3'
                    >
                      <Checkbox
                        checked={Boolean(selected[String(key)])}
                        disabled={!canSelectUser(user)}
                        onChange={(e) =>
                          setUserSelected(user, Boolean(e?.target?.checked))
                        }
                      />
                      <div className='min-w-0 flex-1'>
                        <div className='flex items-start justify-between gap-2'>
                          <div className='min-w-0'>
                            <div className='truncate font-medium'>
                              #{key} {user.username || '-'}
                            </div>
                            <div className='truncate text-xs text-[var(--semi-color-text-2)]'>
                              {user.email || '-'}
                            </div>
                          </div>
                          <div className='flex shrink-0 flex-col items-end gap-1'>
                            {renderStatusTag(user, t)}
                            {renderRoleTag(user.role, t)}
                          </div>
                        </div>
                        <div className='mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-[var(--semi-color-text-2)]'>
                          <span>
                            {t('额度')}:{' '}
                            <span className='tabular-nums text-[var(--semi-color-text-0)]'>
                              {typeof user.quota === 'number'
                                ? renderQuota(user.quota)
                                : '-'}
                            </span>
                          </span>
                          <span>
                            {t('最近出现')}: {formatLastSeen(user)}
                          </span>
                        </div>
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>
        )}
      </div>
    );
  };

  const footer = (
    <div className='flex flex-col gap-2 md:flex-row md:items-center md:justify-between'>
      <Text type='secondary'>{t('已选 {{count}} 项', { count: selectedCount })}</Text>
      <div className='flex flex-wrap gap-2'>
        <Button
          type='danger'
          theme='light'
          loading={actionLoading === 'selected-disable'}
          disabled={
            selectedCount === 0 ||
            actionLoading !== '' ||
            groupActionLoading !== ''
          }
          onClick={() => confirmSelected('disable')}
        >
          {t('封禁选中')}
        </Button>
        <Button
          type='primary'
          theme='light'
          loading={actionLoading === 'selected-enable'}
          disabled={
            selectedCount === 0 ||
            actionLoading !== '' ||
            groupActionLoading !== ''
          }
          onClick={() => confirmSelected('enable')}
        >
          {t('解禁选中')}
        </Button>
        <Button
          type='danger'
          theme='borderless'
          loading={actionLoading === 'all-disable'}
          disabled={
            effectiveTotal === 0 ||
            actionLoading !== '' ||
            groupActionLoading !== ''
          }
          onClick={() => confirmAll('disable')}
        >
          {t('封禁全部')}
        </Button>
        <Button
          type='primary'
          theme='borderless'
          loading={actionLoading === 'all-enable'}
          disabled={
            effectiveTotal === 0 ||
            actionLoading !== '' ||
            groupActionLoading !== ''
          }
          onClick={() => confirmAll('enable')}
        >
          {t('解禁全部')}
        </Button>
        <Button theme='borderless' onClick={onClose}>
          {t('关闭')}
        </Button>
      </div>
    </div>
  );

  return (
    <Modal
      visible={visible}
      centered
      width={isMobile ? '100%' : 960}
      title={
        <div className='flex min-w-0 flex-col'>
          <span>{t('关联账号')}</span>
          <span className='truncate text-xs font-normal text-[var(--semi-color-text-2)]'>
            {subtitle}
          </span>
        </div>
      }
      bodyStyle={{ padding: 16, maxHeight: '62vh', overflowY: 'auto' }}
      onCancel={onClose}
      footer={footer}
      maskClosable={actionLoading === '' && groupActionLoading === ''}
      closeOnEsc={actionLoading === '' && groupActionLoading === ''}
    >
      <div>
        {isInviter && (
          <div className='mb-3 flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between'>
            <div className='min-w-0 overflow-x-auto'>
              <RadioGroup
                type='button'
                value={viewMode}
                onChange={(e) => handleChangeView(e.target.value)}
              >
                <Radio value={VIEW_IP}>{t('按 IP 分组')}</Radio>
                <Radio value={VIEW_TIME}>{t('按注册时间')}</Radio>
                <Radio value={VIEW_FLAT}>{t('平铺列表')}</Radio>
              </RadioGroup>
            </div>
            {viewMode === VIEW_TIME && (
              <div className='flex shrink-0 items-center gap-2'>
                <Text type='secondary' size='small'>
                  {t('时间窗口')}
                </Text>
                <Select
                  value={timeWindow}
                  onChange={handleChangeTimeWindow}
                  className='w-28'
                >
                  <Select.Option value={3600}>{t('1小时')}</Select.Option>
                  <Select.Option value={21600}>{t('6小时')}</Select.Option>
                  <Select.Option value={86400}>{t('1天')}</Select.Option>
                </Select>
              </div>
            )}
          </div>
        )}

        {isGroupedView ? (
          <>
            <div className='mb-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-[var(--semi-color-text-2)]'>
              <span>{t('共 {{total}} 个账号', { total: effectiveTotal })}</span>
              {groupsMeta.truncated && (
                <span className='text-[var(--semi-color-warning)]'>
                  {t('数据量过大，已加载前 2000 人')}
                </span>
              )}
              {groupsMeta.groups_truncated && (
                <span className='text-[var(--semi-color-warning)]'>
                  {t('分组过多，仅显示前 {{count}} 组', {
                    count: groups.length,
                  })}
                </span>
              )}
            </div>
            <Spin spinning={groupsLoading}>
              {groupsLoading && groups.length === 0 ? (
                <div className='py-8 text-center text-sm text-[var(--semi-color-text-2)]'>
                  {t('加载中')}
                </div>
              ) : groupsError ? (
                <div className='py-8 text-center text-sm text-[var(--semi-color-text-2)]'>
                  <div>{t('加载失败')}</div>
                  <Button
                    size='small'
                    theme='borderless'
                    onClick={() =>
                      loadGroups(
                        viewMode === VIEW_TIME ? VIEW_TIME : VIEW_IP,
                        timeWindow,
                      )
                    }
                  >
                    {t('重试')}
                  </Button>
                </div>
              ) : groups.length === 0 ? (
                <div className='py-8 text-center text-sm text-[var(--semi-color-text-2)]'>
                  {t('暂无数据')}
                </div>
              ) : (
                <div className='space-y-2'>
                  {groups.map((group) => renderGroupCard(group))}
                </div>
              )}
            </Spin>
          </>
        ) : (
          <>
            <div className='mb-3 flex flex-wrap items-center justify-between gap-2'>
              <Checkbox
                checked={allPageSelected}
            indeterminate={somePageSelected}
            disabled={pageSelectable.length === 0 || actionLoading !== ''}
            onChange={(e) => setPageSelected(Boolean(e?.target?.checked))}
          >
            {t('全选本页')}
          </Checkbox>
          <div className='flex flex-wrap items-center gap-2'>
            <Button
              size='small'
              theme='borderless'
              disabled={page <= 1 || actionLoading !== ''}
              onClick={() => goToPage(page - 1)}
            >
              {t('上一页')}
            </Button>
            <Text type='secondary' size='small'>
              {t('共 {{total}} 个账号', { total })}
            </Text>
            <Button
              size='small'
              theme='borderless'
              disabled={page >= totalPages || actionLoading !== ''}
              onClick={() => goToPage(page + 1)}
            >
              {t('下一页')}
            </Button>
          </div>
        </div>

        <Spin spinning={loading}>
          <div className='hidden md:block'>
            <Table
              columns={columns}
              dataSource={pagedItems}
              rowKey={(record) => getUserRowKey(record)}
              rowSelection={rowSelection}
              pagination={false}
              size='small'
              scroll={{ x: 'max-content' }}
              empty={t('暂无数据')}
            />
          </div>

          <div className='space-y-2 md:hidden'>
            {loading && pagedItems.length === 0 ? (
              <div className='py-8 text-center text-sm text-[var(--semi-color-text-2)]'>
                {t('加载中')}
              </div>
            ) : pagedItems.length === 0 ? (
              <div className='py-8 text-center text-sm text-[var(--semi-color-text-2)]'>
                {t('暂无数据')}
              </div>
            ) : (
              pagedItems.map((user) => {
                const key = getUserRowKey(user);
                return (
                  <div
                    key={key}
                    className='flex items-start gap-3 rounded-lg border border-[var(--semi-color-border)] p-3'
                  >
                    <Checkbox
                      checked={Boolean(selected[String(key)])}
                      disabled={!canSelectUser(user)}
                      onChange={(e) =>
                        setUserSelected(user, Boolean(e?.target?.checked))
                      }
                    />
                    <div className='min-w-0 flex-1'>
                      <div className='flex items-start justify-between gap-2'>
                        <div className='min-w-0'>
                          <div className='truncate font-medium'>
                            #{key} {user.username || '-'}
                          </div>
                          <div className='truncate text-xs text-[var(--semi-color-text-2)]'>
                            {user.email || '-'}
                          </div>
                        </div>
                        <div className='flex shrink-0 flex-col items-end gap-1'>
                          {renderStatusTag(user, t)}
                          {renderRoleTag(user.role, t)}
                        </div>
                      </div>
                      <div className='mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-[var(--semi-color-text-2)]'>
                        <span>
                          {t('额度')}:{' '}
                          <span className='tabular-nums text-[var(--semi-color-text-0)]'>
                            {typeof user.quota === 'number'
                              ? renderQuota(user.quota)
                              : '-'}
                          </span>
                        </span>
                        <span>
                          {t('最近出现')}: {formatLastSeen(user)}
                        </span>
                      </div>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </Spin>
          </>
        )}
      </div>
    </Modal>
  );
}
