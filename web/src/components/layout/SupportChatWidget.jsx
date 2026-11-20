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

import React, { useContext, useEffect, useRef, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Button, Input, Spin } from '@douyinfe/semi-ui';
import {
  IconClose,
  IconCustomerSupport,
  IconSend,
} from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../helpers';
import { UserContext } from '../../context/User';

const CONVERSATION_ENDPOINT = '/api/support/conversation';
const POLL_INTERVAL_MS = 2000;
const UNREAD_POLL_INTERVAL_MS = 5000;

function unwrapPayload(response) {
  return response?.data?.data ?? response?.data ?? null;
}

function normalizeConversation(payload) {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
    return null;
  }

  const source =
    payload.conversation && typeof payload.conversation === 'object'
      ? payload.conversation
      : payload;
  const id =
    source.id ??
    source.conversation_id ??
    payload.conversation_id ??
    payload.id;

  if (id === null || id === undefined || id === '') {
    return null;
  }

  const status = String(source.status ?? '').toLowerCase();
  const adminId =
    source.admin_id ??
    source.admin_user_id ??
    source.operator_id ??
    source.assigned_admin_id;
  const hasAdminId =
    adminId !== null &&
    adminId !== undefined &&
    adminId !== '' &&
    adminId !== 0 &&
    adminId !== '0';
  const adminJoined =
    hasAdminId ||
    source.admin_joined === true ||
    [
      'active',
      'serving',
      'connected',
      'assigned',
      'in_progress',
      'processing',
    ].includes(status);
  const closed =
    source.closed === true ||
    source.is_closed === true ||
    ['closed', 'ended', 'resolved', 'finished'].includes(status);
  const unreadValue = Number(
    source.user_unread ?? source.user_unread_count ?? source.unread_count ?? 0,
  );

  return {
    id,
    status: closed ? 'closed' : 'open',
    adminJoined,
    userUnread: Number.isFinite(unreadValue) && unreadValue > 0 ? unreadValue : 0,
  };
}

function normalizeMessage(raw, index) {
  if (!raw || typeof raw !== 'object') {
    return null;
  }

  const id = raw.id ?? raw.message_id ?? raw.msg_id;
  const contentValue = raw.content ?? raw.message ?? raw.text ?? raw.body;
  if (contentValue === null || contentValue === undefined) {
    return null;
  }

  const senderValue = String(
    raw.sender_type ?? raw.sender ?? raw.role ?? raw.author_type ?? raw.from ?? '',
  ).toLowerCase();
  const direction = String(raw.direction ?? '').toLowerCase();
  const isAdmin =
    raw.is_admin === true ||
    raw.is_admin === 1 ||
    String(raw.is_admin ?? '').toLowerCase() === 'true' ||
    raw.from_admin === true ||
    raw.is_from_admin === true ||
    ['admin', 'agent', 'support', 'staff', 'operator', 'assistant'].includes(
      senderValue,
    ) ||
    senderValue.includes('admin') ||
    senderValue.includes('agent') ||
    direction.includes('admin') ||
    direction.includes('agent');
  const numericId = Number(id);
  const createdAt =
    raw.created_at ??
    raw.created_time ??
    raw.timestamp ??
    raw.time ??
    raw.sent_at;
  const senderName =
    raw.sender_name ??
    raw.sender_nickname ??
    raw.admin_name ??
    raw.user_name ??
    raw.nickname;

  return {
    key:
      id !== null && id !== undefined && id !== ''
        ? `id-${id}`
        : `fallback-${index}-${String(createdAt ?? '')}-${String(
            contentValue,
          ).slice(0, 24)}`,
    id: id === null || id === undefined || id === '' ? null : id,
    numericId: Number.isFinite(numericId) ? numericId : 0,
    isAdmin,
    content: String(contentValue),
    createdAt,
    senderName:
      typeof senderName === 'string' && senderName.trim()
        ? senderName.trim()
        : '',
  };
}

function normalizeMessages(payload) {
  let list = null;

  if (Array.isArray(payload)) {
    list = payload;
  } else if (Array.isArray(payload?.messages)) {
    list = payload.messages;
  } else if (Array.isArray(payload?.items)) {
    list = payload.items;
  } else if (Array.isArray(payload?.list)) {
    list = payload.list;
  } else if (Array.isArray(payload?.data)) {
    list = payload.data;
  } else if (Array.isArray(payload?.data?.messages)) {
    list = payload.data.messages;
  }

  if (!list) {
    return [];
  }

  return list
    .map((item, index) => normalizeMessage(item, index))
    .filter(Boolean);
}

