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
  InputNumber,
  Modal,
  Pagination,
  Slider,
  Spin,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { ArrowLeft, Send, Settings, UserRound, X } from 'lucide-react';
import { IconCustomerSupport } from '@douyinfe/semi-icons';
import {
  API,
  can,
  getUserIdFromLocalStorage,
  PERMISSIONS,
  showError,
  showSuccess,
} from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import {
  buildSupportMessageFormData,
  createSupportComposerItem,
  extractClipboardImageFiles,
  releaseSupportComposerItem,
  validateSupportImageFiles,
} from '../../components/support/supportAttachmentUtils';
import { SupportAttachmentGrid } from '../../components/support/SupportAttachmentView';
import {
  SupportComposerAttachments,
  SupportImagePicker,
} from '../../components/support/SupportComposerAttachments';

const { Text, Title } = Typography;

const PAGE_SIZE = 20;
const MESSAGE_PAGE_SIZE = 100;
const INCREMENTAL_MESSAGE_LIMIT = 50;
const LIST_POLL_INTERVAL = 3000;
const MESSAGE_POLL_INTERVAL = 2000;
const DEFAULT_RETENTION_DAYS = 180;
const MAX_RETENTION_DAYS = 3650;
const MIN_UI_SCALE = 80;
const MAX_UI_SCALE = 160;
const MIN_TEXT_SCALE = 80;
const MAX_TEXT_SCALE = 160;
const MIN_PROMINENCE = 0;
const MAX_PROMINENCE = 100;

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
  const [composerAttachments, setComposerAttachments] = useState([]);
  const [sending, setSending] = useState(false);
  const [claiming, setClaiming] = useState(false);
  const [closing, setClosing] = useState(false);
  const [retentionVisible, setRetentionVisible] = useState(false);
  const [retentionDays, setRetentionDays] = useState(DEFAULT_RETENTION_DAYS);
  const [uiScale, setUiScale] = useState(100);
  const [textScale, setTextScale] = useState(100);
  const [prominence, setProminence] = useState(100);
  const [retentionLoading, setRetentionLoading] = useState(false);
  const [retentionSaving, setRetentionSaving] = useState(false);

  const listLoadingRef = useRef(false);
  const messagesLoadingRef = useRef(false);
  const selectedIdRef = useRef(null);
  const lastMessageIdRef = useRef(null);
  const lastReadIdRef = useRef('');
  const listErrorRef = useRef(false);
  const messagesErrorRef = useRef(false);
  const messagesContainerRef = useRef(null);
  const composerAttachmentsRef = useRef([]);

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

  const resetComposerAttachments = useCallback(() => {
    const items = composerAttachmentsRef.current;
    composerAttachmentsRef.current = [];
    items.forEach(releaseSupportComposerItem);
    setComposerAttachments([]);
  }, []);

  const appendComposerFiles = useCallback(
    (files) => {
      const current = composerAttachmentsRef.current;
      const { accepted, errors } = validateSupportImageFiles(
        files,
        current.length,
      );
      if (accepted.length > 0) {
        const next = [...current, ...accepted.map(createSupportComposerItem)];
        composerAttachmentsRef.current = next;
        setComposerAttachments(next);
      }
      const reportedCodes = new Set();
      errors.forEach((error) => {
        const code = error?.code;
        if (reportedCodes.has(code)) {
          return;
        }
        reportedCodes.add(code);
        if (code === 'unsupported') {
          showError(t('仅支持 JPG、PNG、GIF、WebP 图片'));
        } else if (code === 'too_large') {
          showError(t('单张图片不能超过 5MB'));
        } else if (code === 'too_many') {
          showError(t('每条消息最多发送 4 张图片'));
        }
      });
    },
    [t],
  );

  const handleComposerPaste = (event) => {
    if (sending) {
      return;
    }
    const files = extractClipboardImageFiles(event);
    if (files.length === 0) {
      return;
    }
    appendComposerFiles(files);
  };

  const handleSelectComposerFiles = (files) => {
    if (sending) {
      return;
    }
    appendComposerFiles(files);
  };

  const handleRemoveComposerAttachment = (key) => {
    if (sending) {
      return;
    }
    const current = composerAttachmentsRef.current;
    const target = current.find((item) => item.key === key);
    if (!target) {
      return;
    }
    releaseSupportComposerItem(target);
    const next = current.filter((item) => item.key !== key);
    composerAttachmentsRef.current = next;
    setComposerAttachments(next);
  };

  useEffect(() => {
    return () => {
      composerAttachmentsRef.current.forEach(releaseSupportComposerItem);
      composerAttachmentsRef.current = [];
    };
  }, []);

  const selectConversation = (conversation) => {
    if (!conversation) {
      return;
    }
    selectedIdRef.current = conversation.id;
    setSelectedId(conversation.id);
    setSelectedConversation(conversation);
    setMessages([]);
    setInput('');
    resetComposerAttachments();
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
    resetComposerAttachments();
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
    const attachments = composerAttachmentsRef.current;
    if (conversationId == null || sending) {
      return;
    }
    if (!content && attachments.length === 0) {
      return;
    }
    setSending(true);
    try {
      const url = `/api/support/admin/conversations/${conversationId}/messages`;
      let res;
      if (attachments.length > 0) {
        const formData = buildSupportMessageFormData(
          content,
          attachments.map((item) => item.file),
        );
        res = await API.post(url, formData, { skipErrorHandler: true });
      } else {
        res = await API.post(url, { content }, { skipErrorHandler: true });
      }
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
      resetComposerAttachments();
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
      resetComposerAttachments();
      fetchConversations({ silent: true });
    } catch (error) {
      showError(resolveErrorMessage(error, t('加载客服工作台失败')));
    } finally {
      setClosing(false);
    }
  };

  const openRetentionModal = async () => {
    setRetentionVisible(true);
    setRetentionLoading(true);
    try {
      const res = await API.get('/api/support/admin/settings', {
        skipErrorHandler: true,
      });
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('加载留存设置失败'));
      }
      const data = res.data?.data || {};
      const parseValue = (raw, fallback) => {
        const parsed = Number(raw);
        return raw == null || !Number.isFinite(parsed) ? fallback : parsed;
      };
      setRetentionDays(parseValue(data.retention_days, DEFAULT_RETENTION_DAYS));
      setUiScale(parseValue(data.ui_scale, 100));
      setTextScale(parseValue(data.text_scale, 100));
      setProminence(parseValue(data.prominence, 100));
    } catch (error) {
      showError(resolveErrorMessage(error, t('加载留存设置失败')));
    } finally {
      setRetentionLoading(false);
    }
  };

  const closeRetentionModal = () => {
    if (retentionSaving) {
      return;
    }
    setRetentionVisible(false);
  };

  const handleSaveRetention = async () => {
    if (retentionSaving || retentionLoading) {
      return;
    }
    const rawDays = retentionDays;
    const days = Number(rawDays);
    if (
      rawDays === '' ||
      rawDays == null ||
      !Number.isInteger(days) ||
      days < 0 ||
      days > MAX_RETENTION_DAYS
    ) {
      showError(t('留存天数需为 0 至 3650 的整数'));
      return;
    }
    const invalidScale =
      !Number.isInteger(uiScale) ||
      uiScale < MIN_UI_SCALE ||
      uiScale > MAX_UI_SCALE ||
      !Number.isInteger(textScale) ||
      textScale < MIN_TEXT_SCALE ||
      textScale > MAX_TEXT_SCALE ||
      !Number.isInteger(prominence) ||
      prominence < MIN_PROMINENCE ||
      prominence > MAX_PROMINENCE;
    if (invalidScale) {
      showError(t('外观参数需在允许的百分比范围内'));
      return;
    }
    setRetentionSaving(true);
    try {
      const res = await API.put(
        '/api/support/admin/settings',
        {
          retention_days: days,
          ui_scale: uiScale,
          text_scale: textScale,
          prominence,
        },
        { skipErrorHandler: true },
      );
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('保存留存设置失败'));
      }
      showSuccess(t('客服设置已保存'));
      setRetentionVisible(false);
    } catch (error) {
      showError(resolveErrorMessage(error, t('保存留存设置失败')));
    } finally {
      setRetentionSaving(false);
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
      const content = getMessageContent(message);
      const hasContent = content.trim().length > 0;
      const attachments = Array.isArray(message?.attachments)
        ? message.attachments
        : [];
      return (
        <div
          key={getMessageKey(message, index)}
          className={`flex mb-3 ${userMessage ? 'justify-start' : 'justify-end'}`}
        >
          <div style={{ maxWidth: '82%', minWidth: 0 }}>
            <div
              className={`text-xs mb-1 ${
                userMessage ? 'text-left' : 'text-right'
              }`}
              style={{ color: 'var(--semi-color-text-2)' }}
            >
              {userMessage ? t('用户') : t('客服')} ·{' '}
              {formatTime(getMessageTime(message))}
            </div>
            {hasContent && (
              <div
                style={{
                  padding: '12px 15px',
                  borderRadius: 10,
                  background: userMessage
                    ? 'var(--semi-color-fill-0)'
                    : 'var(--semi-color-primary-light-default)',
                  color: 'var(--semi-color-text-0)',
                  fontSize: 16,
                  lineHeight: 1.65,
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                }}
              >
                {content}
              </div>
            )}
            <SupportAttachmentGrid
              attachments={attachments}
              className={hasContent ? 'mt-1' : ''}
            />
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
            onPaste={handleComposerPaste}
          >
            <SupportComposerAttachments
              items={composerAttachments}
              onRemove={handleRemoveComposerAttachment}
              disabled={sending}
            />
            <TextArea
              value={input}
              onChange={setInput}
              placeholder={t('请输入消息')}
              autosize={{ minRows: 1, maxRows: 5 }}
              style={{ fontSize: 15.5 }}
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
            <div className='flex items-center justify-between gap-2 mt-2'>
              <SupportImagePicker
                onSelectFiles={handleSelectComposerFiles}
                disabled={sending}
              />
              <Button
                type='primary'
                theme='solid'
                icon={<Send size={14} />}
                loading={sending}
                disabled={
                  sending || (!input.trim() && composerAttachments.length === 0)
                }
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
  const canManageSupport = can(PERMISSIONS.SUPPORT_MANAGE);

  const statusOptions = [
    { key: 'all', label: t('全部') },
    { key: 'open', label: t('进行中') },
    { key: 'closed', label: t('已关闭') },
  ];

  return (
    <div className='mt-[60px] px-2 pb-6'>
      <div className='flex items-center justify-between gap-2 flex-wrap mb-3'>
        <div className='flex items-center gap-2'>
          <Title heading={4} style={{ margin: 0 }}>
            {t('客服工作台')}
          </Title>
          <Badge count={unreadCount} overflowCount={99} />
        </div>
        {canManageSupport && (
          <Button
            size='small'
            type='tertiary'
            theme='light'
            icon={<Settings size={14} />}
            onClick={openRetentionModal}
          >
            {t('客服设置')}
          </Button>
        )}
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

      <Modal
        title={t('客服设置')}
        visible={retentionVisible}
        onCancel={closeRetentionModal}
        onOk={handleSaveRetention}
        okText={t('保存')}
        cancelText={t('取消')}
        confirmLoading={retentionLoading || retentionSaving}
        maskClosable={!retentionSaving}
        closable={!retentionSaving}
        width={isMobile ? '100%' : 560}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Text type='secondary' size='small' style={{ lineHeight: 1.6 }}>
            {t(
              '已关闭的会话超过设定天数后，将自动删除会话记录、聊天消息和图片附件。0 表示永久保留。',
            )}
          </Text>
          <div>
            <Text strong style={{ display: 'block', marginBottom: 8 }}>
              {t('留存天数（天）')}
            </Text>
            <InputNumber
              min={0}
              max={MAX_RETENTION_DAYS}
              precision={0}
              step={1}
              value={retentionDays}
              disabled={retentionLoading || retentionSaving}
              onChange={setRetentionDays}
              style={{ width: '100%' }}
            />
          </div>

          <div
            className='support-settings-divider'
            role='separator'
            aria-hidden='true'
          />

          <div className='support-settings-appearance-head'>
            <Text strong>{t('客服入口外观')}</Text>
            <Button
              size='small'
              theme='borderless'
              type='tertiary'
              disabled={retentionLoading || retentionSaving}
              onClick={() => {
                setUiScale(100);
                setTextScale(100);
                setProminence(100);
              }}
            >
              {t('恢复默认')}
            </Button>
          </div>
          <Text type='secondary' size='small' style={{ lineHeight: 1.6 }}>
            {t('拖动滑块调整客服入口与聊天窗口的整体大小、文字大小和显眼度，保存后对访客和客服生效。')}
          </Text>

          <div
            className='support-settings-preview'
            style={{
              '--support-ui-scale': uiScale / 100,
              '--support-text-scale': textScale / 100,
              '--support-prominence': prominence / 100,
            }}
          >
            <div className='support-settings-preview__caption'>
              <span className='support-settings-preview__title'>
                {t('在线客服')}
              </span>
              <span className='support-settings-preview__sub'>
                {t('有问题随时联系')}
              </span>
            </div>
            <span className='support-settings-preview__fab'>
              <IconCustomerSupport size='large' />
            </span>
          </div>

          <div className='support-settings-slider'>
            <div className='support-settings-slider__head'>
              <Text strong>{t('卡片大小')}</Text>
              <Text type='secondary'>{uiScale}%</Text>
            </div>
            <Slider
              min={MIN_UI_SCALE}
              max={MAX_UI_SCALE}
              step={5}
              value={uiScale}
              disabled={retentionLoading || retentionSaving}
              onChange={setUiScale}
            />
          </div>

          <div className='support-settings-slider'>
            <div className='support-settings-slider__head'>
              <Text strong>{t('文字大小')}</Text>
              <Text type='secondary'>{textScale}%</Text>
            </div>
            <Slider
              min={MIN_TEXT_SCALE}
              max={MAX_TEXT_SCALE}
              step={5}
              value={textScale}
              disabled={retentionLoading || retentionSaving}
              onChange={setTextScale}
            />
          </div>

          <div className='support-settings-slider'>
            <div className='support-settings-slider__head'>
              <Text strong>{t('显眼度')}</Text>
              <Text type='secondary'>{prominence}%</Text>
            </div>
            <Slider
              min={MIN_PROMINENCE}
              max={MAX_PROMINENCE}
              step={5}
              value={prominence}
              disabled={retentionLoading || retentionSaving}
              onChange={setProminence}
            />
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default Support;
