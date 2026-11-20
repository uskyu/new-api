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

import React, { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { Spin } from '@douyinfe/semi-ui';
import { IconClose, IconImage } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import {
  fetchSupportAttachmentBlobUrl,
  formatSupportImageSize,
} from './supportAttachmentUtils';

function useAttachmentObjectUrl(attachmentId, variant, enabled = true) {
  const [state, setState] = useState({ status: 'idle', url: '' });

  useEffect(() => {
    if (!enabled || attachmentId === null || attachmentId === undefined) {
      setState({ status: 'idle', url: '' });
      return undefined;
    }

    let cancelled = false;
    let objectUrl = '';
    setState({ status: 'loading', url: '' });

    fetchSupportAttachmentBlobUrl(attachmentId, variant)
      .then(({ url }) => {
        if (cancelled) {
          URL.revokeObjectURL(url);
          return;
        }
        objectUrl = url;
        setState({ status: 'ready', url });
      })
      .catch((error) => {
        if (!cancelled) {
          setState({
            status: error?.status === 404 ? 'expired' : 'error',
            url: '',
          });
        }
      });

    return () => {
      cancelled = true;
      if (objectUrl) {
        URL.revokeObjectURL(objectUrl);
      }
    };
  }, [attachmentId, variant, enabled]);

  return state;
}

function SupportAttachmentPlaceholder({ expired }) {
  const { t } = useTranslation();
  return (
    <span className='support-attachment-thumb__placeholder'>
      <IconImage size='large' />
      <span>{expired ? t('图片已过期') : t('图片加载失败')}</span>
    </span>
  );
}

function SupportAttachmentThumb({ attachment, onOpen }) {
  const { t } = useTranslation();
  const expired = Boolean(attachment?.expired);
  const { status, url } = useAttachmentObjectUrl(
    attachment?.id,
    'thumb',
    !expired,
  );

  return (
    <button
      type='button'
      className={`support-attachment-thumb ${
        expired || status === 'expired' || status === 'error'
          ? 'is-placeholder'
          : ''
      }`}
      onClick={() => onOpen?.(attachment)}
      aria-label={t('查看图片')}
    >
      {expired || status === 'expired' || status === 'error' ? (
        <SupportAttachmentPlaceholder
          expired={expired || status === 'expired'}
        />
      ) : status === 'ready' && url ? (
        <img src={url} alt={attachment?.file_name || t('图片')} loading='lazy' />
      ) : (
        <span className='support-attachment-thumb__loading'>
          <Spin size='small' />
        </span>
      )}
    </button>
  );
}

function SupportAttachmentLightbox({ attachment, onClose }) {
  const { t } = useTranslation();
  const expired = Boolean(attachment?.expired);
  const { status, url } = useAttachmentObjectUrl(
    attachment?.id,
    'full',
    !expired,
  );

  useEffect(() => {
    const handleKeyDown = (event) => {
      if (event.key === 'Escape') {
        onClose?.();
      }
    };
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [onClose]);

  return createPortal(
    <div
      className='support-attachment-lightbox'
      role='dialog'
      aria-modal='true'
      aria-label={t('图片预览')}
      onClick={onClose}
    >
      <div
        className='support-attachment-lightbox__dialog'
        onClick={(event) => event.stopPropagation()}
      >
        <button
          type='button'
          className='support-attachment-lightbox__close'
          onClick={onClose}
          aria-label={t('关闭')}
        >
          <IconClose />
        </button>
        {expired || status === 'expired' || status === 'error' ? (
          <div className='support-attachment-lightbox__state'>
            <IconImage size='extra-large' />
            <span>
              {expired || status === 'expired'
                ? t('图片已过期')
                : t('图片加载失败')}
            </span>
          </div>
        ) : status === 'ready' && url ? (
          <img
            className='support-attachment-lightbox__image'
            src={url}
            alt={attachment?.file_name || t('图片')}
          />
        ) : (
          <div className='support-attachment-lightbox__state'>
            <Spin />
          </div>
        )}
        {(attachment?.file_name || attachment?.size_bytes) && (
          <div className='support-attachment-lightbox__meta'>
            {attachment?.file_name}
            {attachment?.size_bytes
              ? ` · ${formatSupportImageSize(attachment.size_bytes)}`
              : ''}
          </div>
        )}
      </div>
    </div>,
    document.body,
  );
}

export function SupportAttachmentGrid({ attachments = [], className = '' }) {
  const [previewAttachment, setPreviewAttachment] = useState(null);
  const validAttachments = Array.from(attachments || []).filter(
    (attachment) => attachment && attachment.id !== null && attachment.id !== undefined,
  );

  if (validAttachments.length === 0) {
    return null;
  }

  return (
    <>
      <div
        className={`support-attachment-grid ${
          validAttachments.length === 1 ? 'is-single' : ''
        } ${className}`.trim()}
      >
        {validAttachments.map((attachment) => (
          <SupportAttachmentThumb
            key={attachment.id}
            attachment={attachment}
            onOpen={setPreviewAttachment}
          />
        ))}
      </div>
      {previewAttachment && (
        <SupportAttachmentLightbox
          attachment={previewAttachment}
          onClose={() => setPreviewAttachment(null)}
        />
      )}
    </>
  );
}

export default SupportAttachmentGrid;
