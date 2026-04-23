const DB_NAME = 'ai_image_db';
const DB_VERSION = 1;
const STORE_SESSIONS = 'sessions';
const STORE_MESSAGES = 'messages';

const LOCAL_STORAGE_SESSION_KEY = 'ai_image_current_session';
const LOCAL_STORAGE_PREFS_KEY = 'ai_image_prefs';
const DEFAULT_MODEL = 'gpt-image-2';
export const MAX_HISTORY_RECORDS = 30;
const MAX_HISTORY_MESSAGES = MAX_HISTORY_RECORDS * 2;

const sanitizeImageUrl = (value) => {
  if (typeof value !== 'string') return value;
  if (value.startsWith('blob:')) {
    return '[local-image-preview]';
  }
  return value;
};

const sanitizeMessageContent = (content, role) => {
  if (!Array.isArray(content)) {
    if (role === 'user' && typeof content === 'string' && content.includes('data:image/')) {
      return content.replace(/data:image\/[^)\s]+/g, '[local-image-preview]');
    }
    return content;
  }
  return content.map((item) => {
    if (!item || typeof item !== 'object') return item;
    if (item.type === 'image_url' && item.image_url) {
      return {
        ...item,
        image_url:
          typeof item.image_url === 'string'
            ? sanitizeImageUrl(item.image_url)
            : {
                ...item.image_url,
                url: sanitizeImageUrl(item.image_url.url),
              },
      };
    }
    return item;
  });
};

const sanitizeMessageForStorage = (message) => {
  const role = message?.role;
  return {
    ...message,
    content: sanitizeMessageContent(message?.content, role),
    parts: Array.isArray(message?.parts)
      ? message.parts.map((part) => ({
          ...part,
          image_url: part?.image_url
            ? {
                ...part.image_url,
                url:
                  role === 'user'
                    ? sanitizeImageUrl(part.image_url.url)
                    : part.image_url.url,
              }
            : part?.image_url,
        }))
      : message?.parts,
  };
};

const ensureRequestSuccess = (request) =>
  new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });

const ensureCursorComplete = (request, onCursor) =>
  new Promise((resolve, reject) => {
    request.onsuccess = () => {
      const cursor = request.result;
      if (!cursor) {
        resolve();
        return;
      }
      onCursor(cursor);
      cursor.continue();
    };
    request.onerror = () => reject(request.error);
  });

