import React, { useMemo } from 'react';
import { Chat, Empty, Typography } from '@douyinfe/semi-ui';
import { MessageSquare, Sparkles } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import AIConsoleInputRender from './AIConsoleInputRender';
import { OptimizedMessageContent } from '../playground/OptimizedComponents';
import { getLogo } from '../../helpers';

const ActionButton = ({ label, tone = 'default', onClick }) => {
  const toneClass =
    tone === 'danger'
      ? 'hover:bg-rose-50 hover:text-rose-600'
      : tone === 'primary'
        ? 'hover:bg-blue-50 hover:text-blue-600'
        : 'hover:bg-slate-100';

  return (
    <button
      type='button'
      onClick={onClick}
      className={`rounded-md border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-600 transition ${toneClass}`}
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
  draftFiles,
  onAddImage,
  onRemoveImage,
  onAddFile,
  onRemoveFile,
  onMessageSend,
  onMessageCopy,
  onMessageReset,
  onMessageDelete,
  onToggleReasoningExpansion,
  onStopGenerator,
  onClearMessages,
  inputControlsNode,
  headerTitle,
  headerDescription,
  placeholder,
  emptyTitle,
  emptyDescription,
  headerIcon,
  emptyIcon,
  headerAccessory,
  featurePills = [],
  promptSuggestions = [],
  modelBrands = [],
}) => {
  const { t } = useTranslation();

  const resolvedHeaderIcon = headerIcon || (
    <MessageSquare size={20} className='text-white' />
  );
  const resolvedEmptyIcon = emptyIcon || (
    <Sparkles size={28} className='text-sky-500' />
  );

  const renderInputArea = React.useCallback(
    (props) => (
      <AIConsoleInputRender
        {...props}
        draftImages={draftImages}
        draftFiles={draftFiles}
        onAddImage={onAddImage}
        onRemoveImage={onRemoveImage}
        onAddFile={onAddFile}
        onRemoveFile={onRemoveFile}
        inputControlsNode={inputControlsNode}
      />
    ),
    [
      draftFiles,
      draftImages,
      inputControlsNode,
      onAddFile,
      onAddImage,
      onRemoveFile,
      onRemoveImage,
    ],
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
      <div className='relative flex h-full items-center justify-center px-5'>
        <Empty
          className='solo-empty-hero'
          image={
            <div className='flex h-16 min-w-40 items-center justify-center rounded-lg border border-slate-200 bg-white px-5'>
              <img src={getLogo()} alt='AI' className='h-10 w-auto' />
            </div>
          }
          title={emptyTitle || t('今天想让我帮你做什么？')}
          description={
            emptyDescription || t('直接输入问题，或者上传一张图片开始')
          }
        />
        {featurePills.length > 0 || promptSuggestions.length > 0 ? (
          <div className='pointer-events-auto absolute bottom-[18%] left-1/2 hidden w-full max-w-[760px] -translate-x-1/2 px-5 lg:block'>
            <div className='mb-5 flex flex-wrap justify-center gap-2'>
              {featurePills.map((item) => (
                <span
                  key={item.key || item.label}
                  className={`inline-flex h-9 items-center gap-2 rounded-md border px-3 text-sm font-medium ${
                    item.active
                      ? 'border-blue-200 bg-blue-50 text-blue-700'
                      : 'border-slate-200 bg-white text-slate-600'
                  }`}
                >
                  {item.icon}
                  <span>{item.label}</span>
                  {item.badge ? (
                    <span className='rounded bg-slate-100 px-1.5 py-0.5 text-[11px] text-slate-400'>
                      {item.badge}
                    </span>
                  ) : null}
                </span>
              ))}
            </div>
            <div className='grid gap-3 sm:grid-cols-3'>
              {promptSuggestions.map((item) => (
                <div
                  key={item}
                  className='rounded-lg border border-slate-200 bg-white px-4 py-3 text-left text-sm font-medium leading-6 text-slate-700'
                >
                  {item}
                </div>
              ))}
            </div>
          </div>
        ) : null}
        {modelBrands.length > 0 ? (
          <div className='pointer-events-none absolute bottom-[9%] left-1/2 hidden -translate-x-1/2 flex-wrap justify-center gap-2 opacity-75 lg:flex'>
            {modelBrands.map((brand) => (
              <span
                key={brand}
                className='rounded-md border border-slate-200 bg-white px-3 py-1 text-xs font-medium text-slate-500'
              >
                {brand}
              </span>
            ))}
          </div>
        ) : null}
      </div>
    ),
    [
      emptyDescription,
      emptyTitle,
      featurePills,
      modelBrands,
      promptSuggestions,
      resolvedEmptyIcon,
      t,
    ],
  );

  return (
    <div className='solo-workbench-panel relative flex h-full min-h-0 flex-1 flex-col overflow-hidden rounded-lg border border-slate-200 bg-white'>
      <div className='solo-panel-overlay inset-x-0 top-0 h-28 bg-[linear-gradient(90deg,rgba(45,212,191,0.20),rgba(255,255,255,0.45),rgba(56,189,248,0.22))] blur-2xl' />

      {!hideHeader && (
        <div className='relative border-b border-slate-200 px-5 py-4 sm:px-6'>
          <div className='flex flex-wrap items-center justify-between gap-4'>
            <div className='flex min-w-0 items-center gap-3'>
              <div className='flex h-10 w-10 items-center justify-center rounded-lg bg-blue-50 text-blue-600'>
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
            {headerAccessory ? (
              <div className='min-w-0 flex-1 lg:flex-none'>
                {headerAccessory}
              </div>
            ) : null}
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
