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

import React from 'react';
import { Tag, Typography } from '@douyinfe/semi-ui';

const { Text } = Typography;

// 风控接口里用户主键为 user_id，部分上下文对象（如邀请人）用 id。
export function getUserRowKey(user) {
  return user?.user_id ?? user?.id;
}

export function isUserBanned(user) {
  return Number(user?.status) === 2;
}

export function formatUserLabel(user) {
  const userId = getUserRowKey(user);
  const username = user?.username || '';
  const email = (user?.email || '').trim();
  const idPart = userId === undefined || userId === null ? '#' : `#${userId}`;
  return `${idPart} ${username}${email ? ` · ${email}` : ''}`;
}

// mode: 'any' 同时匹配 id / username / display_name；'username' 只匹配名称；
// 'user_id' 只匹配精确 id。关键词为空时不命中任何用户。
export function userMatchesKeyword(user, keyword, mode = 'any') {
  const normalized = (keyword || '').trim().toLowerCase();
  if (!normalized || !user) {
    return false;
  }
  const userId = getUserRowKey(user);
  const idMatched =
    userId !== undefined && userId !== null && String(userId) === normalized;
  if (mode === 'user_id') {
    return idMatched;
  }
  const username = (user.username || '').toLowerCase();
  const displayName = (user.display_name || '').toLowerCase();
  if (mode === 'username') {
    return username.includes(normalized) || displayName.includes(normalized);
  }
  return (
    idMatched ||
    username.includes(normalized) ||
    displayName.includes(normalized)
  );
}

/**
 * 关联用户内联列表：默认展示 max 个 tag，其余折叠为 +N，点击 +N 打开账号管理弹窗。
 * 已封禁账号使用红色 tag；isHighlighted 命中的 tag 使用危险色描边与文字高亮。
 */
export default function UserInlineList({
  users,
  max = 3,
  isHighlighted,
  onShowMore,
}) {
  const list = Array.isArray(users) ? users.filter(Boolean) : [];
  if (list.length === 0) {
    return <Text type='tertiary'>-</Text>;
  }
  const visibleCount = Math.max(1, max);
  const visible = list.slice(0, visibleCount);
  const rest = list.length - visible.length;
  return (
    <div className='flex min-w-0 flex-wrap items-center gap-1'>
      {visible.map((user, index) => {
        const key = getUserRowKey(user);
        const highlighted =
          typeof isHighlighted === 'function' ? isHighlighted(user) : false;
        return (
          <Tag
            key={key ?? `user-${index}`}
            color={isUserBanned(user) ? 'red' : 'white'}
            shape='circle'
            size='small'
            className={onShowMore ? 'cursor-pointer' : undefined}
            onClick={onShowMore}
            style={
              highlighted
                ? {
                    borderColor: 'var(--semi-color-danger)',
                    color: 'var(--semi-color-danger)',
                  }
                : undefined
            }
          >
            <span
              className='inline-block max-w-[180px] truncate align-bottom sm:max-w-[240px]'
              title={formatUserLabel(user)}
            >
              {formatUserLabel(user)}
            </span>
          </Tag>
        );
      })}
      {rest > 0 && (
        <Tag
          color='white'
          shape='circle'
          size='small'
          className='cursor-pointer'
          onClick={onShowMore}
        >
          +{rest}
        </Tag>
      )}
    </div>
  );
}
