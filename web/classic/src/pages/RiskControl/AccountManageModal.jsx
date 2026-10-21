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

// 用户表 9 列固定宽度之和（1100）+ 行选择列 48 = 1148。
// 弹窗宽 1200 时表格可视区约 1103px，横向仅剩约 45px，配合左侧固定列即可。
const TABLE_SCROLL_X = 1148;
// 桌面弹窗宽度自适应：1440 屏取满 1200；1024 等窄屏自动收窄到视口 - 64px，
// 保证左右各留 32px 边距，弹窗不会被视口裁掉（1440 下计算结果仍为 1200）。
const MODAL_DESKTOP_WIDTH = 'min(1200px, calc(100vw - 64px))';

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

// 用户活跃度判据（严格按契约 H 节表格，判断顺序即优先级）。
const getActivityMeta = (user) => {
  const requestCount = Number(user?.request_count) || 0;
  const usedQuota = Number(user?.used_quota) || 0;
  const checkinCount = Number(user?.checkin_count) || 0;
  if (requestCount === 0 && usedQuota === 0 && checkinCount === 0) {
    return { key: 'zombie', label: '僵尸号', color: 'red' };
  }
  if (requestCount === 0 && checkinCount > 0) {
    return { key: 'checkin_only', label: '只签到未调用', color: 'orange' };
  }
  if (requestCount === 0 && usedQuota > 0) {
    return { key: 'quota_only', label: '有消耗无请求', color: 'yellow' };
  }
  return { key: 'active', label: '有调用', color: 'green' };
};

const sumUserField = (users, field) =>
  (Array.isArray(users) ? users : []).reduce(
    (sum, user) => sum + (Number(user?.[field]) || 0),
    0,
  );

// 合计金额为空（<=0）时显示 -，不显示 0 造成误解。
const formatQuotaTotal = (value) =>
  Number(value) > 0 ? renderQuota(Number(value)) : '-';

const renderActivityTag = (user, t) => {
  const meta = getActivityMeta(user);
  return (
    <Tag color={meta.color} shape='circle' size='small'>
      {t(meta.label)}
    </Tag>
  );
};

const renderRechargeCell = (user, t) => {
  const rechargeQuota = Number(user?.recharge_quota) || 0;
  if (rechargeQuota <= 0) {
    return <Text type='tertiary'>-</Text>;
  }
  const rechargeCount = Number(user?.recharge_count) || 0;
  return (
    <span className='tabular-nums'>
      {renderQuota(rechargeQuota)}
      <span className='ml-1 text-xs text-[var(--semi-color-text-2)]'>
        {t('（{{count}} 次）', { count: rechargeCount })}
      </span>
    </span>
  );
};

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

