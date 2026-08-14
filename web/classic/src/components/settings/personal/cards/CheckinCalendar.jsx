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

import React, { useState, useEffect, useMemo } from 'react';
import {
  Card,
  Calendar,
  Button,
  Input,
  Typography,
  Avatar,
  Spin,
  Tooltip,
  Collapsible,
  Modal,
} from '@douyinfe/semi-ui';
import {
  CalendarCheck,
  Gift,
  Check,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';
import Turnstile from 'react-turnstile';
import { API, showError, showSuccess, renderQuota } from '../../../../helpers';

const CheckinCalendar = ({ t, status, turnstileEnabled, turnstileSiteKey }) => {
  const [loading, setLoading] = useState(false);
  const [checkinLoading, setCheckinLoading] = useState(false);
  const [turnstileModalVisible, setTurnstileModalVisible] = useState(false);
  const [turnstileWidgetKey, setTurnstileWidgetKey] = useState(0);
  const [captchaModalVisible, setCaptchaModalVisible] = useState(false);
  const [captchaData, setCaptchaData] = useState(null);
  const [captchaAnswer, setCaptchaAnswer] = useState('');
  const [captchaLoading, setCaptchaLoading] = useState(false);
  const [checkinData, setCheckinData] = useState({
    enabled: false,
    stats: {
      checked_in_today: false,
      total_checkins: 0,
      total_quota: 0,
      checkin_count: 0,
      records: [],
    },
  });
  const [currentMonth, setCurrentMonth] = useState(
    new Date().toISOString().slice(0, 7),
  );
  // 初始加载状态，用于避免折叠状态闪烁
  const [initialLoaded, setInitialLoaded] = useState(false);
  // 折叠状态：null 表示未确定（等待首次加载）
  const [isCollapsed, setIsCollapsed] = useState(null);

  // 创建日期到额度的映射，方便快速查找
  const checkinRecordsMap = useMemo(() => {
    const map = {};
    const records = checkinData.stats?.records || [];
    records.forEach((record) => {
      map[record.checkin_date] = record.quota_awarded;
    });
    return map;
  }, [checkinData.stats?.records]);

  // 计算本月获得的额度
  const monthlyQuota = useMemo(() => {
    const records = checkinData.stats?.records || [];
    return records.reduce(
      (sum, record) => sum + (record.quota_awarded || 0),
      0,
    );
  }, [checkinData.stats?.records]);

  const normalizeTiers = (tiers) => {
    const source = Array.isArray(tiers) ? tiers : [];
    const seenThresholds = new Set();
    return source
      .filter((tier) => {
        const threshold = Number(tier?.threshold);
        const minQuota = Number(tier?.min_quota);
        const maxQuota = Number(tier?.max_quota);
        if (
          !Number.isFinite(threshold) ||
          !Number.isFinite(minQuota) ||
          !Number.isFinite(maxQuota) ||
          threshold < 0 ||
          minQuota < 0 ||
          maxQuota < minQuota ||
          seenThresholds.has(threshold)
        ) {
          return false;
        }
        seenThresholds.add(threshold);
        return true;
      })
      .sort((a, b) => Number(a.threshold) - Number(b.threshold));
  };

  const isLegacyQuotaMetric = checkinData?.bonus_metric === 'quota_consumed';
  const requestCountTiers = useMemo(
    () =>
      normalizeTiers(
        checkinData?.request_count_tiers ??
          (isLegacyQuotaMetric ? [] : checkinData?.bonus_tiers),
      ),
    [
      checkinData?.request_count_tiers,
      checkinData?.bonus_tiers,
      isLegacyQuotaMetric,
    ],
  );
  const quotaConsumedTiers = useMemo(
    () =>
      normalizeTiers(
        checkinData?.quota_consumed_tiers ??
          (isLegacyQuotaMetric ? checkinData?.bonus_tiers : []),
      ),
    [
      checkinData?.quota_consumed_tiers,
      checkinData?.bonus_tiers,
      isLegacyQuotaMetric,
    ],
  );
  const requestCountMatchedTier =
    checkinData?.request_count_tier ??
    (!isLegacyQuotaMetric ? checkinData?.bonus_tier : null);
  const quotaConsumedMatchedTier =
    checkinData?.quota_consumed_tier ??
    (isLegacyQuotaMetric ? checkinData?.bonus_tier : null);
  const selectedBonusMetric =
    checkinData?.selected_bonus_metric ??
    (checkinData?.bonus_tier ? checkinData?.bonus_metric : '');
  const selectedBonusTier =
    checkinData?.selected_bonus_tier ?? checkinData?.bonus_tier;
  const hasMatchedTier = Boolean(selectedBonusTier);

  // 获取签到状态
  const fetchCheckinStatus = async (month) => {
    const isFirstLoad = !initialLoaded;
    setLoading(true);
    try {
      const res = await API.get(`/api/user/checkin?month=${month}`);
      const { success, data, message } = res.data;
      if (success) {
        setCheckinData(data);
        // 首次加载时，根据签到状态设置折叠状态
        if (isFirstLoad) {
          setIsCollapsed(data.stats?.checked_in_today ?? false);
          setInitialLoaded(true);
        }
      } else {
        showError(message || t('获取签到状态失败'));
        if (isFirstLoad) {
          setIsCollapsed(false);
          setInitialLoaded(true);
        }
      }
    } catch (error) {
      showError(t('获取签到状态失败'));
      if (isFirstLoad) {
        setIsCollapsed(false);
        setInitialLoaded(true);
      }
    } finally {
      setLoading(false);
    }
  };

  const postCheckin = async (token) => {
    const url = token
      ? `/api/user/checkin?turnstile=${encodeURIComponent(token)}`
      : '/api/user/checkin';
    return API.post(url);
  };

  const shouldTriggerTurnstile = (message) => {
    if (!turnstileEnabled) return false;
    if (typeof message !== 'string') return true;
    return message.includes('Turnstile');
  };

  const showCheckinSuccess = (data) => {
    if (Number(data?.quota_awarded) === 0) {
      showSuccess(t('签到成功，昨日活跃度未达标，本次无额度奖励'));
      return;
    }
    showSuccess(t('签到成功！获得') + ' ' + renderQuota(data?.quota_awarded));
  };

  const doCheckin = async (token) => {
    setCheckinLoading(true);
    try {
      const res = await postCheckin(token);
      const { success, data, message } = res.data;
      if (success) {
        showCheckinSuccess(data);
        // 刷新签到状态
        fetchCheckinStatus(currentMonth);
        setTurnstileModalVisible(false);
      } else {
        if (!token && shouldTriggerTurnstile(message)) {
          if (!turnstileSiteKey) {
            showError('Turnstile is enabled but site key is empty.');
            return;
          }
          setTurnstileModalVisible(true);
          return;
        }
        if (token && shouldTriggerTurnstile(message)) {
          setTurnstileWidgetKey((v) => v + 1);
        }
        showError(message || t('签到失败'));
      }
    } catch (error) {
      showError(t('签到失败'));
    } finally {
      setCheckinLoading(false);
    }
  };

  const fetchCheckinCaptcha = async () => {
    setCaptchaLoading(true);
    try {
      const res = await API.get('/api/user/checkin/captcha');
      const { success, data, message } = res.data;
      if (success) {
        setCaptchaData(data);
        setCaptchaAnswer('');
        setCaptchaModalVisible(true);
      } else {
        showError(message || t('获取验证码失败'));
      }
    } catch (error) {
      showError(t('获取验证码失败'));
    } finally {
      setCaptchaLoading(false);
    }
  };

  const doCheckinWithCaptcha = async () => {
    if (!captchaData?.captcha_id || !captchaAnswer.trim()) {
      showError(t('请输入验证码'));
      return;
    }
    setCheckinLoading(true);
    try {
      const url =
        `/api/user/checkin?captcha_id=${encodeURIComponent(captchaData.captcha_id)}` +
        `&captcha_answer=${encodeURIComponent(captchaAnswer.trim())}`;
      const res = await API.post(url);
      const { success, data, message } = res.data;
      if (success) {
        showCheckinSuccess(data);
        fetchCheckinStatus(currentMonth);
        setCaptchaModalVisible(false);
      } else {
        const errorText = String(message || '');
        showError(message || t('签到失败'));
        if (
          errorText.includes('次数过多') ||
          errorText.includes('过期') ||
          errorText.includes('不存在')
        ) {
          setCaptchaModalVisible(false);
        }
      }
    } catch (error) {
      showError(t('签到失败'));
    } finally {
      setCheckinLoading(false);
    }
  };

  useEffect(() => {
    if (status?.checkin_enabled) {
      fetchCheckinStatus(currentMonth);
    }
  }, [status?.checkin_enabled, currentMonth]);

  // 如果签到功能未启用，不显示组件
  if (!status?.checkin_enabled) {
    return null;
  }

  // 日期渲染函数 - 显示签到状态和获得的额度
  const dateRender = (dateString) => {
    // Semi Calendar 传入的 dateString 是 Date.toString() 格式
    // 需要转换为 YYYY-MM-DD 格式来匹配后端数据
    const date = new Date(dateString);
    if (isNaN(date.getTime())) {
      return null;
    }
    // 使用本地时间格式化，避免时区问题
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const formattedDate = `${year}-${month}-${day}`; // YYYY-MM-DD
    const quotaAwarded = checkinRecordsMap[formattedDate];
    const isCheckedIn = quotaAwarded !== undefined;

    if (isCheckedIn) {
      return (
        <Tooltip
          content={`${t('获得')} ${renderQuota(quotaAwarded)}`}
          position='top'
        >
          <div className='absolute inset-0 flex flex-col items-center justify-center cursor-pointer'>
            <div className='w-6 h-6 rounded-full bg-green-500 flex items-center justify-center mb-0.5 shadow-sm'>
              <Check size={14} className='text-white' strokeWidth={3} />
            </div>
            <div className='text-[10px] font-medium text-green-600 dark:text-green-400 leading-none'>
              {renderQuota(quotaAwarded)}
            </div>
          </div>
        </Tooltip>
      );
    }
    return null;
  };

  // 处理月份变化
  const handleMonthChange = (date) => {
    const month = date.toISOString().slice(0, 7);
    setCurrentMonth(month);
  };

  return (
    <Card className='!rounded-2xl'>
      <Modal
        title={t('输入验证码')}
        visible={captchaModalVisible}
        footer={null}
        centered
        onCancel={() => {
          setCaptchaModalVisible(false);
          setCaptchaAnswer('');
        }}
      >
        <div className='flex flex-col items-center gap-3 py-2'>
          <img
            src={captchaData?.image_base64}
            alt='captcha'
            className='max-w-full h-auto rounded-lg border'
          />
          <Input
            placeholder={t('请输入验证码')}
            value={captchaAnswer}
            onChange={(value) => setCaptchaAnswer(value)}
            onEnterPress={doCheckinWithCaptcha}
            autoFocus
            style={{ width: '100%' }}
          />
          <div className='flex gap-2'>
            <Button loading={captchaLoading} onClick={fetchCheckinCaptcha}>
              {t('刷新验证码')}
            </Button>
            <Button
              type='primary'
              loading={checkinLoading}
              onClick={doCheckinWithCaptcha}
            >
              {t('确认签到')}
            </Button>
          </div>
        </div>
      </Modal>
      <Modal
        title='Security Check'
        visible={turnstileModalVisible}
        footer={null}
        centered
        onCancel={() => {
          setTurnstileModalVisible(false);
          setTurnstileWidgetKey((v) => v + 1);
        }}
      >
        <div className='flex justify-center py-2'>
          <Turnstile
            key={turnstileWidgetKey}
            sitekey={turnstileSiteKey}
            onVerify={(token) => {
              doCheckin(token);
            }}
            onExpire={() => {
              setTurnstileWidgetKey((v) => v + 1);
            }}
          />
        </div>
      </Modal>

      {/* 卡片头部 */}
      <div className='flex items-center justify-between'>
        <div
          className='flex items-center flex-1 cursor-pointer'
          onClick={() => setIsCollapsed(!isCollapsed)}
        >
          <Avatar
            size='small'
            className='mr-3 anime-icon-bubble anime-icon-bubble-pink'
          >
            <CalendarCheck size={16} />
          </Avatar>
          <div className='flex-1'>
            <div className='flex items-center gap-2'>
              <Typography.Text className='text-lg font-medium'>
                {t('每日签到')}
              </Typography.Text>
              {isCollapsed ? (
                <ChevronDown size={16} className='text-gray-400' />
              ) : (
                <ChevronUp size={16} className='text-gray-400' />
              )}
            </div>
            <div className='text-xs text-gray-500 dark:text-gray-400'>
              {!initialLoaded
                ? t('正在加载签到状态...')
                : checkinData.stats?.checked_in_today
                  ? t('今日已签到，累计签到') +
                    ` ${checkinData.stats?.total_checkins || 0} ` +
                    t('天')
                  : checkinData?.bonus_enabled
                    ? t('达标后随机获得对应档位奖励')
                    : t('当前未配置签到奖励档位')}
            </div>
          </div>
        </div>
        <Button
          type='primary'
          theme='solid'
          className='btn-anime-gradient'
          icon={<Gift size={16} />}
          onClick={() =>
            checkinData?.captcha_enabled ? fetchCheckinCaptcha() : doCheckin()
          }
          loading={checkinLoading || !initialLoaded}
          disabled={!initialLoaded || checkinData.stats?.checked_in_today}
        >
          {!initialLoaded
            ? t('加载中...')
            : checkinData.stats?.checked_in_today
              ? t('今日已签到')
              : t('立即签到')}
        </Button>
      </div>

      {/* 可折叠内容 */}
      <Collapsible isOpen={isCollapsed === false} keepDOM>
        {/* 签到统计 */}
        <div className='grid grid-cols-3 gap-3 mb-4 mt-4'>
          <div className='anime-stat-tile anime-stat-tile-pink'>
            <div className='text-xl font-bold anime-text-pink'>
              {checkinData.stats?.total_checkins || 0}
            </div>
            <div className='text-xs text-gray-500'>{t('累计签到')}</div>
          </div>
          <div className='anime-stat-tile anime-stat-tile-peach'>
            <div className='text-xl font-bold anime-text-peach'>
              {renderQuota(monthlyQuota, 6)}
            </div>
            <div className='text-xs text-gray-500'>{t('本月获得')}</div>
          </div>
          <div className='anime-stat-tile anime-stat-tile-sky'>
            <div className='text-xl font-bold anime-text-sky'>
              {renderQuota(checkinData.stats?.total_quota || 0, 6)}
            </div>
            <div className='text-xs text-gray-500'>{t('累计获得')}</div>
          </div>
        </div>

        {/* 两套指标同时展示：各自取最高命中档，最终只发放奖励上限更高的一套 */}
        {checkinData?.bonus_enabled && (
          <div className='mb-4 space-y-4'>
            {[
              {
                key: 'request_count',
                title: t('按昨日调用次数'),
                metric: checkinData.yesterday_calls ?? 0,
                metricText: `${checkinData.yesterday_calls ?? 0} ${t('次')}`,
                tiers: requestCountTiers,
                matchedTier: requestCountMatchedTier,
                isSelected: selectedBonusMetric === 'request_count',
                formatThreshold: (value) => `${value} ${t('次')}`,
              },
              {
                key: 'quota_consumed',
                title: t('按昨日消耗额度'),
                metric: checkinData.yesterday_quota ?? 0,
                metricText: renderQuota(checkinData.yesterday_quota ?? 0, 6),
                tiers: quotaConsumedTiers,
                matchedTier: quotaConsumedMatchedTier,
                isSelected: selectedBonusMetric === 'quota_consumed',
                formatThreshold: (value) => renderQuota(value, 6),
              },
            ].map((metricConfig) => {
              const matchedThreshold = Number(
                metricConfig.matchedTier?.threshold,
              );
              const hasMatch = Boolean(metricConfig.matchedTier);
              return (
                <div
                  key={metricConfig.key}
                  className='rounded-xl anime-tier-panel overflow-hidden'
                >
                  <div className='flex flex-col gap-1 px-3 py-2 anime-tier-header sm:flex-row sm:items-center sm:justify-between'>
                    <span className='text-xs font-semibold text-gray-700 dark:text-gray-300'>
                      {metricConfig.title}
                    </span>
                    <span className='text-[11px] break-words text-gray-500 dark:text-gray-400 sm:text-right'>
                      {metricConfig.metricText}
                    </span>
                  </div>
                  <div
                    className={`anime-tier-summary ${
                      hasMatch ? '' : 'anime-tier-summary-empty'
                    }`}
                  >
                    {hasMatch ? (
                      <>
                        <span className='font-semibold'>
                          {metricConfig.isSelected
                            ? t('最终采用此规则')
                            : t('已达标，仅作为候选规则')}
                        </span>{' '}
                        <span className='break-words'>
                          {t('奖励范围')}:{' '}
                          {renderQuota(metricConfig.matchedTier.min_quota, 6)} -{' '}
                          {renderQuota(metricConfig.matchedTier.max_quota, 6)}
                        </span>
                      </>
                    ) : (
                      <span className='font-semibold'>
                        {metricConfig.tiers.length === 0
                          ? t('未配置档位')
                          : t('未达标')}
                      </span>
                    )}
                  </div>
                  <div className='divide-y divide-gray-100 dark:divide-gray-800'>
                    {metricConfig.tiers.map((tier) => {
                      const threshold = Number(tier.threshold);
                      const isCurrent =
                        hasMatch && threshold === matchedThreshold;
                      const isAchieved =
                        Number(metricConfig.metric) >= threshold;
                      return (
                        <div
                          key={`${metricConfig.key}-${threshold}`}
                          className={`flex flex-col gap-1.5 px-3 py-2 sm:flex-row sm:items-center sm:justify-between ${
                            isCurrent
                              ? 'anime-tier-active'
                              : isAchieved
                                ? 'anime-tier-achieved'
                                : 'anime-tier-row'
                          }`}
                        >
                          <span className='flex min-w-0 flex-wrap items-center gap-1.5'>
                            <span
                              className={`text-xs ${
                                isCurrent
                                  ? 'font-semibold anime-text-pink'
                                  : 'text-gray-600 dark:text-gray-400'
                              }`}
                            >
                              ≥ {metricConfig.formatThreshold(threshold)}
                            </span>
                            <span
                              className={`anime-tier-status ${
                                isCurrent
                                  ? 'anime-tier-badge'
                                  : isAchieved
                                    ? 'anime-tier-status-achieved'
                                    : 'anime-tier-status-pending'
                              }`}
                            >
                              {isCurrent
                                ? metricConfig.isSelected
                                  ? t('已达标 · 最终档位')
                                  : t('已达标 · 候选档位')
                                : isAchieved
                                  ? t('已达标')
                                  : t('未达标')}
                            </span>
                          </span>
                          <span className='min-w-0 text-xs break-words text-gray-600 dark:text-gray-400 sm:text-right'>
                            {t('奖励范围')}:{' '}
                            <span className={isCurrent ? 'font-semibold' : ''}>
                              {renderQuota(tier.min_quota, 6)} -{' '}
                              {renderQuota(tier.max_quota, 6)}
                            </span>
                          </span>
                        </div>
                      );
                    })}
                  </div>
                </div>
              );
            })}
            <div
              className={`anime-tier-summary ${
                hasMatchedTier ? '' : 'anime-tier-summary-empty'
              }`}
            >
              {hasMatchedTier
                ? `${t('最终奖励来源')}: ${
                    selectedBonusMetric === 'quota_consumed'
                      ? t('昨日消耗额度')
                      : t('昨日调用次数')
                  }`
                : t('昨日未达标，本次签到奖励为 0')}
              <div className='mt-0.5 text-[10px] font-normal opacity-80'>
                {t('两套规则只选择一套奖励，不叠加')}
              </div>
            </div>
          </div>
        )}

        {/* 签到日历 - 使用更紧凑的样式 */}
        <Spin spinning={loading}>
          <div className='border rounded-lg overflow-hidden checkin-calendar max-w-md mx-auto w-full'>
            <style>{`
            .checkin-calendar .semi-calendar {
              font-size: 13px;
            }
            .checkin-calendar .semi-calendar-month-header {
              padding: 8px 12px;
            }
            .checkin-calendar .semi-calendar-month-week-row {
              height: 28px;
            }
            .checkin-calendar .semi-calendar-month-week-row th {
              font-size: 12px;
              padding: 4px 0;
            }
            .checkin-calendar .semi-calendar-month-grid-row {
              height: auto;
            }
            .checkin-calendar .semi-calendar-month-grid-row td {
              height: 44px;
              padding: 2px;
            }
            .checkin-calendar .semi-calendar-month-grid-row-cell {
              position: relative;
              height: 100%;
            }
            .checkin-calendar .semi-calendar-month-grid-row-cell-day {
              position: absolute;
              top: 4px;
              left: 50%;
              transform: translateX(-50%);
              font-size: 12px;
              z-index: 1;
            }
            .checkin-calendar .semi-calendar-month-same {
              background: transparent;
            }
            .checkin-calendar .semi-calendar-month .semi-calendar-today .semi-calendar-today-date {
              background: var(--semi-color-primary);
              color: #fff;
              border-radius: 50%;
            }
          `}</style>
            <Calendar
              mode='month'
              onChange={handleMonthChange}
              dateGridRender={(dateString, date) => dateRender(dateString)}
            />
          </div>
        </Spin>

        {/* 签到说明 */}
        <div className='mt-3 p-2.5 anime-note-panel rounded-xl'>
          <Typography.Text type='tertiary' className='text-xs'>
            <ul className='list-disc list-inside space-y-0.5'>
              <li>
                {checkinData?.bonus_enabled
                  ? t('两套规则同时判定，最终只发放奖励上限更高的一套')
                  : t('当前未配置签到奖励档位，签到奖励为 0')}
              </li>
              <li>{t('签到奖励将直接添加到您的账户余额')}</li>
              <li>{t('每日仅可签到一次，请勿重复签到')}</li>
            </ul>
          </Typography.Text>
        </div>
      </Collapsible>
    </Card>
  );
};

export default CheckinCalendar;
