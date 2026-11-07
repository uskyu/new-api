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
  Card,
  Col,
  Form,
  InputNumber,
  Row,
  SideSheet,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconClose,
  IconDelete,
  IconPlus,
  IconSave,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { showError, showSuccess } from '../../../helpers';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import {
  buildBlindBoxPayload,
  createBlindBox,
  formatPrice,
  parseTiers,
  updateBlindBox,
} from '../api';

const { Text, Title } = Typography;

const DEFAULT_TIER = {
  min_multiplier: 1,
  max_multiplier: 1,
  probability: 100,
};

const MAX_PRICE = 1000000;
const MAX_STOCK = 1000000;
const MAX_PER_USER_LIMIT = 1000000;
const MAX_TIERS = 100;

function buildInitialValues(box) {
  return {
    name: box?.name || '',
    price: box ? Number(box.price || 0) : 0,
    stock_total: box ? Number(box.stock_total || 0) : 0,
    per_user_limit: box ? Number(box.per_user_limit || 0) : 0,
    sort: box ? Number(box.sort || 0) : 0,
    remark: box?.remark || '',
    enabled: box ? box.enabled !== false : true,
  };
}

function buildInitialTiers(box) {
  const parsed = parseTiers(box?.tiers);
  if (parsed.length === 0) {
    return [{ ...DEFAULT_TIER }];
  }
  return parsed;
}

function validateBox(values, tiers, t) {
  const name = (values?.name || '').trim();
  if (!name) {
    return t('请输入盲盒名称');
  }
  if (name.length > 64) {
    return t('盲盒名称不能超过 64 个字符');
  }

  const price = Number(values?.price);
  if (!Number.isFinite(price) || price <= 0) {
    return t('售价必须大于 0');
  }
  if (price > MAX_PRICE) {
    return t('盲盒售价不能超过 1000000 元');
  }

  const stockTotal = Number(values?.stock_total ?? 0);
  if (!Number.isInteger(stockTotal) || stockTotal < 0) {
    return t('库存总量必须为不小于 0 的整数');
  }
  if (stockTotal > MAX_STOCK) {
    return t('库存总量不能超过 1000000');
  }

  const perUserLimit = Number(values?.per_user_limit ?? 0);
  if (!Number.isInteger(perUserLimit) || perUserLimit < 0) {
    return t('每人限购必须为不小于 0 的整数');
  }
  if (perUserLimit > MAX_PER_USER_LIMIT) {
    return t('每人限购不能超过 1000000');
  }

  const sort = Number(values?.sort ?? 0);
  if (!Number.isInteger(sort)) {
    return t('排序必须为整数');
  }

  if (!Array.isArray(tiers) || tiers.length === 0) {
    return t('至少需要配置一个倍率档位');
  }
  if (tiers.length > MAX_TIERS) {
    return t('倍率档位不能超过 100 个');
  }

  for (let index = 0; index < tiers.length; index++) {
    const prefix = `${t('倍率档位')} ${index + 1}: `;
    const min = Number(tiers[index]?.min_multiplier);
    const max = Number(tiers[index]?.max_multiplier);
    const probability = Number(tiers[index]?.probability);

    if (!Number.isFinite(min) || min <= 0) {
      return `${prefix}${t('最低倍率必须大于 0')}`;
    }
    if (!Number.isFinite(max) || max < min) {
      return `${prefix}${t('最高倍率不能小于最低倍率')}`;
    }
    if (max > 100) {
      return `${prefix}${t('最高倍率不能超过 100')}`;
    }
    if (!Number.isFinite(probability) || probability <= 0) {
      return `${prefix}${t('概率必须大于 0')}`;
    }
    if (probability > 100) {
      return `${prefix}${t('概率不能超过 100')}`;
    }
  }

  const probabilitySum = tiers.reduce(
    (sum, tier) => sum + (Number(tier?.probability) || 0),
    0,
  );
  if (Math.abs(probabilitySum - 100) > 0.01) {
    return `${t('概率合计必须为 100%，当前为')} ${probabilitySum.toFixed(2)}%`;
  }

  return null;
}

