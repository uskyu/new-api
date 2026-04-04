import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { API, processGroupsData } from '../../helpers';
import { API_ENDPOINTS } from '../../constants/playground.constants';
import {
  clearMessagesForSession,
  createSessionRecord,
  deleteSession,
  ensureDefaultSession,
  getCurrentSessionId,
  getDefaultModel,
  loadAllSessions,
  loadSessionMessages,
  persistCurrentSessionId,
  putSession,
  readStoredPrefs,
  replaceSessionMessages,
  writeStoredPrefs,
} from '../../utils/aiConsoleStorage';

const DEFAULT_SESSION_TITLE = '新对话';
const MAX_TITLE_LENGTH = 24;

const sortModelNames = (models) =>
  [...models].sort((left, right) => {
    const leftIsGpt = left.toLowerCase().startsWith('gpt');
    const rightIsGpt = right.toLowerCase().startsWith('gpt');

    if (leftIsGpt && !rightIsGpt) return -1;
    if (!leftIsGpt && rightIsGpt) return 1;
    return left.localeCompare(right);
  });

const buildModelOptions = (userModels, pricingModels = []) => {
  const pricingMap = new Map(
    pricingModels.map((model) => [model.model_name, model]),
  );

  return sortModelNames(userModels)
    .filter((modelName) => pricingMap.has(modelName))
    .map((modelName) => {
      const pricingMeta = pricingMap.get(modelName);
      return {
        value: modelName,
        label: modelName,
        vendorName: pricingMeta?.vendor_name || '',
        description: pricingMeta?.description || '',
        enableGroups: Array.isArray(pricingMeta?.enable_groups)
          ? pricingMeta.enable_groups
          : [],
      };
    });
};

const getMessageText = (message) => {
  if (!message) return '';
  if (Array.isArray(message.content)) {
    const textItem = message.content.find((item) => item.type === 'text');
    return textItem?.text || '';
  }
  return typeof message.content === 'string' ? message.content : '';
};

const getPreviewText = (messages) => {
  const latest = [...messages]
    .reverse()
    .find((message) => message.status !== 'loading');
  return getMessageText(latest).trim().slice(0, 80);
};

const getSessionTitle = (messages) => {
  const firstUser = messages.find((message) => message.role === 'user');
  const title = getMessageText(firstUser).trim();
  if (!title) {
    return DEFAULT_SESSION_TITLE;
  }
  return title.slice(0, MAX_TITLE_LENGTH);
};

const sortSessions = (sessions) =>
  [...sessions].sort((left, right) => right.updatedAt - left.updatedAt);

const upsertSession = (sessions, nextSession) => {
  const filtered = sessions.filter((session) => session.id !== nextSession.id);
  return sortSessions([nextSession, ...filtered]);
};

