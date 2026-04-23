import React, { useMemo, useRef, useState } from 'react';
import { Button, Empty, Spin, TextArea, Typography } from '@douyinfe/semi-ui';
import { Check, ImagePlus, Loader2, Sparkles, Wand2, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import useAiImageState from '../../hooks/ai-image/useAiImageState';
import { API_ENDPOINTS, MESSAGE_ROLES } from '../../constants/playground.constants';
import { MAX_HISTORY_RECORDS } from '../../utils/aiImageStorage';
import {
  buildApiPayload,
  copy,
  getUserIdFromLocalStorage,
  handleApiError,
  showError,
  showSuccess,
} from '../../helpers';

const RESOLUTION_OPTIONS = ['1K', '2K', '4K'];
const ASPECT_RATIO_OPTIONS = ['1:1', '3:2', '4:3', '16:9', '9:16'];
const BATCH_COUNT_OPTIONS = [1, 2, 4];
const PROMPT_OPTIMIZER_SYSTEM_PROMPT = [
  'You are an AI image prompt optimizer.',
  'Rewrite the user prompt into one stronger image-generation prompt in Simplified Chinese.',
  'Keep the meaning, add useful visual detail, and make it directly usable for image generation.',
  'Return plain text only. Do not use markdown, JSON, lists, or explanations.',
].join('\n');

const createDraftImageEntry = (file, overrides = {}) => ({
  id: `draft-${Date.now()}-${Math.random().toString(16).slice(2)}`,
  file,
  previewUrl: URL.createObjectURL(file),
  name: file.name,
  type: file.type,
  size: file.size,
  sourceDataUrl: '',
  ...overrides,
});

const revokeDraftImage = (image) => {
  if (image?.previewUrl?.startsWith('blob:')) {
    URL.revokeObjectURL(image.previewUrl);
  }
};

const revokeDraftImages = (images = []) => {
  images.forEach(revokeDraftImage);
};

const normalizeImageSource = (value, mimeType = 'image/png') => {
  if (typeof value !== 'string' || value.trim() === '') return '';
  const source = value.trim();
  if (source.startsWith('data:image/') || source.startsWith('http')) return source;
  return `data:${mimeType || 'image/png'};base64,${source}`;
};

const extractImageUrlsFromMarkdown = (content) => {
  if (typeof content !== 'string') return [];
  const matches = [...content.matchAll(/!\[[^\]]*\]\((data:image\/[^)]+|https?:\/\/[^)]+)\)/g)];
  return matches.map((match) => match[1]).filter(Boolean);
};

const extractImageUrlsFromParts = (parts = []) =>
  parts
    .map((part) => {
      if (part?.inlineData?.data) {
        return normalizeImageSource(
          part.inlineData.data,
          part.inlineData.mimeType || 'image/png',
        );
      }
      if (part?.inline_data?.data) {
        return normalizeImageSource(
          part.inline_data.data,
          part.inline_data.mime_type || 'image/png',
        );
      }
      if (part?.image_url?.url) return normalizeImageSource(part.image_url.url);
      if (part?.imageUrl?.url) return normalizeImageSource(part.imageUrl.url);
      if (typeof part?.data === 'string' && (part?.mimeType || part?.mime_type)) {
        return normalizeImageSource(part.data, part.mimeType || part.mime_type);
      }
      if (typeof part?.text === 'string') {
        return extractImageUrlsFromMarkdown(part.text)[0] || '';
      }
      return '';
    })
    .filter(Boolean);

const getMessageText = (message) => {
  if (!message) return '';
  if (Array.isArray(message.content)) {
    const textItem = message.content.find((item) => item.type === 'text');
    return textItem?.text || '';
  }
  return typeof message.content === 'string' ? message.content : '';
};

const extractGenerationRecord = (userMessage, assistantMessage) => {
  const prompt = getMessageText(userMessage).trim();
  const imageUrls = [
    ...extractImageUrlsFromMarkdown(getMessageText(assistantMessage)),
    ...extractImageUrlsFromParts(assistantMessage?.parts || []),
  ];

  if (imageUrls.length === 0) return null;

  return {
    id: assistantMessage?.id || `record-${Date.now()}`,
    userMessageId: userMessage?.id,
    assistantMessageId: assistantMessage?.id,
    prompt,
    images: [...new Set(imageUrls)],
    createdAt: assistantMessage?.createAt || Date.now(),
  };
};

const taskToGenerationRecord = (task) => {
  if (!task?.result_url) {
    return null;
  }
  return {
    id: task.task_id || `task-${task.id}`,
    userMessageId: null,
    assistantMessageId: null,
    prompt: task.prompt || '',
    images: [task.result_url],
    createdAt: task.created_at || Date.now(),
  };
};

const trimMessagesToHistoryLimit = (messages = []) => messages.slice(-(MAX_HISTORY_RECORDS * 2));

const createImageAssistantMessage = (imageUrls, prompt) => ({
  role: MESSAGE_ROLES.ASSISTANT,
  content: imageUrls.map((url) => `![image](${url})`).join('\n\n'),
  parts: imageUrls.map((url) => ({ image_url: { url } })),
  prompt,
  createAt: Date.now(),
  id: `assistant-${Date.now()}-${Math.random().toString(16).slice(2)}`,
  status: 'complete',
});

const extractOptimizedPrompt = (rawContent) => {
  if (typeof rawContent !== 'string') return '';
  const normalized = rawContent.trim();
  if (!normalized || normalized.includes('data:image/') || normalized.length > 5000) {
    return '';
  }

  try {
    const parsed = JSON.parse(normalized);
    if (typeof parsed?.prompt === 'string' && parsed.prompt.trim()) {
      return parsed.prompt.trim();
    }
    if (
      typeof parsed?.optimized_prompt === 'string' &&
      parsed.optimized_prompt.trim()
    ) {
      return parsed.optimized_prompt.trim();
    }
    if (Array.isArray(parsed?.suggestions)) {
      return (
        parsed.suggestions.find(
          (item) => typeof item === 'string' && item.trim(),
        ) || ''
      );
    }
  } catch {
    // Fall through and treat it as plain text.
  }

  return normalized
    .split('\n')
    .map((item) => item.replace(/^\s*[-\d.)、]+\s*/, '').trim())
    .filter(Boolean)
    .join(' ')
    .trim();
};

