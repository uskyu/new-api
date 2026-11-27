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
import { useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Button, Input, Spin } from '@douyinfe/semi-ui';
import {
  IconClose,
  IconCustomerSupport,
  IconSend,
} from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../helpers';
import { UserContext } from '../../context/User';
import {
  buildSupportMessageFormData,
  createSupportComposerItem,
  extractClipboardImageFiles,
  releaseSupportComposerItem,
  validateSupportImageFiles,
} from '../support/supportAttachmentUtils';
import SupportAttachmentGrid from '../support/SupportAttachmentView';
import {
  SupportComposerAttachments,
  SupportImagePicker,
} from '../support/SupportComposerAttachments';

const CONVERSATION_ENDPOINT = '/api/support/conversation';
const POLL_INTERVAL_MS = 2000;
const UNREAD_POLL_INTERVAL_MS = 5000;

const SUPPORT_IMAGE_ERROR_KEYS = {
  unsupported: '仅支持 JPG、PNG、GIF、WebP 图片',
  too_large: '单张图片不能超过 5MB',
  too_many: '每条消息最多发送 4 张图片',
};

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
  const attachments = Array.from(raw.attachments || [])
    .filter((attachment) => attachment && attachment.id != null)
    .map((attachment) => ({
      id: attachment.id,
      file_name: attachment.file_name ?? '',
      mime_type: attachment.mime_type ?? '',
      size_bytes: attachment.size_bytes ?? null,
      width: attachment.width ?? null,
      height: attachment.height ?? null,
      expired: Boolean(attachment.expired),
    }));
  let contentValue = raw.content ?? raw.message ?? raw.text ?? raw.body;
  if (contentValue === null || contentValue === undefined) {
    if (attachments.length === 0) {
      return null;
    }
    contentValue = '';
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
    attachments,
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

const SUPPORT_ENTRY_POSITION_KEY = 'support-chat-entry-position';

function readSupportEntryPosition() {
  try {
    const raw = localStorage.getItem(SUPPORT_ENTRY_POSITION_KEY);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw);
    if (
      !parsed ||
      !Number.isFinite(Number(parsed.left)) ||
      !Number.isFinite(Number(parsed.top))
    ) {
      return null;
    }
    return { left: Number(parsed.left), top: Number(parsed.top) };
  } catch {
    return null;
  }
}

