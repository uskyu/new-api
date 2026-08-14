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
  renderQuota,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

const DEFAULT_INPUTS = {
  'checkin_setting.enabled': false,
  'checkin_setting.captcha_enabled': false,
  'checkin_setting.captcha_kind': 'math',
  'checkin_setting.bonus_metric': 'request_count',
  'checkin_setting.request_count_tiers': '[]',
  'checkin_setting.quota_consumed_tiers': '[]',
};

// 指标 -> 档位存储键 映射
const TIER_KEY_BY_METRIC = {
  request_count: 'checkin_setting.request_count_tiers',
  quota_consumed: 'checkin_setting.quota_consumed_tiers',
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
  // 两套独立档位：按次 / 按额度，切换指标互不覆盖
  const [tiersByMetric, setTiersByMetric] = useState({
    request_count: [],
    quota_consumed: [],
  });
  const refForm = useRef();

  const metricConfigs = [
    {
      key: 'request_count',
      title: t('按昨日调用次数'),
      thresholdLabel: t('昨日调用门槛（次）'),
      mobileThresholdLabel: t('门槛（次数）'),
      placeholder: t('次数门槛'),
    },
    {
      key: 'quota_consumed',
      title: t('按昨日消耗额度'),
      thresholdLabel: t('昨日消耗额度门槛'),
      mobileThresholdLabel: t('门槛（token）'),
      placeholder: t('额度门槛'),
    },
  ];

  function handleFieldChange(fieldName) {
    return (value) => {
      setInputs((prev) => ({ ...prev, [fieldName]: value }));
    };
  }

  function updateTiers(metric, nextTiers) {
    const key = TIER_KEY_BY_METRIC[metric];
    const sorted = [...nextTiers].sort(
      (a, b) => Number(a.threshold) - Number(b.threshold),
    );
    setTiersByMetric((prev) => ({ ...prev, [metric]: nextTiers }));
    setInputs((prev) => ({
      ...prev,
      [key]: JSON.stringify(sorted),
    }));
  }

  function handleTierChange(metric, index, field, value) {
    const tiers = tiersByMetric[metric] || [];
    const nextTiers = tiers.map((tier, i) =>
      i === index ? { ...tier, [field]: Number(value) || 0 } : tier,
    );
    updateTiers(metric, nextTiers);
  }

  function handleAddTier(metric) {
    const tiers = tiersByMetric[metric] || [];
    const step = metric === 'quota_consumed' ? 50000 : 50;
    const maxThreshold = tiers.reduce(
      (max, tier) => Math.max(max, Number(tier.threshold) || 0),
      0,
    );
    const nextThreshold = tiers.length === 0 ? step : maxThreshold + step;
    updateTiers(metric, [
      ...tiers,
      {
        threshold: Math.max(0, nextThreshold),
        min_quota: 2000,
        max_quota: 20000,
      },
    ]);
  }

  function handleRemoveTier(metric, index) {
    const tiers = tiersByMetric[metric] || [];
    updateTiers(
      metric,
      tiers.filter((_, i) => i !== index),
    );
  }

  function validateTiers(metric, tiers) {
    if (!Array.isArray(tiers)) return `${metric}: ${t('档位配置无效')}`;
    const seen = new Set();
    for (const tier of tiers) {
      const threshold = Number(tier?.threshold);
      const minQuota = Number(tier?.min_quota);
      const maxQuota = Number(tier?.max_quota);
      if (![threshold, minQuota, maxQuota].every(Number.isInteger))
        return `${metric}: ${t('档位必须是整数')}`;
      if (threshold < 0) return `${metric}: ${t('档位门槛不能为负数')}`;
      if (minQuota < 0 || maxQuota < 0)
        return `${metric}: ${t('档位奖励额度不能为负数')}`;
      if (minQuota > maxQuota)
        return `${metric}: ${t('档位最低奖励不能高于最高奖励')}`;
      if (seen.has(threshold))
        return `${metric}: ${t('档位门槛不能重复：')} ${threshold}`;
      seen.add(threshold);
    }
    return null;
  }

  async function onSubmit() {
    for (const config of metricConfigs) {
      const validateError = validateTiers(
        config.key,
        tiersByMetric[config.key] || [],
      );
      if (validateError) return showError(validateError);
    }
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
      .then((responses) => {
        const failed = responses.find((response) => !response?.data?.success);
        if (failed) {
          showError(failed.data?.message || t('保存失败，请重试'));
          return;
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
      if (
        typeof DEFAULT_INPUTS[key] === 'boolean' &&
        typeof value === 'string'
      ) {
        value = value === 'true' || value === '1';
      }
      currentInputs[key] = value;
    }
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    setTiersByMetric({
      request_count: parseTiers(
        currentInputs['checkin_setting.request_count_tiers'],
      ),
      quota_consumed: parseTiers(
        currentInputs['checkin_setting.quota_consumed_tiers'],
      ),
    });
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
              {t(
                '签到奖励全部来自活跃档位：未达标奖励为 0，命中多个档位仅按最高档发放，各档不叠加',
              )}
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

          <Form.Section text={t('活跃奖励档位')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t(
                '按调用次数和消耗额度同时判定，每套取命中门槛最高档，比较奖励上限后只发放较高的一套，不叠加',
              )}
            </Typography.Text>
            {inputs['checkin_setting.enabled'] && (
              <div className='mt-3 grid grid-cols-1 gap-5 xl:grid-cols-2'>
                {metricConfigs.map((config) => {
                  const tiers = tiersByMetric[config.key] || [];
                  return (
                    <div
                      key={config.key}
                      className='rounded-xl border border-gray-200 p-3 dark:border-gray-700'
                    >
                      <div className='mb-3 flex items-center justify-between gap-2'>
                        <div className='text-sm font-semibold text-gray-700 dark:text-gray-200'>
                          {config.title}
                        </div>
                      </div>
                      <div className='hidden grid-cols-4 gap-2 pb-2 text-xs font-medium text-gray-500 md:grid'>
                        <div>{config.thresholdLabel}</div>
                        <div>{t('最低奖励（额度）')}</div>
                        <div>{t('最高奖励（额度）')}</div>
                        <div>{t('操作')}</div>
                      </div>
                      {tiers.map((tier, index) => (
                        <div
                          key={`${config.key}-${index}`}
                          className='mb-2 grid grid-cols-2 gap-2 rounded-lg border bg-slate-50 p-3 dark:bg-slate-800 md:grid-cols-4'
                        >
                          <div className='flex flex-col gap-1'>
                            <span className='text-xs text-gray-500 md:hidden'>
                              {config.mobileThresholdLabel}
                            </span>
                            <InputNumber
                              value={tier.threshold}
                              min={0}
                              placeholder={config.placeholder}
                              onChange={(value) =>
                                handleTierChange(
                                  config.key,
                                  index,
                                  'threshold',
                                  value,
                                )
                              }
                              style={{ width: '100%' }}
                            />
                            {config.key === 'quota_consumed' && (
                              <span className='break-words text-[10px] leading-tight text-gray-400'>
                                {t('约')} {renderQuota(tier.threshold, 6)}
                              </span>
                            )}
                          </div>
                          <div className='flex flex-col gap-1'>
                            <span className='text-xs text-gray-500 md:hidden'>
                              {t('最低奖励（额度）')}
                            </span>
                            <InputNumber
                              value={tier.min_quota}
                              min={0}
                              placeholder={t('最低（额度）')}
                              onChange={(value) =>
                                handleTierChange(
                                  config.key,
                                  index,
                                  'min_quota',
                                  value,
                                )
                              }
                              style={{ width: '100%' }}
                            />
                            <span className='break-words text-[10px] leading-tight text-gray-400'>
                              {t('约')} {renderQuota(tier.min_quota, 6)}
                            </span>
                          </div>
                          <div className='flex flex-col gap-1'>
                            <span className='text-xs text-gray-500 md:hidden'>
                              {t('最高奖励（额度）')}
                            </span>
                            <InputNumber
                              value={tier.max_quota}
                              min={0}
                              placeholder={t('最高（额度）')}
                              onChange={(value) =>
                                handleTierChange(
                                  config.key,
                                  index,
                                  'max_quota',
                                  value,
                                )
                              }
                              style={{ width: '100%' }}
                            />
                            <span className='break-words text-[10px] leading-tight text-gray-400'>
                              {t('约')} {renderQuota(tier.max_quota, 6)}
                            </span>
                          </div>
                          <div className='col-span-2 flex items-center justify-end md:col-span-1'>
                            <Button
                              type='danger'
                              theme='borderless'
                              icon={<Trash2 size={16} />}
                              onClick={() =>
                                handleRemoveTier(config.key, index)
                              }
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
                        onClick={() => handleAddTier(config.key)}
                      >
                        {t('添加档位')}
                      </Button>
                    </div>
                  );
                })}
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
