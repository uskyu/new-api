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

const DEFAULT_ERROR_MESSAGE = '请求失败，请稍后重试';

const getErrorMessage = (error) => {
  const responseData = error?.response?.data;
  if (typeof responseData?.message === 'string' && responseData.message) {
    return responseData.message;
  }
  if (typeof responseData?.data === 'string' && responseData.data) {
    return responseData.data;
  }
  if (typeof error?.message === 'string' && error.message) {
    return error.message;
  }
  return DEFAULT_ERROR_MESSAGE;
};

/**
 * 统一请求封装：
 * - 不触发全局错误提示，由页面按场景自行处理（轮询期间需要静默失败）
 * - 兼容 common.ApiSuccess（success=true / message=""）和
 *   TopUp 风格的 { message: "success", data, url } 响应
 */
const request = async (config) => {
  let response;
  try {
    response = await API.request({ skipErrorHandler: true, ...config });
  } catch (error) {
    throw new Error(getErrorMessage(error));
  }

  const body = response?.data;
  if (!body || body.success === false || body.message === 'error') {
    const fallback =
      typeof body?.data === 'string' && body.data
        ? body.data
        : body?.message || DEFAULT_ERROR_MESSAGE;
    throw new Error(fallback);
  }
  return body;
};

/**
 * 用户端盲盒列表
 * GET /api/blind-box/list
 * 返回 { enabled, cycle_id, boxes }
 */
export async function getBlindBoxList() {
  const body = await request({
    method: 'GET',
    url: '/api/blind-box/list',
  });
  const data = body?.data || {};
  return {
    enabled: data.enabled === true,
    cycle_id: Number(data.cycle_id) || 0,
    boxes: Array.isArray(data.boxes) ? data.boxes : [],
  };
}

/**
 * 当前用户的盲盒订单
 * GET /api/blind-box/orders/self
 * 分页结构为 common.PageInfo：{ page, page_size, total, items }
 */
export async function getMyBlindBoxOrders(page = 1, pageSize = 20) {
  const body = await request({
    method: 'GET',
    url: '/api/blind-box/orders/self',
    params: { p: page, page_size: pageSize },
  });
  const data = body?.data || {};
  return {
    items: Array.isArray(data.items) ? data.items : [],
    total: Number(data.total) || 0,
    page: Number(data.page) || page,
    page_size: Number(data.page_size) || pageSize,
  };
}

/**
 * 购买盲盒（易支付）
 * POST /api/blind-box/purchase
 * 返回 { data: 支付参数, url: 支付地址, trade_no: 订单号 }
 */
export async function purchaseBlindBox(boxId, paymentMethod) {
  const body = await request({
    method: 'POST',
    url: '/api/blind-box/purchase',
    data: {
      box_id: boxId,
      payment_method: paymentMethod,
    },
  });

  const params = body?.data;
  const url = body?.url;
  if (
    !url ||
    typeof url !== 'string' ||
    !params ||
    typeof params !== 'object'
  ) {
    throw new Error(body?.message || '拉起支付失败');
  }

  return {
    url,
    params,
    tradeNo: typeof body?.trade_no === 'string' ? body.trade_no : '',
  };
}

// 盲盒只支持易支付；以下支付方式不能被盲盒购买流程使用
const EXCLUDED_EPAY_METHOD_TYPES = [
  'stripe',
  'creem',
  'waffo',
  'waffo_pancake',
];

/**
 * 读取充值的可用支付方式，仅保留易支付方式。
 * GET /api/user/topup/info
 * 请求失败或没有可用方式时返回空数组，由页面回退到 alipay。
 */
export async function getEpayPaymentMethods() {
  try {
    const body = await request({
      method: 'GET',
      url: '/api/user/topup/info',
    });
    let methods = body?.data?.pay_methods;
    if (typeof methods === 'string') {
      try {
        methods = JSON.parse(methods);
      } catch (error) {
        methods = [];
      }
    }
    if (!Array.isArray(methods)) {
      return [];
    }

    const seenTypes = new Set();
    return methods
      .filter(
        (method) =>
          method &&
          typeof method.type === 'string' &&
          method.type.length > 0 &&
          !EXCLUDED_EPAY_METHOD_TYPES.includes(method.type),
      )
      .filter((method) => {
        if (seenTypes.has(method.type)) return false;
        seenTypes.add(method.type);
        return true;
      })
      .map((method) => ({
        type: method.type,
        name: method.name || method.type,
        icon: method.icon || '',
        color: method.color || '',
      }));
  } catch (error) {
    return [];
  }
}
