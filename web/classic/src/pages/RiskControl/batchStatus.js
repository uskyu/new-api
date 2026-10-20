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

import { API, showSuccess } from '../../helpers';

/**
 * 调用批量封禁/解禁接口。
 * scope: 'users'（勾选账号）| 'shared_ip'（同 IP 范围）| 'inviter'（邀请人范围）
 * 业务失败返回 { ok: false, message }；网络/HTTP 异常原样抛出，由调用方
 * showError 统一处理（保留 401 跳转登录等既有行为）。
 */
export async function submitBatchStatus({
  action,
  scope,
  userIds,
  ip,
  source,
  inviterId,
}) {
  let res;
  if (scope === 'users') {
    res = await API.post(
      '/api/risk-control/users/batch-status',
      { action, user_ids: userIds || [] },
      { skipErrorHandler: true },
    );
  } else if (scope === 'shared_ip') {
    res = await API.post(
      '/api/risk-control/shared-ips/batch-status',
      {
        action,
        ip,
        // source 为空表示该 IP 的全部来源。
        source: source === 'login' || source === 'token' ? source : '',
      },
      { skipErrorHandler: true },
    );
  } else if (scope === 'inviter') {
    res = await API.post(
      `/api/risk-control/inviters/${inviterId}/batch-status`,
      { action },
      { skipErrorHandler: true },
    );
  } else {
    return { ok: false, message: '' };
  }
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
}

// 统一展示批量操作结果：成功数量 + 可选跳过数量。
export function notifyBatchStatus(action, result, t) {
  const count = Number(result?.updated) || 0;
  if (action === 'disable') {
    showSuccess(t('封禁成功，共 {{count}} 个账号', { count }));
  } else {
    showSuccess(t('解禁成功，共 {{count}} 个账号', { count }));
  }
  const skippedCount = Array.isArray(result?.skipped)
    ? result.skipped.length
    : 0;
  if (skippedCount > 0) {
    showSuccess(t('已跳过 {{count}} 个账号', { count: skippedCount }));
  }
}
