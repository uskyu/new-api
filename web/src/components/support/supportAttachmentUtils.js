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

import { getUserIdFromLocalStorage } from '../../helpers';

export const SUPPORT_IMAGE_MAX_COUNT = 4;
export const SUPPORT_IMAGE_MAX_BYTES = 5 * 1024 * 1024;
export const SUPPORT_IMAGE_ACCEPT =
  'image/png,image/jpeg,image/gif,image/webp,.png,.jpg,.jpeg,.gif,.webp';

const SUPPORTED_IMAGE_MIME_TYPES = new Set([
  'image/png',
  'image/jpeg',
  'image/gif',
  'image/webp',
]);

const SUPPORTED_IMAGE_EXTENSION = /\.(png|jpg|jpeg|gif|webp)$/i;

export function isSupportedSupportImage(file) {
  if (!file) {
    return false;
  }
  if (SUPPORTED_IMAGE_MIME_TYPES.has(String(file.type || '').toLowerCase())) {
    return true;
  }
  return SUPPORTED_IMAGE_EXTENSION.test(String(file.name || ''));
}

export function formatSupportImageSize(bytes) {
  const size = Number(bytes);
  if (!Number.isFinite(size) || size <= 0) {
    return '';
  }
  if (size < 1024) {
    return `${size}B`;
  }
  if (size < 1024 * 1024) {
    return `${Math.round(size / 1024)}KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(1)}MB`;
}

/**
 * Validate files selected from the picker or clipboard.
 * Returns error codes so the caller can translate them.
 */
export function validateSupportImageFiles(files, currentCount = 0) {
  const accepted = [];
  const errors = [];
  const list = Array.from(files || []);

  for (const file of list) {
    if (!isSupportedSupportImage(file) || file.size <= 0) {
      errors.push({ code: 'unsupported', file });
      continue;
    }
    if (file.size > SUPPORT_IMAGE_MAX_BYTES) {
      errors.push({ code: 'too_large', file });
      continue;
    }
    if (currentCount + accepted.length >= SUPPORT_IMAGE_MAX_COUNT) {
      errors.push({ code: 'too_many', file });
      break;
    }
    accepted.push(file);
  }

  return { accepted, errors };
}

/**
 * Read image files from a paste event. Only intercepts the event when the
 * clipboard actually contains images, so plain text paste keeps working.
 */
export function extractClipboardImageFiles(event) {
  const clipboardData = event?.clipboardData;
  if (!clipboardData) {
    return [];
  }

  const files = [];
  const items = Array.from(clipboardData.items || []);
  for (const item of items) {
    if (item.kind !== 'file' || !String(item.type || '').startsWith('image/')) {
      continue;
    }
    const file = item.getAsFile?.();
    if (file) {
      files.push(file);
    }
  }

  if (files.length > 0) {
    event.preventDefault();
  }
  return files;
}

export function createSupportComposerItem(file) {
  const fallbackName = `support-image-${Date.now()}.png`;
  return {
    key: `${file?.name || fallbackName}-${file?.size ?? 0}-${
      file?.lastModified ?? 0
    }-${Math.random().toString(36).slice(2, 8)}`,
    file,
    previewUrl: URL.createObjectURL(file),
  };
}

export function releaseSupportComposerItem(item) {
  if (item?.previewUrl) {
    URL.revokeObjectURL(item.previewUrl);
  }
}

export function buildSupportMessageFormData(content, files) {
  const formData = new FormData();
  formData.append('content', content || '');
  Array.from(files || []).forEach((file, index) => {
    const fallbackName = `support-image-${Date.now()}-${index + 1}.png`;
    formData.append('files', file, file.name || fallbackName);
  });
  return formData;
}

export function getSupportAttachmentEndpoint(attachmentId, variant = 'full') {
  return `/api/support/attachments/${attachmentId}?variant=${variant}`;
}

export async function fetchSupportAttachmentBlobUrl(attachmentId, variant = 'full') {
  const response = await fetch(getSupportAttachmentEndpoint(attachmentId, variant), {
    headers: {
      'New-Api-User': getUserIdFromLocalStorage(),
    },
  });
  if (!response.ok) {
    const error = new Error(`support attachment fetch failed: ${response.status}`);
    error.status = response.status;
    throw error;
  }
  const blob = await response.blob();
  return { url: URL.createObjectURL(blob), blob };
}