const readFileAsDataUrl = (file) =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '');
    reader.onerror = () => reject(reader.error || new Error('failed to read file'));
    reader.readAsDataURL(file);
  });

const buildImageContent = (prompt, imageUrls = []) => [
  { type: 'text', text: prompt },
  ...imageUrls.filter(Boolean).map((url) => ({
    type: 'image_url',
    image_url: { url },
  })),
];

const toGeminiInlinePart = (dataUrl) => {
  if (typeof dataUrl !== 'string') return null;
  const matched = dataUrl.match(/^data:(.+?);base64,(.+)$/);
  if (!matched) return null;
  return {
    inlineData: {
      mimeType: matched[1] || 'image/png',
      data: matched[2] || '',
    },
  };
};

const isOpenAIImageModel = (modelName) => {
  const lower = String(modelName || '').toLowerCase();
  return lower.includes('gpt-image') || lower.includes('dall-e');
};

const getOpenAIImageSize = (aspectRatio) => {
  switch (aspectRatio) {
    case '9:16':
      return '1024x1536';
    case '16:9':
    case '3:2':
    case '4:3':
      return '1536x1024';
    case '1:1':
    default:
      return '1024x1024';
  }
};

const buildGeminiNativeImagePayload = ({
  prompt,
  imageUrls,
  resolution,
  aspectRatio,
}) => ({
  contents: [
    {
      role: 'user',
      parts: [
        { text: prompt },
        ...imageUrls
          .map((imageUrl) => toGeminiInlinePart(imageUrl))
          .filter(Boolean),
      ],
    },
  ],
  generationConfig: {
    responseModalities: ['TEXT', 'IMAGE'],
    imageConfig: {
      aspectRatio: aspectRatio,
      imageSize: resolution,
    },
  },
});

const parseImageResponse = async (response) => {
  const data = await response.json();
  const message = data?.choices?.[0]?.message || {};
  const content = message.content || data?.choices?.[0]?.delta?.content || '';
  const contentParts = Array.isArray(content) ? content : [];
  const messageParts = Array.isArray(message.parts) ? message.parts : [];
  const geminiParts = Array.isArray(data?.candidates)
    ? data.candidates.flatMap((candidate) => candidate?.content?.parts || [])
    : [];

  const imageUrls = [
    ...((Array.isArray(data?.data)
      ? data.data.flatMap((item) => {
          if (item?.url) {
            return [normalizeImageSource(item.url)];
          }
          if (typeof item?.b64_json === 'string' && item.b64_json.trim()) {
            return [normalizeImageSource(item.b64_json, 'image/png')];
          }
          return [];
        })
      : [])),
    ...extractImageUrlsFromMarkdown(typeof content === 'string' ? content : ''),
    ...extractImageUrlsFromParts(contentParts),
    ...extractImageUrlsFromParts(messageParts),
    ...extractImageUrlsFromParts(geminiParts),
  ];

  return {
    raw: data,
    content,
    imageUrls: [...new Set(imageUrls)],
  };
};

const postJsonPayload = async (payload) => {
  const response = await fetch(API_ENDPOINTS.CHAT_COMPLETIONS, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'New-Api-User': getUserIdFromLocalStorage(),
    },
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    let errorBody = '';
    try {
      errorBody = await response.text();
    } catch {
      errorBody = '';
    }
    throw new Error(`HTTP error! status: ${response.status}, body: ${errorBody}`);
  }

  return response;
};

