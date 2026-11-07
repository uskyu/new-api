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

/**
 * 盲盒售价是真实支付的人民币金额，与管理端保持一致，始终显示 ¥。
 */
export function formatPrice(value) {
  const amount = Number(value);
  if (!Number.isFinite(amount)) {
    return '¥0.00';
  }
  return `¥${amount.toFixed(2)}`;
}

/**
 * 抽中倍率最多保留 4 位小数，并去掉末尾多余的 0。
 */
export function formatMultiplier(value) {
  const multiplier = Number(value);
  if (!Number.isFinite(multiplier) || multiplier <= 0) {
    return '-';
  }
  return String(parseFloat(multiplier.toFixed(4)));
}