function extractConversation(payload) {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
    return null;
  }

  if (payload.conversation && typeof payload.conversation === 'object') {
    return normalizeConversation(payload.conversation);
  }

  if (payload.session && typeof payload.session === 'object') {
    return normalizeConversation(payload.session);
  }

  if (
    payload.id !== undefined &&
    (payload.status !== undefined || payload.user_unread !== undefined)
  ) {
    return normalizeConversation(payload);
  }

  return null;
}

function parseTimestamp(value) {
  if (value === null || value === undefined || value === '') {
    return 0;
  }

  if (typeof value === 'number') {
    return value < 1e12 ? value * 1000 : value;
  }

  const numericValue = Number(value);
  if (Number.isFinite(numericValue)) {
    return numericValue < 1e12 ? numericValue * 1000 : numericValue;
  }

  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? 0 : parsed;
}

function formatMessageTime(value) {
  if (value === null || value === undefined || value === '') {
    return '';
  }

  const date = new Date(parseTimestamp(value));
  if (Number.isNaN(date.getTime())) {
    return '';
  }

  return date.toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
  });
}

function mergeMessages(current, incoming) {
  const map = new Map(current.map((message) => [message.key, message]));
  incoming.forEach((message) => {
    map.set(message.key, message);
  });

  return Array.from(map.values()).sort((left, right) => {
    if (left.numericId && right.numericId) {
      return left.numericId - right.numericId;
    }

    const leftTime = parseTimestamp(left.createdAt);
    const rightTime = parseTimestamp(right.createdAt);
    return leftTime - rightTime;
  });
}