const postOpenAIImagePayload = async (payload, selectedGroup) => {
  const query = selectedGroup
    ? `?group=${encodeURIComponent(selectedGroup)}`
    : '';
  const response = await fetch(`${API_ENDPOINTS.IMAGE_GENERATIONS}${query}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'New-Api-User': getUserIdFromLocalStorage(),
    },
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    let errorBody = '';
    try {
      errorBody = await response.text();
    } catch {
      errorBody = '';
    }
    throw new Error(`HTTP error! status: ${response.status}, body: ${errorBody}`);
  }

  return response;
};

const postOpenAIImageEditPayload = async ({ model, prompt, images, selectedGroup }) => {
  const query = selectedGroup
    ? `?group=${encodeURIComponent(selectedGroup)}`
    : '';
  const formData = new FormData();
  formData.append('model', model);
  formData.append('prompt', prompt);
  formData.append('n', '1');

  images.filter((image) => image?.file).forEach((image, index) => {
    formData.append(index === 0 ? 'image' : 'image[]', image.file, image.name || image.file.name);
  });

  const response = await fetch(`${API_ENDPOINTS.IMAGE_EDITS}${query}`, {
    method: 'POST',
    headers: {
      'New-Api-User': getUserIdFromLocalStorage(),
    },
    body: formData,
  });

  if (!response.ok) {
    let errorBody = '';
    try {
      errorBody = await response.text();
    } catch {
      errorBody = '';
    }
    throw new Error(`HTTP error! status: ${response.status}, body: ${errorBody}`);
  }

  return response;
};

const GalleryImage = ({ src, alt, active, onClick }) => (
  <button
    type='button'
    onClick={onClick}
    className={`overflow-hidden rounded-[22px] border transition ${
      active
        ? 'border-sky-400 shadow-[0_18px_44px_rgba(59,130,246,0.18)]'
        : 'border-white/70 hover:border-slate-300'
    }`}
  >
    <img src={src} alt={alt} className='h-24 w-24 object-cover sm:h-28 sm:w-28' />
  </button>
);

const openImageInNewTab = (imageUrl) => {
  if (!imageUrl) return;
  window.open(imageUrl, '_blank', 'noopener,noreferrer');
};

const createImageUserMessage = (prompt, draftImages = []) => ({
  role: MESSAGE_ROLES.USER,
  content: [
    { type: 'text', text: prompt },
    ...draftImages.map((image) => ({
      type: 'image_url',
      image_url: { url: image.previewUrl },
    })),
  ],
  createAt: Date.now(),
  id: `user-${Date.now()}-${Math.random().toString(16).slice(2)}`,
});

const AIImage = () => {
  const { t } = useTranslation();
  const {
    ready,
    storageError,
    messages,
    setMessages,
    groups,
    models,
    textModels,
    selectedModel,
    selectedGroup,
    setSelectedModel,
    setSelectedGroup,
    promptOptimizerModel,
    setPromptOptimizerModel,
    draftImages,
    setDraftImages,
    clearCurrentSession,
    markSessionActivity,
  } = useAiImageState();

  const [prompt, setPrompt] = useState('');
  const [isGenerating, setIsGenerating] = useState(false);
  const [activeRecordId, setActiveRecordId] = useState(null);
  const [resolution, setResolution] = useState('1K');
  const [aspectRatio, setAspectRatio] = useState('1:1');
  const [batchCount, setBatchCount] = useState(1);
  const [isOptimizingPrompt, setIsOptimizingPrompt] = useState(false);
  const [optimizedPromptDraft, setOptimizedPromptDraft] = useState('');
  const [originalPrompt, setOriginalPrompt] = useState('');
  const [activeImageIndex, setActiveImageIndex] = useState(0);
  const [remoteTasks, setRemoteTasks] = useState([]);
  const previousDraftImagesRef = useRef([]);
  const selectedModelRef = useRef('');
  const promptOptimizerModelRef = useRef('');

  const groupOptions = groups.map((group) => ({
    value: group.value,
    label: group.fullLabel || group.label,
  }));

  const modelOptions = models.map((model) => ({
    value: model.value,
    label: model.label,
  }));

  const textModelOptions = textModels.map((model) => ({
    value: model.value,
    label: model.label,
  }));
  const fallbackImageModel = selectedModel || modelOptions[0]?.value || '';

  React.useEffect(() => {
    selectedModelRef.current = selectedModel || '';
  }, [selectedModel]);

  React.useEffect(() => {
    promptOptimizerModelRef.current = promptOptimizerModel || '';
  }, [promptOptimizerModel]);

  const localRecords = useMemo(() => {
    const nextRecords = [];
    for (let index = 0; index < messages.length; index += 1) {
      const current = messages[index];
      const next = messages[index + 1];
      if (current?.role !== MESSAGE_ROLES.USER || next?.role !== MESSAGE_ROLES.ASSISTANT) {
        continue;
      }
      const record = extractGenerationRecord(current, next);
      if (record) nextRecords.push(record);
    }
    return nextRecords.reverse();
  }, [messages]);

  const records = useMemo(() => {
    const remoteRecords = remoteTasks
      .map((task) => taskToGenerationRecord(task))
      .filter(Boolean);
    return [...remoteRecords, ...localRecords]
      .sort((left, right) => (right.createdAt || 0) - (left.createdAt || 0))
      .slice(0, MAX_HISTORY_RECORDS);
  }, [localRecords, remoteTasks]);

  const activeRecord =
    records.find((record) => record.id === activeRecordId) || records[0] || null;
  const activeImages = activeRecord?.images || [];
  const activeImage =
    activeImages[activeImageIndex] || activeImages[0] || '';

  React.useEffect(() => {
    const previousImages = previousDraftImagesRef.current;
    const currentIds = new Set(draftImages.map((image) => image.id));
    previousImages
      .filter((image) => !currentIds.has(image.id))
      .forEach(revokeDraftImage);
    previousDraftImagesRef.current = draftImages;
  }, [draftImages]);

  React.useEffect(
    () => () => {
      revokeDraftImages(previousDraftImagesRef.current);
    },
    [],
  );

  React.useEffect(() => {
    if (!activeRecordId && records.length > 0) {
      setActiveRecordId(records[0].id);
    } else if (
      activeRecordId &&
      !records.some((record) => record.id === activeRecordId)
    ) {
      setActiveRecordId(records[0]?.id || null);
    }
  }, [activeRecordId, records]);

  React.useEffect(() => {
    if (!activeRecord) {
      setActiveImageIndex(0);
      return;
    }
    if (activeImageIndex >= activeImages.length) {
      setActiveImageIndex(0);
    }
  }, [activeImageIndex, activeImages.length, activeRecord]);

  const handleAddDraftImage = React.useCallback(
    (files) => {
      const nextFiles = Array.from(files || []).filter(Boolean);
      if (nextFiles.length === 0) return;
      setDraftImages((previous) => [
        ...previous,
        ...nextFiles.map(createDraftImageEntry),
      ]);
    },
    [setDraftImages],
  );

  const handleRemoveDraftImage = React.useCallback(
    (index) => {
      setDraftImages((previous) =>
        previous.filter((_, itemIndex) => itemIndex !== index),
      );
    },
    [setDraftImages],
  );

  const clearDraftImages = React.useCallback(() => {
    setDraftImages([]);
  }, [setDraftImages]);

  const serializeDraftImagesForRequest = React.useCallback(async () => {
    const images = draftImages.filter(Boolean);
    if (images.length === 0) {
      return [];
    }
    return Promise.all(
      images.map(async (image) => {
        if (
          typeof image?.sourceDataUrl === 'string' &&
          image.sourceDataUrl.startsWith('data:image/')
        ) {
          return image.sourceDataUrl;
        }
        if (image?.file) {
          return readFileAsDataUrl(image.file);
        }
        return '';
      }),
    );
  }, [draftImages]);

  const loadRemoteTasks = React.useCallback(async () => {
    const response = await fetch('/api/ai-image/tasks?p=1&page_size=30', {
      headers: {
        Accept: 'application/json',
        'New-Api-User': getUserIdFromLocalStorage(),
      },
    });
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    const result = await response.json();
    if (!result?.success) {
      throw new Error(result?.message || 'failed to load image tasks');
    }
    setRemoteTasks(Array.isArray(result?.data?.items) ? result.data.items : []);
  }, []);

  React.useEffect(() => {
    if (!ready) {
      return undefined;
    }
    let cancelled = false;
    const refresh = async () => {
      try {
        if (!cancelled) {
          await loadRemoteTasks();
        }
      } catch {
        // Ignore background polling failures.
      }
    };
    refresh();
    const timer = window.setInterval(() => {
      if (document.visibilityState === 'visible') {
        refresh();
      }
    }, 3000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [loadRemoteTasks, ready]);

  const submitImageTask = React.useCallback(
    async (itemPrompt) => {
      const effectiveModel = selectedModelRef.current || modelOptions[0]?.value || '';
      if (!effectiveModel) {
        throw new Error('image model is required');
      }
      const response = await fetch('/api/ai-image/tasks', {
        method: 'POST',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
          'New-Api-User': getUserIdFromLocalStorage(),
        },
        body: JSON.stringify({
          model: effectiveModel,
          prompt: itemPrompt,
          group: selectedGroup,
          size: getOpenAIImageSize(aspectRatio),
          n: 1,
        }),
      });
      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(`HTTP error! status: ${response.status}, body: ${errorBody}`);
      }
      const result = await response.json();
      if (!result?.success) {
        throw new Error(result?.message || 'submit image task failed');
      }
      return result.data;
    },
    [aspectRatio, modelOptions, selectedGroup],
  );

  const requestOneImage = React.useCallback(
    async (itemPrompt) => {
      const effectiveModel = selectedModelRef.current || modelOptions[0]?.value || '';
      if (!effectiveModel) {
        throw new Error('image model is required');
      }

      if (isOpenAIImageModel(effectiveModel)) {
        const response = draftImages.length > 0
          ? await postOpenAIImageEditPayload({
              model: effectiveModel,
              prompt: itemPrompt,
              images: draftImages,
              selectedGroup,
            })
          : await postOpenAIImagePayload(
              {
                model: effectiveModel,
                prompt: itemPrompt,
                size: getOpenAIImageSize(aspectRatio),
                n: 1,
              },
              selectedGroup,
            );
        return parseImageResponse(response);
      }

      const imageUrls = await serializeDraftImagesForRequest();
      const payload = buildGeminiNativeImagePayload({
        prompt: itemPrompt,
        imageUrls,
        resolution,
        aspectRatio,
      });

      const query = selectedGroup
        ? `?group=${encodeURIComponent(selectedGroup)}`
        : '';
      const response = await fetch(
        `${API_ENDPOINTS.GEMINI_NATIVE_MODELS}/${encodeURIComponent(
          effectiveModel,
        )}:generateContent${query}`,
        {
          method: 'POST',
          headers: {
            'Accept': 'application/json',
            'Content-Type': 'application/json',
            'New-Api-User': getUserIdFromLocalStorage(),
          },
          body: JSON.stringify(payload),
        },
      );

      if (!response.ok) {
        let errorBody = '';
        try {
          errorBody = await response.text();
        } catch {
          errorBody = '';
        }
        throw new Error(`HTTP error! status: ${response.status}, body: ${errorBody}`);
      }

      return parseImageResponse(response);
    },
    [aspectRatio, draftImages, modelOptions, resolution, selectedGroup, serializeDraftImagesForRequest],
  );

  const appendGeneration = React.useCallback(
    (items) => {
      const createdMessages = items.flatMap(({ itemPrompt, imageUrls }) => [
        createImageUserMessage(itemPrompt, draftImages),
        createImageAssistantMessage(imageUrls, itemPrompt),
      ]);

      if (createdMessages.length === 0) return;
      markSessionActivity();
      setMessages((previous) => trimMessagesToHistoryLimit([...previous, ...createdMessages]));
      const lastAssistant = [...createdMessages]
        .reverse()
        .find((message) => message.role === MESSAGE_ROLES.ASSISTANT);
      setActiveRecordId(lastAssistant?.id || null);
      setActiveImageIndex(0);
    },
    [draftImages, markSessionActivity, setMessages],
  );

  const appendBatchGeneration = React.useCallback(
    (tasks, imageUrls) => {
      if (!Array.isArray(imageUrls) || imageUrls.length === 0) return;

      const combinedPrompt = tasks.join('\n').trim();
      const userMessage = createImageUserMessage(combinedPrompt, draftImages);
      const assistantMessage = createImageAssistantMessage(
        imageUrls,
        combinedPrompt,
      );

      markSessionActivity();
      setMessages((previous) =>
        trimMessagesToHistoryLimit([...previous, userMessage, assistantMessage]),
      );
      setActiveRecordId(assistantMessage.id);
      setActiveImageIndex(0);
    },
    [draftImages, markSessionActivity, setMessages],
  );

  const handleGenerate = React.useCallback(async () => {
    const trimmedPrompt = prompt.trim();
    if (!trimmedPrompt) {
      showError(t('请输入提示词'));
      return;
    }
    if (!fallbackImageModel) {
      showError(t('请选择模型'));
      return;
    }
    if (!selectedModelRef.current && fallbackImageModel) {
      selectedModelRef.current = fallbackImageModel;
      setSelectedModel(fallbackImageModel);
    }

    setIsGenerating(true);
    try {
      if (isOpenAIImageModel(fallbackImageModel)) {
        await submitImageTask(trimmedPrompt);
        await loadRemoteTasks();
        setPrompt('');
        setOptimizedPromptDraft('');
        setOriginalPrompt('');
        clearDraftImages();
        showSuccess(t('已加入生成队列'));
        return;
      }
      const parsed = await requestOneImage(trimmedPrompt);
      if (parsed.imageUrls.length === 0) {
        showError(t('未返回可展示的图片，请检查模型返回格式'));
        return;
      }
      appendGeneration([{ itemPrompt: trimmedPrompt, imageUrls: parsed.imageUrls }]);
      setPrompt('');
      setOptimizedPromptDraft('');
      setOriginalPrompt('');
      clearDraftImages();
      showSuccess(t('图片生成成功'));
    } catch (error) {
      const errorInfo = handleApiError(error);
      showError(errorInfo.error || t('请求发生错误'));
    } finally {
      setIsGenerating(false);
    }
  }, [
    appendGeneration,
    clearDraftImages,
    fallbackImageModel,
    loadRemoteTasks,
    prompt,
    requestOneImage,
    setSelectedModel,
    submitImageTask,
    t,
  ]);

  const handleBatchGenerate = React.useCallback(async () => {
    const prompts = prompt
      .split('\n')
      .map((item) => item.trim())
      .filter(Boolean);

    if (!fallbackImageModel) {
      showError(t('请选择模型'));
      return;
    }
    if (!selectedModelRef.current && fallbackImageModel) {
      selectedModelRef.current = fallbackImageModel;
      setSelectedModel(fallbackImageModel);
    }
    if (prompts.length === 0) {
      showError(t('请输入提示词'));
      return;
    }

    const tasks = Array.from({ length: batchCount }, (_, index) =>
      prompts.length > 1 ? prompts[index % prompts.length] : prompts[0],
    );

    setIsGenerating(true);
    try {
      if (isOpenAIImageModel(fallbackImageModel)) {
        await Promise.all(tasks.map((itemPrompt) => submitImageTask(itemPrompt)));
        await loadRemoteTasks();
        setPrompt('');
        setOptimizedPromptDraft('');
        setOriginalPrompt('');
        clearDraftImages();
        showSuccess(t('批量任务已提交'));
        return;
      }
      const results = await Promise.all(
        tasks.map(async (itemPrompt) => {
          const parsed = await requestOneImage(itemPrompt);
          return { itemPrompt, imageUrls: parsed.imageUrls };
        }),
      );
      const validResults = results.filter((item) => item.imageUrls.length > 0);
      if (validResults.length === 0) {
        showError(t('未返回可展示的图片，请检查模型返回格式'));
        return;
      }
      appendBatchGeneration(
        tasks,
        results.flatMap((item) => item.imageUrls || []),
      );
      setPrompt('');
      setOptimizedPromptDraft('');
      setOriginalPrompt('');
      clearDraftImages();
      showSuccess(t('批量生成完成'));
    } catch (error) {
      const errorInfo = handleApiError(error);
      showError(errorInfo.error || t('请求发生错误'));
    } finally {
      setIsGenerating(false);
    }
  }, [
    appendBatchGeneration,
    batchCount,
    clearDraftImages,
    fallbackImageModel,
    loadRemoteTasks,
    prompt,
    requestOneImage,
    setSelectedModel,
    submitImageTask,
    t,
  ]);

  const handleClearHistory = React.useCallback(() => {
    clearCurrentSession();
    setActiveRecordId(null);
    setPrompt('');
    setOptimizedPromptDraft('');
    setOriginalPrompt('');
    showSuccess(t('已清空最近记录'));
  }, [clearCurrentSession, t]);

  const handleCopyPrompt = React.useCallback(
    async (record) => {
      if (!record?.prompt) {
        showError(t('没有可复制的提示词'));
        return;
      }

      const ok = await copy(record.prompt);
      if (ok) {
        showSuccess(t('提示词已复制'));
      } else {
        showError(t('复制失败，请手动复制'));
      }
    },
    [t],
  );

  const handleDownloadImage = React.useCallback(
    (record) => {
      const imageUrl = record?.images?.[0];
      if (!imageUrl) {
        showError(t('没有可下载的图片'));
        return;
      }

      const anchor = document.createElement('a');
      anchor.href = imageUrl;
      anchor.download = `ai-image-${record.id || Date.now()}.png`;
      document.body.appendChild(anchor);
      anchor.click();
      document.body.removeChild(anchor);
      showSuccess(t('开始下载图片'));
    },
    [t],
  );

  const handleDeleteRecord = React.useCallback(
    (record) => {
      if (!record) return;

      setMessages((previous) =>
        previous.filter(
          (message) =>
            message.id !== record.userMessageId &&
            message.id !== record.assistantMessageId,
        ),
      );

      if (activeRecordId === record.id) {
        setActiveRecordId(null);
      }

      showSuccess(t('已从本地删除该记录'));
    },
    [activeRecordId, setMessages, t],
  );

  const handleReuseImage = React.useCallback(
    async (imageUrl) => {
      if (!imageUrl) {
        showError(t('没有可引用的图片'));
        return;
      }

      try {
        if (typeof imageUrl === 'string' && imageUrl.startsWith('data:image/')) {
          const [meta, base64Data] = imageUrl.split(',', 2);
          const mimeType =
            meta.match(/^data:(.+?);base64$/)?.[1] || 'image/png';
          const binary = atob(base64Data || '');
          const bytes = new Uint8Array(binary.length);
          for (let index = 0; index < binary.length; index += 1) {
            bytes[index] = binary.charCodeAt(index);
          }
          const extension = mimeType.split('/')[1] || 'png';
          const file = new File(
            [bytes],
            `referenced-${Date.now()}.${extension}`,
            { type: mimeType },
          );
          setDraftImages((previous) => [
            ...previous,
            createDraftImageEntry(file, { sourceDataUrl: imageUrl }),
          ]);
          showSuccess(t('宸插紩鐢ㄥ埌鍙傝€冨浘'));
          return;
        }

        const response = await fetch(imageUrl);
        const blob = await response.blob();
        const extension = blob.type?.split('/')[1] || 'png';
        const file = new File([blob], `referenced-${Date.now()}.${extension}`, {
          type: blob.type || 'image/png',
        });
        setDraftImages((previous) => [...previous, createDraftImageEntry(file)]);
        showSuccess(t('已引用到参考图'));
      } catch (error) {
        const errorInfo = handleApiError(error);
        showError(errorInfo.error || t('引用图片失败'));
      }
    },
    [setDraftImages, t],
  );

  const handleOptimizePrompt = React.useCallback(async () => {
    const trimmedPrompt = prompt.trim();
    if (!trimmedPrompt) {
      showError(t('请先输入提示词'));
      return;
    }
    const effectivePromptOptimizerModel =
      promptOptimizerModelRef.current || promptOptimizerModel;
    if (!effectivePromptOptimizerModel) {
      showError(t('当前没有可用于优化提示词的文本模型'));
      return;
    }

    setIsOptimizingPrompt(true);
    try {
      const payload = buildApiPayload(
        [
          {
            role: MESSAGE_ROLES.USER,
            content: buildImageContent(
              trimmedPrompt,
              await serializeDraftImagesForRequest(),
            ),
          },
        ],
        PROMPT_OPTIMIZER_SYSTEM_PROMPT,
        {
          model: effectivePromptOptimizerModel,
          group: selectedGroup || '',
          stream: false,
        },
        {},
      );
      payload.model = effectivePromptOptimizerModel;
      const response = await postJsonPayload(payload);
      const data = await response.json();
      const content = data?.choices?.[0]?.message?.content || '';
      const nextPrompt = extractOptimizedPrompt(content);
      if (!nextPrompt) {
        showError(t('未获取到有效的优化结果，请切换文本模型后重试'));
        return;
      }

      setOriginalPrompt(trimmedPrompt);
      setOptimizedPromptDraft(nextPrompt);
      setPrompt(nextPrompt);
    } catch (error) {
      const errorInfo = handleApiError(error);
      showError(errorInfo.error || t('优化提示词失败'));
    } finally {
      setIsOptimizingPrompt(false);
    }
  }, [prompt, promptOptimizerModel, selectedGroup, serializeDraftImagesForRequest, t]);

  const handleRevertOptimizedPrompt = React.useCallback(() => {
    setPrompt(originalPrompt);
    setOptimizedPromptDraft('');
    setOriginalPrompt('');
  }, [originalPrompt]);

  const handleConfirmOptimizedPrompt = React.useCallback(() => {
    if (optimizedPromptDraft) {
      setPrompt(optimizedPromptDraft);
    }
    setOptimizedPromptDraft('');
    setOriginalPrompt('');
    showSuccess(t('已采用优化后的提示词'));
  }, [optimizedPromptDraft, t]);

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
          title={t('AI 绘图初始化失败')}
          description={t('当前浏览器可能禁用了本地存储，请稍后重试')}
        />
      </div>
    );
  }

  return (
    <div className='mt-[64px] min-h-[calc(100vh-64px)] bg-[#eef2f7] px-2 py-3 sm:px-3 lg:px-4'>
      <div className='mx-auto flex min-h-[calc(100vh-88px)] w-full max-w-none flex-col gap-4'>
        <section className='rounded-[32px] border border-white/70 bg-white/75 p-4 shadow-[0_24px_80px_rgba(15,23,42,0.08)] backdrop-blur-xl sm:p-5'>
          <div className='flex min-h-[60vh] flex-col gap-4 xl:flex-row'>
            <div className='flex min-w-0 flex-1 flex-col gap-4'>
              <div className='flex items-center gap-3'>
                <div className='flex h-12 w-12 items-center justify-center rounded-[18px] bg-sky-50 text-sky-600 shadow-[0_16px_36px_rgba(15,23,42,0.08)]'>
                  <ImagePlus size={22} />
                </div>
                <div>
                  <Typography.Title heading={4} className='!mb-0 !text-slate-900'>
                    {t('AI 绘图')}
                  </Typography.Title>
                  <Typography.Text className='!text-sm !text-slate-500'>
                    {t('选择 Google 图片模型，输入提示词后直接查看生成结果')}
                  </Typography.Text>
                </div>
              </div>

              <div className='grid gap-3 md:grid-cols-2'>
                <label className='flex flex-col gap-2'>
                  <span className='text-xs font-medium uppercase tracking-[0.18em] text-slate-400'>
                    {t('分组')}
                  </span>
                  <select
                    value={selectedGroup}
                    onChange={(event) => setSelectedGroup(event.target.value)}
                    className='h-11 rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'
                  >
                    {groupOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                </label>

                <label className='flex flex-col gap-2'>
                  <span className='text-xs font-medium uppercase tracking-[0.18em] text-slate-400'>
                      {t('绘图模型')}
                  </span>
                  <select
                    value={selectedModel}
                    onChange={(event) => {
                      selectedModelRef.current = event.target.value;
                      setSelectedModel(event.target.value);
                    }}
                    className='h-11 rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'
                  >
                    <option value=''>{t('选择模型')}</option>
                    {modelOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                </label>
              </div>

              <div className='rounded-[28px] border border-slate-200 bg-slate-50/70 p-4'>
                <div className='mb-3 grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
                  <label className='flex flex-col gap-2'>
                    <span className='text-xs font-medium uppercase tracking-[0.18em] text-slate-400'>
                      {t('分辨率')}
                    </span>
                    <select
                      value={resolution}
                      onChange={(event) => setResolution(event.target.value)}
                      className='h-11 rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'
                    >
                      {RESOLUTION_OPTIONS.map((item) => (
                        <option key={item} value={item}>
                          {item}
                        </option>
                      ))}
                    </select>
                  </label>

                  <label className='flex flex-col gap-2'>
                    <span className='text-xs font-medium uppercase tracking-[0.18em] text-slate-400'>
                      {t('图像比例')}
                    </span>
                    <select
                      value={aspectRatio}
                      onChange={(event) => setAspectRatio(event.target.value)}
                      className='h-11 rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'
                    >
                      {ASPECT_RATIO_OPTIONS.map((item) => (
                        <option key={item} value={item}>
                          {item}
                        </option>
                      ))}
                    </select>
                  </label>

                  <label className='flex flex-col gap-2'>
                    <span className='text-xs font-medium uppercase tracking-[0.18em] text-slate-400'>
                      {t('批量张数')}
                    </span>
                    <select
                      value={batchCount}
                      onChange={(event) => setBatchCount(Number(event.target.value))}
                      className='h-11 rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'
                    >
                      {BATCH_COUNT_OPTIONS.map((item) => (
                        <option key={item} value={item}>
                          {item}
                        </option>
                      ))}
                    </select>
                  </label>
                </div>

                <TextArea
                  value={prompt}
                  onChange={(value) => {
                    setPrompt(value);
                    if (optimizedPromptDraft) {
                      setOptimizedPromptDraft('');
                      setOriginalPrompt('');
                    }
                  }}
                  autosize={{ minRows: 7, maxRows: 14 }}
                  placeholder={t('描述你想生成的图片；批量生成时可一行一个提示词')}
                  className='!bg-transparent'
                />

                <Typography.Text className='mt-2 block text-xs text-slate-500'>
                  {promptOptimizerModel
                    ? t('提示词优化当前使用模型：{{model}}', {
                        model: promptOptimizerModel,
                      })
                    : t('提示词优化当前未找到可用文本模型')}
                </Typography.Text>

                <div className='mt-2 max-w-[360px]'>
                  <select
                    value={promptOptimizerModel}
                    onChange={(event) => {
                      promptOptimizerModelRef.current = event.target.value;
                      setPromptOptimizerModel(event.target.value);
                    }}
                    className='h-10 w-full rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'
                  >
                    <option value=''>{t('选择提示词优化模型')}</option>
                    {textModelOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                </div>

                {optimizedPromptDraft && (
                  <div className='mt-3 flex flex-wrap items-center gap-2 rounded-[18px] border border-emerald-100 bg-emerald-50/80 px-3 py-2'>
                    <Typography.Text className='!text-sm !text-emerald-700'>
                      {t('已生成优化后的提示词，可恢复原文或确认采用')}
                    </Typography.Text>
                    <Button
                      theme='light'
                      type='tertiary'
                      className='!rounded-full'
                      onClick={handleRevertOptimizedPrompt}
                    >
                      {t('恢复原提示词')}
                    </Button>
                    <Button
                      theme='solid'
                      type='primary'
                      icon={<Check size={16} />}
                      className='!rounded-full'
                      onClick={handleConfirmOptimizedPrompt}
                    >
                      {t('采用优化结果')}
                    </Button>
                  </div>
                )}

                {draftImages.length > 0 && (
                  <div className='mt-3 flex flex-wrap gap-2'>
                    {draftImages.map((image, index) => (
                      <div
                        key={image.id || `${index}-${image.previewUrl}`}
                        className='group relative overflow-hidden rounded-[18px] border border-white/70 bg-white'
                      >
                        <img
                          src={image.previewUrl}
                          alt={`${t('参考图')} ${index + 1}`}
                          className='h-24 w-24 object-cover'
                        />
                        <button
                          type='button'
                          onClick={() => handleRemoveDraftImage(index)}
                          className='absolute right-1.5 top-1.5 flex h-6 w-6 items-center justify-center rounded-full bg-black/70 text-white'
                        >
                          <X size={12} />
                        </button>
                      </div>
                    ))}
                  </div>
                )}

                <div className='mt-3 flex flex-wrap items-center gap-3'>
                  <label className='inline-flex cursor-pointer items-center gap-2 rounded-full border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm'>
                    <ImagePlus size={16} />
                    <span>{t('上传参考图')}</span>
                    <input
                      type='file'
                      accept='image/*'
                      multiple
                      className='hidden'
                      onChange={(event) => {
                        const files = Array.from(event.target.files || []);
                        event.target.value = '';
                        if (!files.length) return;
                        handleAddDraftImage(files);
                      }}
                    />
                  </label>

                  <Button
                    theme='solid'
                    type='primary'
                    icon={
                      isGenerating ? (
                        <Loader2 size={16} className='animate-spin' />
                      ) : (
                        <Wand2 size={16} />
                      )
                    }
                    loading={isGenerating}
                    onClick={batchCount > 1 ? handleBatchGenerate : handleGenerate}
                    className='!rounded-full'
                  >
                    {t('生成图片')}
                  </Button>

                  <Button
                    theme='light'
                    type='primary'
                    icon={
                      isOptimizingPrompt ? (
                        <Loader2 size={16} className='animate-spin' />
                      ) : (
                        <Sparkles size={16} />
                      )
                    }
                    loading={isOptimizingPrompt}
                    disabled={isGenerating}
                    onClick={handleOptimizePrompt}
                    className='!rounded-full'
                  >
                    {t('优化提示词')}
                  </Button>

                  <Button
                    theme='light'
                    type='tertiary'
                    disabled={isGenerating}
                    onClick={handleBatchGenerate}
                    className='!rounded-full'
                  >
                    {t('批量生成')}
                  </Button>

                  <Button
                    theme='borderless'
                    type='tertiary'
                    disabled={isGenerating || records.length === 0}
                    onClick={handleClearHistory}
                    className='!rounded-full'
                  >
                    {t('清空记录')}
                  </Button>
                </div>

                <Typography.Text className='mt-3 block text-xs text-slate-500'>
                  {t(
                    '图片仅保存在当前浏览器本地，删除浏览器缓存或记录后将会消失，请及时保存。',
                  )}
                </Typography.Text>
              </div>
            </div>

            <div className='w-full xl:w-[52%]'>
              <div className='flex h-full min-h-[70vh] flex-col rounded-[28px] border border-white/70 bg-white shadow-[0_18px_50px_rgba(15,23,42,0.08)]'>
                <div className='border-b border-slate-100 px-4 py-3'>
                  <Typography.Text className='!text-xs !font-medium !uppercase !tracking-[0.2em] !text-slate-400'>
                    {t('当前结果')}
                  </Typography.Text>
                </div>
                <div className='flex flex-1 items-center justify-center p-4'>
                  {activeImage ? (
                    <div className='flex h-full w-full flex-col'>
                      <button
                        type='button'
                        onClick={() => openImageInNewTab(activeImage)}
                        className='mx-auto flex h-[420px] w-full max-w-[680px] items-center justify-center overflow-hidden rounded-[24px] bg-slate-50 shadow-[0_20px_60px_rgba(15,23,42,0.12)]'
                        title={t('点击查看大图')}
                      >
                        <img
                          src={activeImage}
                          alt={activeRecord?.prompt || 'generated image'}
                          className='h-full w-full object-contain'
                        />
                      </button>
                      {activeRecord?.prompt ? (
                        <p className='mt-3 text-sm text-slate-500'>
                          {activeRecord.prompt}
                        </p>
                      ) : null}
                      {activeImages.length > 1 ? (
                        <div className='mt-3 flex flex-wrap gap-2'>
                          {activeImages.map((imageUrl, index) => (
                            <button
                              key={`${activeRecord.id}-${index}`}
                              type='button'
                              onClick={() => setActiveImageIndex(index)}
                              className={`overflow-hidden rounded-2xl border transition ${
                                index === activeImageIndex
                                  ? 'border-sky-400 shadow-[0_10px_24px_rgba(59,130,246,0.18)]'
                                  : 'border-slate-200 hover:border-slate-300'
                              }`}
                            >
                              <img
                                src={imageUrl}
                                alt={`${activeRecord.prompt || 'generated image'} ${index + 1}`}
                                className='h-16 w-16 object-cover sm:h-20 sm:w-20'
                              />
                            </button>
                          ))}
                        </div>
                      ) : null}
                      <div className='mt-3 flex flex-wrap gap-2'>
                        <Button
                          theme='light'
                          type='primary'
                          className='!rounded-full'
                          onClick={() => handleReuseImage(activeImage)}
                        >
                          {t('引用为参考图')}
                        </Button>
                      </div>
                    </div>
                  ) : (
                    <Empty
                      image={<ImagePlus size={40} className='text-slate-400' />}
                      title={t('还没有生成图片')}
                      description={t('输入提示词后，生成结果会直接显示在这里')}
                    />
                  )}
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className='rounded-[32px] border border-white/70 bg-white/75 p-4 shadow-[0_24px_80px_rgba(15,23,42,0.08)] backdrop-blur-xl sm:p-5'>
          <div className='mb-4 flex items-center justify-between gap-3'>
            <div>
              <Typography.Title heading={6} className='!mb-0 !text-slate-900'>
                {t('最近生成')}
              </Typography.Title>
              <Typography.Text className='!ml-2 !text-sm !font-medium !text-red-600'>
                {t('（当前部分模型的引用功能和下载功能正在开发中，建议使用鼠标右键保存或长按保存）')}
              </Typography.Text>
              <Typography.Text className='!text-sm !text-slate-500'>
                {t('点击下方缩略图即可切换查看历史结果')}
              </Typography.Text>
            </div>
            <Typography.Text className='!text-sm !text-slate-400'>
              {t('{{count}} 条记录', { count: records.length })}
            </Typography.Text>
          </div>

          {records.length > 0 ? (
            <div className='flex gap-3 overflow-x-auto pb-1'>
              {records.map((record) => (
                <div key={record.id} className='w-[168px] shrink-0 space-y-2'>
                  <GalleryImage
                    src={record.images[0]}
                    alt={record.prompt || 'generated image'}
                    active={record.id === activeRecord?.id}
                    onClick={() => {
                      setActiveRecordId(record.id);
                      setActiveImageIndex(0);
                    }}
                  />
                  <div className='w-full px-1'>
                    <button
                      type='button'
                      className='h-12 w-full overflow-hidden rounded-xl px-1 py-1 text-left text-sm leading-5 text-slate-700 transition hover:bg-slate-100/80'
                      onClick={() => handleCopyPrompt(record)}
                      title={record.prompt || t('未命名提示词')}
                    >
                      <span className='block h-10 max-w-full overflow-hidden text-ellipsis break-all line-clamp-2'>
                        {record.prompt || t('未命名提示词')}
                      </span>
                    </button>
                    <div className='mt-2 grid grid-cols-3 gap-1.5'>
                      <Button
                        theme='light'
                        type='tertiary'
                        size='small'
                        className='!h-7 !rounded-full !px-2 !text-xs'
                        onClick={() => handleDownloadImage(record)}
                      >
                        {t('下载')}
                      </Button>
                      <Button
                        theme='light'
                        type='primary'
                        size='small'
                        className='!h-7 !rounded-full !px-2 !text-xs'
                        onClick={() => handleReuseImage(record.images?.[0])}
                      >
                        {t('引用')}
                      </Button>
                      <Button
                        theme='light'
                        type='danger'
                        size='small'
                        className='!h-7 !rounded-full !px-2 !text-xs'
                        onClick={() => handleDeleteRecord(record)}
                      >
                        {t('删除')}
                      </Button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <Empty
              image={<Sparkles size={36} className='text-slate-400' />}
              title={t('暂无历史')}
              description={t('成功生成的图片会保留在这里，方便翻动和重新选择')}
            />
          )}
        </section>
      </div>
    </div>
  );
};

export default AIImage;
