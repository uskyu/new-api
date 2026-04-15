import React, { useRef } from 'react';
import { Button, Modal, Typography } from '@douyinfe/semi-ui';
import { ImagePlus, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const readFileAsDataUrl = (file) =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result);
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(file);
  });

const AIConsoleInputRender = ({
  detailProps,
  draftImages,
  onAddImage,
  onRemoveImage,
}) => {
  const { t } = useTranslation();
  const fileInputRef = useRef(null);
  const { clearContextNode, inputNode, sendNode, onClick } = detailProps;

  const handleRequestClear = React.useCallback(
    (event) => {
      event?.stopPropagation?.();
      if (!clearContextNode?.props?.onClick) {
        return;
      }

      Modal.confirm({
        title: t('确认清空'),
        content: t('确定要清空当前对话内容吗？'),
        okText: t('确定'),
        cancelText: t('取消'),
        onOk: () => clearContextNode.props.onClick(event),
      });
    },
    [clearContextNode, t],
  );

  const styledActionNode = (node, extraClassName, extraStyle = {}) =>
    node
      ? React.cloneElement(node, {
          className: `!rounded-full flex-shrink-0 transition-all ${extraClassName} ${node.props.className || ''}`,
          onClick:
            node === clearContextNode ? handleRequestClear : node.props.onClick,
          style: {
            ...node.props.style,
            width: '38px',
            height: '38px',
            minWidth: '38px',
            padding: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            ...extraStyle,
          },
        })
      : null;

  const styledClearNode = styledActionNode(
    clearContextNode,
    '!bg-white/75 hover:!bg-red-500 hover:!text-white !backdrop-blur-md',
    {
      border: '1px solid rgba(255,255,255,0.6)',
      boxShadow: '0 12px 32px rgba(15, 23, 42, 0.08)',
    },
  );

  const styledSendNode = styledActionNode(
    sendNode,
    '!bg-slate-900 hover:!bg-black',
    {
      boxShadow: '0 18px 36px rgba(15, 23, 42, 0.18)',
    },
  );

  const handlePickImage = async (event) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) {
      return;
    }

    const dataUrl = await readFileAsDataUrl(file);
    onAddImage?.(dataUrl);
  };

  return (
    <div className='px-3 pb-3 pt-2 sm:px-5 sm:pb-5' onClick={onClick}>
      {draftImages.length > 0 && (
        <div className='mb-3 flex flex-wrap gap-2'>
          {draftImages.map((image, index) => (
            <div
              key={`${index}-${image.length}`}
              className='group relative h-20 w-20 overflow-hidden rounded-2xl border border-white/70 bg-white/80 shadow-[0_18px_40px_rgba(15,23,42,0.10)] backdrop-blur-md'
            >
              <img
                src={image}
                alt={`${t('已上传图片')} ${index + 1}`}
                className='h-full w-full object-cover'
              />
              <button
                type='button'
                onClick={(event) => {
                  event.stopPropagation();
                  onRemoveImage?.(index);
                }}
                className='absolute right-1.5 top-1.5 flex h-6 w-6 items-center justify-center rounded-full bg-black/70 text-white opacity-100 transition md:opacity-0 md:group-hover:opacity-100'
                aria-label={t('删除图片')}
              >
                <X size={12} />
              </button>
            </div>
          ))}
        </div>
      )}

      <div
        className='rounded-[28px] border border-white/70 bg-white/75 p-2 shadow-[0_24px_80px_rgba(15,23,42,0.08)] backdrop-blur-xl transition-shadow hover:shadow-[0_28px_90px_rgba(15,23,42,0.12)]'
        title={t('支持图片上传与粘贴')}
      >
        <div className='flex items-center gap-2'>
          {styledClearNode}
          <input
            ref={fileInputRef}
            type='file'
            accept='image/*'
            className='hidden'
            onChange={handlePickImage}
          />
          <Button
            theme='borderless'
            type='tertiary'
            icon={<ImagePlus size={16} />}
            className='!rounded-full !bg-white/75 hover:!bg-sky-50 hover:!text-sky-600'
            style={{
              width: 38,
              height: 38,
              minWidth: 38,
              padding: 0,
              border: '1px solid rgba(255,255,255,0.6)',
              boxShadow: '0 12px 32px rgba(15, 23, 42, 0.08)',
            }}
            onClick={(event) => {
              event.stopPropagation();
              fileInputRef.current?.click();
            }}
            aria-label={t('上传图片')}
          />
          <div className='min-w-0 flex-1 overflow-hidden rounded-[22px] bg-transparent px-1'>
            {inputNode}
          </div>
          {styledSendNode}
        </div>
      </div>

      <Typography.Text className='mt-2 block pl-2 text-xs text-slate-500'>
        {t('历史记录仅保存在当前浏览器')}
      </Typography.Text>
    </div>
  );
};

export default AIConsoleInputRender;
