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

import React, { useEffect, useState, useContext } from 'react';
import {
  Button,
  Card,
  Col,
  Form,
  Input,
  Row,
  Switch,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../../../context/Status';
import { MoveDown, MoveUp, Plus, Trash2 } from 'lucide-react';
import {
  createDefaultHeaderNavModules,
  isSafeTopNavHref,
  MAX_CUSTOM_TOP_NAV_LINKS,
  MAX_CUSTOM_TOP_NAV_TITLE_LENGTH,
  MAX_CUSTOM_TOP_NAV_URL_LENGTH,
  parseHeaderNavModules,
} from '../../../helpers/headerNav';

const { Text } = Typography;

export default function SettingsHeaderNavModules(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [statusState, statusDispatch] = useContext(StatusContext);

  // 顶栏模块管理状态
  const [headerNavModules, setHeaderNavModules] = useState(
    createDefaultHeaderNavModules,
  );

  // 处理顶栏模块配置变更
  function handleHeaderNavModuleChange(moduleKey) {
    return (checked) => {
      const newModules = { ...headerNavModules };
      if (moduleKey === 'pricing') {
        // 对于pricing模块，只更新enabled属性
        newModules[moduleKey] = {
          ...newModules[moduleKey],
          enabled: checked,
        };
      } else {
        newModules[moduleKey] = checked;
      }
      setHeaderNavModules(newModules);
    };
  }

  // 处理模型广场权限控制变更
  function handlePricingAuthChange(checked) {
    const newModules = { ...headerNavModules };
    newModules.pricing = {
      ...newModules.pricing,
      requireAuth: checked,
    };
    setHeaderNavModules(newModules);
  }

  // 重置顶栏模块为默认配置
  function resetHeaderNavModules() {
    setHeaderNavModules(createDefaultHeaderNavModules());
    showSuccess(t('已重置为默认配置'));
  }

  function addCustomLink() {
    if (
      (headerNavModules.customLinks || []).length >= MAX_CUSTOM_TOP_NAV_LINKS
    ) {
      showError(
        t('最多只能添加 {{count}} 个自定义导航', {
          count: MAX_CUSTOM_TOP_NAV_LINKS,
        }),
      );
      return;
    }
    const id =
      typeof crypto !== 'undefined' && crypto.randomUUID
        ? crypto.randomUUID()
        : `custom-${Date.now()}`;
    setHeaderNavModules((current) => ({
      ...current,
      customLinks: [
        ...(current.customLinks || []),
        { id, title: '', url: '', enabled: true },
      ],
    }));
  }

  function updateCustomLink(id, field, value) {
    setHeaderNavModules((current) => ({
      ...current,
      customLinks: (current.customLinks || []).map((link) =>
        link.id === id ? { ...link, [field]: value } : link,
      ),
    }));
  }

  function deleteCustomLink(id) {
    setHeaderNavModules((current) => ({
      ...current,
      customLinks: (current.customLinks || []).filter((link) => link.id !== id),
    }));
  }

  function moveCustomLink(index, direction) {
    setHeaderNavModules((current) => {
      const links = [...(current.customLinks || [])];
      const target = index + direction;
      if (target < 0 || target >= links.length) return current;
      [links[index], links[target]] = [links[target], links[index]];
      return { ...current, customLinks: links };
    });
  }

  // 保存配置
  async function onSubmit() {
    const customLinks = headerNavModules.customLinks || [];
    for (const link of customLinks) {
      const title = String(link.title || '').trim();
      const url = String(link.url || '').trim();
      if (!title || title.length > MAX_CUSTOM_TOP_NAV_TITLE_LENGTH) {
        showError(
          t('自定义导航名称不能为空，且不能超过 {{count}} 个字符', {
            count: MAX_CUSTOM_TOP_NAV_TITLE_LENGTH,
          }),
        );
        return;
      }
      if (
        !url ||
        url.length > MAX_CUSTOM_TOP_NAV_URL_LENGTH ||
        !isSafeTopNavHref(url)
      ) {
        showError(t('导航链接仅支持站内路径或 http/https 地址'));
        return;
      }
    }
    const nextModules = {
      ...headerNavModules,
      customLinks: customLinks.map((link) => ({
        ...link,
        title: link.title.trim(),
        url: link.url.trim(),
      })),
    };
    setLoading(true);
    try {
      const res = await API.put('/api/option/', {
        key: 'HeaderNavModules',
        value: JSON.stringify(nextModules),
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('保存成功'));

        // 立即更新StatusContext中的状态
        statusDispatch({
          type: 'set',
          payload: {
            ...statusState.status,
            HeaderNavModules: JSON.stringify(nextModules),
          },
        });

        // 刷新父组件状态
        if (props.refresh) {
          await props.refresh();
        }
      } else {
        showError(message);
      }
    } catch (error) {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    // 从 props.options 中获取配置
    if (props.options && props.options.HeaderNavModules) {
      setHeaderNavModules(
        parseHeaderNavModules(props.options.HeaderNavModules),
      );
    }
  }, [props.options]);

  // 模块配置数据
  const moduleConfigs = [
    {
      key: 'home',
      title: t('首页'),
      description: t('用户主页，展示系统信息'),
    },
    {
      key: 'console',
      title: t('控制台'),
      description: t('用户控制面板，管理账户'),
    },
    {
      key: 'pricing',
      title: t('模型广场'),
      description: t('模型定价，需要登录访问'),
      hasSubConfig: true, // 标识该模块有子配置
    },
    {
      key: 'docs',
      title: t('文档'),
      description: t('系统文档和帮助信息'),
    },
    {
      key: 'about',
      title: t('关于'),
      description: t('关于系统的详细信息'),
    },
  ];

  return (
    <Card>
      <Form.Section
        text={t('顶栏管理')}
        extraText={t('控制顶栏模块显示状态，全局生效')}
      >
        <Row gutter={[16, 16]} style={{ marginBottom: '24px' }}>
          {moduleConfigs.map((module) => (
            <Col key={module.key} xs={24} sm={12} md={6} lg={6} xl={6}>
              <Card
                style={{
                  borderRadius: '8px',
                  border: '1px solid var(--semi-color-border)',
                  transition: 'all 0.2s ease',
                  background: 'var(--semi-color-bg-1)',
                  minHeight: '80px',
                }}
                bodyStyle={{ padding: '16px' }}
                hoverable
              >
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    height: '100%',
                  }}
                >
                  <div style={{ flex: 1, textAlign: 'left' }}>
                    <div
                      style={{
                        fontWeight: '600',
                        fontSize: '14px',
                        color: 'var(--semi-color-text-0)',
                        marginBottom: '4px',
                      }}
                    >
                      {module.title}
                    </div>
                    <Text
                      type='secondary'
                      size='small'
                      style={{
                        fontSize: '12px',
                        color: 'var(--semi-color-text-2)',
                        lineHeight: '1.4',
                        display: 'block',
                      }}
                    >
                      {module.description}
                    </Text>
                  </div>
                  <div style={{ marginLeft: '16px' }}>
                    <Switch
                      checked={
                        module.key === 'pricing'
                          ? headerNavModules[module.key]?.enabled
                          : headerNavModules[module.key]
                      }
                      onChange={handleHeaderNavModuleChange(module.key)}
                      size='default'
                    />
                  </div>
                </div>

                {/* 为模型广场添加权限控制子开关 */}
                {module.key === 'pricing' &&
                  (module.key === 'pricing'
                    ? headerNavModules[module.key]?.enabled
                    : headerNavModules[module.key]) && (
                    <div
                      style={{
                        borderTop: '1px solid var(--semi-color-border)',
                        marginTop: '12px',
                        paddingTop: '12px',
                      }}
                    >
                      <div
                        style={{
                          display: 'flex',
                          justifyContent: 'space-between',
                          alignItems: 'center',
                        }}
                      >
                        <div style={{ flex: 1, textAlign: 'left' }}>
                          <div
                            style={{
                              fontWeight: '500',
                              fontSize: '12px',
                              color: 'var(--semi-color-text-1)',
                              marginBottom: '2px',
                            }}
                          >
                            {t('需要登录访问')}
                          </div>
                          <Text
                            type='secondary'
                            size='small'
                            style={{
                              fontSize: '11px',
                              color: 'var(--semi-color-text-2)',
                              lineHeight: '1.4',
                              display: 'block',
                            }}
                          >
                            {t('开启后未登录用户无法访问模型广场')}
                          </Text>
                        </div>
                        <div style={{ marginLeft: '16px' }}>
                          <Switch
                            checked={
                              headerNavModules.pricing?.requireAuth || false
                            }
                            onChange={handlePricingAuthChange}
                            size='default'
                          />
                        </div>
                      </div>
                    </div>
                  )}
              </Card>
            </Col>
          ))}
        </Row>

        <div className='mb-6'>
          <div className='mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
            <div>
              <div className='font-semibold'>{t('自定义顶部导航')}</div>
              <Text type='secondary' size='small'>
                {t('可添加站内路径或外部链接，关闭后保留配置但不显示')}
              </Text>
            </div>
            <Button
              type='primary'
              theme='light'
              icon={<Plus size={16} />}
              onClick={addCustomLink}
              disabled={
                (headerNavModules.customLinks || []).length >=
                MAX_CUSTOM_TOP_NAV_LINKS
              }
            >
              {t('新增顶部导航')}
            </Button>
          </div>

          <div className='flex flex-col gap-2'>
            {(headerNavModules.customLinks || []).length === 0 ? (
              <Text type='tertiary'>{t('暂未添加自定义顶部导航')}</Text>
            ) : (
              headerNavModules.customLinks.map((link, index) => (
                <div
                  key={link.id}
                  className='grid grid-cols-1 gap-2 border-b border-solid border-[var(--semi-color-border)] py-3 md:grid-cols-[minmax(160px,0.8fr)_minmax(260px,1.6fr)_auto] md:items-center'
                >
                  <Input
                    value={link.title}
                    maxLength={MAX_CUSTOM_TOP_NAV_TITLE_LENGTH}
                    placeholder={t('导航名称')}
                    onChange={(value) =>
                      updateCustomLink(link.id, 'title', value)
                    }
                  />
                  <Input
                    value={link.url}
                    maxLength={MAX_CUSTOM_TOP_NAV_URL_LENGTH}
                    placeholder='/console 或 https://example.com'
                    onChange={(value) =>
                      updateCustomLink(link.id, 'url', value)
                    }
                  />
                  <div className='flex items-center justify-end gap-1'>
                    <Switch
                      checked={link.enabled}
                      onChange={(checked) =>
                        updateCustomLink(link.id, 'enabled', checked)
                      }
                    />
                    <Tooltip content={t('上移')}>
                      <Button
                        type='tertiary'
                        theme='borderless'
                        icon={<MoveUp size={16} />}
                        disabled={index === 0}
                        onClick={() => moveCustomLink(index, -1)}
                      />
                    </Tooltip>
                    <Tooltip content={t('下移')}>
                      <Button
                        type='tertiary'
                        theme='borderless'
                        icon={<MoveDown size={16} />}
                        disabled={
                          index === headerNavModules.customLinks.length - 1
                        }
                        onClick={() => moveCustomLink(index, 1)}
                      />
                    </Tooltip>
                    <Tooltip content={t('删除')}>
                      <Button
                        type='danger'
                        theme='borderless'
                        icon={<Trash2 size={16} />}
                        onClick={() => deleteCustomLink(link.id)}
                      />
                    </Tooltip>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        <div
          style={{
            display: 'flex',
            gap: '12px',
            justifyContent: 'flex-start',
            alignItems: 'center',
            paddingTop: '8px',
            borderTop: '1px solid var(--semi-color-border)',
          }}
        >
          <Button
            size='default'
            type='tertiary'
            onClick={resetHeaderNavModules}
            style={{
              borderRadius: '6px',
              fontWeight: '500',
            }}
          >
            {t('重置为默认')}
          </Button>
          <Button
            size='default'
            type='primary'
            onClick={onSubmit}
            loading={loading}
            style={{
              borderRadius: '6px',
              fontWeight: '500',
              minWidth: '100px',
            }}
          >
            {t('保存设置')}
          </Button>
        </div>
      </Form.Section>
    </Card>
  );
}
