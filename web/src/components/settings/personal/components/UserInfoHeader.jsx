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
import { Avatar, Card, Tag, Badge } from '@douyinfe/semi-ui';
import {
  isRoot,
  isAdmin,
  renderQuota,
  stringToColor,
} from '../../../../helpers';

const UserInfoHeader = ({ t, userState }) => {
  const getUsername = () => {
    if (userState.user) {
      return userState.user.username;
    } else {
      return 'null';
    }
  };

  const getAvatarText = () => {
    const username = getUsername();
    if (username && username.length > 0) {
      return username.slice(0, 2).toUpperCase();
    }
    return 'NA';
  };

  return (
    <Card
      className='!rounded-2xl overflow-hidden'
      cover={
        <div
          className='relative h-32'
          style={{
            '--palette-primary-darkerChannel': '0 75 80',
            backgroundImage: `linear-gradient(0deg, rgba(var(--palette-primary-darkerChannel) / 80%), rgba(var(--palette-primary-darkerChannel) / 80%)), url('/cover-4.webp')`,
            backgroundSize: 'cover',
            backgroundPosition: 'center',
            backgroundRepeat: 'no-repeat',
          }}
        >
          {/* 用户信息内容 */}
          <div className='relative z-10 h-full flex flex-col justify-end p-6'>
            <div className='flex items-center'>
              <div className='flex items-stretch gap-3 sm:gap-4 flex-1 min-w-0'>
                <Avatar size='large' color={stringToColor(getUsername())}>
                  {getAvatarText()}
                </Avatar>
                <div className='flex-1 min-w-0 flex flex-col justify-between'>
                  <div
                    className='text-3xl font-bold truncate'
                    style={{ color: 'white' }}
                  >
                    {getUsername()}
                  </div>
                  <div className='flex flex-wrap items-center gap-2'>
                    {isRoot() ? (
                      <Tag
                        size='large'
                        shape='circle'
                        style={{ color: 'white' }}
                      >
                        {t('超级管理员')}
                      </Tag>
                    ) : isAdmin() ? (
                      <Tag
                        size='large'
                        shape='circle'
                        style={{ color: 'white' }}
                      >
                        {t('管理员')}
                      </Tag>
                    ) : (
                      <Tag
                        size='large'
                        shape='circle'
                        style={{ color: 'white' }}
                      >
                        {t('普通用户')}
                      </Tag>
                    )}
                    <Tag size='large' shape='circle' style={{ color: 'white' }}>
                      ID: {userState?.user?.id}
                    </Tag>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      }
    >
      <Badge count={t('当前余额')} position='rightTop' type='danger'>
        <div className='text-2xl sm:text-3xl md:text-4xl font-bold tracking-wide'>
          {renderQuota(userState?.user?.quota)}
        </div>
      </Badge>
    </Card>
  );
};

export default UserInfoHeader;
