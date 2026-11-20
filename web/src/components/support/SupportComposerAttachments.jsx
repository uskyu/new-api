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

import React, { useRef } from 'react';
import { Button } from '@douyinfe/semi-ui';
import { IconClose, IconImage } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import {
  SUPPORT_IMAGE_ACCEPT,
  SUPPORT_IMAGE_MAX_COUNT,
} from './supportAttachmentUtils';

export function SupportImagePicker({
  onSelectFiles,
  disabled = false,
  className = '',
}) {
  const { t } = useTranslation();
  const inputRef = useRef(null);

  return (
    <>
      <Button
        className={className}
        theme='borderless'
        type='tertiary'
        size='small'
        icon={<IconImage />}
        disabled={disabled}
        onClick={() => inputRef.current?.click()}
      >
        {t('图片')}
      </Button>
      <input
        ref={inputRef}
        type='file'
        accept={SUPPORT_IMAGE_ACCEPT}
        multiple
        hidden
        onChange={(event) => {
          const files = Array.from(event.target.files || []);
          event.target.value = '';
          if (files.length > 0) {
            onSelectFiles?.(files);
          }
        }}
      />
    </>
  );
}

export function SupportComposerAttachments({
  items = [],
  onRemove,
  disabled = false,
}) {
  const { t } = useTranslation();

  if (!items || items.length === 0) {
    return null;
  }

  return (
    <div className='support-composer-attachments'>
      {items.map((item) => (
        <div key={item.key} className='support-composer-attachment'>
          <img src={item.previewUrl} alt='' />
          <button
            type='button'
            className='support-composer-attachment__remove'
            onClick={() => onRemove?.(item.key)}
            disabled={disabled}
            aria-label={t('移除图片')}
          >
            <IconClose size='small' />
          </button>
        </div>
      ))}
      <span className='support-composer-attachments__count'>
        {items.length}/{SUPPORT_IMAGE_MAX_COUNT}
      </span>
    </div>
  );
}

export default SupportComposerAttachments;
