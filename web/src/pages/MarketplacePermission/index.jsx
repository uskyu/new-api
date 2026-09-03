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

import React, { useContext, useEffect, useState } from 'react';
import {
  Button,
  Card,
  InputNumber,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconEdit, IconRefresh } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, getLobeHubIcon, renderQuota, showError, showSuccess } from '../../helpers';
import { displayAmountToQuota, quotaToDisplayAmount } from '../../helpers/quota';
import { StatusContext } from '../../context/Status';

const MarketplacePermissionPage = () => {
  const { t } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const currencyReady = Boolean(statusState?.status?.quota_per_unit);
  const [vendors, setVendors] = useState([]);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState(null);
  const [amount, setAmount] = useState(0);

  const loadVendors = async () => {
    setLoading(true);
    try {
      const pageSize = 100;
      let page = 1;
      let total = 0;
      const allVendors = [];
      do {
        const response = await API.get(
          `/api/vendors/?p=${page}&page_size=${pageSize}`,
        );
        if (!response.data?.success) throw new Error(response.data?.message);
        const pageData = response.data.data || {};
        const items = pageData.items || [];
        allVendors.push(...items);
        total = Number(pageData.total || 0);
        page += 1;
        if (items.length === 0) break;
      } while (allVendors.length < total);
      setVendors(allVendors);
    } catch (error) {
      showError(error.response?.data?.message || t('加载供应商列表失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadVendors();
  }, []);

  const openEditor = (vendor) => {
    if (!currencyReady) {
      showError(t('金额配置加载中，请稍后重试'));
      return;
    }
    setEditing(vendor);
    setAmount(quotaToDisplayAmount(vendor.marketplace_quota_threshold).toFixed(2));
  };

  const save = async () => {
    if (!editing) return;
    const numericAmount = Number(amount);
    if (!Number.isFinite(numericAmount) || numericAmount < 0) {
      showError(t('请输入有效的非负金额'));
      return;
    }
    setLoading(true);
    try {
      const response = await API.put(
        `/api/vendors/${editing.id}/marketplace-threshold`,
        { marketplace_quota_threshold: displayAmountToQuota(numericAmount) },
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess(t('模型广场权限更新成功'));
      setEditing(null);
      await loadVendors();
    } catch (error) {
      showError(error.response?.data?.message || t('更新失败'));
    } finally {
      setLoading(false);
    }
  };

  const columns = [
    {
      title: t('供应商'),
      dataIndex: 'name',
      render: (_, vendor) => (
        <div className='flex items-center gap-2'>
          {getLobeHubIcon(vendor.icon || 'Layers', 18)}
          <span>{vendor.name}</span>
        </div>
      ),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (status) => (
        <Tag color={status === 1 ? 'green' : 'grey'}>
          {status === 1 ? t('启用') : t('禁用')}
        </Tag>
      ),
    },
    {
      title: t('模型广场最低累计消费'),
      dataIndex: 'marketplace_quota_threshold',
      render: (value) => {
        if (value <= 0) {
          return <Typography.Text type='tertiary'>{t('无限制')}</Typography.Text>;
        }
        return currencyReady ? renderQuota(value) : t('加载中');
      },
    },
    {
      title: t('操作'),
      render: (_, vendor) => (
        <Button icon={<IconEdit />} onClick={() => openEditor(vendor)}>
          {t('编辑')}
        </Button>
      ),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <Card
        title={t('模型广场权限')}
        headerExtraContent={
          <Button icon={<IconRefresh />} onClick={loadVendors} loading={loading}>
            {t('刷新')}
          </Button>
        }
      >
        <Typography.Text type='tertiary'>
          {t('仅控制供应商旗下模型在模型广场的显示，不影响模型启用、API 调用或渠道路由。')}
        </Typography.Text>
        <Table
          className='mt-4'
          columns={columns}
          dataSource={vendors}
          loading={loading}
          rowKey='id'
          pagination={false}
        />
      </Card>
      {editing && (
        <div className='fixed inset-0 z-50 flex items-center justify-center bg-black/30'>
          <Card className='w-[min(92vw,420px)]' title={`${t('编辑')}：${editing.name}`}>
            <div className='mb-1 font-medium'>
              {t('模型广场最低累计消费')}
            </div>
            <InputNumber
              value={amount}
              onChange={setAmount}
              min={0}
              precision={2}
              className='w-full'
            />
            <Typography.Text type='tertiary' size='small'>
              {t('0 表示不限制，仅影响模型广场显示。')}
            </Typography.Text>
            <div className='mt-4 flex justify-end gap-2'>
              <Button onClick={() => setEditing(null)}>{t('取消')}</Button>
              <Button theme='solid' type='primary' onClick={save} loading={loading}>
                {t('保存')}
              </Button>
            </div>
          </Card>
        </div>
      )}
    </div>
  );
};

export default MarketplacePermissionPage;
