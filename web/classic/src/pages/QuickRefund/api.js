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

import { API } from '../../helpers';

const unwrap = (response) => {
  const body = response?.data;
  if (!body?.success) {
    throw new Error(body?.message || '请求失败');
  }
  return body.data;
};

const request = async (config) => {
  try {
    return unwrap(await API.request({ ...config, skipErrorHandler: true }));
  } catch (error) {
    const message =
      error?.response?.data?.message ||
      error?.response?.data?.error ||
      error?.message ||
      '请求失败，请稍后重试';
    throw new Error(message);
  }
};

export const getRefundOptions = (startTime, endTime) =>
  request({
    method: 'GET',
    url: '/api/refund/admin/options',
    params: { start_time: startTime, end_time: endTime },
  });

export const previewRefund = (payload) =>
  request({
    method: 'POST',
    url: '/api/refund/admin/preview',
    data: payload,
  });

export const createRefundBatch = (payload) =>
  request({
    method: 'POST',
    url: '/api/refund/admin/batches',
    data: payload,
  });

export const getRefundBatches = (page = 1, pageSize = 20) =>
  request({
    method: 'GET',
    url: '/api/refund/admin/batches',
    params: { p: page, page_size: pageSize },
  });

export const getRefundBatch = (id, itemPage = 1, itemPageSize = 20) =>
  request({
    method: 'GET',
    url: `/api/refund/admin/batches/${id}`,
    params: { item_page: itemPage, item_page_size: itemPageSize },
  });