export default function useAiConsoleState() {
  const storedPrefs = readStoredPrefs();
  const [ready, setReady] = useState(false);
  const [storageError, setStorageError] = useState(null);
  const [sessions, setSessions] = useState([]);
  const [currentSessionId, setCurrentSessionId] = useState(null);
  const [messages, setMessages] = useState([]);
  const [models, setModels] = useState([]);
  const [groups, setGroups] = useState([]);
  const [selectedModel, setSelectedModel] = useState(
    storedPrefs.selectedModel || getDefaultModel(),
  );
  const [selectedGroup, setSelectedGroup] = useState(
    storedPrefs.selectedGroup || '',
  );
  const [draftImages, setDraftImages] = useState([]);

  const initializedRef = useRef(false);
  const suppressNextPersistRef = useRef(false);
  const activityRef = useRef(false);

  const currentSession = useMemo(
    () => sessions.find((session) => session.id === currentSessionId) || null,
    [currentSessionId, sessions],
  );

  const filteredModels = useMemo(() => {
    if (!selectedGroup) {
      return models;
    }
    return models.filter((model) => {
      if (!Array.isArray(model.enableGroups) || model.enableGroups.length === 0) {
        return true;
      }
      return model.enableGroups.includes(selectedGroup);
    });
  }, [models, selectedGroup]);

  const refreshSessions = useCallback(async () => {
    const loaded = await loadAllSessions();
    setSessions(loaded);
    return loaded;
  }, []);

  const loadGroups = useCallback(async () => {
    try {
      const response = await API.get(API_ENDPOINTS.USER_GROUPS);
      const { success, data } = response.data;
      if (!success || !data || typeof data !== 'object') {
        return;
      }

      const userGroup =
        JSON.parse(localStorage.getItem('user') || '{}')?.group || '';
      const nextGroups = processGroupsData(data, userGroup);
      setGroups(nextGroups);

      const hasSelected = nextGroups.some(
        (group) => group.value === selectedGroup,
      );
      if (!hasSelected && nextGroups.length > 0) {
        setSelectedGroup(nextGroups[0].value);
      }
    } catch {
      // Ignore group load failures and keep the empty fallback.
    }
  }, [selectedGroup]);

  const loadModels = useCallback(async () => {
    try {
      const [userModelsResult, pricingResult] = await Promise.allSettled([
        API.get(API_ENDPOINTS.USER_MODELS),
        API.get('/api/pricing', { skipErrorHandler: true }),
      ]);

      const userModels =
        userModelsResult.status === 'fulfilled' &&
        userModelsResult.value.data?.success &&
        Array.isArray(userModelsResult.value.data?.data)
          ? userModelsResult.value.data.data
          : [];

      const pricingModels =
        pricingResult.status === 'fulfilled' &&
        pricingResult.value.data?.success &&
        Array.isArray(pricingResult.value.data?.data)
          ? pricingResult.value.data.data
          : [];

      const nextModels = buildModelOptions(userModels, pricingModels);
      setModels(nextModels);
    } catch {
      // Keep fallback model if loading fails.
    }
  }, []);

  const hydrateSession = useCallback(async (sessionId) => {
    if (!sessionId) {
      setMessages([]);
      return;
    }

    suppressNextPersistRef.current = true;
    const loadedMessages = await loadSessionMessages(sessionId);
    setMessages(loadedMessages);
    setDraftImages([]);
  }, []);

  useEffect(() => {
    if (initializedRef.current) {
      return;
    }

    initializedRef.current = true;

    const initialize = async () => {
      try {
        await Promise.all([loadModels(), loadGroups()]);
        await ensureDefaultSession();
        const loadedSessions = await refreshSessions();
        const storedSessionId = getCurrentSessionId();
        const activeSession =
          loadedSessions.find((session) => session.id === storedSessionId) ||
          loadedSessions[0] ||
          null;

        if (activeSession) {
          persistCurrentSessionId(activeSession.id);
          setCurrentSessionId(activeSession.id);
          if (activeSession.model) {
            setSelectedModel(activeSession.model);
          }
          if (typeof activeSession.group === 'string') {
            setSelectedGroup(activeSession.group);
          }
          await hydrateSession(activeSession.id);
        }

        setReady(true);
      } catch (error) {
        setStorageError(error);
        setReady(true);
      }
    };

    initialize();
  }, [hydrateSession, loadGroups, loadModels, refreshSessions]);

  useEffect(() => {
    if (filteredModels.length === 0) {
      return;
    }

    const hasSelected = filteredModels.some(
      (model) => model.value === selectedModel,
    );

    if (!hasSelected) {
      setSelectedModel(filteredModels[0].value);
    }
  }, [filteredModels, selectedModel]);

  useEffect(() => {
    writeStoredPrefs({ selectedModel, selectedGroup });
  }, [selectedGroup, selectedModel]);

  useEffect(() => {
    if (suppressNextPersistRef.current) {
      suppressNextPersistRef.current = false;
      return;
    }

    if (!ready || !currentSessionId) {
      return;
    }

    const persist = async () => {
      const derivedTitle =
        currentSession?.customTitle && currentSession?.title
          ? currentSession.title
          : getSessionTitle(messages);

      const nextSession = {
        ...(currentSession ||
          createSessionRecord({
            title: DEFAULT_SESSION_TITLE,
            model: selectedModel,
            group: selectedGroup,
          })),
        id: currentSessionId,
        model: selectedModel,
        group: selectedGroup,
        title: derivedTitle,
        updatedAt: activityRef.current
          ? Date.now()
          : currentSession?.updatedAt || Date.now(),
        lastMessagePreview: getPreviewText(messages),
      };

      await putSession(nextSession);
      await replaceSessionMessages(currentSessionId, messages);
      setSessions((previous) => upsertSession(previous, nextSession));
      activityRef.current = false;
    };

    const timeoutId = setTimeout(() => {
      persist().catch(() => {});
    }, 120);

    return () => clearTimeout(timeoutId);
  }, [
    currentSession,
    currentSessionId,
    messages,
    ready,
    selectedGroup,
    selectedModel,
  ]);

  const createSession = useCallback(async () => {
    const nextSession = createSessionRecord({
      title: DEFAULT_SESSION_TITLE,
      model: selectedModel,
      group: selectedGroup,
    });
    await putSession(nextSession);
    setSessions((previous) => upsertSession(previous, nextSession));
    setCurrentSessionId(nextSession.id);
    persistCurrentSessionId(nextSession.id);
    setMessages([]);
    setDraftImages([]);
    return nextSession;
  }, [selectedGroup, selectedModel]);

  const switchSession = useCallback(
    async (sessionId) => {
      if (!sessionId || sessionId === currentSessionId) {
        return;
      }

      suppressNextPersistRef.current = true;
      const targetSession = sessions.find((session) => session.id === sessionId);
      persistCurrentSessionId(sessionId);
      setCurrentSessionId(sessionId);
      if (targetSession?.model) {
        setSelectedModel(targetSession.model);
      }
      if (typeof targetSession?.group === 'string') {
        setSelectedGroup(targetSession.group);
      }
      await hydrateSession(sessionId);
    },
    [currentSessionId, hydrateSession, sessions],
  );

  const renameSession = useCallback(
    async (sessionId, title) => {
      const normalizedTitle = title.trim() || DEFAULT_SESSION_TITLE;
      const targetSession = sessions.find((session) => session.id === sessionId);
      if (!targetSession) {
        return;
      }

      const nextSession = {
        ...targetSession,
        title: normalizedTitle,
        customTitle: true,
        updatedAt: Date.now(),
      };
      await putSession(nextSession);
      setSessions((previous) => upsertSession(previous, nextSession));
    },
    [sessions],
  );

  const clearCurrentSession = useCallback(async () => {
    if (!currentSessionId) {
      return;
    }

    setMessages([]);
    setDraftImages([]);
    await clearMessagesForSession(currentSessionId);

    if (currentSession) {
      const nextSession = {
        ...currentSession,
        title:
          currentSession.customTitle && currentSession.title
            ? currentSession.title
            : DEFAULT_SESSION_TITLE,
        lastMessagePreview: '',
        updatedAt: Date.now(),
      };
      await putSession(nextSession);
      setSessions((previous) => upsertSession(previous, nextSession));
    }
  }, [currentSession, currentSessionId]);

  const markSessionActivity = useCallback(() => {
    activityRef.current = true;
  }, []);

  const removeSession = useCallback(
    async (sessionId) => {
      await deleteSession(sessionId);
      const remainingSessions = sessions.filter((session) => session.id !== sessionId);
      setSessions(remainingSessions);

      if (sessionId !== currentSessionId) {
        return;
      }

      if (remainingSessions.length === 0) {
        const nextSession = await createSession();
        persistCurrentSessionId(nextSession.id);
        setCurrentSessionId(nextSession.id);
        await hydrateSession(nextSession.id);
        return;
      }

      const fallback = remainingSessions[0];
      persistCurrentSessionId(fallback.id);
      setCurrentSessionId(fallback.id);
      if (fallback.model) {
        setSelectedModel(fallback.model);
      }
      if (typeof fallback.group === 'string') {
        setSelectedGroup(fallback.group);
      }
      await hydrateSession(fallback.id);
    },
    [createSession, currentSessionId, hydrateSession, sessions],
  );

  return {
    ready,
    storageError,
    sessions,
    currentSessionId,
    currentSession,
    messages,
    setMessages,
    models: filteredModels,
    allModels: models,
    groups,
    selectedModel,
    selectedGroup,
    setSelectedModel,
    setSelectedGroup,
    draftImages,
    setDraftImages,
    createSession,
    switchSession,
    renameSession,
    removeSession,
    clearCurrentSession,
    markSessionActivity,
  };
}
