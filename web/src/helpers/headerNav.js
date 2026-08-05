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

export const MAX_CUSTOM_TOP_NAV_LINKS = 8;
export const MAX_CUSTOM_TOP_NAV_TITLE_LENGTH = 24;
export const MAX_CUSTOM_TOP_NAV_URL_LENGTH = 2048;

export const createDefaultHeaderNavModules = () => ({
  home: true,
  console: true,
  pricing: {
    enabled: true,
    requireAuth: false,
  },
  docs: true,
  about: true,
  customLinks: [],
});

export const isSafeTopNavHref = (value) => {
  const href = String(value || '').trim();
  if (href.startsWith('/') && !href.startsWith('//')) return true;
  try {
    const parsed = new URL(href);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:';
  } catch {
    return false;
  }
};

export const isExternalTopNavHref = (value) => {
  try {
    const parsed = new URL(String(value || '').trim());
    return parsed.protocol === 'http:' || parsed.protocol === 'https:';
  } catch {
    return false;
  }
};

export const parseHeaderNavCustomLinks = (raw) => {
  if (!Array.isArray(raw)) return [];
  return raw
    .slice(0, MAX_CUSTOM_TOP_NAV_LINKS)
    .map((item, index) => {
      if (!item || typeof item !== 'object') return null;
      const title = typeof item.title === 'string' ? item.title.trim() : '';
      const url = typeof item.url === 'string' ? item.url.trim() : '';
      if (
        !title ||
        title.length > MAX_CUSTOM_TOP_NAV_TITLE_LENGTH ||
        !url ||
        url.length > MAX_CUSTOM_TOP_NAV_URL_LENGTH ||
        !isSafeTopNavHref(url)
      ) {
        return null;
      }
      return {
        id: String(item.id || `custom-link-${index + 1}`).slice(0, 64),
        title,
        url,
        enabled: item.enabled !== false,
      };
    })
    .filter(Boolean);
};

export const parseHeaderNavModules = (raw) => {
  const defaults = createDefaultHeaderNavModules();
  let parsed = raw;
  if (typeof raw === 'string') {
    try {
      parsed = JSON.parse(raw);
    } catch {
      return defaults;
    }
  }
  if (!parsed || typeof parsed !== 'object') return defaults;

  const pricing =
    typeof parsed.pricing === 'boolean'
      ? { enabled: parsed.pricing, requireAuth: false }
      : {
          enabled: parsed.pricing?.enabled !== false,
          requireAuth: parsed.pricing?.requireAuth === true,
        };
  return {
    home: parsed.home !== false,
    console: parsed.console !== false,
    pricing,
    docs: parsed.docs !== false,
    about: parsed.about !== false,
    customLinks: parseHeaderNavCustomLinks(parsed.customLinks),
  };
};
