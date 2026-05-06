import React, { useContext, useMemo, useRef, useState } from 'react';
import { Button, Empty, Input, Spin } from '@douyinfe/semi-ui';
import {
  Bot,
  Check,
  Clock3,
  MessageSquarePlus,
  PanelLeftOpen,
  Pencil,
  Sparkles,
  Trash2,
  X,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import useAiConsoleState from '../../hooks/ai-console/useAiConsoleState';
import { useApiRequest } from '../../hooks/playground/useApiRequest';
import { useMessageActions } from '../../hooks/playground/useMessageActions';
import {
  DEBUG_TABS,
  MESSAGE_ROLES,
} from '../../constants/playground.constants';
import {
  buildApiPayload,
  buildMessageContent,
  createLoadingAssistantMessage,
  createMessage,
  encodeToBase64,
  getUserIdFromLocalStorage,
  getLogo,
  showError,
  showSuccess,
  stringToColor,
} from '../../helpers';
import AIConsoleChatPanel from '../../components/ai-console/AIConsoleChatPanel';

const generateAvatarDataUrl = (username) => {
  if (!username) {
    return getLogo();
  }

  const firstLetter = username[0].toUpperCase();
  const bgColor = stringToColor(username);
  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32">
      <circle cx="16" cy="16" r="16" fill="${bgColor}" />
      <text x="50%" y="50%" dominant-baseline="central" text-anchor="middle" font-size="16" fill="#ffffff" font-family="sans-serif">${firstLetter}</text>
    </svg>
  `;
  return `data:image/svg+xml;base64,${encodeToBase64(svg)}`;
};

const SessionItem = ({
  session,
  active,
  editing,
  renameValue,
  onSelect,
  onStartRename,
  onRenameChange,
  onRenameSave,
  onRenameCancel,
  onDelete,
  t,
}) => (
  <div
    className={`group relative overflow-hidden rounded-[20px] border px-3.5 py-3 text-left transition-all ${
      active
        ? 'border-sky-200 bg-sky-50/85 shadow-[0_12px_30px_rgba(14,165,233,0.12)]'
        : 'border-white/50 bg-white/50 hover:border-slate-200 hover:bg-white/80'
    }`}
  >
    {active ? (
      <span className='absolute inset-y-3 left-0 w-1 rounded-r-full bg-sky-500' />
    ) : null}
    {editing ? (
      <div className='space-y-2'>
        <Input
          value={renameValue}
          onChange={onRenameChange}
          placeholder={t('输入会话名称')}
          onEnterPress={onRenameSave}
        />
        <div className='flex items-center justify-end gap-2'>
          <button
            type='button'
            onClick={onRenameCancel}
            className='flex h-7 w-7 items-center justify-center rounded-full bg-black/5 text-slate-500'
          >
            <X size={14} />
          </button>
          <button
            type='button'
            onClick={onRenameSave}
            className='flex h-7 w-7 items-center justify-center rounded-full bg-slate-900 text-white'
          >
            <Check size={14} />
          </button>
        </div>
      </div>
    ) : (
      <div className='flex items-start gap-2'>
        <button
          type='button'
          onClick={onSelect}
          className='min-w-0 flex-1 text-left'
        >
          <div className='truncate text-sm font-medium text-slate-900'>
            {session.title || t('新对话')}
          </div>
          <div className='mt-1 truncate text-xs text-slate-500'>
            {session.lastMessagePreview || t('还没有消息')}
          </div>
        </button>
        <button
          type='button'
          onClick={onStartRename}
          className='mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-black/5 text-slate-500 opacity-100 transition hover:bg-slate-900 hover:text-white md:opacity-0 md:group-hover:opacity-100'
          aria-label={t('编辑名称')}
        >
          <Pencil size={13} />
        </button>
        <button
          type='button'
          onClick={onDelete}
          className='mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-black/5 text-slate-500 opacity-100 transition hover:bg-rose-500 hover:text-white md:opacity-0 md:group-hover:opacity-100'
          aria-label={t('删除会话')}
        >
          <Trash2 size={13} />
        </button>
      </div>
    )}
  </div>
);

const NativeSelect = ({
  value,
  options,
  onChange,
  placeholder,
  className = '',
}) => (
  <select
    value={value}
    onChange={(event) => onChange(event.target.value)}
    className={`h-11 w-full rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400 ${className}`}
  >
    {options.length === 0 ? <option value=''>{placeholder}</option> : null}
    {options.map((option) => (
      <option key={option.value} value={option.value}>
        {option.label}
      </option>
    ))}
  </select>
);

const MAX_DRAFT_IMAGES = 5;
const REASONING_EFFORT_OPTIONS = [
  { value: '', label: '关闭' },
  { value: 'low', label: '低' },
  { value: 'medium', label: '中' },
  { value: 'high', label: '高' },
];

const AIConsole = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const styleState = { isMobile };
  const [userState] = useContext(UserContext);
  const [, setDebugData] = useState({
    request: null,
    response: null,
    timestamp: null,
  });
  const [, setActiveDebugTab] = useState(DEBUG_TABS.PREVIEW);
  const [showMobileSidebar, setShowMobileSidebar] = useState(false);
  const [editingSessionId, setEditingSessionId] = useState(null);
  const [editingSessionName, setEditingSessionName] = useState('');
  const chatRef = useRef(null);
  const sseSourceRef = useRef(null);

  const {
    ready,
    storageError,
    sessions,
    currentSessionId,
    messages,
    setMessages,
    groups,
    models,
    selectedModel,
    selectedGroup,
    setSelectedModel,
    setSelectedGroup,
    reasoningEffort,
    setReasoningEffort,
    draftImages,
    setDraftImages,
    draftFiles,
    setDraftFiles,
    createSession,
    switchSession,
    renameSession,
    removeSession,
    clearCurrentSession,
    markSessionActivity,
  } = useAiConsoleState();

  const saveMessagesImmediately = React.useCallback(
    async (messagesToSave) => {
      setMessages(messagesToSave);
    },
    [setMessages],
  );

  const { sendRequest, onStopGenerator } = useApiRequest(
    setMessages,
    setDebugData,
    setActiveDebugTab,
    sseSourceRef,
    saveMessagesImmediately,
  );

  const roleInfo = useMemo(
    () => ({
      user: {
        name: userState?.user?.username || 'User',
        avatar: generateAvatarDataUrl(userState?.user?.username),
      },
      assistant: {
        name: 'Assistant',
        avatar: getLogo(),
      },
      system: {
        name: 'System',
        avatar: getLogo(),
      },
    }),
    [userState?.user?.username],
  );

  const onMessageSend = React.useCallback(
    (content) => {
      const trimmed = typeof content === 'string' ? content.trim() : '';
      const fileContext = draftFiles
        .map((file, index) => {
          const markdown = String(file?.markdown || '').trim();
          if (!markdown) return '';
          return [
            `文件 ${index + 1}：《${file.filename || '未命名文件'}》`,
            '```markdown',
            markdown,
            '```',
          ].join('\n');
        })
        .filter(Boolean)
        .join('\n\n');
      if (!trimmed && draftImages.length === 0 && !fileContext) {
        return;
      }
      const textContent = fileContext
        ? [
            trimmed,
            '',
            '以下是用户上传文件的解析内容，请作为本轮对话上下文：',
            fileContext,
          ]
            .filter((item) => item !== '')
            .join('\n')
        : trimmed;

      const loadingMessage = createLoadingAssistantMessage();
      const userMessage = createMessage(
        MESSAGE_ROLES.USER,
        buildMessageContent(textContent, draftImages, draftImages.length > 0),
        draftFiles.length > 0
          ? {
              attachments: draftFiles.map((file) => ({
                filename: file.filename,
                size: file.size,
                warnings: file.warnings || [],
              })),
            }
          : {},
      );

      markSessionActivity();
      setMessages((previous) => {
        const requestMessages = [...previous, userMessage];
        const nextMessages = [...requestMessages, loadingMessage];
        const payload = buildApiPayload(
          requestMessages,
          null,
          {
            model: selectedModel,
            group: selectedGroup || '',
            stream: true,
          },
          {},
        );
        if (reasoningEffort) {
          payload.reasoning_effort = reasoningEffort;
        }

        sendRequest(payload, true);
        return nextMessages;
      });

      setDraftImages([]);
      setDraftFiles([]);
    },
    [
      draftImages,
      draftFiles,
      selectedGroup,
      selectedModel,
      reasoningEffort,
      markSessionActivity,
      sendRequest,
      setDraftImages,
      setDraftFiles,
      setMessages,
    ],
  );

  const onToggleReasoningExpansion = React.useCallback(
    (messageId) => {
      setMessages((previous) =>
        previous.map((message) =>
          message.id === messageId && message.role === MESSAGE_ROLES.ASSISTANT
            ? {
                ...message,
                isReasoningExpanded: !message.isReasoningExpanded,
              }
            : message,
        ),
      );
    },
    [setMessages],
  );

  const messageActions = useMessageActions(
    messages,
    setMessages,
    onMessageSend,
    saveMessagesImmediately,
  );

  const handleRemoveDraftImage = React.useCallback(
    (index) => {
      setDraftImages((previous) =>
        previous.filter((_, itemIndex) => itemIndex !== index),
      );
    },
    [setDraftImages],
  );

  const handleAddDraftImage = React.useCallback(
    (imageDataUrl) => {
      if (!imageDataUrl) return;
      setDraftImages((previous) => {
        if (previous.length >= MAX_DRAFT_IMAGES) {
          showError(
            t('最多支持 {{count}} 张图片', { count: MAX_DRAFT_IMAGES }),
          );
          return previous;
        }
        return [...previous, imageDataUrl];
      });
    },
    [setDraftImages, t],
  );

  const handleAddDraftFile = React.useCallback(
    async (file) => {
      if (!file) return;
      const formData = new FormData();
      formData.append('file', file);
      try {
        const response = await fetch('/api/ai-console/files/parse', {
          method: 'POST',
          headers: {
            'New-Api-User': getUserIdFromLocalStorage(),
          },
          body: formData,
        });
        const result = await response.json();
        if (!result?.success) {
          throw new Error(result?.message || 'failed to parse file');
        }
        const data = result.data || {};
        setDraftFiles((previous) => [
          ...previous,
          {
            id: `file-${Date.now()}-${Math.random().toString(16).slice(2)}`,
            filename: data.filename || file.name,
            markdown: data.markdown || '',
            warnings: Array.isArray(data.warnings) ? data.warnings : [],
            size: file.size,
            type: file.type,
          },
        ]);
        showSuccess(t('文件已解析'));
      } catch (error) {
        showError(error.message || t('文件解析失败'));
      }
    },
    [setDraftFiles, t],
  );

  const handleRemoveDraftFile = React.useCallback(
    (index) => {
      setDraftFiles((previous) =>
        previous.filter((_, itemIndex) => itemIndex !== index),
      );
    },
    [setDraftFiles],
  );

  const handleClearMessages = React.useCallback(() => {
    clearCurrentSession();
  }, [clearCurrentSession]);

  const startRenameSession = React.useCallback((session) => {
    setEditingSessionId(session.id);
    setEditingSessionName(session.title || '');
  }, []);

  const saveRenameSession = React.useCallback(async () => {
    if (!editingSessionId) {
      return;
    }
    await renameSession(editingSessionId, editingSessionName);
    setEditingSessionId(null);
    setEditingSessionName('');
  }, [editingSessionId, editingSessionName, renameSession]);

  const cancelRenameSession = React.useCallback(() => {
    setEditingSessionId(null);
    setEditingSessionName('');
  }, []);

  const groupOptions = groups.map((group) => ({
    value: group.value,
    label: group.fullLabel || group.label,
  }));

  const modelOptions = models.map((model) => ({
    value: model.value,
    label: model.label,
  }));

  const reasoningOptions = REASONING_EFFORT_OPTIONS.map((option) => ({
    value: option.value,
    label: t(option.label),
  }));

  const featurePills = useMemo(
    () => [
      {
        key: 'chat',
        label: t('聊天'),
        active: true,
        icon: <MessageSquarePlus size={15} />,
      },
      {
        key: 'agent',
        label: t('综合 Agent'),
        icon: <Sparkles size={15} />,
      },
      {
        key: 'draw',
        label: t('绘图'),
        badge: t('待开发'),
        icon: <Sparkles size={15} />,
      },
      {
        key: 'data',
        label: t('数据'),
        badge: t('待开发'),
        icon: <Sparkles size={15} />,
      },
    ],
    [t],
  );

  const promptSuggestions = useMemo(
    () => [
      t('帮我把这个想法拆成可执行计划'),
      t('阅读下面内容并提炼关键结论'),
      t('帮我写一份简洁的产品介绍'),
    ],
    [t],
  );

  if (!ready) {
    return (
      <div className='mt-[64px] flex h-[calc(100vh-64px)] items-center justify-center bg-[#eef2f7]'>
        <Spin spinning size='large' />
      </div>
    );
  }

  if (storageError) {
    return (
      <div className='mt-[64px] flex h-[calc(100vh-64px)] items-center justify-center bg-[#eef2f7] px-6'>
        <Empty
          image={<Sparkles size={40} className='text-slate-900' />}
          title={t('AI 控制台初始化失败')}
          description={t('当前浏览器可能禁用了本地存储，请稍后重试')}
        />
      </div>
    );
  }

  const renderChatConfigControls = (compact = false) => (
    <div
      className={`grid gap-2 rounded-[24px] border border-cyan-100/80 bg-white/80 p-2 shadow-[0_18px_46px_rgba(8,47,73,0.10)] backdrop-blur-xl ${
        compact
          ? 'grid-cols-1'
          : 'w-full grid-cols-1 sm:grid-cols-3 xl:w-[780px]'
      }`}
    >
      <label className='min-w-0'>
        <span className='mb-1 block px-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-400'>
          {t('分组')}
        </span>
        <NativeSelect
          value={selectedGroup}
          options={groupOptions}
          onChange={setSelectedGroup}
          placeholder={t('选择分组')}
          className='!h-10 !rounded-full !bg-white/90 !text-xs'
        />
      </label>

      <label className='min-w-0'>
        <span className='mb-1 block px-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-400'>
          {t('模型')}
        </span>
        <NativeSelect
          value={selectedModel}
          options={modelOptions}
          onChange={setSelectedModel}
          placeholder={t('选择模型')}
          className='!h-10 !rounded-full !bg-white/90 !text-xs !font-bold'
        />
      </label>

      <label className='min-w-0'>
        <span className='mb-1 block px-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-400'>
          {t('思考深度')}
        </span>
        <NativeSelect
          value={reasoningEffort}
          options={reasoningOptions}
          onChange={setReasoningEffort}
          placeholder={t('选择思考深度')}
          className='!h-10 !rounded-full !bg-white/90 !text-xs'
        />
      </label>
    </div>
  );

  const sidebarContent = (
    <div className='solo-side-panel flex h-full flex-col overflow-hidden rounded-[34px] border border-cyan-100/70 bg-white/75 shadow-[0_24px_70px_rgba(8,47,73,0.12)] backdrop-blur-2xl'>
      <div className='border-b border-cyan-100/70 p-4'>
        <div className='mb-4 flex items-center justify-between gap-3'>
          <img src={getLogo()} alt='AI' className='h-8 w-auto object-contain' />
          <span className='rounded-full border border-teal-100 bg-teal-50 px-3 py-1 text-xs font-bold text-teal-700'>
            Command
          </span>
        </div>
        <div className='mb-3 grid grid-cols-2 gap-2'>
          <div className='rounded-[18px] border border-white/70 bg-white/60 px-3 py-2 shadow-sm'>
            <div className='flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-[0.12em] text-slate-400'>
              <Clock3 size={12} />
              {t('会话')}
            </div>
            <div className='mt-1 text-lg font-black text-slate-900'>
              {sessions.length}
            </div>
          </div>
          <div className='rounded-[18px] border border-white/70 bg-white/60 px-3 py-2 shadow-sm'>
            <div className='flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-[0.12em] text-slate-400'>
              <Bot size={12} />
              {t('模型')}
            </div>
            <div className='mt-1 truncate text-sm font-black text-slate-900'>
              {selectedModel || t('待选择')}
            </div>
          </div>
        </div>
        <Button
          theme='solid'
          type='primary'
          icon={<MessageSquarePlus size={16} />}
          className='!h-12 !w-full !rounded-[20px] !bg-[#0f172a] !text-white shadow-[0_18px_34px_rgba(15,23,42,0.20)]'
          onClick={() => createSession()}
        >
          {t('新对话')}
        </Button>
      </div>

      <div className='min-h-0 flex-1 overflow-y-auto px-3 py-4'>
        <div className='mb-3 flex items-end justify-between gap-3 px-2'>
          <div>
            <div className='text-xs font-bold uppercase tracking-[0.22em] text-teal-600'>
              MEMORY
            </div>
            <div className='mt-1 text-sm font-bold text-slate-900'>
              {t('最近会话')}
            </div>
          </div>
          <span className='rounded-full border border-white/70 bg-white/70 px-2.5 py-1 text-xs font-bold text-slate-500 shadow-sm'>
            live
          </span>
        </div>
        <div className='space-y-2'>
          {sessions.map((session) => (
            <SessionItem
              key={session.id}
              session={session}
              active={session.id === currentSessionId}
              editing={editingSessionId === session.id}
              renameValue={editingSessionName}
              onSelect={() => {
                switchSession(session.id);
                setShowMobileSidebar(false);
              }}
              onStartRename={() => startRenameSession(session)}
              onRenameChange={(value) => setEditingSessionName(value)}
              onRenameSave={saveRenameSession}
              onRenameCancel={cancelRenameSession}
              onDelete={() => removeSession(session.id)}
              t={t}
            />
          ))}
        </div>
      </div>

      <div className='border-t border-white/60 px-4 py-4 lg:hidden'>
        {renderChatConfigControls(true)}
      </div>
    </div>
  );

  return (
    <div className='ai-console-page solo-ai-surface mt-[64px] h-[calc(100vh-64px)] overflow-hidden'>
      {isMobile && showMobileSidebar && (
        <button
          type='button'
          className='absolute inset-0 z-10 mt-[64px] bg-black/10 backdrop-blur-[1px]'
          onClick={() => setShowMobileSidebar(false)}
          aria-label={t('关闭侧栏')}
        />
      )}

      <div className='relative flex h-full gap-4 p-3 sm:p-4 lg:p-5'>
        <aside className='hidden h-full w-[340px] shrink-0 lg:block'>
          {sidebarContent}
        </aside>

        <main className='flex min-w-0 flex-1 flex-col gap-3'>
          <div className='lg:hidden'>
            <div className='pointer-events-none absolute left-3 top-3 z-20 sm:left-4 sm:top-4'>
              <div className='pointer-events-auto flex items-center gap-2 rounded-full border border-white/70 bg-white/80 p-2 shadow-[0_16px_36px_rgba(15,23,42,0.10)] backdrop-blur-xl'>
                <Button
                  theme='borderless'
                  type='tertiary'
                  icon={<PanelLeftOpen size={18} />}
                  className='!rounded-full !bg-black/5'
                  onClick={() => setShowMobileSidebar((previous) => !previous)}
                />
              </div>
            </div>

            {showMobileSidebar && (
              <div className='absolute inset-y-3 left-3 z-20 w-[min(82vw,320px)] sm:left-4 sm:inset-y-4'>
                {sidebarContent}
              </div>
            )}
          </div>

          <div className='min-h-0 flex-1'>
            <AIConsoleChatPanel
              chatRef={chatRef}
              messages={messages}
              roleInfo={roleInfo}
              styleState={styleState}
              hideHeader={isMobile}
              draftImages={draftImages}
              draftFiles={draftFiles}
              onAddImage={handleAddDraftImage}
              onRemoveImage={handleRemoveDraftImage}
              onAddFile={handleAddDraftFile}
              onRemoveFile={handleRemoveDraftFile}
              onMessageSend={onMessageSend}
              onMessageCopy={messageActions.handleMessageCopy}
              onMessageReset={messageActions.handleMessageReset}
              onMessageDelete={messageActions.handleMessageDelete}
              onToggleReasoningExpansion={onToggleReasoningExpansion}
              onStopGenerator={onStopGenerator}
              onClearMessages={handleClearMessages}
              headerTitle={t('聊天工作台')}
              headerDescription={t('对话、写作、代码、总结和推理都从这里开始')}
              emptyTitle={t('把任务交给 AI 工作台')}
              emptyDescription={t(
                '直接开始对话、写作、代码、总结和推理，后续可以继续扩展为可执行任务的 Agent 工作流。',
              )}
              featurePills={featurePills}
              promptSuggestions={promptSuggestions}
              modelBrands={['DeepSeek', 'OpenAI', 'Kimi', 'Gemini']}
              headerAccessory={renderChatConfigControls(false)}
            />
          </div>
        </main>
      </div>
    </div>
  );
};

export default AIConsole;
