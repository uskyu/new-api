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

const ADMIN_BASE = '/api/blind-box/admin';

/**
 * 管理端盲盒设置
 * GET /api/blind-box/admin/setting
 * 返回 { enabled, cycle_id, order_timeout_minutes }
 */
export function getBlindBoxSetting() {
  return API.get(`${ADMIN_BASE}/setting`);
}

/**
 * 管理端保存盲盒设置
 * PUT /api/blind-box/admin/setting
 * 只需提交 enabled 与 order_timeout_minutes；
 * 关闭 -> 开启 时由后端自动生成新的 cycle_id。
 */
export function updateBlindBoxSetting(setting = {}) {
  return API.put(`${ADMIN_BASE}/setting`, {
    enabled: !!setting.enabled,
    order_timeout_minutes: Number(setting.order_timeout_minutes || 0),
  });
}

/**
 * 管理端盲盒列表（包含已下架盲盒）
 * GET /api/blind-box/admin/boxes
 * 后端会附带 min_reward / max_reward / remaining
 */
export function getAdminBlindBoxes() {
  return API.get(`${ADMIN_BASE}/boxes`);
}

/**
 * 新建盲盒
 * POST /api/blind-box/admin/box
 */
export function createBlindBox(box) {
  return API.post(`${ADMIN_BASE}/box`, box);
}

/**
 * 更新盲盒
 * PUT /api/blind-box/admin/box/:id
 */
export function updateBlindBox(id, box) {
  return API.put(`${ADMIN_BASE}/box/${id}`, box);
}

/**
 * 管理端盲盒订单列表
 * GET /api/blind-box/admin/orders
 * params: p, page_size, box_id, user_id, status, keyword,
 *         start_timestamp, end_timestamp
 */
export function getAdminBlindBoxOrders(params = {}) {
  return API.get(`${ADMIN_BASE}/orders`, { params });
}

/**
 * 手动补单（对 pending / expired 订单执行开奖入账）
 * POST /api/blind-box/admin/order/complete
 */
export function completeBlindBoxOrder(tradeNo) {
  return API.post(`${ADMIN_BASE}/order/complete`, { trade_no: tradeNo });
}

/**
 * 解析盲盒倍率档位。
 * 后端 BlindBox.Tiers 为 TEXT 字段，接口返回的是 JSON 字符串；
 * 这里同时兼容数组形式，避免不同部署下的反序列化差异。
 */
export function parseTiers(tiers) {
  let data = tiers;
  if (typeof data === 'string') {
    if (!data.trim()) return [];
    try {
      data = JSON.parse(data);
    } catch (e) {
      return [];
    }
  }
  if (!Array.isArray(data)) return [];
  return data.map((tier) => ({
    min_multiplier: Number(tier?.min_multiplier) || 0,
    max_multiplier: Number(tier?.max_multiplier) || 0,
    probability: Number(tier?.probability) || 0,
  }));
}

/**
 * 构造新建/更新盲盒的请求体。
 * 按契约后端的 BlindBox.Tiers 字段为 JSON 字符串，因此这里统一序列化。
 */
export function buildBlindBoxPayload(box = {}) {
  let tiers = box.tiers;
  if (typeof tiers !== 'string') {
    tiers = JSON.stringify(Array.isArray(tiers) ? tiers : []);
  }
  const price = Number(box.price || 0);
  return {
    name: (box.name || '').trim(),
    price: Number.isFinite(price) ? Math.round(price * 100) / 100 : 0,
    stock_total: Number(box.stock_total || 0),
    per_user_limit: Number(box.per_user_limit || 0),
    enabled: !!box.enabled,
    sort: Number(box.sort || 0),
    tiers,
    remark: box.remark || '',
  };
}

/**
 * 格式化盲盒售价 / 售价换算后的可得余额。
 * 盲盒仅接入易支付，price 直接作为易支付金额（单位：元），
 * 因此统一按人民币展示，不做汇率换算。
 */
export function formatPrice(value) {
  const amount = Number(value);
  if (!Number.isFinite(amount)) return '¥0.00';
  return `¥${amount.toFixed(2)}`;
}

/**
 * 获取盲盒可得余额范围 [min, max]。
 * 优先使用后端返回的 min_reward / max_reward，
 * 缺失时按 price × 档位最低/最高倍率兜底计算。
 */
export function getBoxRewardRange(box) {
  const apiMin = Number(box?.min_reward);
  const apiMax = Number(box?.max_reward);
  if (Number.isFinite(apiMin) && Number.isFinite(apiMax) && apiMax > 0) {
    return [apiMin, apiMax];
  }

  const price = Number(box?.price);
  if (!Number.isFinite(price) || price <= 0) return [0, 0];

  const tiers = parseTiers(box?.tiers);
  const minMultipliers = tiers
    .map((tier) => tier.min_multiplier)
    .filter((value) => Number.isFinite(value) && value > 0);
  const maxMultipliers = tiers
    .map((tier) => tier.max_multiplier)
    .filter((value) => Number.isFinite(value) && value > 0);

  return [
    price * (minMultipliers.length ? Math.min(...minMultipliers) : 0),
    price * (maxMultipliers.length ? Math.max(...maxMultipliers) : 0),
  ];
}
