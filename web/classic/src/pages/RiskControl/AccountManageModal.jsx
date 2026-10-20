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
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, timestamp2string } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { getUserRowKey, isUserBanned } from './UserInlineList';
import { notifyBatchStatus, submitBatchStatus } from './batchStatus';

const { Text } = Typography;
const PAGE_SIZE = 20;
// 角色 >= 管理员（10）的账号前端置灰不可勾选，服务端仍会二次校验并返回 skipped。
const SELECTABLE_ROLE_LIMIT = 10;

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

  useEffect(() => {
    if (!visible) {
      return;
    }
    setSelected({});
    setPage(1);
    setActionLoading('');
    if (mode === 'inviter') {
      setItems([]);
      setTotal(0);
      loadInvitees(1);
    } else {
      // 使邀请人模式下仍在途的请求失效。
      loadSeqRef.current += 1;
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
          {t('共 {{total}} 个账号', { total })}
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

  const columns = useMemo(
    () => [
      {
        title: t('用户ID'),
        dataIndex: 'user_id',
        width: 90,
        render: (value, record) => value ?? getUserRowKey(record) ?? '-',
      },
      {
        title: t('用户名'),
        dataIndex: 'username',
        width: 160,
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
        title: t('角色'),
        dataIndex: 'role',
        width: 130,
        render: (value) => renderRoleTag(value, t),
      },
      {
        title: t('最近出现'),
        dataIndex: 'last_seen_at',
        width: 180,
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

  const footer = (
    <div className='flex flex-col gap-2 md:flex-row md:items-center md:justify-between'>
      <Text type='secondary'>{t('已选 {{count}} 项', { count: selectedCount })}</Text>
      <div className='flex flex-wrap gap-2'>
        <Button
          type='danger'
          theme='light'
          loading={actionLoading === 'selected-disable'}
          disabled={selectedCount === 0 || actionLoading !== ''}
          onClick={() => confirmSelected('disable')}
        >
          {t('封禁选中')}
        </Button>
        <Button
          type='primary'
          theme='light'
          loading={actionLoading === 'selected-enable'}
          disabled={selectedCount === 0 || actionLoading !== ''}
          onClick={() => confirmSelected('enable')}
        >
          {t('解禁选中')}
        </Button>
        <Button
          type='danger'
          theme='borderless'
          loading={actionLoading === 'all-disable'}
          disabled={total === 0 || actionLoading !== ''}
          onClick={() => confirmAll('disable')}
        >
          {t('封禁全部')}
        </Button>
        <Button
          type='primary'
          theme='borderless'
          loading={actionLoading === 'all-enable'}
          disabled={total === 0 || actionLoading !== ''}
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
      maskClosable={!actionLoading}
      closeOnEsc={!actionLoading}
    >
      <div>
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
                      <div className='mt-1 text-xs text-[var(--semi-color-text-2)]'>
                        {t('最近出现')}: {formatLastSeen(user)}
                      </div>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </Spin>
      </div>
    </Modal>
  );
}
