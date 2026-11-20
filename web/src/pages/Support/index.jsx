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

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { useTranslation } from 'react-i18next';
import {
  Badge,
  Button,
  Card,
  Empty,
  Pagination,
  Spin,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { ArrowLeft, Send, UserRound, X } from 'lucide-react';
import { API, getUserIdFromLocalStorage, showError } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const PAGE_SIZE = 20;
const MESSAGE_PAGE_SIZE = 100;
const INCREMENTAL_MESSAGE_LIMIT = 50;
const LIST_POLL_INTERVAL = 3000;
const MESSAGE_POLL_INTERVAL = 2000;

const resolveErrorMessage = (error, fallback) => {
  const responseMessage = error?.response?.data?.message;
  if (responseMessage) {
    return responseMessage;
  }
  if (error?.name && error.name !== 'AxiosError' && error?.message) {
    return error.message;
  }
  return fallback;
};

const normalizeId = (id) => {
  const numeric = Number(id);
  return Number.isFinite(numeric) ? numeric : id;
};

const isConversationClosed = (conversation) =>
  String(conversation?.status || '').toLowerCase() === 'closed';

const getConversationName = (conversation) => {
  if (!conversation) {
    return '';
  }
  return (
    conversation.display_name ||
    conversation.username ||
    (conversation.user_id != null ? `#${conversation.user_id}` : '')
  );
};

const toDate = (value) => {
  if (value === null || value === undefined || value === '') {
    return null;
  }
  let date;
  if (typeof value === 'number' || /^\d+$/.test(String(value))) {
    const numeric = Number(value);
    if (!Number.isFinite(numeric)) {
      return null;
    }
    date = new Date(numeric < 1000000000000 ? numeric * 1000 : numeric);
  } else {
    date = new Date(value);
  }
  return Number.isNaN(date.getTime()) ? null : date;
};

const formatTime = (value) => {
  const date = toDate(value);
  if (!date) {
    return '';
  }
  const now = new Date();
  const pad = (number) => String(number).padStart(2, '0');
  const time = `${pad(date.getHours())}:${pad(date.getMinutes())}`;
  const sameDay =
    date.getFullYear() === now.getFullYear() &&
    date.getMonth() === now.getMonth() &&
    date.getDate() === now.getDate();
  if (sameDay) {
    return time;
  }
  const day = `${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
  if (date.getFullYear() === now.getFullYear()) {
    return `${day} ${time}`;
  }
  return `${date.getFullYear()}-${day} ${time}`;
};

const getMessageTime = (message) =>
  message?.created_at ??
  message?.createdAt ??
  message?.time ??
  message?.timestamp;

const getMessageContent = (message) => {
  if (!message) {
    return '';
  }
  if (typeof message.content === 'string') {
    return message.content;
  }
  if (message.content == null && typeof message.text === 'string') {
    return message.text;
  }
  if (message.content == null) {
    return '';
  }
  return String(message.content);
};

const getMessageKey = (message, index) => {
  if (message?.id != null) {
    return `id-${message.id}`;
  }
  return `${getMessageTime(message) ?? ''}-${getMessageContent(message)}-${index}`;
};

const getLastMessageId = (messages) => {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index]?.id != null) {
      return messages[index].id;
    }
  }
  return null;
};

const mergeMessages = (previous, incoming) => {
  const merged = new Map();
  const add = (message, index) => {
    if (!message) {
      return;
    }
    let key;
    if (message.id != null) {
      key = `id-${message.id}`;
    } else {
      key = `${getMessageTime(message) ?? ''}-${getMessageContent(message)}-${message.sender_role ?? message.sender_type ?? message.role ?? ''}-${index}`;
    }
    merged.set(key, message);
  };
  previous.forEach(add);
  incoming.forEach(add);
  return Array.from(merged.values());
};

const isUserMessage = (message, conversation) => {
  const role = String(
    message?.sender_role ??
      message?.sender_type ??
      message?.from_role ??
      message?.role ??
      '',
  ).toLowerCase();
  if (['user', 'customer', 'visitor', 'client'].includes(role)) {
    return true;
  }
  if (
    ['admin', 'support', 'assistant', 'agent', 'staff', 'operator'].includes(
      role,
    )
  ) {
    return false;
  }
  const senderId = message?.sender_id ?? message?.user_id;
  if (senderId != null && conversation?.user_id != null) {
    return String(senderId) === String(conversation.user_id);
  }
  if (typeof message?.is_admin === 'boolean') {
    return !message.is_admin;
  }
  return false;
};

const Support = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const currentUser = useMemo(() => {
    try {
      return JSON.parse(localStorage.getItem('user') || '{}') || {};
    } catch (error) {
      return {};
    }
  }, []);
  const currentUserId = currentUser.id ?? getUserIdFromLocalStorage();

  const [statusFilter, setStatusFilter] = useState('open');
  const [conversations, setConversations] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [listLoading, setListLoading] = useState(false);
  const [unreadCount, setUnreadCount] = useState(0);
  const [selectedId, setSelectedId] = useState(null);
  const [selectedConversation, setSelectedConversation] = useState(null);
  const [messages, setMessages] = useState([]);
  const [messagesLoading, setMessagesLoading] = useState(false);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [claiming, setClaiming] = useState(false);
  const [closing, setClosing] = useState(false);

  const listLoadingRef = useRef(false);
  const messagesLoadingRef = useRef(false);
  const selectedIdRef = useRef(null);
  const lastMessageIdRef = useRef(null);
  const lastReadIdRef = useRef('');
  const listErrorRef = useRef(false);
  const messagesErrorRef = useRef(false);
  const messagesContainerRef = useRef(null);

  const fetchConversations = useCallback(
    async ({ silent = false } = {}) => {
      if (listLoadingRef.current) {
        return;
      }
      listLoadingRef.current = true;
      if (!silent) {
        setListLoading(true);
      }
      try {
        const params = { page, page_size: PAGE_SIZE };
        if (statusFilter !== 'all') {
          params.status = statusFilter;
        }
        const res = await API.get('/api/support/admin/conversations', {
          params,
          skipErrorHandler: true,
        });
        if (!res.data?.success) {
          throw new Error(res.data?.message || t('加载客服工作台失败'));
        }
        const data = res.data?.data || {};
        const items = Array.isArray(data)
          ? data
          : Array.isArray(data.items)
            ? data.items
            : [];
        setConversations(items);
        setTotal(Number(data.total ?? items.length));
        if (data.page != null) {
          setPage(Number(data.page));
        }
        setSelectedConversation((previous) => {
          if (!previous) {
            return previous;
          }
          const updated = items.find(
            (item) => String(item.id) === String(previous.id),
          );
          return updated ? { ...previous, ...updated } : previous;
        });
        listErrorRef.current = false;
      } catch (error) {
        if (!listErrorRef.current) {
          showError(resolveErrorMessage(error, t('加载客服工作台失败')));
          listErrorRef.current = true;
        }
      } finally {
        listLoadingRef.current = false;
        if (!silent) {
          setListLoading(false);
        }
      }
    },
    [page, statusFilter, t],
  );

  const fetchUnread = useCallback(async () => {
    try {
      const res = await API.get('/api/support/admin/unread', {
        skipErrorHandler: true,
      });
      if (res.data?.success) {
        setUnreadCount(Number(res.data?.data?.count || 0));
      }
    } catch {
      // Polling errors are surfaced by the conversation list request.
    }
  }, []);

  const markRead = useCallback(async (conversationId, lastMessageId) => {
    if (conversationId == null || lastMessageId == null) {
      return;
    }
    const readKey = String(lastMessageId);
    if (lastReadIdRef.current === readKey) {
      return;
    }
    try {
      await API.post(
        `/api/support/admin/conversations/${conversationId}/read`,
        { last_message_id: normalizeId(lastMessageId) },
        { skipErrorHandler: true },
      );
      lastReadIdRef.current = readKey;
      setConversations((previous) =>
        previous.map((item) =>
          String(item.id) === String(conversationId)
            ? { ...item, admin_unread: 0 }
            : item,
        ),
      );
    } catch {
      // Read receipts are best-effort and should not interrupt polling.
    }
  }, []);

  const fetchMessages = useCallback(
    async ({ incremental = false, silent = false } = {}) => {
      const conversationId = selectedIdRef.current;
      if (conversationId == null || messagesLoadingRef.current) {
        return;
      }
      messagesLoadingRef.current = true;
      if (!silent) {
        setMessagesLoading(true);
      }
      try {
        const params = {
          limit: incremental ? INCREMENTAL_MESSAGE_LIMIT : MESSAGE_PAGE_SIZE,
        };
        if (incremental && lastMessageIdRef.current != null) {
          params.after_id = normalizeId(lastMessageIdRef.current);
        }
        const res = await API.get(
          `/api/support/admin/conversations/${conversationId}/messages`,
          { params, skipErrorHandler: true },
        );
        if (selectedIdRef.current !== conversationId) {
          return;
        }
        if (!res.data?.success) {
          throw new Error(res.data?.message || t('加载消息失败'));
        }
        const data = res.data?.data || {};
        const incoming = Array.isArray(data.messages)
          ? data.messages
          : Array.isArray(data)
            ? data
            : [];
        if (data.conversation) {
          setSelectedConversation((previous) =>
            previous
              ? { ...previous, ...data.conversation }
              : data.conversation,
          );
        }
        setMessages((previous) =>
          mergeMessages(incremental ? previous : [], incoming),
        );
        const latestId = getLastMessageId(incoming);
        if (latestId != null) {
          lastMessageIdRef.current = latestId;
          markRead(conversationId, latestId);
        }
        messagesErrorRef.current = false;
      } catch (error) {
        if (
          selectedIdRef.current === conversationId &&
          !messagesErrorRef.current
        ) {
          showError(resolveErrorMessage(error, t('加载消息失败')));
          messagesErrorRef.current = true;
        }
      } finally {
        messagesLoadingRef.current = false;
        if (!silent) {
          setMessagesLoading(false);
        }
      }
    },
    [markRead, t],
  );

  useEffect(() => {
    fetchConversations();
    fetchUnread();
    const timer = setInterval(() => {
      if (document.visibilityState === 'hidden') {
        return;
      }
      fetchConversations({ silent: true });
      fetchUnread();
    }, LIST_POLL_INTERVAL);
    return () => clearInterval(timer);
  }, [fetchConversations, fetchUnread]);

  useEffect(() => {
    if (selectedId == null) {
      return undefined;
    }
    fetchMessages();
    const timer = setInterval(() => {
      if (document.visibilityState === 'hidden') {
        return;
      }
      fetchMessages({ incremental: true, silent: true });
    }, MESSAGE_POLL_INTERVAL);
    return () => clearInterval(timer);
  }, [fetchMessages, selectedId]);

  useEffect(() => {
    const container = messagesContainerRef.current;
    if (container) {
      container.scrollTop = container.scrollHeight;
    }
  }, [messages, selectedId]);

  const selectConversation = (conversation) => {
    if (!conversation) {
      return;
    }
    selectedIdRef.current = conversation.id;
    setSelectedId(conversation.id);
    setSelectedConversation(conversation);
    setMessages([]);
    setInput('');
    lastMessageIdRef.current = null;
    lastReadIdRef.current = '';
    messagesErrorRef.current = false;
  };

  const clearSelection = () => {
    selectedIdRef.current = null;
    setSelectedId(null);
    setSelectedConversation(null);
    setMessages([]);
    setInput('');
    lastMessageIdRef.current = null;
    lastReadIdRef.current = '';
  };

  const handleStatusFilterChange = (nextStatus) => {
    if (nextStatus === statusFilter) {
      return;
    }
    listErrorRef.current = false;
    setStatusFilter(nextStatus);
    setPage(1);
    clearSelection();
  };

  const handlePageChange = (nextPage) => {
    if (nextPage === page) {
      return;
    }
    listErrorRef.current = false;
    setPage(nextPage);
  };

  const handleSend = async () => {
    const content = input.trim();
    const conversationId = selectedIdRef.current;
    if (!content || conversationId == null || sending) {
      return;
    }
    setSending(true);
    try {
      const res = await API.post(
        `/api/support/admin/conversations/${conversationId}/messages`,
        { content },
        { skipErrorHandler: true },
      );
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('消息发送失败，请重试'));
      }
      const data = res.data?.data || {};
      const created = data.message || (data.content ? data : null);
      if (created) {
        setMessages((previous) => mergeMessages(previous, [created]));
        const latestId = getLastMessageId([created]);
        if (latestId != null) {
          lastMessageIdRef.current = latestId;
        }
      }
      setInput('');
      fetchMessages({ incremental: true, silent: true });
      fetchConversations({ silent: true });
    } catch (error) {
      showError(resolveErrorMessage(error, t('消息发送失败，请重试')));
    } finally {
      setSending(false);
    }
  };

  const handleClaim = async () => {
    const conversationId = selectedIdRef.current;
    if (conversationId == null || claiming) {
      return;
    }
    setClaiming(true);
    try {
      const res = await API.post(
        `/api/support/admin/conversations/${conversationId}/claim`,
        {},
        { skipErrorHandler: true },
      );
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('加载客服工作台失败'));
      }
      const data = res.data?.data || {};
      const updated = data.conversation || data;
      const updatedData = updated && typeof updated === 'object' ? updated : {};
      setSelectedConversation((previous) =>
        previous
          ? {
              ...previous,
              ...updatedData,
              assigned_admin_id: updatedData.assigned_admin_id ?? currentUserId,
              assigned_admin_name:
                updatedData.assigned_admin_name ??
                currentUser.display_name ??
                currentUser.username ??
                '',
              status: updatedData.status || previous.status,
            }
          : previous,
      );
      fetchConversations({ silent: true });
    } catch (error) {
      showError(resolveErrorMessage(error, t('加载客服工作台失败')));
    } finally {
      setClaiming(false);
    }
  };

  const handleClose = async () => {
    const conversationId = selectedIdRef.current;
    if (conversationId == null || closing) {
      return;
    }
    setClosing(true);
    try {
      const res = await API.post(
        `/api/support/admin/conversations/${conversationId}/close`,
        {},
        { skipErrorHandler: true },
      );
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('加载客服工作台失败'));
      }
      const data = res.data?.data || {};
      const updated = data.conversation || data;
      const updatedData = updated && typeof updated === 'object' ? updated : {};
      setSelectedConversation((previous) =>
        previous ? { ...previous, ...updatedData, status: 'closed' } : previous,
      );
      fetchConversations({ silent: true });
    } catch (error) {
      showError(resolveErrorMessage(error, t('加载客服工作台失败')));
    } finally {
      setClosing(false);
    }
  };

  const renderConversationList = () => {
    if (listLoading && conversations.length === 0) {
      return (
        <div className='flex justify-center py-10'>
          <Spin />
        </div>
      );
    }
    if (conversations.length === 0) {
      return (
        <div className='flex justify-center py-10 px-4'>
          <Empty
            title={
              statusFilter === 'open' ? t('没有可接待的会话') : t('暂无会话')
            }
          />
        </div>
      );
    }
    return conversations.map((conversation) => {
      const selected = String(selectedId) === String(conversation.id);
      const name = getConversationName(conversation);
      const preview = conversation.last_message_preview || '';
      const unread = Number(conversation.admin_unread || 0);
      return (
        <div
          key={conversation.id ?? conversation.user_id}
          role='button'
          tabIndex={0}
          aria-pressed={selected}
          onClick={() => selectConversation(conversation)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ' ') {
              event.preventDefault();
              selectConversation(conversation);
            }
          }}
          style={{
            padding: '12px',
            borderBottom: '1px solid var(--semi-color-border)',
            cursor: 'pointer',
            background: selected
              ? 'var(--semi-color-primary-light-default)'
              : 'transparent',
            outline: 'none',
          }}
        >
          <div className='flex items-center justify-between gap-2'>
            <div className='flex items-center gap-2 min-w-0'>
              <div
                style={{
                  width: 32,
                  height: 32,
                  flexShrink: 0,
                  borderRadius: '50%',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  background: 'var(--semi-color-fill-0)',
                  color: 'var(--semi-color-text-1)',
                  fontSize: 14,
                  fontWeight: 600,
                }}
              >
                {String(name).trim().charAt(0).toUpperCase() || '?'}
              </div>
              <div className='min-w-0'>
                <div className='truncate font-medium'>{name}</div>
                <div
                  className='truncate text-xs'
                  style={{ color: 'var(--semi-color-text-2)' }}
                >
                  {conversation.username || `#${conversation.user_id}`}
                </div>
              </div>
            </div>
            <div className='flex flex-col items-end gap-1 flex-shrink-0'>
              <span
                className='text-xs'
                style={{ color: 'var(--semi-color-text-2)' }}
              >
                {formatTime(conversation.last_message_at)}
              </span>
              {unread > 0 && (
                <Tag color='red' size='small' shape='circle'>
                  {unread > 99 ? '99+' : unread}
                </Tag>
              )}
            </div>
          </div>
          {preview && (
            <div
              className='truncate text-sm mt-1'
              style={{ color: 'var(--semi-color-text-1)' }}
            >
              {preview}
            </div>
          )}
        </div>
      );
    });
  };

  const renderMessages = () => {
    if (messagesLoading && messages.length === 0) {
      return (
        <div className='flex justify-center py-10'>
          <Spin />
        </div>
      );
    }
    return messages.map((message, index) => {
      const userMessage = isUserMessage(message, selectedConversation);
      return (
        <div
          key={getMessageKey(message, index)}
          className={`flex mb-3 ${userMessage ? 'justify-start' : 'justify-end'}`}
        >
          <div style={{ maxWidth: '78%', minWidth: 0 }}>
            <div
              className={`text-xs mb-1 ${
                userMessage ? 'text-left' : 'text-right'
              }`}
              style={{ color: 'var(--semi-color-text-2)' }}
            >
              {userMessage ? t('用户') : t('客服')} ·{' '}
              {formatTime(getMessageTime(message))}
            </div>
            <div
              style={{
                padding: '10px 12px',
                borderRadius: 8,
                background: userMessage
                  ? 'var(--semi-color-fill-0)'
                  : 'var(--semi-color-primary-light-default)',
                color: 'var(--semi-color-text-0)',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
              }}
            >
              {getMessageContent(message)}
            </div>
          </div>
        </div>
      );
    });
  };

  const renderConversationPanel = () => {
    if (!selectedConversation) {
      return (
        <div
          className='flex justify-center items-center'
          style={{ height: '100%' }}
        >
          <Empty title={t('请选择会话')} />
        </div>
      );
    }
    const closed = isConversationClosed(selectedConversation);
    const assignedAdminId = selectedConversation.assigned_admin_id;
    const assignedAdminName = selectedConversation.assigned_admin_name;
    const name = getConversationName(selectedConversation);
    let statusColor = 'orange';
    let statusText = t('等待客服接入');
    if (closed) {
      statusColor = 'grey';
      statusText = t('会话已关闭');
    } else if (assignedAdminId != null) {
      statusColor = 'green';
      statusText = t('客服已接入');
    }
    return (
      <>
        <div
          style={{
            padding: '12px 16px',
            borderBottom: '1px solid var(--semi-color-border)',
          }}
        >
          <div className='flex items-center justify-between gap-2'>
            <div className='flex items-center gap-2 min-w-0'>
              {isMobile && (
                <Button
                  type='tertiary'
                  theme='borderless'
                  icon={<ArrowLeft size={16} />}
                  onClick={clearSelection}
                  aria-label='back'
                />
              )}
              <div className='min-w-0'>
                <div className='truncate font-semibold'>{name}</div>
                <div
                  className='truncate text-xs'
                  style={{ color: 'var(--semi-color-text-2)' }}
                >
                  {selectedConversation.username ||
                    `#${selectedConversation.user_id}`}
                </div>
              </div>
            </div>
            <div className='flex items-center gap-2 flex-shrink-0'>
              {!closed && assignedAdminId == null && (
                <Button
                  size='small'
                  type='primary'
                  theme='light'
                  icon={<UserRound size={14} />}
                  loading={claiming}
                  onClick={handleClaim}
                >
                  {t('领取会话')}
                </Button>
              )}
              {!closed && (
                <Button
                  size='small'
                  type='danger'
                  theme='light'
                  icon={<X size={14} />}
                  loading={closing}
                  onClick={handleClose}
                >
                  {t('关闭会话')}
                </Button>
              )}
            </div>
          </div>
          <div className='flex items-center gap-2 flex-wrap mt-2'>
            <Tag color={statusColor} size='small'>
              {statusText}
            </Tag>
            {!closed && assignedAdminId != null && assignedAdminName && (
              <Text type='secondary' size='small'>
                {t('客服')}: {assignedAdminName}
              </Text>
            )}
          </div>
        </div>

        <div
          ref={messagesContainerRef}
          style={{
            flex: 1,
            overflowY: 'auto',
            padding: 16,
            background: 'var(--semi-color-bg-0)',
          }}
        >
          {renderMessages()}
        </div>

        {closed ? (
          <div
            style={{
              padding: '12px 16px',
              borderTop: '1px solid var(--semi-color-border)',
            }}
          >
            <Text type='secondary'>{t('会话已关闭')}</Text>
          </div>
        ) : (
          <div
            style={{
              padding: 12,
              borderTop: '1px solid var(--semi-color-border)',
            }}
          >
            <TextArea
              value={input}
              onChange={setInput}
              placeholder={t('请输入消息')}
              autosize={{ minRows: 1, maxRows: 4 }}
              disabled={sending}
              onKeyDown={(event) => {
                if (
                  event.key === 'Enter' &&
                  !event.shiftKey &&
                  !event.nativeEvent.isComposing
                ) {
                  event.preventDefault();
                  handleSend();
                }
              }}
            />
            <div className='flex justify-end mt-2'>
              <Button
                type='primary'
                theme='solid'
                icon={<Send size={14} />}
                loading={sending}
                disabled={!input.trim()}
                onClick={handleSend}
              >
                {t('发送')}
              </Button>
            </div>
          </div>
        )}
      </>
    );
  };

  const panelHeight = isMobile ? 'calc(100vh - 190px)' : 'calc(100vh - 240px)';

  const statusOptions = [
    { key: 'all', label: t('全部') },
    { key: 'open', label: t('进行中') },
    { key: 'closed', label: t('已关闭') },
  ];

  return (
    <div className='mt-[60px] px-2 pb-6'>
      <div className='flex items-center gap-2 mb-3'>
        <Title heading={4} style={{ margin: 0 }}>
          {t('客服工作台')}
        </Title>
        <Badge count={unreadCount} overflowCount={99} />
      </div>

      <div
        style={{
          display: 'flex',
          gap: 12,
          height: panelHeight,
          minHeight: 420,
        }}
      >
        {(!isMobile || selectedId == null) && (
          <Card
            className='flex-shrink-0'
            style={{
              width: isMobile ? '100%' : 320,
              height: '100%',
              minWidth: 0,
            }}
            bodyStyle={{
              padding: 0,
              height: '100%',
              display: 'flex',
              flexDirection: 'column',
              overflow: 'hidden',
            }}
          >
            <div
              style={{
                padding: '12px',
                borderBottom: '1px solid var(--semi-color-border)',
              }}
            >
              <div className='flex items-center justify-between gap-2'>
                <Text strong>{t('客服会话')}</Text>
                <Text type='tertiary' size='small'>
                  {total}
                </Text>
              </div>
              <div className='flex items-center gap-1 mt-2 flex-wrap'>
                {statusOptions.map((option) => (
                  <Button
                    key={option.key}
                    size='small'
                    type={statusFilter === option.key ? 'primary' : 'tertiary'}
                    theme={statusFilter === option.key ? 'solid' : 'borderless'}
                    onClick={() => handleStatusFilterChange(option.key)}
                  >
                    {option.label}
                  </Button>
                ))}
              </div>
            </div>

            <div style={{ flex: 1, overflowY: 'auto' }}>
              {renderConversationList()}
            </div>

            {total > PAGE_SIZE && (
              <div
                style={{
                  padding: '8px 12px',
                  borderTop: '1px solid var(--semi-color-border)',
                }}
              >
                <Pagination
                  currentPage={page}
                  pageSize={PAGE_SIZE}
                  total={total}
                  size='small'
                  onPageChange={handlePageChange}
                />
              </div>
            )}
          </Card>
        )}

        {(!isMobile || selectedId != null) && (
          <Card
            style={{ flex: 1, minWidth: 0, height: '100%' }}
            bodyStyle={{
              padding: 0,
              height: '100%',
              display: 'flex',
              flexDirection: 'column',
              overflow: 'hidden',
            }}
          >
            {renderConversationPanel()}
          </Card>
        )}
      </div>
    </div>
  );
};

export default Support;
