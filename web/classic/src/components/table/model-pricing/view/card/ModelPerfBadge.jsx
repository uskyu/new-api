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
import { Tooltip } from '@douyinfe/semi-ui';

const SERIES_LENGTH = 30;

const formatThroughput = (tps) => {
  if (!Number.isFinite(tps) || tps <= 0) return '—';
  return `${tps.toLocaleString(undefined, { maximumFractionDigits: 2 })} tps`;
};

const formatLatency = (milliseconds) => {
  if (!Number.isFinite(milliseconds) || milliseconds <= 0) return '—';
  return `${milliseconds.toLocaleString(undefined, { maximumFractionDigits: 0 })} ms`;
};

const formatMinute = (timestamp) =>
  new Date(timestamp * 1000).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });

const getBarStyle = (point) => {
  const base = { height: '100%' };
  if (!point || point.request_count <= 0) {
    return { ...base, backgroundColor: 'var(--semi-color-fill-2)' };
  }
  const successRate = Number(point.success_rate);
  if (!Number.isFinite(successRate)) {
    return { ...base, backgroundColor: 'var(--semi-color-fill-2)' };
  }
  if (successRate >= 99.9) {
    return { ...base, backgroundColor: 'var(--semi-color-success)' };
  }
  if (successRate >= 99) {
    return { ...base, backgroundColor: 'var(--semi-color-warning)' };
  }
  if (successRate >= 95) {
    return { ...base, backgroundColor: '#f59e0b' };
  }
  if (successRate >= 90) {
    return { ...base, backgroundColor: '#ea580c' };
  }
  return { ...base, backgroundColor: 'var(--semi-color-danger)' };
};

const ModelPerfBadge = ({ className = '', isMobile = false, perf, t }) => {
  if (!perf?.series || perf.series.length !== SERIES_LENGTH) return null;

  // Header rate is derived from the same 30-minute series so the label always
  // matches the bars, unlike the hours-wide success_rate field.
  const totalRequests = perf.series.reduce(
    (sum, point) => sum + (Number(point.request_count) || 0),
    0,
  );
  const totalSuccess = perf.series.reduce(
    (sum, point) => sum + (Number(point.success_count) || 0),
    0,
  );
  const overallSuccessRate =
    totalRequests > 0
      ? `${((totalSuccess / totalRequests) * 100).toFixed(1)}%`
      : '—';
  const details = `${t('最近30分钟')} · ${t('成功率')}: ${overallSuccessRate}`;
  return (
    <div
      className={`w-[150px] max-w-full shrink-0 ${className}`}
      role='img'
      aria-label={details}
    >
      <div className='flex h-[18px] min-w-0 items-end gap-px'>
        {perf.series.map((point) => {
          const barStyle = getBarStyle(point);
          const pointDetails =
            point.request_count > 0
              ? `${formatMinute(point.ts)}\n${t('请求数')}: ${point.request_count}\n${t('成功率')}: ${Number(point.success_rate).toFixed(1)}%\n${t('平均延迟')}: ${formatLatency(Number(point.avg_latency_ms))}\n${t('吞吐量')}: ${formatThroughput(Number(point.avg_tps))}`
              : `${formatMinute(point.ts)}\n${t('无请求')}`;
          return (
            <Tooltip
              key={point.ts}
              content={
                <div className='whitespace-pre-line text-xs leading-5'>
                  {pointDetails}
                </div>
              }
              position='top'
              trigger={isMobile ? 'click' : 'hover'}
              clickToHide
              showArrow
            >
              <span
                className='flex h-full min-w-0 flex-1 cursor-help items-end rounded-sm transition-opacity hover:opacity-75'
                aria-label={pointDetails}
                onClick={(event) => event.stopPropagation()}
              >
                <span
                  className='block w-full rounded-sm'
                  style={barStyle}
                  aria-hidden='true'
                />
              </span>
            </Tooltip>
          );
        })}
      </div>
    </div>
  );
};

export default ModelPerfBadge;