// 「封禁/解禁全部」确认内容（邀请人范围）：默认同时处理邀请人本人。
const InviterAllConfirmContent = ({
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

// 手机端用户卡片：状态/角色/活跃度 + 额度/充值/历史消耗/请求数（桌面表格的移动端等价）。
const MobileUserCard = ({ user, selected, onSelect, t }) => {
  const key = getUserRowKey(user);
  const usedQuota = Number(user?.used_quota) || 0;
  const requestCount = Number(user?.request_count) || 0;
  return (
    <div className='flex items-start gap-3 rounded-lg border border-[var(--semi-color-border)] p-3'>
      <Checkbox
        checked={Boolean(selected[String(key)])}
        disabled={!canSelectUser(user)}
        onChange={(e) => onSelect(user, Boolean(e?.target?.checked))}
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
            {renderActivityTag(user, t)}
          </div>
        </div>
        <div className='mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-[var(--semi-color-text-2)]'>
          <span className='flex items-center gap-1'>
            {t('充值')}: {renderRechargeCell(user, t)}
          </span>
          <span>
            {t('额度')}:{' '}
            <span className='tabular-nums text-[var(--semi-color-text-0)]'>
              {typeof user.quota === 'number' ? renderQuota(user.quota) : '-'}
            </span>
          </span>
          <span>
            {t('历史消耗')}:{' '}
            <span className='tabular-nums text-[var(--semi-color-text-0)]'>
              {usedQuota > 0 ? renderQuota(usedQuota) : '-'}
            </span>
          </span>
          <span>
            {t('请求数')}: {requestCount}
          </span>
          <span>
            {t('最近出现')}: {formatLastSeen(user)}
          </span>
        </div>
      </div>
    </div>
  );
};

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

  // inviter 模式视图状态（shared 模式不使用）。默认「平铺列表」（契约 G）。
  const [viewMode, setViewMode] = useState(VIEW_FLAT);
  const [timeWindow, setTimeWindow] = useState(DEFAULT_TIME_WINDOW);
  const [groupsLoading, setGroupsLoading] = useState(false);
  const [groups, setGroups] = useState([]);
  const [groupsError, setGroupsError] = useState(false);
  const [groupsMeta, setGroupsMeta] = useState(EMPTY_GROUPS_META);
  const [expandedGroups, setExpandedGroups] = useState({});
  const [groupActionLoading, setGroupActionLoading] = useState('');
  // 快捷筛选「仅看零充值」：只做本地过滤，不发请求（契约 I）。
  const [zeroRechargeOnly, setZeroRechargeOnly] = useState(false);
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
    setZeroRechargeOnly(false);
    if (mode === 'inviter') {
      // 默认进入「平铺列表」视图（契约 G）；分组视图在用户切换时再加载。
      groupsSeqRef.current += 1;
      setGroupsLoading(false);
      setViewMode(VIEW_FLAT);
      setTimeWindow(DEFAULT_TIME_WINDOW);
      loadInvitees(1);
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

  // 「仅看零充值」只过滤当前页/当前组，不发请求（契约 I）。
  const displayedFlatItems = useMemo(
    () =>
      zeroRechargeOnly
        ? pagedItems.filter((user) => (Number(user?.recharge_quota) || 0) <= 0)
        : pagedItems,
    [pagedItems, zeroRechargeOnly],
  );

  // 弹窗头部合计（契约 C）：基于当前已加载数据本地求和，分组视图汇总已加载成员。
  const summaryUsers = useMemo(() => {
    const base = isGroupedView
      ? groups.flatMap((group) =>
          Array.isArray(group?.users) ? group.users : [],
        )
      : items;
    if (!zeroRechargeOnly) {
      return base;
    }
    return base.filter((user) => (Number(user?.recharge_quota) || 0) <= 0);
  }, [isGroupedView, groups, items, zeroRechargeOnly]);
  const summaryTotal = isGroupedView
    ? Number(groupsMeta.total_users) || 0
    : total;
  const summaryRecharge = sumUserField(summaryUsers, 'recharge_quota');
  const summaryUsed = sumUserField(summaryUsers, 'used_quota');

  const pageSelectable = useMemo(
    () => displayedFlatItems.filter(canSelectUser),
    [displayedFlatItems],
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

  const runBatch = async (action, scope, includeInviter = false) => {
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
        result = await submitInviterScopeStatus({
          action,
          inviterId,
          includeInviter,
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
    const scopeText =
      action === 'disable'
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
      const includeInviterRef = { current: true };
      const inviteeCount = Number(effectiveTotal) || 0;
      Modal.confirm({
        title:
          action === 'disable'
            ? t('确认封禁该邀请人的全部被邀请账号')
            : t('确认解禁该邀请人的全部被邀请账号'),
        content: (
          <InviterAllConfirmContent
            t={t}
            action={action}
            inviteeCount={inviteeCount}
            includeInviterRef={includeInviterRef}
          />
        ),
        centered: true,
        okText: action === 'disable' ? t('封禁') : t('解禁'),
        cancelText: t('取消'),
        okType: action === 'disable' ? 'danger' : 'primary',
        onOk: () => runBatch(action, 'all', includeInviterRef.current),
      });
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
    fixed: 'left',
    width: 48,
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
        fixed: 'left',
        render: (value, record) => value ?? getUserRowKey(record) ?? '-',
      },
      {
        title: t('用户名'),
        dataIndex: 'username',
        width: 120,
        fixed: 'left',
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
        width: 150,
        render: (value) => <span className='break-all'>{value || '-'}</span>,
      },
      {
        title: t('状态'),
        width: 88,
        render: (_, record) => renderStatusTag(record, t),
      },
      {
        title: t('活跃度'),
        width: 160,
        render: (_, record) => {
          const usedQuota = Number(record?.used_quota) || 0;
          const requestCount = Number(record?.request_count) || 0;
          return (
            <div className='min-w-0'>
              {renderActivityTag(record, t)}
              <div className='mt-1 text-xs text-[var(--semi-color-text-2)]'>
                {t('历史消耗')}:{' '}
                <span className='tabular-nums'>
                  {usedQuota > 0 ? renderQuota(usedQuota) : '-'}
                </span>
              </div>
              <div className='text-xs text-[var(--semi-color-text-2)]'>
                {t('请求数')}: {requestCount}
              </div>
            </div>
          );
        },
      },
      {
        title: t('充值'),
        dataIndex: 'recharge_quota',
        width: 140,
        render: (_, record) => renderRechargeCell(record, t),
      },
      {
        title: t('额度'),
        dataIndex: 'quota',
        width: 96,
        render: (value) => (
          <span className='tabular-nums'>
            {typeof value === 'number' ? renderQuota(value) : '-'}
          </span>
        ),
      },
      {
        title: t('角色'),
        dataIndex: 'role',
        width: 96,
        render: (value) => renderRoleTag(value, t),
      },
      {
        title: t('最近出现'),
        dataIndex: 'last_seen_at',
        width: 170,
        render: (_, record) => formatLastSeen(record),
      },
    ],
    [t],
  );

  const rowSelection = {
    // 行选择列与「用户ID/用户名」一起固定在左侧，横向滚动时勾选入口不丢失。
    fixed: 'left',
    width: 48,
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
    // 契约 C/H/I：组头合计与活跃度汇总（基于已加载成员，本地求和）。
    const groupRechargeSum = sumUserField(groupUsers, 'recharge_quota');
    const groupUsedSum = sumUserField(groupUsers, 'used_quota');
    const groupFullyLoaded = groupUsers.length >= userCount;
    const isZeroRechargeGroup =
      groupUsers.length > 0 && groupFullyLoaded && groupRechargeSum <= 0;
    const hasActivityCounts = [
      'zombie_count',
      'checkin_only_count',
      'active_count',
    ].some((field) => group?.[field] !== undefined);
    const zombieCount = Number(group?.zombie_count) || 0;
    const checkinOnlyCount = Number(group?.checkin_only_count) || 0;
    const activeCount = Number(group?.active_count) || 0;
    const groupVisibleUsers = zeroRechargeOnly
      ? groupUsers.filter((user) => (Number(user?.recharge_quota) || 0) <= 0)
      : groupUsers;
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
              {hasActivityCounts && (
                <span className='mt-1 flex flex-wrap items-center gap-1.5'>
                  <Tag color='red' shape='circle' size='small'>
                    {t('僵尸号')} {zombieCount}
                  </Tag>
                  <Tag color='orange' shape='circle' size='small'>
                    {t('只签到未调用')} {checkinOnlyCount}
                  </Tag>
                  <Tag color='green' shape='circle' size='small'>
                    {t('有调用')} {activeCount}
                  </Tag>
                </span>
              )}
              {isZeroRechargeGroup && (
                <span className='mt-1 block'>
                  <Tag color='orange' shape='circle' size='small'>
                    {t('零充值')}
                  </Tag>
                </span>
              )}
              <span className='mt-1 block text-xs text-[var(--semi-color-text-2)]'>
                {t('充值合计')}: {formatQuotaTotal(groupRechargeSum)} ·{' '}
                {t('历史消耗合计')}: {formatQuotaTotal(groupUsedSum)}
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
            {zeroRechargeOnly && groupVisibleUsers.length === 0 && (
              <div className='mb-2 text-xs text-[var(--semi-color-text-2)]'>
                {t('暂无零充值账号')}
              </div>
            )}
            <div className='hidden md:block'>
              <Table
                columns={columns}
                dataSource={groupVisibleUsers}
                rowKey={(record) => getUserRowKey(record)}
                rowSelection={buildGroupRowSelection(groupVisibleUsers)}
                pagination={false}
                size='small'
                scroll={{ x: TABLE_SCROLL_X }}
                empty={t('暂无数据')}
              />
            </div>
            <div className='space-y-2 md:hidden'>
              {groupVisibleUsers.length === 0 ? (
                <div className='py-4 text-center text-sm text-[var(--semi-color-text-2)]'>
                  {t('暂无数据')}
                </div>
              ) : (
                groupVisibleUsers.map((user) => {
                  const key = getUserRowKey(user);
                  return (
                    <MobileUserCard
                      key={key}
                      user={user}
                      selected={selected}
                      onSelect={setUserSelected}
                      t={t}
                    />
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
      width={isMobile ? '100%' : MODAL_DESKTOP_WIDTH}
      title={
        <div className='flex min-w-0 flex-col'>
          <span>{t('关联账号')}</span>
          <span className='truncate text-xs font-normal text-[var(--semi-color-text-2)]'>
            {subtitle}
          </span>
          <span className='text-xs font-normal text-[var(--semi-color-text-2)]'>
            {summaryTotal > 0
              ? `${t('共 {{total}} 个账号', { total: summaryTotal })} · ${t('充值合计')}: ${formatQuotaTotal(
                  summaryRecharge,
                )} · ${t('历史消耗合计')}: ${formatQuotaTotal(summaryUsed)}`
              : '-'}
          </span>
        </div>
      }
      bodyStyle={{
        padding: 0,
        height: isMobile ? '54vh' : '62vh',
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
      }}
      onCancel={onClose}
      footer={footer}
      maskClosable={actionLoading === '' && groupActionLoading === ''}
      closeOnEsc={actionLoading === '' && groupActionLoading === ''}
    >
      <div className='flex h-full min-h-0 flex-col'>
        <div className='shrink-0 border-b border-[var(--semi-color-border)] px-4 py-2'>
          {isInviter && (
            <div className='mb-2 flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between'>
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
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <Checkbox
              checked={zeroRechargeOnly}
              onChange={(e) =>
                setZeroRechargeOnly(Boolean(e?.target?.checked))
              }
            >
              {t('仅看零充值')}
            </Checkbox>
            {isGroupedView && (
              <div className='flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-[var(--semi-color-text-2)]'>
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
            )}
          </div>
        </div>

        <div className='min-h-0 flex-1 overflow-y-auto px-4 py-3'>
          {isGroupedView ? (
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
          ) : (
            <Spin spinning={loading}>
              <div className='hidden md:block'>
                <Table
                  columns={columns}
                  dataSource={displayedFlatItems}
                  rowKey={(record) => getUserRowKey(record)}
                  rowSelection={rowSelection}
                  pagination={false}
                  size='small'
                  scroll={{ x: TABLE_SCROLL_X }}
                  empty={t('暂无数据')}
                />
              </div>

              <div className='space-y-2 md:hidden'>
                {loading && displayedFlatItems.length === 0 ? (
                  <div className='py-8 text-center text-sm text-[var(--semi-color-text-2)]'>
                    {t('加载中')}
                  </div>
                ) : displayedFlatItems.length === 0 ? (
                  <div className='py-8 text-center text-sm text-[var(--semi-color-text-2)]'>
                    {zeroRechargeOnly ? t('暂无零充值账号') : t('暂无数据')}
                  </div>
                ) : (
                  displayedFlatItems.map((user) => (
                    <MobileUserCard
                      key={getUserRowKey(user)}
                      user={user}
                      selected={selected}
                      onSelect={setUserSelected}
                      t={t}
                    />
                  ))
                )}
              </div>
            </Spin>
          )}
        </div>

        {isGroupedView ? (
          <div className='shrink-0 border-t border-[var(--semi-color-border)] px-4 py-2'>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <Text type='secondary' size='small'>
                {t('共 {{total}} 个账号', { total: effectiveTotal })}
              </Text>
              <Text type='secondary' size='small'>
                {t('共 {{count}} 组', { count: groups.length })}
              </Text>
            </div>
          </div>
        ) : (
          <div className='shrink-0 border-t border-[var(--semi-color-border)] px-4 py-2'>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <Checkbox
                checked={allPageSelected}
                indeterminate={somePageSelected}
                disabled={pageSelectable.length === 0 || actionLoading !== ''}
                onChange={(e) => setPageSelected(Boolean(e?.target?.checked))}
              >
                {t('全选本页')}
              </Checkbox>
              <div className='flex flex-wrap items-center justify-end gap-2'>
                <Button
                  size='small'
                  theme='borderless'
                  disabled={page <= 1 || actionLoading !== ''}
                  onClick={() => goToPage(page - 1)}
                >
                  {t('上一页')}
                </Button>
                <Text type='secondary' size='small'>
                  {t('共 {{total}} 个账号', { total })} ·{' '}
                  {t('第 {{page}}/{{pages}} 页', {
                    page,
                    pages: totalPages,
                  })}
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
          </div>
        )}
      </div>
    </Modal>
  );
}