const BoxEditor = ({ visible, box, onCancel, onSuccess }) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);

  const [loading, setLoading] = useState(false);
  const [formValues, setFormValues] = useState(buildInitialValues(box));
  const [tiers, setTiers] = useState(buildInitialTiers(box));
  const [formKey, setFormKey] = useState(0);

  const isEdit = Number(box?.id) > 0;

  useEffect(() => {
    if (!visible) {
      return;
    }
    setFormValues(buildInitialValues(box));
    setTiers(buildInitialTiers(box));
    setFormKey((prev) => prev + 1);
  }, [visible, box]);

  const probabilitySum = useMemo(
    () =>
      tiers.reduce((sum, tier) => sum + (Number(tier?.probability) || 0), 0),
    [tiers],
  );

  const probabilityValid = Math.abs(probabilitySum - 100) <= 0.01;

  const price = Number(formValues?.price);
  const rewardRange = useMemo(() => {
    if (!Number.isFinite(price) || price <= 0) {
      return null;
    }
    const minMultipliers = tiers
      .map((tier) => Number(tier?.min_multiplier))
      .filter((value) => Number.isFinite(value) && value > 0);
    const maxMultipliers = tiers
      .map((tier) => Number(tier?.max_multiplier))
      .filter((value) => Number.isFinite(value) && value > 0);
    if (minMultipliers.length === 0 || maxMultipliers.length === 0) {
      return null;
    }
    return [
      price * Math.min(...minMultipliers),
      price * Math.max(...maxMultipliers),
    ];
  }, [price, tiers]);

  const averageMultiplier = useMemo(
    () =>
      tiers.reduce((sum, tier) => {
        const probability = Number(tier?.probability) || 0;
        const min = Number(tier?.min_multiplier) || 0;
        const max = Number(tier?.max_multiplier) || 0;
        return sum + (probability / 100) * ((min + max) / 2);
      }, 0),
    [tiers],
  );

  const updateTier = (index, field, value) => {
    setTiers((prev) =>
      prev.map((tier, tierIndex) =>
        tierIndex === index ? { ...tier, [field]: value } : tier,
      ),
    );
  };

  const addTier = () => {
    setTiers((prev) => {
      if (prev.length >= MAX_TIERS) {
        return prev;
      }
      return [
        ...prev,
        { min_multiplier: 1, max_multiplier: 1, probability: 0 },
      ];
    });
  };

  const removeTier = (index) => {
    setTiers((prev) =>
      prev.length > 1
        ? prev.filter((_, tierIndex) => tierIndex !== index)
        : prev,
    );
  };

  const handleSubmit = async () => {
    let values = formValues;
    if (formApiRef.current) {
      try {
        await formApiRef.current.validate();
        values = formApiRef.current.getValues();
      } catch (e) {
        showError(t('请检查输入'));
        return;
      }
    }

    const validationError = validateBox(values, tiers, t);
    if (validationError) {
      showError(validationError);
      return;
    }

    const payload = buildBlindBoxPayload({
      name: (values.name || '').trim(),
      price: Number(values.price),
      stock_total: Number(values.stock_total ?? 0),
      per_user_limit: Number(values.per_user_limit ?? 0),
      sort: Number(values.sort ?? 0),
      remark: values.remark || '',
      enabled: values.enabled !== false,
      tiers: tiers.map((tier) => ({
        min_multiplier: Number(tier.min_multiplier),
        max_multiplier: Number(tier.max_multiplier),
        probability: Number(tier.probability),
      })),
    });

    setLoading(true);
    try {
      const res = isEdit
        ? await updateBlindBox(box.id, payload)
        : await createBlindBox(payload);
      if (res.data?.success) {
        showSuccess(isEdit ? t('更新成功') : t('创建成功'));
        onSuccess?.();
      } else {
        showError(
          res.data?.message || (isEdit ? t('更新失败') : t('创建失败')),
        );
      }
    } catch (e) {
      showError(isEdit ? t('更新失败') : t('创建失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <SideSheet
      visible={visible}
      placement='right'
      width={isMobile ? '100%' : 640}
      bodyStyle={{ padding: 0 }}
      onCancel={onCancel}
      title={
        <Space>
          <Tag color={isEdit ? 'blue' : 'green'} shape='circle'>
            {isEdit ? t('编辑') : t('新建')}
          </Tag>
          <Title heading={4} className='m-0'>
            {isEdit ? t('编辑盲盒') : t('新建盲盒')}
          </Title>
        </Space>
      }
      footer={
        <div className='flex justify-end'>
          <Space>
            <Button
              theme='solid'
              type='primary'
              icon={<IconSave />}
              loading={loading}
              disabled={!probabilityValid}
              onClick={() => formApiRef.current?.submitForm()}
            >
              {t('保存')}
            </Button>
            <Button
              theme='light'
              type='primary'
              icon={<IconClose />}
              onClick={onCancel}
            >
              {t('取消')}
            </Button>
          </Space>
        </div>
      }
    >
      <Spin spinning={loading}>
        <Form
          key={formKey}
          initValues={buildInitialValues(box)}
          getFormApi={(api) => (formApiRef.current = api)}
          onValueChange={(values) => setFormValues(values)}
          onSubmit={handleSubmit}
        >
          <div className='p-2'>
            <Card
              className='!rounded-2xl shadow-sm border-0 mb-4'
              bodyStyle={{ padding: 16 }}
            >
              <div className='mb-3'>
                <Text className='text-lg font-medium'>{t('基本信息')}</Text>
                <div className='text-xs text-gray-600'>
                  {t('盲盒名称、售价、库存与限购规则')}
                </div>
              </div>

              <Row gutter={12}>
                <Col span={24}>
                  <Form.Input
                    field='name'
                    label={t('名称')}
                    placeholder={t('例如：幸运盲盒')}
                    required
                    maxLength={64}
                    showClear
                    rules={[{ required: true, message: t('请输入盲盒名称') }]}
                  />
                </Col>

                <Col span={12}>
                  <Form.InputNumber
                    field='price'
                    label={t('售价')}
                    required
                    min={0}
                    max={MAX_PRICE}
                    precision={2}
                    extraText={t('用户支付的真实金额，保留两位小数')}
                    style={{ width: '100%' }}
                    rules={[{ required: true, message: t('请输入售价') }]}
                  />
                </Col>

                <Col span={12}>
                  <Form.InputNumber
                    field='stock_total'
                    label={t('库存总量')}
                    min={0}
                    max={MAX_STOCK}
                    precision={0}
                    extraText={t('0 表示不限量，支付成功后扣减库存')}
                    style={{ width: '100%' }}
                  />
                </Col>

                <Col span={12}>
                  <Form.InputNumber
                    field='per_user_limit'
                    label={t('每人限购')}
                    min={0}
                    max={MAX_PER_USER_LIMIT}
                    precision={0}
                    extraText={t('0 表示不限购，按当前活动周期统计')}
                    style={{ width: '100%' }}
                  />
                </Col>

                <Col span={12}>
                  <Form.InputNumber
                    field='sort'
                    label={t('排序')}
                    precision={0}
                    extraText={t('数值越大越靠前')}
                    style={{ width: '100%' }}
                  />
                </Col>

                <Col span={12}>
                  <Form.Switch
                    field='enabled'
                    label={t('启用状态')}
                    checkedText={t('上架')}
                    uncheckedText={t('下架')}
                  />
                </Col>

                <Col span={24}>
                  <Form.TextArea
                    field='remark'
                    label={t('备注')}
                    placeholder={t('仅管理员可见')}
                    autosize
                  />
                </Col>
              </Row>
            </Card>

            <Card
              className='!rounded-2xl shadow-sm border-0 mb-4'
              bodyStyle={{ padding: 16 }}
            >
              <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-3'>
                <div>
                  <Text className='text-lg font-medium'>{t('倍率档位')}</Text>
                  <div className='text-xs text-gray-600'>
                    {t(
                      '用户抽中档位后，在最低倍率与最高倍率之间随机开奖；所有档位概率合计必须为 100%',
                    )}
                  </div>
                </div>
                <Button
                  size='small'
                  type='primary'
                  theme='light'
                  icon={<IconPlus />}
                  disabled={tiers.length >= MAX_TIERS}
                  onClick={addTier}
                >
                  {t('新增档位')}
                </Button>
              </div>

              <div className='flex items-center gap-2 text-xs text-gray-500 mb-1'>
                <span className='flex-1'>{t('最低倍率')}</span>
                <span className='flex-1'>{t('最高倍率')}</span>
                <span className='flex-1'>{t('概率(%)')}</span>
                <span style={{ width: 32 }} />
              </div>

              {tiers.map((tier, index) => (
                <div key={index} className='flex items-center gap-2 mb-2'>
                  <InputNumber
                    value={tier.min_multiplier}
                    min={0}
                    max={100}
                    step={0.1}
                    precision={4}
                    placeholder='1.00'
                    style={{ flex: 1 }}
                    onChange={(value) =>
                      updateTier(index, 'min_multiplier', value)
                    }
                  />
                  <InputNumber
                    value={tier.max_multiplier}
                    min={0}
                    max={100}
                    step={0.1}
                    precision={4}
                    placeholder='1.00'
                    style={{ flex: 1 }}
                    onChange={(value) =>
                      updateTier(index, 'max_multiplier', value)
                    }
                  />
                  <InputNumber
                    value={tier.probability}
                    min={0}
                    max={100}
                    step={0.01}
                    precision={2}
                    placeholder='0'
                    style={{ flex: 1 }}
                    onChange={(value) =>
                      updateTier(index, 'probability', value)
                    }
                  />
                  <Button
                    icon={<IconDelete />}
                    type='danger'
                    theme='borderless'
                    size='small'
                    disabled={tiers.length <= 1}
                    onClick={() => removeTier(index)}
                  />
                </div>
              ))}

              <div
                className='mt-3 p-3 rounded-lg'
                style={{ background: 'var(--semi-color-fill-0)' }}
              >
                <div className='flex flex-wrap items-center gap-x-6 gap-y-1 text-sm'>
                  <Text type={probabilityValid ? 'secondary' : 'danger'}>
                    {`${t('概率合计')}: ${probabilitySum.toFixed(2)}%`}
                  </Text>
                  <Text type='secondary'>
                    {`${t('可得余额范围')}: ${
                      rewardRange
                        ? `${formatPrice(rewardRange[0])} ~ ${formatPrice(rewardRange[1])}`
                        : '-'
                    }`}
                  </Text>
                  <Text type='secondary'>
                    {`${t('预计平均倍率')}: ×${averageMultiplier.toFixed(2)}`}
                  </Text>
                </div>
                {!probabilityValid ? (
                  <Text type='danger' size='small' className='block mt-2'>
                    {t('概率合计必须为 100% 才能保存')}
                  </Text>
                ) : null}
              </div>
            </Card>
          </div>
        </Form>
      </Spin>
    </SideSheet>
  );
};

export default BoxEditor;
