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

import React, { useEffect, useState, useRef } from 'react';
import {
  Button,
  Col,
  Form,
  InputNumber,
  Row,
  Spin,
  Typography,
} from '@douyinfe/semi-ui';
import { Plus, Trash2 } from 'lucide-react';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

const DEFAULT_INPUTS = {
  'checkin_setting.enabled': false,
  'checkin_setting.min_quota': 1000,
  'checkin_setting.max_quota': 10000,
  'checkin_setting.captcha_enabled': false,
  'checkin_setting.captcha_kind': 'math',
  'checkin_setting.bonus_enabled': false,
  'checkin_setting.bonus_metric': 'request_count',
  'checkin_setting.bonus_tiers': '[]',
};

const parseTiers = (value) => {
  if (!value) return [];
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed : [];
  } catch (error) {
    return [];
  }
};

export default function SettingsCheckin(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState(DEFAULT_INPUTS);
  const [inputsRow, setInputsRow] = useState(DEFAULT_INPUTS);
  const [tiers, setTiers] = useState([]);
  const refForm = useRef();

  function handleFieldChange(fieldName) {
    return (value) => {
      setInputs((prev) => ({ ...prev, [fieldName]: value }));
    };
  }

  function updateTiers(nextTiers) {
    setTiers(nextTiers);
    setInputs((prev) => ({
      ...prev,
      'checkin_setting.bonus_tiers': JSON.stringify(nextTiers),
    }));
  }

  function handleTierChange(index, field, value) {
    const nextTiers = tiers.map((tier, i) =>
      i === index ? { ...tier, [field]: Number(value) || 0 } : tier,
    );
    updateTiers(nextTiers);
  }

  function handleAddTier() {
    updateTiers([
      ...tiers,
      { threshold: 50, min_quota: 2000, max_quota: 20000 },
    ]);
  }

  function handleRemoveTier(index) {
    updateTiers(tiers.filter((_, i) => i !== index));
  }

  async function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow);
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    const requestQueue = updateArray.map((item) =>
      API.put('/api/option/', {
        key: item.key,
        value: String(inputs[item.key]),
      }),
    );
    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (requestQueue.length === 1) {
          if (res.includes(undefined)) return;
        } else if (requestQueue.length > 1) {
          if (res.includes(undefined))
            return showError(t('部分保存失败，请重试'));
        }
        showSuccess(t('保存成功'));
        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  useEffect(() => {
    const currentInputs = {};
    for (const key of Object.keys(DEFAULT_INPUTS)) {
      let value =
        props.options &&
        Object.prototype.hasOwnProperty.call(props.options, key)
          ? props.options[key]
          : DEFAULT_INPUTS[key];
      if (typeof DEFAULT_INPUTS[key] === 'boolean' && typeof value === 'string') {
        value = value === 'true' || value === '1';
      }
      currentInputs[key] = value;
    }
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    setTiers(parseTiers(currentInputs['checkin_setting.bonus_tiers']));
    if (refForm.current) {
      refForm.current.setValues(currentInputs);
    }
  }, [props.options]);

  return (
    <>
      <Spin spinning={loading}>
        <Form
          initValues={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('签到设置')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('签到功能允许用户每日签到获取随机额度奖励')}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'checkin_setting.enabled'}
                  label={t('启用签到功能')}
                  size='default'
                  checkedText='开'
                  uncheckedText='关'
                  onChange={handleFieldChange('checkin_setting.enabled')}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.min_quota'}
                  label={t('签到最小额度')}
                  placeholder={t('签到奖励的最小额度')}
                  onChange={handleFieldChange('checkin_setting.min_quota')}
                  min={0}
                  disabled={!inputs['checkin_setting.enabled']}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'checkin_setting.max_quota'}
                  label={t('签到最大额度')}
                  placeholder={t('签到奖励的最大额度')}
                  onChange={handleFieldChange('checkin_setting.max_quota')}
                  min={0}
                  disabled={!inputs['checkin_setting.enabled']}
                />
              </Col>
            </Row>
          </Form.Section>

          <Form.Section text={t('签到验证码')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('开启后用户签到前需要输入图形验证码，防止脚本自动签到')}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'checkin_setting.captcha_enabled'}
                  label={t('启用签到验证码')}
                  size='default'
                  checkedText='开'
                  uncheckedText='关'
                  disabled={!inputs['checkin_setting.enabled']}
                  onChange={handleFieldChange(
                    'checkin_setting.captcha_enabled',
                  )}
                />
              </Col>
              {inputs['checkin_setting.captcha_enabled'] && (
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.Select
                    field={'checkin_setting.captcha_kind'}
                    label={t('验证码类型')}
                    optionList={[
                      { label: t('数学算式'), value: 'math' },
                      { label: t('纯数字'), value: 'digit' },
                    ]}
                    onChange={handleFieldChange('checkin_setting.captcha_kind')}
                  />
                </Col>
              )}
            </Row>
          </Form.Section>

          <Form.Section text={t('活跃阶梯奖励')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t(
                '根据昨日调用量命中最高档，签到奖励使用该档最低至最高区间随机发放',
              )}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'checkin_setting.bonus_enabled'}
                  label={t('启用活跃阶梯奖励')}
                  size='default'
                  checkedText='开'
                  uncheckedText='关'
                  disabled={!inputs['checkin_setting.enabled']}
                  onChange={handleFieldChange('checkin_setting.bonus_enabled')}
                />
              </Col>
              {inputs['checkin_setting.bonus_enabled'] && (
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.Select
                    field={'checkin_setting.bonus_metric'}
                    label={t('统计口径')}
                    optionList={[
                      { label: t('昨日调用次数'), value: 'request_count' },
                      { label: t('昨日消耗额度'), value: 'quota_consumed' },
                    ]}
                    onChange={handleFieldChange('checkin_setting.bonus_metric')}
                  />
                </Col>
              )}
            </Row>
            {inputs['checkin_setting.bonus_enabled'] && (
              <div className='mt-3'>
                <div className='hidden md:grid grid-cols-4 gap-2 mb-2 text-xs font-medium text-gray-500'>
                  <div>
                    {inputs['checkin_setting.bonus_metric'] === 'quota_consumed'
                      ? t('昨日消耗额度门槛')
                      : t('昨日调用门槛（次）')}
                  </div>
                  <div>{t('最低奖励（额度）')}</div>
                  <div>{t('最高奖励（额度）')}</div>
                  <div>{t('操作')}</div>
                </div>
                {tiers.map((tier, index) => (
                  <div
                    key={index}
                    className='grid grid-cols-2 md:grid-cols-4 gap-2 mb-2 p-3 border rounded-lg bg-slate-50 dark:bg-slate-800'
                  >
                    <div className='flex flex-col gap-1'>
                      <span className='text-xs text-gray-500 md:hidden'>
                        {inputs['checkin_setting.bonus_metric'] ===
                        'quota_consumed'
                          ? t('门槛（token）')
                          : t('门槛（次数）')}
                      </span>
                      <InputNumber
                        value={tier.threshold}
                        min={0}
                        placeholder={
                          inputs['checkin_setting.bonus_metric'] ===
                          'quota_consumed'
                            ? t('额度门槛')
                            : t('次数门槛')
                        }
                        onChange={(value) =>
                          handleTierChange(index, 'threshold', value)
                        }
                        style={{ width: '100%' }}
                      />
                    </div>
                    <div className='flex flex-col gap-1'>
                      <span className='text-xs text-gray-500 md:hidden'>
                        {t('最低奖励（token）')}
                      </span>
                      <InputNumber
                        value={tier.min_quota}
                        min={0}
                        placeholder={t('最低（额度）')}
                        onChange={(value) =>
                          handleTierChange(index, 'min_quota', value)
                        }
                        style={{ width: '100%' }}
                      />
                    </div>
                    <div className='flex flex-col gap-1'>
                      <span className='text-xs text-gray-500 md:hidden'>
                        {t('最高奖励（token）')}
                      </span>
                      <InputNumber
                        value={tier.max_quota}
                        min={0}
                        placeholder={t('最高（额度）')}
                        onChange={(value) =>
                          handleTierChange(index, 'max_quota', value)
                        }
                        style={{ width: '100%' }}
                      />
                    </div>
                    <div className='col-span-2 flex items-center justify-end md:col-span-1'>
                      <Button
                        type='danger'
                        theme='borderless'
                        icon={<Trash2 size={16} />}
                        onClick={() => handleRemoveTier(index)}
                      >
                        {t('删除')}
                      </Button>
                    </div>
                  </div>
                ))}
                <Button
                  type='primary'
                  theme='light'
                  icon={<Plus size={16} />}
                  onClick={handleAddTier}
                >
                  {t('添加档位')}
                </Button>
              </div>
            )}
          </Form.Section>

          <Row style={{ marginTop: 16 }}>
            <Button size='default' onClick={onSubmit}>
              {t('保存签到设置')}
            </Button>
          </Row>
        </Form>
      </Spin>
    </>
  );
}
