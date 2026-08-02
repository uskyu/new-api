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

const QUICK_LOGIN_RETURN_KEY = 'quick_login_return_path';
const QUICK_LOGIN_RETURN_TTL = 10 * 60 * 1000;

function getQuickLoginPath(location) {
  if (!location || location.pathname !== '/quick-login') {
    return '';
  }
  const search = location.search || '';
  const code = new URLSearchParams(search).get('code');
  if (!code) {
    return '';
  }
  return `${location.pathname}${search}`;
}

export function rememberQuickLoginReturn(location) {
  const path = getQuickLoginPath(location);
  if (!path) {
    return;
  }
  sessionStorage.setItem(
    QUICK_LOGIN_RETURN_KEY,
    JSON.stringify({ path, createdAt: Date.now() }),
  );
}

export function consumeQuickLoginReturn() {
  const raw = sessionStorage.getItem(QUICK_LOGIN_RETURN_KEY);
  sessionStorage.removeItem(QUICK_LOGIN_RETURN_KEY);
  if (!raw) {
    return '';
  }
  try {
    const value = JSON.parse(raw);
    if (
      typeof value.path !== 'string' ||
      !value.path.startsWith('/quick-login?') ||
      typeof value.createdAt !== 'number' ||
      Date.now() - value.createdAt > QUICK_LOGIN_RETURN_TTL
    ) {
      return '';
    }
    return value.path;
  } catch {
    return '';
  }
}