function clampSupportEntryPosition(left, top, width, height) {
  const margin = 8;
  const maxLeft = Math.max(margin, window.innerWidth - width - margin);
  const maxTop = Math.max(margin, window.innerHeight - height - margin);
  return {
    left: Math.min(Math.max(left, margin), maxLeft),
    top: Math.min(Math.max(top, margin), maxTop),
  };
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
  const navigate = useNavigate();
  const { t } = useTranslation();

  const [isOpen, setIsOpen] = useState(false);
  const [conversation, setConversation] = useState(null);
  const [messages, setMessages] = useState([]);
  const [inputValue, setInputValue] = useState('');
  const [composerAttachments, setComposerAttachments] = useState([]);
  const [isCreating, setIsCreating] = useState(false);
  const [isLoadingMessages, setIsLoadingMessages] = useState(false);
  const [conversationErrorKey, setConversationErrorKey] = useState('');
  const [messagesErrorKey, setMessagesErrorKey] = useState('');
  const [isSending, setIsSending] = useState(false);
  const [isClosing, setIsClosing] = useState(false);
  const [entryPosition, setEntryPosition] = useState(readSupportEntryPosition);
  const [isDraggingEntry, setIsDraggingEntry] = useState(false);
  const [appearance, setAppearance] = useState({
    uiScale: 100,
    textScale: 100,
    prominence: 100,
    customTitle: '',
    customSubLogin: '',
    customSubGuest: '',
    bottomOffset: 96,
  });

  const conversationRef = useRef(null);
  const messagesRef = useRef([]);
  const lastMessageIdRef = useRef(0);
  const pollTimerRef = useRef(null);
  const refreshMessagesRef = useRef(null);
  const messageListRef = useRef(null);
  const mountedRef = useRef(true);
  const composerAttachmentsRef = useRef([]);
  const entryRef = useRef(null);
  const dragRef = useRef(null);
  const suppressEntryClickRef = useRef(false);

  useEffect(() => {
    const clampCurrent = () => {
      const element = entryRef.current;
      if (!element || !entryPosition) {
        return;
      }
      const rect = element.getBoundingClientRect();
      const next = clampSupportEntryPosition(
        entryPosition.left,
        entryPosition.top,
        rect.width,
        rect.height,
      );
      if (next.left !== entryPosition.left || next.top !== entryPosition.top) {
        setEntryPosition(next);
      }
    };
    window.addEventListener('resize', clampCurrent);
    clampCurrent();
    return () => window.removeEventListener('resize', clampCurrent);
  }, [entryPosition, appearance.uiScale, appearance.textScale]);

  const pathname = location.pathname;
  const isSupportWorkbench =
    pathname === '/console/support' ||
    pathname.startsWith('/console/support/');
  const isLoggedIn = Boolean(userState?.user);
  // 客服入口在所有前台/控制台页面展示；客服工作台本身已有完整聊天界面，
  // 不再重复显示右下角入口，避免挡住会话列表。
  const visible = !isSupportWorkbench;

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
  const appearanceStyle = {
    '--support-ui-scale': appearance.uiScale / 100,
    '--support-text-scale': appearance.textScale / 100,
    '--support-prominence': appearance.prominence / 100,
    '--support-bottom-offset': `${appearance.bottomOffset}px`,
  };
  const entryTitle = appearance.customTitle || t('在线客服');
  const entrySubtitle = isLoggedIn
    ? appearance.customSubLogin || t('有问题随时联系')
    : appearance.customSubGuest || t('登录后即可联系');
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
    composerAttachmentsRef.current = composerAttachments;
  }, [composerAttachments]);

  useEffect(() => {
    let active = true;

    const parsePercent = (value, fallback) => {
      const parsed = Number(value);
      return Number.isFinite(parsed) ? parsed : fallback;
    };

    const loadAppearance = async () => {
      try {
        const response = await API.get('/api/support/appearance', {
          skipErrorHandler: true,
          disableDuplicate: true,
        });
        if (!active) {
          return;
        }
        const data = unwrapPayload(response);
        if (!data || typeof data !== 'object') {
          return;
        }
        const parseText = (value) =>
          typeof value === 'string' ? value.trim() : '';
        setAppearance({
          uiScale: parsePercent(data.ui_scale, 100),
          textScale: parsePercent(data.text_scale, 100),
          prominence: parsePercent(data.prominence, 100),
          customTitle: parseText(data.custom_title),
          customSubLogin: parseText(data.custom_sub_login),
          customSubGuest: parseText(data.custom_sub_guest),
          bottomOffset: parsePercent(data.bottom_offset, 96),
        });
      } catch {
        // Appearance settings are optional; keep the local defaults.
      }
    };

    loadAppearance();
    const timer = window.setInterval(loadAppearance, 60000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, []);

  useEffect(() => {
    return () => {
      composerAttachmentsRef.current.forEach(releaseSupportComposerItem);
      composerAttachmentsRef.current = [];
    };
  }, []);

  useEffect(() => {
    if (isLoggedIn) {
      return;
    }

    conversationRef.current = null;
    messagesRef.current = [];
    lastMessageIdRef.current = 0;
    setConversation(null);
    setMessages([]);
    setConversationErrorKey('');
    setMessagesErrorKey('');
    setIsCreating(false);
    setIsLoadingMessages(false);
    setIsSending(false);
    setIsClosing(false);
  }, [isLoggedIn]);

  useEffect(() => {
    if (isOpen) {
      return;
    }

    const pending = composerAttachmentsRef.current;
    if (pending.length === 0) {
      return;
    }

    composerAttachmentsRef.current = [];
    pending.forEach(releaseSupportComposerItem);
    setComposerAttachments([]);
  }, [isOpen]);

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
  }, [isOpen, visible, messages, composerAttachments]);

  useEffect(() => {
    if (!visible || !isOpen) {
      if (!visible && isOpen) {
        setIsOpen(false);
      }
      return undefined;
    }

    if (!isLoggedIn) {
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
  }, [isOpen, visible, isLoggedIn]);

  useEffect(() => {
    if (!isLoggedIn || !visible || isOpen) {
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
  }, [isOpen, visible, isLoggedIn]);

  const openEntryPanel = () => {
    if (suppressEntryClickRef.current) {
      suppressEntryClickRef.current = false;
      return;
    }
    setIsOpen(true);
  };

  const handleEntryPointerDown = (event) => {
    if (event.button !== 0) {
      return;
    }
    const element = entryRef.current;
    if (!element) {
      return;
    }
    const rect = element.getBoundingClientRect();
    dragRef.current = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      originLeft: rect.left,
      originTop: rect.top,
      width: rect.width,
      height: rect.height,
      moved: false,
    };
    element.setPointerCapture?.(event.pointerId);
  };

  const handleEntryPointerMove = (event) => {
    const drag = dragRef.current;
    if (!drag || event.pointerId !== drag.pointerId) {
      return;
    }
    const deltaX = event.clientX - drag.startX;
    const deltaY = event.clientY - drag.startY;
    if (!drag.moved && Math.hypot(deltaX, deltaY) < 5) {
      return;
    }
    drag.moved = true;
    if (!isDraggingEntry) {
      setIsDraggingEntry(true);
    }
    event.preventDefault();
    const next = clampSupportEntryPosition(
      drag.originLeft + deltaX,
      drag.originTop + deltaY,
      drag.width,
      drag.height,
    );
    setEntryPosition(next);
  };

  const finishEntryDrag = (event) => {
    const drag = dragRef.current;
    if (!drag) {
      return;
    }
    if (event.pointerId !== drag.pointerId) {
      return;
    }
    dragRef.current = null;
    setIsDraggingEntry(false);
    if (!drag.moved) {
      return;
    }
    suppressEntryClickRef.current = true;
    window.setTimeout(() => {
      suppressEntryClickRef.current = false;
    }, 0);
    const rect = entryRef.current?.getBoundingClientRect();
    if (rect) {
      const next = clampSupportEntryPosition(
        rect.left,
        rect.top,
        rect.width,
        rect.height,
      );
      setEntryPosition(next);
      try {
        localStorage.setItem(SUPPORT_ENTRY_POSITION_KEY, JSON.stringify(next));
      } catch {
        // Ignore storage failures; the position stays for this session.
      }
    }
  };

  const handleClosePanel = () => {
    setIsOpen(false);
  };

  const appendComposerFiles = (files) => {
    const { accepted, errors } = validateSupportImageFiles(
      files,
      composerAttachmentsRef.current.length,
    );

    if (accepted.length > 0) {
      const next = [
        ...composerAttachmentsRef.current,
        ...accepted.map((file) => createSupportComposerItem(file)),
      ];
      composerAttachmentsRef.current = next;
      setComposerAttachments(next);
    }

    const seenCodes = new Set();
    errors.forEach(({ code }) => {
      if (seenCodes.has(code)) {
        return;
      }
      seenCodes.add(code);
      const messageKey = SUPPORT_IMAGE_ERROR_KEYS[code];
      if (messageKey) {
        showError(t(messageKey));
      }
    });
  };

  const handlePaste = (event) => {
    const current = conversationRef.current;
    if (!current || current.status === 'closed' || isCreating || isSending) {
      return;
    }

    const files = extractClipboardImageFiles(event);
    if (files.length === 0) {
      return;
    }

    appendComposerFiles(files);
  };

  const handleRemoveComposerAttachment = (key) => {
    if (isSending) {
      return;
    }

    const target = composerAttachmentsRef.current.find(
      (item) => item.key === key,
    );
    if (!target) {
      return;
    }

    releaseSupportComposerItem(target);
    const next = composerAttachmentsRef.current.filter(
      (item) => item.key !== key,
    );
    composerAttachmentsRef.current = next;
    setComposerAttachments(next);
  };

  const handleSend = async () => {
    const content = inputValue.trim();
    const files = composerAttachmentsRef.current.map((item) => item.file);
    const current = conversationRef.current;

    if (
      (!content && files.length === 0) ||
      !current ||
      current.status === 'closed' ||
      isSending
    ) {
      return;
    }

    setIsSending(true);

    try {
      const url = `${CONVERSATION_ENDPOINT}/${current.id}/messages`;
      const response = files.length
        ? await API.post(
            url,
            buildSupportMessageFormData(content, files),
            { skipErrorHandler: true },
          )
        : await API.post(url, { content }, { skipErrorHandler: true });
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
      const sentItems = composerAttachmentsRef.current;
      composerAttachmentsRef.current = [];
      sentItems.forEach(releaseSupportComposerItem);
      setComposerAttachments([]);
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
      <div
        ref={entryRef}
        className={`support-chat-entry${
          isDraggingEntry ? ' is-dragging' : ''
        }`}
        style={{
          ...appearanceStyle,
          ...(entryPosition
            ? {
                left: `${entryPosition.left}px`,
                top: `${entryPosition.top}px`,
                right: 'auto',
                bottom: 'auto',
              }
            : null),
        }}
        onPointerDown={handleEntryPointerDown}
        onPointerMove={handleEntryPointerMove}
        onPointerUp={finishEntryDrag}
        onPointerCancel={finishEntryDrag}
      >
        <span
          className='support-chat-entry__caption'
          role='button'
          tabIndex={0}
          aria-label={t('在线客服')}
          onClick={openEntryPanel}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ' ') {
              event.preventDefault();
              openEntryPanel();
            }
          }}
        >
          <span className='support-chat-entry__caption-title'>
            {entryTitle}
          </span>
          <span className='support-chat-entry__caption-sub'>
            {entrySubtitle}
          </span>
        </span>
        <button
          type='button'
          className='support-chat-fab'
          aria-label={t('在线客服')}
          title={t('在线客服')}
          onClick={openEntryPanel}
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
          {isLoggedIn && unreadCount > 0 && (
            <span className='support-chat-fab__badge'>
              {unreadCount > 99 ? '99+' : unreadCount}
            </span>
          )}
        </button>
      </div>
    );
  }

  if (!isLoggedIn) {
    return (
      <div
        className='support-chat-panel'
        role='dialog'
        aria-label={t('在线客服')}
        style={appearanceStyle}
      >
        <div className='support-chat-panel__header'>
          <div className='support-chat-panel__heading'>
            <span className='support-chat-panel__avatar' aria-hidden='true'>
              <IconCustomerSupport size='extra-large' />
            </span>
            <div className='support-chat-panel__title-block'>
              <div className='support-chat-panel__title'>{entryTitle}</div>
              <span className='support-chat-panel__status is-waiting'>
                <span className='support-chat-panel__status-dot' aria-hidden='true' />
                {t('未登录')}
              </span>
            </div>
          </div>
          <Button
            theme='borderless'
            type='tertiary'
            icon={<IconClose size='large' />}
            size='large'
            aria-label='关闭'
            onClick={handleClosePanel}
          />
        </div>
        <div className='support-chat-panel__body'>
          <div className='support-chat-panel__state'>
            <IconCustomerSupport size='extra-large' />
            <span className='support-chat-panel__state-text'>
              {t('登录后才可发送消息')}
            </span>
          </div>
        </div>
        <div className='support-chat-panel__footer'>
          <Button
            theme='solid'
            type='primary'
            style={{ width: '100%' }}
            onClick={() => {
              setIsOpen(false);
              navigate('/login');
            }}
          >
            {t('去登录')}
          </Button>
        </div>
      </div>
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
          const hasText = Boolean(message.content.trim());
          const mediaOnly = !hasText && message.attachments.length > 0;

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
              <div
                className={`support-chat-message__bubble${
                  mediaOnly ? ' support-chat-message__bubble--media' : ''
                }`}
              >
                {hasText && message.content}
                {message.attachments.length > 0 && (
                  <SupportAttachmentGrid attachments={message.attachments} />
                )}
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
    (Boolean(inputValue.trim()) || composerAttachments.length > 0);

  return (
    <div
      className='support-chat-panel'
      role='dialog'
      aria-label={t('在线客服')}
      style={appearanceStyle}
      onPaste={handlePaste}
    >
      <div className='support-chat-panel__header'>
        <div className='support-chat-panel__heading'>
          <span className='support-chat-panel__avatar' aria-hidden='true'>
            <IconCustomerSupport size='extra-large' />
          </span>
          <div className='support-chat-panel__title-block'>
            <div className='support-chat-panel__title'>{entryTitle}</div>
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
          icon={<IconClose size='large' />}
          size='large'
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
        <SupportComposerAttachments
          items={composerAttachments}
          onRemove={handleRemoveComposerAttachment}
          disabled={isSending}
        />
        <div className='support-chat-panel__composer'>
          <SupportImagePicker
            className='support-chat-panel__image-picker'
            onSelectFiles={appendComposerFiles}
            disabled={
              !conversation || conversationClosed || isCreating || isSending
            }
          />
          <Input
            className='support-chat-panel__input'
            size='large'
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
            size='large'
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