const SupportChatWidget = () => {
  const [userState] = useContext(UserContext);
  const location = useLocation();
  const { t } = useTranslation();

  const [isOpen, setIsOpen] = useState(false);
  const [conversation, setConversation] = useState(null);
  const [messages, setMessages] = useState([]);
  const [inputValue, setInputValue] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  const [isLoadingMessages, setIsLoadingMessages] = useState(false);
  const [conversationErrorKey, setConversationErrorKey] = useState('');
  const [messagesErrorKey, setMessagesErrorKey] = useState('');
  const [isSending, setIsSending] = useState(false);
  const [isClosing, setIsClosing] = useState(false);

  const conversationRef = useRef(null);
  const messagesRef = useRef([]);
  const lastMessageIdRef = useRef(0);
  const pollTimerRef = useRef(null);
  const refreshMessagesRef = useRef(null);
  const messageListRef = useRef(null);
  const mountedRef = useRef(true);

  const pathname = location.pathname;
  const isConsoleRoute =
    pathname === '/console' || pathname.startsWith('/console/');
  const isSupportWorkbench =
    pathname === '/console/support' ||
    pathname.startsWith('/console/support/');
  const visible =
    Boolean(userState?.user) && isConsoleRoute && !isSupportWorkbench;

  const conversationClosed = conversation?.status === 'closed';
  const adminJoined =
    Boolean(conversation?.adminJoined) ||
    messages.some((message) => message.isAdmin);
  const statusLabel = conversationClosed
    ? t('会话已关闭')
    : adminJoined
      ? t('客服已接入')
      : t('等待客服接入');
  const statusClass = conversationClosed
    ? 'is-closed'
    : adminJoined
      ? 'is-active'
      : 'is-waiting';
  const unreadCount = conversation?.userUnread || 0;
  const userName =
    userState?.user?.username ||
    userState?.user?.display_name ||
    userState?.user?.email ||
    '';

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    if (!isOpen) {
      return undefined;
    }

    const handleKeyDown = (event) => {
      if (event.key === 'Escape') {
        setIsOpen(false);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen || !visible) {
      return undefined;
    }

    const messageList = messageListRef.current;
    if (messageList) {
      messageList.scrollTop = messageList.scrollHeight;
    }
  }, [isOpen, visible, messages]);

  useEffect(() => {
    if (!visible || !isOpen) {
      if (!visible && isOpen) {
        setIsOpen(false);
      }
      return undefined;
    }

    let active = true;
    let inFlight = false;

    const stopPolling = () => {
      if (pollTimerRef.current !== null) {
        clearInterval(pollTimerRef.current);
        pollTimerRef.current = null;
      }
    };

    const applyConversation = (rawConversation) => {
      const next = normalizeConversation(rawConversation);
      if (!next) {
        return;
      }

      conversationRef.current = next;
      if (active) {
        setConversation(next);
      }
      if (next.status === 'closed') {
        stopPolling();
      }
    };

    const recordLastMessageId = (nextMessages) => {
      nextMessages.forEach((message) => {
        if (
          message.numericId &&
          message.numericId > lastMessageIdRef.current
        ) {
          lastMessageIdRef.current = message.numericId;
        }
      });
    };

    const appendMessages = (incoming) => {
      if (!incoming.length || !active) {
        return false;
      }

      const next = mergeMessages(messagesRef.current, incoming);
      const changed =
        next.length !== messagesRef.current.length ||
        incoming.some(
          (message) =>
            !messagesRef.current.some((item) => item.key === message.key),
        );

      messagesRef.current = next;
      recordLastMessageId(incoming);
      setMessages(next);
      return changed;
    };

    const markConversationRead = async (conversationId) => {
      if (!active || !lastMessageIdRef.current) {
        return;
      }

      try {
        await API.post(
          `${CONVERSATION_ENDPOINT}/${conversationId}/read`,
          { last_message_id: lastMessageIdRef.current },
          { skipErrorHandler: true },
        );
        if (!active) {
          return;
        }

        const current = conversationRef.current;
        if (current && current.id === conversationId) {
          const next = { ...current, userUnread: 0 };
          conversationRef.current = next;
          setConversation(next);
        }
      } catch {
        // A failed read receipt is non-blocking; unread state will refresh
        // with the next conversation poll/reopen.
      }
    };

    const loadMessages = async (conversationId, { initial = false } = {}) => {
      if (!active || inFlight) {
        return;
      }

      inFlight = true;
      if (initial) {
        setIsLoadingMessages(true);
      }

      try {
        const response = await API.get(
          `${CONVERSATION_ENDPOINT}/${conversationId}/messages`,
          {
            params: { after_id: lastMessageIdRef.current || 0 },
            skipErrorHandler: true,
            disableDuplicate: true,
          },
        );
        if (!active) {
          return;
        }

        const payload = unwrapPayload(response);
        const incomingMessages = normalizeMessages(payload);
        const rawConversation =
          payload?.conversation && typeof payload.conversation === 'object'
            ? payload.conversation
            : payload?.session && typeof payload.session === 'object'
              ? payload.session
              : null;

        if (rawConversation) {
          applyConversation(rawConversation);
        }

        const hasNewMessages = appendMessages(incomingMessages);
        setMessagesErrorKey('');

        if (hasNewMessages || initial || conversationRef.current?.userUnread) {
          await markConversationRead(conversationId);
        }
      } catch {
        if (active) {
          setMessagesErrorKey('加载消息失败');
        }
      } finally {
        inFlight = false;
        if (active && initial) {
          setIsLoadingMessages(false);
        }
      }
    };

    const startPolling = (conversationId) => {
      stopPolling();
      pollTimerRef.current = window.setInterval(() => {
        loadMessages(conversationId);
      }, POLL_INTERVAL_MS);
    };

    refreshMessagesRef.current = (conversationId) => {
      if (conversationId) {
        loadMessages(conversationId);
      }
    };

    const bootstrap = async () => {
      const existing = conversationRef.current;
      if (existing && existing.status !== 'closed') {
        setConversationErrorKey('');
        setMessagesErrorKey('');
        setIsCreating(false);
        await loadMessages(existing.id, {
          initial: messagesRef.current.length === 0,
        });
        if (
          active &&
          conversationRef.current?.status !== 'closed'
        ) {
          startPolling(existing.id);
        }
        return;
      }

      setIsCreating(true);
      setConversationErrorKey('');
      setMessagesErrorKey('');
      setIsLoadingMessages(false);
      conversationRef.current = null;
      setConversation(null);
      messagesRef.current = [];
      setMessages([]);
      lastMessageIdRef.current = 0;

      try {
        const response = await API.post(
          CONVERSATION_ENDPOINT,
          {},
          { skipErrorHandler: true },
        );
        if (!active) {
          return;
        }

        const payload = unwrapPayload(response);
        const normalized = normalizeConversation(payload);
        if (!normalized) {
          throw new Error('Invalid support conversation response');
        }

        conversationRef.current = normalized;
        setConversation(normalized);
        setIsCreating(false);

        const embeddedMessages = normalizeMessages(payload);
        if (embeddedMessages.length) {
          const appended = embeddedMessages.some(
            (message) => !messagesRef.current.some((item) => item.key === message.key),
          );
          messagesRef.current = mergeMessages(
            messagesRef.current,
            embeddedMessages,
          );
          recordLastMessageId(embeddedMessages);
          setMessages(messagesRef.current);
          if (appended) {
            await markConversationRead(normalized.id);
          }
        }

        await loadMessages(normalized.id, {
          initial: embeddedMessages.length === 0,
        });
        if (
          active &&
          conversationRef.current?.status !== 'closed'
        ) {
          startPolling(normalized.id);
        }
      } catch {
        if (!active) {
          return;
        }
        setConversationErrorKey('加载会话失败');
        setIsCreating(false);
        setIsLoadingMessages(false);
      }
    };

    bootstrap().catch(() => {
      if (active) {
        setConversationErrorKey('加载会话失败');
        setIsCreating(false);
      }
    });

    return () => {
      active = false;
      stopPolling();
      refreshMessagesRef.current = null;
    };
  }, [isOpen, visible]);

  useEffect(() => {
    if (!visible || isOpen) {
      return undefined;
    }

    let active = true;
    let inFlight = false;

    const pollUnread = async () => {
      if (!active || inFlight) {
        return;
      }

      inFlight = true;
      try {
        const response = await API.get(
          `${CONVERSATION_ENDPOINT}/current`,
          {
            skipErrorHandler: true,
            disableDuplicate: true,
          },
        );
        if (!active) {
          return;
        }

        const payload = unwrapPayload(response);
        const normalized = normalizeConversation(payload);
        if (normalized) {
          conversationRef.current = normalized;
          setConversation(normalized);
          return;
        }

        const current = conversationRef.current;
        if (current) {
          const next = { ...current, userUnread: 0 };
          conversationRef.current = next;
          setConversation(next);
        }
      } catch {
        // Unread polling is best-effort; the panel will refresh when opened.
      } finally {
        inFlight = false;
      }
    };

    pollUnread();
    const timer = window.setInterval(pollUnread, UNREAD_POLL_INTERVAL_MS);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [isOpen, visible]);

  const handleClosePanel = () => {
    setIsOpen(false);
  };

  const handleSend = async () => {
    const content = inputValue.trim();
    const current = conversationRef.current;

    if (
      !content ||
      !current ||
      current.status === 'closed' ||
      isSending
    ) {
      return;
    }

    setIsSending(true);

    try {
      const response = await API.post(
        `${CONVERSATION_ENDPOINT}/${current.id}/messages`,
        { content },
        { skipErrorHandler: true },
      );
      if (!mountedRef.current) {
        return;
      }

      const payload = unwrapPayload(response);
      const sentMessage = normalizeMessage(
        payload?.message && typeof payload.message === 'object'
          ? payload.message
          : payload,
        0,
      );

      setInputValue('');
      if (sentMessage) {
        const next = mergeMessages(messagesRef.current, [sentMessage]);
        messagesRef.current = next;
        setMessages(next);
        if (sentMessage.numericId > lastMessageIdRef.current) {
          lastMessageIdRef.current = sentMessage.numericId;
        }
      } else {
        refreshMessagesRef.current?.(current.id);
      }
    } catch (error) {
      if (!mountedRef.current) {
        return;
      }
      showError(
        error?.response?.data?.message || t('消息发送失败，请重试'),
      );
    } finally {
      if (mountedRef.current) {
        setIsSending(false);
      }
    }
  };

  const handleCloseConversation = async () => {
    const current = conversationRef.current;
    if (!current || current.status === 'closed' || isClosing) {
      return;
    }

    setIsClosing(true);

    try {
      const response = await API.post(
        `${CONVERSATION_ENDPOINT}/${current.id}/close`,
        {},
        { skipErrorHandler: true },
      );
      if (!mountedRef.current) {
        return;
      }

      const normalized = normalizeConversation(unwrapPayload(response));
      const next =
        normalized && normalized.id === current.id
          ? { ...normalized, status: 'closed', userUnread: 0 }
          : { ...current, status: 'closed', userUnread: 0 };

      conversationRef.current = next;
      setConversation(next);
      if (pollTimerRef.current !== null) {
        clearInterval(pollTimerRef.current);
        pollTimerRef.current = null;
      }
      showSuccess(t('会话已关闭'));
    } catch (error) {
      if (!mountedRef.current) {
        return;
      }
      showError(error);
    } finally {
      if (mountedRef.current) {
        setIsClosing(false);
      }
    }
  };

  if (!visible) {
    return null;
  }

  if (!isOpen) {
    return (
      <button
        type='button'
        className='support-chat-fab'
        aria-label={t('在线客服')}
        title={t('在线客服')}
        onClick={() => setIsOpen(true)}
      >
        <span className='support-chat-fab__shine' aria-hidden='true' />
        <span className='support-chat-fab__wave' aria-hidden='true' />
        <span
          className='support-chat-fab__wave support-chat-fab__wave--delay'
          aria-hidden='true'
        />
        <span className='support-chat-fab__icon' aria-hidden='true'>
          <IconCustomerSupport size='extra-large' />
        </span>
        {unreadCount > 0 && (
          <span className='support-chat-fab__badge'>
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>
    );
  }

  let bodyContent = null;

  if (isCreating || (!conversation && !conversationErrorKey)) {
    bodyContent = (
      <div className='support-chat-panel__state'>
        <Spin size='small' />
        <span className='support-chat-panel__state-text'>
          {t('等待客服接入')}
        </span>
      </div>
    );
  } else if (conversationErrorKey) {
    bodyContent = (
      <div
        className='support-chat-panel__state support-chat-panel__state--error'
        role='alert'
      >
        {t(conversationErrorKey)}
      </div>
    );
  } else if (!conversation) {
    bodyContent = (
      <div className='support-chat-panel__state'>
        <span className='support-chat-panel__state-text'>
          {t('暂无消息，开始和客服对话吧。')}
        </span>
      </div>
    );
  } else if (isLoadingMessages && messages.length === 0) {
    bodyContent = (
      <div className='support-chat-panel__state'>
        <Spin size='small' />
      </div>
    );
  } else if (messages.length === 0) {
    bodyContent = (
      <div className='support-chat-panel__state'>
        {messagesErrorKey ? (
          <span className='support-chat-panel__state-text support-chat-panel__state-text--error'>
            {t(messagesErrorKey)}
          </span>
        ) : (
          <span className='support-chat-panel__state-text'>
            {t('暂无消息，开始和客服对话吧。')}
          </span>
        )}
      </div>
    );
  } else {
    bodyContent = (
      <>
        {messagesErrorKey && (
          <div className='support-chat-panel__inline-error' role='alert'>
            {t(messagesErrorKey)}
          </div>
        )}
        {messages.map((message) => {
          const sender = message.isAdmin
            ? message.senderName || t('在线客服')
            : message.senderName || userName || '我';
          const time = formatMessageTime(message.createdAt);

          return (
            <div
              key={message.key}
              className={`support-chat-message ${
                message.isAdmin
                  ? 'support-chat-message--admin'
                  : 'support-chat-message--user'
              }`}
            >
              <div className='support-chat-message__meta'>
                <span className='support-chat-message__sender'>{sender}</span>
                {time && (
                  <span className='support-chat-message__time'>{time}</span>
                )}
              </div>
              <div className='support-chat-message__bubble'>
                {message.content}
              </div>
            </div>
          );
        })}
      </>
    );
  }

  const canSend =
    Boolean(conversation) &&
    !conversationClosed &&
    !isSending &&
    Boolean(inputValue.trim());

  return (
    <div
      className='support-chat-panel'
      role='dialog'
      aria-label={t('在线客服')}
    >
      <div className='support-chat-panel__header'>
        <div className='support-chat-panel__heading'>
          <span className='support-chat-panel__avatar' aria-hidden='true'>
            <IconCustomerSupport size='large' />
          </span>
          <div className='support-chat-panel__title-block'>
            <div className='support-chat-panel__title'>{t('在线客服')}</div>
            <span
              className={`support-chat-panel__status ${statusClass}`}
            >
              <span
                className='support-chat-panel__status-dot'
                aria-hidden='true'
              />
              {statusLabel}
            </span>
          </div>
        </div>
        <Button
          theme='borderless'
          type='tertiary'
          icon={<IconClose />}
          aria-label='关闭'
          onClick={handleClosePanel}
        />
      </div>
      <div
        className='support-chat-panel__body'
        ref={messageListRef}
        role='log'
        aria-live='polite'
      >
        {bodyContent}
      </div>
      <div className='support-chat-panel__footer'>
        {conversation && !conversationClosed && (
          <div className='support-chat-panel__actions'>
            <Button
              theme='borderless'
              type='tertiary'
              size='small'
              loading={isClosing}
              disabled={isClosing}
              onClick={handleCloseConversation}
            >
              {t('关闭会话')}
            </Button>
          </div>
        )}
        <div className='support-chat-panel__composer'>
          <Input
            className='support-chat-panel__input'
            value={inputValue}
            placeholder={t('请输入消息')}
            disabled={!conversation || conversationClosed || isCreating}
            onChange={setInputValue}
            onEnterPress={handleSend}
            aria-label={t('请输入消息')}
          />
          <Button
            theme='solid'
            type='primary'
            icon={<IconSend />}
            loading={isSending}
            disabled={!canSend}
            onClick={handleSend}
            className='support-chat-panel__send'
          >
            {t('发送')}
          </Button>
        </div>
      </div>
    </div>
  );
};

export default SupportChatWidget;
