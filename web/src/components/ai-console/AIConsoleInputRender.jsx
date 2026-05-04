import React, { useRef, useState } from 'react';
import { Button, Modal, Typography } from '@douyinfe/semi-ui';
import { FileText, ImagePlus, Loader2, X } from 'lucide-react';
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
  draftFiles = [],
  onAddImage,
  onRemoveImage,
  onAddFile,
  onRemoveFile,
  inputControlsNode,
}) => {
  const { t } = useTranslation();
  const containerRef = useRef(null);
  const fileInputRef = useRef(null);
  const documentInputRef = useRef(null);
  const [isParsingFile, setIsParsingFile] = useState(false);
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

  const handleAddFiles = React.useCallback(
    async (files) => {
      const nextFiles = Array.from(files || []).filter(Boolean);
      if (nextFiles.length === 0) {
        return;
      }
      setIsParsingFile(true);
      try {
        for (const file of nextFiles) {
          await onAddFile?.(file);
        }
      } finally {
        setIsParsingFile(false);
      }
    },
    [onAddFile],
  );

  const handlePickDocument = async (event) => {
    const files = Array.from(event.target.files || []);
    event.target.value = '';
    await handleAddFiles(files);
  };

  const handlePaste = React.useCallback(
    async (event) => {
      const clipboardData = event.clipboardData;
      if (!clipboardData) {
        return;
      }

      const filesFromItems = Array.from(clipboardData.items || [])
        .filter((item) => item.kind === 'file')
        .map((item) => item.getAsFile())
        .filter(Boolean);
      const filesFromList = Array.from(clipboardData.files || []).filter(
        Boolean,
      );
      const files = [...filesFromItems, ...filesFromList].filter(
        (file, index, list) =>
          list.findIndex(
            (item) =>
              item.name === file.name &&
              item.size === file.size &&
              item.type === file.type,
          ) === index,
      );

      if (files.length === 0) {
        return;
      }

      event.preventDefault();

      const imageFiles = files.filter((file) =>
        file.type?.startsWith('image/'),
      );
      const documentFiles = files.filter(
        (file) => !file.type?.startsWith('image/'),
      );
      for (const imageFile of imageFiles) {
        const dataUrl = await readFileAsDataUrl(imageFile);
        onAddImage?.(dataUrl);
      }
      await handleAddFiles(documentFiles);
    },
    [handleAddFiles, onAddImage],
  );

  React.useEffect(() => {
    const container = containerRef.current;
    if (!container) return undefined;
    container.addEventListener('paste', handlePaste, true);
    return () => {
      container.removeEventListener('paste', handlePaste, true);
    };
  }, [handlePaste]);

  return (
    <div
      ref={containerRef}
      className='px-3 pb-3 pt-2 sm:px-5 sm:pb-5'
      onClick={onClick}
    >
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

      {draftFiles.length > 0 && (
        <div className='mb-3 flex flex-wrap gap-2'>
          {draftFiles.map((file, index) => (
            <div
              key={file.id || `${index}-${file.filename}`}
              className='group flex max-w-full items-center gap-2 rounded-2xl border border-white/70 bg-white/80 px-3 py-2 shadow-[0_14px_34px_rgba(15,23,42,0.08)] backdrop-blur-md'
            >
              <div className='flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-sky-50 text-sky-600'>
                <FileText size={16} />
              </div>
              <div className='min-w-0'>
                <div className='max-w-[220px] truncate text-sm font-medium text-slate-800'>
                  {file.filename || t('未命名文件')}
                </div>
                <div className='text-xs text-slate-400'>
                  {file.warnings?.length > 0
                    ? file.warnings.join('；')
                    : t('已解析为 Markdown')}
                </div>
              </div>
              <button
                type='button'
                onClick={(event) => {
                  event.stopPropagation();
                  onRemoveFile?.(index);
                }}
                className='flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-black/5 text-slate-500 transition hover:bg-rose-500 hover:text-white'
                aria-label={t('删除文件')}
              >
                <X size={12} />
              </button>
            </div>
          ))}
        </div>
      )}

      <div className='flex items-end gap-2'>
        {inputControlsNode ? (
          <div className='mb-1 hidden shrink-0 sm:flex'>
            {inputControlsNode}
          </div>
        ) : null}
        <div
          className='min-w-0 flex-1 rounded-[28px] border border-white/70 bg-white/75 p-2 shadow-[0_24px_80px_rgba(15,23,42,0.08)] backdrop-blur-xl transition-shadow hover:shadow-[0_28px_90px_rgba(15,23,42,0.12)]'
          title={t('支持图片上传与粘贴')}
        >
          <div className='flex min-h-[46px] items-center gap-2'>
            {styledClearNode}
            <input
              ref={fileInputRef}
              type='file'
              accept='image/*'
              className='hidden'
              onChange={handlePickImage}
            />
            <input
              ref={documentInputRef}
              type='file'
              accept='.txt,.md,.csv,.json,.log,.pdf,.docx,.xlsx,.pptx'
              multiple
              className='hidden'
              onChange={handlePickDocument}
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
            <Button
              theme='borderless'
              type='tertiary'
              icon={
                isParsingFile ? (
                  <Loader2 size={16} className='animate-spin' />
                ) : (
                  <FileText size={16} />
                )
              }
              loading={isParsingFile}
              className='!rounded-full !bg-white/75 hover:!bg-emerald-50 hover:!text-emerald-600'
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
                documentInputRef.current?.click();
              }}
              aria-label={t('上传文件')}
            />
            <div className='ai-console-input-node min-w-0 flex-1 overflow-hidden rounded-[22px] bg-transparent px-1'>
              {inputNode}
            </div>
            {styledSendNode}
          </div>
        </div>
      </div>

      <Typography.Text className='mt-2 block pl-2 text-xs text-slate-500'>
        {t('支持 Ctrl+V 粘贴图片，办公文件会解析为文本后发送')}
      </Typography.Text>
    </div>
  );
};

export default AIConsoleInputRender;
