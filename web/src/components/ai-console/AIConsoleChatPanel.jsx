import React, { useMemo } from 'react';
import { Chat, Empty, Typography } from '@douyinfe/semi-ui';
import { MessageSquare, Sparkles } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import AIConsoleInputRender from './AIConsoleInputRender';
import { OptimizedMessageContent } from '../playground/OptimizedComponents';

const ActionButton = ({ label, tone = 'default', onClick }) => {
  const toneClass =
    tone === 'danger'
      ? 'hover:bg-rose-500'
      : tone === 'primary'
        ? 'hover:bg-sky-600'
        : 'hover:bg-slate-900';

  return (
    <button
      type='button'
      onClick={onClick}
      className={`rounded-full bg-white/90 px-3 py-1.5 text-[11px] font-medium text-slate-600 shadow-sm transition hover:text-white ${toneClass}`}
    >
      {label}
    </button>
  );
};

const AIConsoleChatPanel = ({
  chatRef,
  messages,
  roleInfo,
  styleState,
  hideHeader = false,
  draftImages,
  onAddImage,
  onRemoveImage,
  onMessageSend,
  onMessageCopy,
  onMessageReset,
  onMessageDelete,
  onToggleReasoningExpansion,
  onStopGenerator,
  onClearMessages,
  headerTitle,
  headerDescription,
  placeholder,
  emptyTitle,
  emptyDescription,
  headerIcon,
  emptyIcon,
}) => {
  const { t } = useTranslation();

  const resolvedHeaderIcon = headerIcon || (
    <MessageSquare size={20} className='text-sky-600' />
  );
  const resolvedEmptyIcon = emptyIcon || (
    <Sparkles size={28} className='text-sky-500' />
  );

  const renderInputArea = React.useCallback(
    (props) => (
      <AIConsoleInputRender
        {...props}
        draftImages={draftImages}
        onAddImage={onAddImage}
        onRemoveImage={onRemoveImage}
      />
    ),
    [draftImages, onAddImage, onRemoveImage],
  );

  const renderCustomChatContent = React.useCallback(
    ({ message, className }) => (
      <OptimizedMessageContent
        message={message}
        className={className}
        styleState={styleState}
        onToggleReasoningExpansion={onToggleReasoningExpansion}
      />
    ),
    [onToggleReasoningExpansion, styleState],
  );

  const renderChatBoxAction = React.useCallback(
    ({ message }) => {
      if (message.status === 'loading' || message.status === 'incomplete') {
        return null;
      }

      return (
        <div className='mt-3 flex flex-wrap gap-2'>
          <ActionButton
            label={t('复制')}
            onClick={() => onMessageCopy(message)}
          />
          <ActionButton
            label={t('重试')}
            tone='primary'
            onClick={() => onMessageReset(message)}
          />
          <ActionButton
            label={t('删除')}
            tone='danger'
            onClick={() => onMessageDelete(message)}
          />
        </div>
      );
    },
    [onMessageCopy, onMessageDelete, onMessageReset, t],
  );

  const emptyContent = useMemo(
    () => (
      <div className='flex h-full items-center justify-center px-5'>
        <Empty
          image={
            <div className='flex h-16 w-16 items-center justify-center rounded-[22px] bg-white/70 shadow-[0_20px_40px_rgba(15,23,42,0.08)] backdrop-blur-xl'>
              {resolvedEmptyIcon}
            </div>
          }
          title={emptyTitle || t('今天想让我帮你做什么？')}
          description={
            emptyDescription || t('直接输入问题，或者上传一张图片开始')
          }
        />
      </div>
    ),
    [emptyDescription, emptyTitle, resolvedEmptyIcon, t],
  );

  return (
    <div className='relative flex h-full min-h-0 flex-1 flex-col overflow-hidden rounded-[30px] border border-white/70 bg-white/55 shadow-[0_32px_100px_rgba(15,23,42,0.10)] backdrop-blur-2xl'>
      <div className='absolute inset-x-0 top-0 h-24 bg-gradient-to-r from-sky-100/60 via-white/20 to-emerald-100/40 blur-2xl' />

      {!hideHeader && (
        <div className='relative border-b border-white/60 px-5 py-4 sm:px-6'>
          <div className='flex items-center gap-3'>
            <div className='flex h-11 w-11 items-center justify-center rounded-[18px] bg-white/85 shadow-[0_16px_36px_rgba(15,23,42,0.08)] backdrop-blur-xl'>
              {resolvedHeaderIcon}
            </div>
            <div className='min-w-0'>
              <Typography.Title heading={5} className='!mb-0 !text-slate-900'>
                {headerTitle || t('AI 控制台')}
              </Typography.Title>
              <Typography.Text className='!text-sm !text-slate-500'>
                {headerDescription || t('输入问题，或上传图片继续')}
              </Typography.Text>
            </div>
          </div>
        </div>
      )}

      <div className='relative min-h-0 flex-1'>
        <Chat
          ref={chatRef}
          chatBoxRenderConfig={{
            renderChatBoxContent: renderCustomChatContent,
            renderChatBoxAction: renderChatBoxAction,
            renderChatBoxTitle: () => null,
          }}
          renderInputArea={renderInputArea}
          roleConfig={roleInfo}
          style={{
            height: '100%',
            maxWidth: '100%',
            overflow: 'hidden',
          }}
          chats={messages}
          onMessageSend={onMessageSend}
          showClearContext
          showStopGenerate
          onStopGenerator={onStopGenerator}
          onClear={onClearMessages}
          className='h-full'
          placeholder={placeholder || t('输入问题，或上传图片继续')}
          emptyContent={emptyContent}
        />
      </div>
    </div>
  );
};

export default AIConsoleChatPanel;
