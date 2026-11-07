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

import React, { useCallback, useEffect, useState } from 'react';
import {
  Button,
  Card,
  Empty,
  InputNumber,
  Modal,
  Space,
  Spin,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconPlus, IconRefresh } from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';
import { showError, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import {
  buildBlindBoxPayload,
  formatPrice,
  getAdminBlindBoxes,
  getBlindBoxSetting,
  getBoxRewardRange,
  updateBlindBox,
  updateBlindBoxSetting,
} from './api';
import BoxEditor from './components/BoxEditor';
import OrdersPanel from './components/OrdersPanel';

const { Text, Title } = Typography;

const DEFAULT_TIMEOUT_MINUTES = 15;

function normalizeSetting(data, fallback = {}) {
  const timeout = Number(data?.order_timeout_minutes);
  const cycleId = Number(data?.cycle_id);
  return {
    enabled:
      typeof data?.enabled === 'boolean' ? data.enabled : !!fallback.enabled,
    cycle_id: Number.isFinite(cycleId) ? cycleId : 0,
    order_timeout_minutes:
      Number.isFinite(timeout) && timeout > 0
        ? timeout
        : DEFAULT_TIMEOUT_MINUTES,
  };
}

const BlindBoxAdmin = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();

  const [settingLoading, setSettingLoading] = useState(false);
  const [settingSaving, setSettingSaving] = useState(false);
  const [setting, setSetting] = useState({
    enabled: false,
    cycle_id: 0,
    order_timeout_minutes: DEFAULT_TIMEOUT_MINUTES,
  });

  const [boxes, setBoxes] = useState([]);
  const [boxesLoading, setBoxesLoading] = useState(false);
  const [togglingId, setTogglingId] = useState(null);

  const [editorVisible, setEditorVisible] = useState(false);
  const [editingBox, setEditingBox] = useState(null);

  const [activeTab, setActiveTab] = useState('boxes');

  const loadSetting = useCallback(async () => {
    setSettingLoading(true);
    try {
      const res = await getBlindBoxSetting();
      if (res.data?.success) {
        setSetting(normalizeSetting(res.data.data));
      } else {
        showError(res.data?.message || t('加载盲盒设置失败'));
      }
    } catch (e) {
      showError(t('加载盲盒设置失败'));
    } finally {
      setSettingLoading(false);
    }
  }, [t]);

  const loadBoxes = useCallback(async () => {
    setBoxesLoading(true);
    try {
      const res = await getAdminBlindBoxes();
      if (res.data?.success) {
        const payload = res.data.data;
        const list = Array.isArray(payload)
          ? payload
          : Array.isArray(payload?.items)
            ? payload.items
            : Array.isArray(payload?.boxes)
              ? payload.boxes
              : [];
        setBoxes(list);
      } else {
        showError(res.data?.message || t('加载盲盒列表失败'));
      }
    } catch (e) {
      showError(t('加载盲盒列表失败'));
    } finally {
      setBoxesLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadSetting();
    loadBoxes();
  }, [loadSetting, loadBoxes]);

  const handleSaveSetting = async () => {
    const timeoutMinutes = Number(setting.order_timeout_minutes);
    if (
      !Number.isFinite(timeoutMinutes) ||
      timeoutMinutes <= 0 ||
      !Number.isInteger(timeoutMinutes)
    ) {
      showError(t('订单超时时间必须为正整数分钟'));
      return;
    }
    if (timeoutMinutes > 10080) {
      showError(t('订单超时时间不能超过 10080 分钟'));
      return;
    }

    setSettingSaving(true);
    try {
      const res = await updateBlindBoxSetting({
        enabled: setting.enabled,
        order_timeout_minutes: timeoutMinutes,
      });
      if (res.data?.success) {
        showSuccess(t('保存成功'));
        await loadSetting();
      } else {
        showError(res.data?.message || t('保存失败'));
      }
    } catch (e) {
      showError(t('保存失败'));
    } finally {
      setSettingSaving(false);
    }
  };

  const openCreateEditor = () => {
    setEditingBox(null);
    setEditorVisible(true);
  };

  const openEditEditor = (box) => {
    setEditingBox(box);
    setEditorVisible(true);
  };

  const closeEditor = () => {
    setEditorVisible(false);
    setEditingBox(null);
  };

  const handleEditorSuccess = () => {
    closeEditor();
    loadBoxes();
  };

  const toggleBoxEnabled = (box) => {
    const nextEnabled = !box.enabled;
    Modal.confirm({
      title: nextEnabled ? t('确认上架') : t('确认下架'),
      content: nextEnabled
        ? t('上架后用户即可在盲盒页面购买该盲盒，是否继续？')
        : t('下架后用户将无法继续购买该盲盒，已产生的订单不受影响。是否继续？'),
      centered: true,
      onOk: async () => {
        setTogglingId(box.id);
        try {
          const res = await updateBlindBox(
            box.id,
            buildBlindBoxPayload({ ...box, enabled: nextEnabled }),
          );
          if (res.data?.success) {
            showSuccess(nextEnabled ? t('上架成功') : t('下架成功'));
            await loadBoxes();
          } else {
            showError(res.data?.message || t('操作失败'));
          }
        } catch (e) {
          showError(t('操作失败'));
        } finally {
          setTogglingId(null);
        }
      },
    });
  };

  const boxColumns = [
    {
      title: t('名称'),
      dataIndex: 'name',
      key: 'name',
      width: 220,
      render: (name, record) => (
        <div className='min-w-0'>
          <div className='font-medium truncate'>{name || '-'}</div>
          {record.remark ? (
            <div className='text-xs text-gray-500 truncate'>
              {record.remark}
            </div>
          ) : null}
        </div>
      ),
    },
    {
      title: t('售价'),
      dataIndex: 'price',
      key: 'price',
      width: 110,
      render: (price) => <Text strong>{formatPrice(price)}</Text>,
    },
    {
      title: t('可得余额范围'),
      key: 'reward_range',
      width: 180,
      render: (_, record) => {
        const [minReward, maxReward] = getBoxRewardRange(record);
        return (
          <Text>{`${formatPrice(minReward)} ~ ${formatPrice(maxReward)}`}</Text>
        );
      },
    },
    {
      title: t('库存'),
      key: 'stock',
      width: 130,
      render: (_, record) => {
        const total = Number(record.stock_total || 0);
        const sold = Number(record.sold_count || 0);
        if (total <= 0) {
          return <Text type='tertiary'>{t('不限量')}</Text>;
        }
        const soldOut = sold >= total;
        return (
          <div className='flex flex-col items-start gap-1'>
            <Text type={soldOut ? 'danger' : undefined}>
              {`${sold} / ${total}`}
            </Text>
            {soldOut ? (
              <Tag color='red' shape='circle' size='small'>
                {t('已售罄')}
              </Tag>
            ) : null}
          </div>
        );
      },
    },
    {
      title: t('每人限购'),
      key: 'per_user_limit',
      width: 110,
      render: (_, record) => {
        const limit = Number(record.per_user_limit || 0);
        if (limit <= 0) {
          return <Text type='tertiary'>{t('不限购')}</Text>;
        }
        return <Text>{`${limit} ${t('个/人')}`}</Text>;
      },
    },
    {
      title: t('状态'),
      key: 'enabled',
      width: 100,
      render: (_, record) =>
        record.enabled ? (
          <Tag color='green' shape='circle' size='small'>
            {t('已上架')}
          </Tag>
        ) : (
          <Tag color='grey' shape='circle' size='small'>
            {t('已下架')}
          </Tag>
        ),
    },
    {
      title: t('排序'),
      dataIndex: 'sort',
      key: 'sort',
      width: 80,
      render: (sort) => <Text>{sort ?? 0}</Text>,
    },
    {
      title: t('操作'),
      key: 'action',
      width: 170,
      render: (_, record) => (
        <Space>
          <Button
            size='small'
            type='primary'
            theme='light'
            onClick={() => openEditEditor(record)}
          >
            {t('编辑')}
          </Button>
          <Button
            size='small'
            type={record.enabled ? 'warning' : 'tertiary'}
            theme='light'
            loading={togglingId === record.id}
            onClick={() => toggleBoxEnabled(record)}
          >
            {record.enabled ? t('下架') : t('上架')}
          </Button>
        </Space>
      ),
    },
  ];

  const emptyBoxes = (
    <Empty
      image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
      darkModeImage={
        <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
      }
      description={t('暂无盲盒，请先新建')}
      style={{ padding: 30 }}
    />
  );

  return (
    <div className='mt-[60px] px-2'>
      <Card className='!rounded-2xl shadow-sm mb-4' bodyStyle={{ padding: 16 }}>
        <Spin spinning={settingLoading}>
          <div className='flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4'>
            <div className='min-w-0'>
              <Title heading={5} className='m-0'>
                {t('盲盒活动设置')}
              </Title>
              <Text type='tertiary' size='small' className='block mt-1'>
                {t(
                  '从关闭切换为开启并保存后，后端会自动生成新的活动周期，每人限购将重新计算',
                )}
              </Text>
              {setting.enabled && setting.cycle_id ? (
                <Text type='tertiary' size='small' className='block mt-1'>
                  {`${t('当前活动周期')}: ${setting.cycle_id}`}
                </Text>
              ) : null}
            </div>

            <div className='flex flex-wrap items-center gap-4'>
              <Space>
                <Text>{t('全局开关')}</Text>
                <Switch
                  checked={setting.enabled}
                  onChange={(enabled) =>
                    setSetting((prev) => ({ ...prev, enabled }))
                  }
                />
                <Tag
                  color={setting.enabled ? 'green' : 'grey'}
                  shape='circle'
                  size='small'
                >
                  {setting.enabled ? t('已开启') : t('已关闭')}
                </Tag>
              </Space>
              <Space>
                <Text>{t('订单超时(分钟)')}</Text>
                <InputNumber
                  value={setting.order_timeout_minutes}
                  min={1}
                  max={10080}
                  precision={0}
                  style={{ width: 120 }}
                  onChange={(value) =>
                    setSetting((prev) => ({
                      ...prev,
                      order_timeout_minutes: value,
                    }))
                  }
                />
              </Space>
              <Button
                type='primary'
                theme='solid'
                loading={settingSaving}
                onClick={handleSaveSetting}
              >
                {t('保存设置')}
              </Button>
            </div>
          </div>
        </Spin>
      </Card>

      <Card className='!rounded-2xl shadow-sm' bodyStyle={{ padding: 16 }}>
        <Tabs activeKey={activeTab} onChange={setActiveTab} type='line'>
          <Tabs.TabPane itemKey='boxes' tab={t('盲盒列表')}>
            <div className='flex flex-wrap items-center justify-end gap-2 mb-3'>
              <Button icon={<IconRefresh />} onClick={loadBoxes}>
                {t('刷新')}
              </Button>
              <Button
                type='primary'
                theme='solid'
                icon={<IconPlus />}
                onClick={openCreateEditor}
              >
                {t('新建盲盒')}
              </Button>
            </div>
            {isMobile ? (
              boxesLoading ? (
                <div className='flex justify-center py-8'>
                  <Spin />
                </div>
              ) : boxes.length === 0 ? (
                emptyBoxes
              ) : (
                <div className='space-y-3'>
                  {boxes.map((box) => {
                    const total = Number(box.stock_total || 0);
                    const sold = Number(box.sold_count || 0);
                    const soldOut = total > 0 && sold >= total;
                    const limit = Number(box.per_user_limit || 0);
                    const [minReward, maxReward] = getBoxRewardRange(box);
                    return (
                      <div
                        key={box.id}
                        className='rounded-xl p-4'
                        style={{
                          border: '1px solid var(--semi-color-border)',
                          background: 'var(--semi-color-bg-2)',
                        }}
                      >
                        <div className='flex items-start justify-between gap-2'>
                          <div className='min-w-0'>
                            <div className='font-medium truncate'>
                              {box.name || '-'}
                            </div>
                            {box.remark ? (
                              <div className='mt-0.5 truncate text-xs text-gray-500'>
                                {box.remark}
                              </div>
                            ) : null}
                          </div>
                          <Tag
                            color={box.enabled ? 'green' : 'grey'}
                            shape='circle'
                            size='small'
                          >
                            {box.enabled ? t('已上架') : t('已下架')}
                          </Tag>
                        </div>
                        <div className='mt-3 space-y-1.5 text-sm'>
                          <div className='flex items-center justify-between gap-3'>
                            <span style={{ color: 'var(--semi-color-text-2)' }}>
                              {t('售价')}
                            </span>
                            <Text strong>{formatPrice(box.price)}</Text>
                          </div>
                          <div className='flex items-center justify-between gap-3'>
                            <span style={{ color: 'var(--semi-color-text-2)' }}>
                              {t('可得余额范围')}
                            </span>
                            <span>{`${formatPrice(minReward)} ~ ${formatPrice(maxReward)}`}</span>
                          </div>
                          <div className='flex items-center justify-between gap-3'>
                            <span style={{ color: 'var(--semi-color-text-2)' }}>
                              {t('库存')}
                            </span>
                            {total <= 0 ? (
                              <Text type='tertiary'>{t('不限量')}</Text>
                            ) : (
                              <span className='flex items-center gap-1.5'>
                                <Text type={soldOut ? 'danger' : undefined}>
                                  {`${sold} / ${total}`}
                                </Text>
                                {soldOut ? (
                                  <Tag color='red' shape='circle' size='small'>
                                    {t('已售罄')}
                                  </Tag>
                                ) : null}
                              </span>
                            )}
                          </div>
                          <div className='flex items-center justify-between gap-3'>
                            <span style={{ color: 'var(--semi-color-text-2)' }}>
                              {t('每人限购')}
                            </span>
                            {limit <= 0 ? (
                              <Text type='tertiary'>{t('不限购')}</Text>
                            ) : (
                              <Text>{`${limit} ${t('个/人')}`}</Text>
                            )}
                          </div>
                        </div>
                        <div className='mt-4 flex gap-2'>
                          <Button
                            block
                            size='small'
                            type='primary'
                            theme='light'
                            onClick={() => openEditEditor(box)}
                          >
                            {t('编辑')}
                          </Button>
                          <Button
                            block
                            size='small'
                            type={box.enabled ? 'warning' : 'tertiary'}
                            theme='light'
                            loading={togglingId === box.id}
                            onClick={() => toggleBoxEnabled(box)}
                          >
                            {box.enabled ? t('下架') : t('上架')}
                          </Button>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )
            ) : (
              <Table
                columns={boxColumns}
                dataSource={boxes}
                loading={boxesLoading}
                rowKey='id'
                pagination={false}
                size='middle'
                scroll={{ x: 'max-content' }}
                empty={emptyBoxes}
              />
            )}
          </Tabs.TabPane>

          <Tabs.TabPane itemKey='orders' tab={t('订单记录')}>
            <OrdersPanel
              boxes={boxes}
              active={activeTab === 'orders'}
              onCompleted={loadBoxes}
            />
          </Tabs.TabPane>
        </Tabs>
      </Card>

      <BoxEditor
        visible={editorVisible}
        box={editingBox}
        onCancel={closeEditor}
        onSuccess={handleEditorSuccess}
      />
    </div>
  );
};

export default BlindBoxAdmin;