export function openDatabase() {
  if (!window.indexedDB) {
    return Promise.reject(new Error('IndexedDB is not available'));
  }

  return new Promise((resolve, reject) => {
    const request = window.indexedDB.open(DB_NAME, DB_VERSION);

    request.onupgradeneeded = (event) => {
      const db = event.target.result;

      if (!db.objectStoreNames.contains(STORE_SESSIONS)) {
        db.createObjectStore(STORE_SESSIONS, { keyPath: 'id' });
      }

      if (!db.objectStoreNames.contains(STORE_MESSAGES)) {
        const messageStore = db.createObjectStore(STORE_MESSAGES, {
          keyPath: 'id',
        });
        messageStore.createIndex('sessionId', 'sessionId', { unique: false });
        messageStore.createIndex('createdAt', 'createdAt', { unique: false });
        messageStore.createIndex('sortIndex', 'sortIndex', { unique: false });
      }
    };

    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

const withDatabase = async (work) => {
  const db = await openDatabase();
  return work(db);
};

export const createSessionRecord = ({
  title = '新绘图',
  model = DEFAULT_MODEL,
  group = '',
  customTitle = false,
} = {}) => ({
  id: `session-${Date.now()}-${Math.random().toString(16).slice(2)}`,
  title,
  model,
  group,
  customTitle,
  updatedAt: Date.now(),
  lastMessagePreview: '',
});

export async function loadAllSessions() {
  const sessions = await withDatabase(async (db) => {
    const tx = db.transaction(STORE_SESSIONS, 'readonly');
    const request = tx.objectStore(STORE_SESSIONS).getAll();
    return ensureRequestSuccess(request);
  });

  return sessions.sort((left, right) => right.updatedAt - left.updatedAt);
}

export async function putSession(session) {
  return withDatabase(async (db) => {
    const tx = db.transaction(STORE_SESSIONS, 'readwrite');
    tx.objectStore(STORE_SESSIONS).put(session);
    return new Promise((resolve, reject) => {
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
    });
  });
}

export async function loadSessionMessages(sessionId) {
  const messages = await withDatabase(async (db) => {
    const tx = db.transaction(STORE_MESSAGES, 'readonly');
    const request = tx
      .objectStore(STORE_MESSAGES)
      .index('sessionId')
      .getAll(IDBKeyRange.only(sessionId));
    return ensureRequestSuccess(request);
  });

  const sortedMessages = messages.sort((left, right) => {
    const leftOrder = Number.isInteger(left.sortIndex) ? left.sortIndex : 0;
    const rightOrder = Number.isInteger(right.sortIndex) ? right.sortIndex : 0;
    if (leftOrder !== rightOrder) {
      return leftOrder - rightOrder;
    }
    return left.createAt - right.createAt;
  });

  return sortedMessages.slice(-MAX_HISTORY_MESSAGES);
}

export async function replaceSessionMessages(sessionId, messages) {
  const recentMessages = messages.slice(-MAX_HISTORY_MESSAGES);

  await withDatabase(async (db) => {
    const clearTx = db.transaction(STORE_MESSAGES, 'readwrite');
    const clearStore = clearTx.objectStore(STORE_MESSAGES);

    await ensureCursorComplete(
      clearStore.index('sessionId').openKeyCursor(IDBKeyRange.only(sessionId)),
      (cursor) => {
        clearStore.delete(cursor.primaryKey);
      },
    );

    return new Promise((resolve, reject) => {
      clearTx.oncomplete = () => resolve();
      clearTx.onerror = () => reject(clearTx.error);
    });
  });

  return withDatabase(async (db) => {
    const writeTx = db.transaction(STORE_MESSAGES, 'readwrite');
    const writeStore = writeTx.objectStore(STORE_MESSAGES);

    recentMessages.forEach((message, index) => {
      writeStore.put({ ...sanitizeMessageForStorage(message), sessionId, sortIndex: index });
    });

    return new Promise((resolve, reject) => {
      writeTx.oncomplete = () => resolve();
      writeTx.onerror = () => reject(writeTx.error);
    });
  });
}

export async function clearMessagesForSession(sessionId) {
  return replaceSessionMessages(sessionId, []);
}

export async function deleteSession(sessionId) {
  await withDatabase(async (db) => {
    const tx = db.transaction(STORE_SESSIONS, 'readwrite');
    tx.objectStore(STORE_SESSIONS).delete(sessionId);
    return new Promise((resolve, reject) => {
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
    });
  });

  await clearMessagesForSession(sessionId);
}

export async function ensureDefaultSession() {
  const sessions = await loadAllSessions();
  if (sessions.length > 0) {
    return sessions[0];
  }

  const session = createSessionRecord();
  await putSession(session);
  persistCurrentSessionId(session.id);
  return session;
}

export function getCurrentSessionId() {
  return localStorage.getItem(LOCAL_STORAGE_SESSION_KEY);
}

export function persistCurrentSessionId(sessionId) {
  localStorage.setItem(LOCAL_STORAGE_SESSION_KEY, sessionId);
}

export function readStoredPrefs() {
  try {
    const raw = localStorage.getItem(LOCAL_STORAGE_PREFS_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
}

export function writeStoredPrefs(prefs) {
  try {
    localStorage.setItem(LOCAL_STORAGE_PREFS_KEY, JSON.stringify(prefs));
  } catch {
    // Ignore lightweight pref persistence failures.
  }
}

export function getDefaultModel() {
  return DEFAULT_MODEL;
}
