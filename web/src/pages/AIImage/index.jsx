import React, { useMemo, useRef, useState } from 'react';
import { Button, Empty, InputNumber, Modal, Pagination, Spin, TextArea, Typography } from '@douyinfe/semi-ui';
import { Bookmark, Brush, Check, Edit3, ImagePlus, Loader2, RotateCcw, Search, Sparkles, Trash2, Wand2, X } from 'lucide-react';
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
const OPENAI_IMAGE_SIZE_OPTIONS = [
  { value: '1024x1024', label: '1024×1024 (1:1)' },
  { value: '1536x1024', label: '1536×1024 (3:2)' },
  { value: '1024x1536', label: '1024×1536 (2:3)' },
  { value: '2048x2048', label: '2048×2048 (1:1 2K)' },
  { value: '2048x1152', label: '2048×1152 (16:9)' },
  { value: '3840x2160', label: '3840×2160 (16:9)' },
  { value: '2160x3840', label: '2160×3840 (9:16)' },
];
const QUICK_BATCH_COUNT_OPTIONS = [1, 2, 4];
const MAX_BATCH_COUNT = 10;
const MAX_PROMPT_LENGTH = 4000;
const MAX_REFERENCE_IMAGES = 5;
const REMOTE_TASK_PAGE_SIZE = 10;
const TASKS_CACHE_KEY = 'ai_image_tasks_cache';
const PROMPT_FAVORITES_PAGE_SIZE = 80;
const PROMPT_FAVORITES_ENDPOINT = '/api/ai-image/prompt-favorites';
const PROMPT_OPTIMIZER_SYSTEM_PROMPT = [
  'You are an AI image prompt optimizer.',
  'Rewrite the user prompt into one stronger image-generation prompt in Simplified Chinese.',
  'Keep the meaning, add useful visual detail, and make it directly usable for image generation.',
  'Return plain text only. Do not use markdown, JSON, lists, or explanations.',
].join('\n');
const ANNOTATION_CANVAS_MAX_SIDE = 1400;
const ANNOTATION_COLORS = [
  { value: '#ef4444', label: '\u7ea2\u8272' },
  { value: '#2563eb', label: '\u84dd\u8272' },
  { value: '#16a34a', label: '\u7eff\u8272' },
  { value: '#f59e0b', label: '\u9ec4\u8272' },
  { value: '#8b5cf6', label: '\u7d2b\u8272' },
  { value: '#f97316', label: '\u6a59\u8272' },
];

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

const createAnnotationId = () =>
  `annotation-${Date.now()}-${Math.random().toString(16).slice(2)}`;

const getAnnotationColorLabel = (color) =>
  ANNOTATION_COLORS.find((item) => item.value === color)?.label || '\u6807\u6ce8';

const getAnnotationLabel = (annotation, index) =>
  annotation?.label || `${getAnnotationColorLabel(annotation?.color)}\u533a\u57df ${index + 1}`;

const getImageAnnotationCount = (image) =>
  Array.isArray(image?.annotations)
    ? image.annotations.filter((item) => item?.prompt?.trim()).length
    : 0;

const buildPromptWithAnnotations = (basePrompt, images = []) => {
  const annotationLines = images.flatMap((image, imageIndex) =>
    (Array.isArray(image?.annotations) ? image.annotations : [])
      .map((annotation, annotationIndex) => ({
        label: getAnnotationLabel(annotation, annotationIndex),
        prompt: String(annotation?.prompt || '').trim(),
        imageIndex,
      }))
      .filter((item) => item.prompt)
      .map((item) => `\u53c2\u8003\u56fe ${item.imageIndex + 1} ${item.label}\uff1a${item.prompt}`),
  );

  if (annotationLines.length === 0) {
    return basePrompt;
  }

  return [
    basePrompt,
    '',
    '\u5c40\u90e8\u4fee\u6539\u8981\u6c42\uff1a',
    ...annotationLines.map((line, index) => `${index + 1}. ${line}`),
    '\u8bf7\u4f18\u5148\u6839\u636e\u53c2\u8003\u56fe\u4e0a\u7684\u5f69\u8272\u5708\u9009\u533a\u57df\u6267\u884c\u8fd9\u4e9b\u5c40\u90e8\u4fee\u6539\u3002',
  ].join('\n');
};

const getAnnotatedPromptCount = (images = []) =>
  images.reduce((count, image) => count + getImageAnnotationCount(image), 0);

const dataUrlToFile = (dataUrl, filename = `annotated-${Date.now()}.png`) => {
  const [meta, base64Data] = String(dataUrl || '').split(',', 2);
  const mimeType = meta.match(/^data:(.+?);base64$/)?.[1] || 'image/png';
  const binary = atob(base64Data || '');
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return new File([bytes], filename, { type: mimeType });
};

const blobToDataUrl = (blob) =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '');
    reader.onerror = () => reject(reader.error || new Error('failed to read blob'));
    reader.readAsDataURL(blob);
  });

const createDraftImageFromImageUrl = async (imageUrl, filenamePrefix = 'referenced') => {
  if (imageUrl?.startsWith('data:image/')) {
    const file = dataUrlToFile(imageUrl, `${filenamePrefix}-${Date.now()}.png`);
    return createDraftImageEntry(file, { sourceDataUrl: imageUrl });
  }
  const response = await fetch(imageUrl);
  if (!response.ok) throw new Error(`fetch failed: ${response.status}`);
  const blob = await response.blob();
  const extension = blob.type?.split('/')[1] || 'png';
  const file = new File([blob], `${filenamePrefix}-${Date.now()}.${extension}`, { type: blob.type || 'image/png' });
  const dataUrl = await blobToDataUrl(blob);
  return createDraftImageEntry(file, { sourceDataUrl: dataUrl });
};

const revokeDraftImage = (image) => {
  if (image?.previewUrl?.startsWith('blob:')) {
    URL.revokeObjectURL(image.previewUrl);
  }
};

const revokeDraftImages = (images = []) => {
  images.forEach(revokeDraftImage);
};

const normalizeBatchCount = (value) => {
  const nextValue = Number(value || 1);
  if (!Number.isFinite(nextValue)) return 1;
  return Math.min(MAX_BATCH_COUNT, Math.max(1, Math.trunc(nextValue)));
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
  if (!task) return null;
  if (task.status === 'FAILED') {
    return {
      id: task.task_id || `task-${task.id}`,
      userMessageId: null,
      assistantMessageId: null,
      prompt: task.prompt || '',
      images: [],
      status: 'FAILED',
      errorMessage: task.error_message || '',
      isRemote: true,
      taskId: task.task_id,
      createdAt: task.created_at || Date.now(),
    };
  }
  if (!task?.result_url) return null;
  return {
    id: task.task_id || `task-${task.id}`,
    userMessageId: null,
    assistantMessageId: null,
    prompt: task.prompt || '',
    images: [task.result_url],
    status: task.status,
    isRemote: true,
    taskId: task.task_id,
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

  images.filter((image) => image?.file).slice(0, MAX_REFERENCE_IMAGES).forEach((image, index) => {
    formData.append('image', image.file, image.name || image.file.name);
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
    <img src={src} alt={alt} loading='lazy' decoding='async' className='h-24 w-24 object-cover sm:h-28 sm:w-28' />
  </button>
);

const ImageAnnotationModal = ({ visible, source, onCancel, onSave }) => {
  const { t } = useTranslation();
  const canvasRef = useRef(null);
  const imageRef = useRef(null);
  const activeAnnotationIdRef = useRef(null);
  const [annotations, setAnnotations] = useState([]);
  const [selectedColor, setSelectedColor] = useState(ANNOTATION_COLORS[0].value);
  const [canvasSize, setCanvasSize] = useState({ width: 900, height: 560 });
  const [isDrawing, setIsDrawing] = useState(false);

  const drawCanvas = React.useCallback(() => {
    const canvas = canvasRef.current;
    const image = imageRef.current;
    if (!canvas || !image) return;
    const ctx = canvas.getContext('2d');
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.fillStyle = '#0f172a';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(image, 0, 0, canvas.width, canvas.height);
    annotations.forEach((annotation, annotationIndex) => {
      const points = Array.isArray(annotation.points) ? annotation.points : [];
      if (points.length < 2) return;
      ctx.save();
      ctx.lineCap = 'round';
      ctx.lineJoin = 'round';
      ctx.lineWidth = 8;
      ctx.strokeStyle = annotation.color || ANNOTATION_COLORS[0].value;
      ctx.shadowColor = 'rgba(15,23,42,0.35)';
      ctx.shadowBlur = 3;
      ctx.beginPath();
      points.forEach((point, pointIndex) => {
        const x = point.x * canvas.width;
        const y = point.y * canvas.height;
        if (pointIndex === 0) {
          ctx.moveTo(x, y);
        } else {
          ctx.lineTo(x, y);
        }
      });
      ctx.stroke();
      const firstPoint = points[0];
      ctx.shadowBlur = 0;
      ctx.fillStyle = annotation.color || ANNOTATION_COLORS[0].value;
      ctx.beginPath();
      ctx.arc(firstPoint.x * canvas.width, firstPoint.y * canvas.height, 16, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = '#ffffff';
      ctx.font = 'bold 15px sans-serif';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(String(annotationIndex + 1), firstPoint.x * canvas.width, firstPoint.y * canvas.height);
      ctx.restore();
    });
  }, [annotations]);

  React.useEffect(() => {
    if (!visible || !source?.image?.previewUrl) return undefined;
    const image = new Image();
    image.onload = () => {
      const ratio = Math.min(
        ANNOTATION_CANVAS_MAX_SIDE / image.naturalWidth,
        ANNOTATION_CANVAS_MAX_SIDE / image.naturalHeight,
        1,
      );
      const width = Math.max(320, Math.round(image.naturalWidth * ratio));
      const height = Math.max(240, Math.round(image.naturalHeight * ratio));
      imageRef.current = image;
      setCanvasSize({ width, height });
      setAnnotations(Array.isArray(source.image.annotations) ? source.image.annotations : []);
    };
    image.src = source.image.sourceDataUrl || source.image.previewUrl;
    return () => {
      imageRef.current = null;
    };
  }, [source, visible]);

  React.useEffect(() => {
    drawCanvas();
  }, [canvasSize, drawCanvas]);

  const getCanvasPoint = React.useCallback((event) => {
    const canvas = canvasRef.current;
    if (!canvas) return null;
    const rect = canvas.getBoundingClientRect();
    const clientX = event.clientX ?? event.touches?.[0]?.clientX;
    const clientY = event.clientY ?? event.touches?.[0]?.clientY;
    if (typeof clientX !== 'number' || typeof clientY !== 'number') return null;
    return {
      x: Math.min(1, Math.max(0, (clientX - rect.left) / rect.width)),
      y: Math.min(1, Math.max(0, (clientY - rect.top) / rect.height)),
    };
  }, []);

  const handlePointerDown = React.useCallback((event) => {
    const point = getCanvasPoint(event);
    if (!point) return;
    event.preventDefault();
    const annotation = {
      id: createAnnotationId(),
      color: selectedColor,
      points: [point],
      prompt: '',
    };
    activeAnnotationIdRef.current = annotation.id;
    setAnnotations((previous) => [...previous, annotation]);
    setIsDrawing(true);
  }, [getCanvasPoint, selectedColor]);

  const handlePointerMove = React.useCallback((event) => {
    if (!isDrawing || !activeAnnotationIdRef.current) return;
    const point = getCanvasPoint(event);
    if (!point) return;
    event.preventDefault();
    setAnnotations((previous) =>
      previous.map((annotation) =>
        annotation.id === activeAnnotationIdRef.current
          ? { ...annotation, points: [...annotation.points, point] }
          : annotation,
      ),
    );
  }, [getCanvasPoint, isDrawing]);

  const stopDrawing = React.useCallback(() => {
    if (!isDrawing) return;
    const activeId = activeAnnotationIdRef.current;
    activeAnnotationIdRef.current = null;
    setIsDrawing(false);
    setAnnotations((previous) =>
      previous.filter((annotation) => annotation.id !== activeId || annotation.points.length > 1),
    );
  }, [isDrawing]);

  const updateAnnotationPrompt = React.useCallback((annotationId, value) => {
    setAnnotations((previous) =>
      previous.map((annotation) =>
        annotation.id === annotationId ? { ...annotation, prompt: value } : annotation,
      ),
    );
  }, []);

  const removeAnnotation = React.useCallback((annotationId) => {
    setAnnotations((previous) => previous.filter((annotation) => annotation.id !== annotationId));
  }, []);

  const handleSave = React.useCallback(() => {
    const canvas = canvasRef.current;
    const image = imageRef.current;
    if (!canvas || !image || !source?.image) return;
    const cleanedAnnotations = annotations
      .map((annotation, index) => ({
        ...annotation,
        label: getAnnotationLabel(annotation, index),
        prompt: String(annotation.prompt || '').trim(),
      }))
      .filter((annotation) => annotation.points.length > 1 && annotation.prompt);
    if (cleanedAnnotations.length === 0) {
      showError(t('\u8bf7\u5148\u5708\u9009\u533a\u57df\u5e76\u586b\u5199\u4fee\u6539\u8981\u6c42'));
      return;
    }

    const outputCanvas = document.createElement('canvas');
    outputCanvas.width = canvas.width;
    outputCanvas.height = canvas.height;
    const ctx = outputCanvas.getContext('2d');
    ctx.drawImage(image, 0, 0, outputCanvas.width, outputCanvas.height);
    cleanedAnnotations.forEach((annotation, annotationIndex) => {
      const points = annotation.points || [];
      ctx.save();
      ctx.lineCap = 'round';
      ctx.lineJoin = 'round';
      ctx.lineWidth = 8;
      ctx.strokeStyle = annotation.color;
      ctx.shadowColor = 'rgba(15,23,42,0.35)';
      ctx.shadowBlur = 3;
      ctx.beginPath();
      points.forEach((point, pointIndex) => {
        const x = point.x * outputCanvas.width;
        const y = point.y * outputCanvas.height;
        if (pointIndex === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      });
      ctx.stroke();
      const firstPoint = points[0];
      ctx.shadowBlur = 0;
      ctx.fillStyle = annotation.color;
      ctx.beginPath();
      ctx.arc(firstPoint.x * outputCanvas.width, firstPoint.y * outputCanvas.height, 16, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = '#ffffff';
      ctx.font = 'bold 15px sans-serif';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(String(annotationIndex + 1), firstPoint.x * outputCanvas.width, firstPoint.y * outputCanvas.height);
      ctx.restore();
    });
    const dataUrl = outputCanvas.toDataURL('image/png');
    const file = dataUrlToFile(dataUrl);
    onSave({
      ...source.image,
      file,
      previewUrl: dataUrl,
      sourceDataUrl: dataUrl,
      name: file.name,
      type: file.type,
      size: file.size,
      annotations: cleanedAnnotations,
      annotated: true,
    });
  }, [annotations, onSave, source, t]);

  return (
    <Modal
      visible={visible}
      title={null}
      footer={null}
      width='min(1180px, calc(100vw - 32px))'
      onCancel={onCancel}
      bodyStyle={{ padding: 0, background: 'transparent' }}
    >
      <div className='overflow-hidden rounded-[28px] border border-white/70 bg-white shadow-[0_24px_80px_rgba(15,23,42,0.18)]'>
        <div className='flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 px-5 py-4'>
          <div>
            <Typography.Title heading={5} className='!mb-1 !text-slate-900'>
              {t('\u753b\u5708\u4fee\u6539\u56fe\u7247')}
            </Typography.Title>
            <Typography.Text className='!text-sm !text-slate-500'>
              {t('\u5728\u56fe\u4e0a\u5708\u9009\u8981\u4fee\u6539\u7684\u90e8\u5206\uff0c\u7136\u540e\u7ed9\u6bcf\u4e2a\u533a\u57df\u5199\u5177\u4f53\u8981\u6c42')}
            </Typography.Text>
          </div>
          <div className='flex flex-wrap items-center gap-2'>
            {ANNOTATION_COLORS.map((item) => (
              <button
                key={item.value}
                type='button'
                title={t(item.label)}
                onClick={() => setSelectedColor(item.value)}
                className={`h-8 w-8 rounded-full border-2 transition ${
                  selectedColor === item.value ? 'border-slate-900 scale-110' : 'border-white shadow'
                }`}
                style={{ backgroundColor: item.value }}
              />
            ))}
            <Button
              theme='light'
              type='tertiary'
              icon={<RotateCcw size={15} />}
              className='!rounded-full'
              onClick={() => setAnnotations([])}
            >
              {t('\u6e05\u7a7a')}
            </Button>
            <Button
              theme='solid'
              type='primary'
              icon={<Check size={16} />}
              className='!rounded-full'
              onClick={handleSave}
            >
              {t('\u4f7f\u7528\u6807\u6ce8')}
            </Button>
          </div>
        </div>

        <div className='grid max-h-[78vh] gap-4 overflow-y-auto bg-slate-50 p-4 lg:grid-cols-[minmax(0,1fr)_360px]'>
          <div className='flex min-h-[420px] items-center justify-center overflow-auto rounded-[24px] bg-slate-950 p-3'>
            <canvas
              ref={canvasRef}
              width={canvasSize.width}
              height={canvasSize.height}
              onPointerDown={handlePointerDown}
              onPointerMove={handlePointerMove}
              onPointerUp={stopDrawing}
              onPointerCancel={stopDrawing}
              onPointerLeave={stopDrawing}
              className='max-h-[70vh] max-w-full cursor-crosshair rounded-[18px] shadow-[0_18px_60px_rgba(0,0,0,0.35)]'
            />
          </div>

          <div className='rounded-[24px] border border-slate-200 bg-white p-4'>
            <Typography.Text className='!text-xs !font-semibold !uppercase !tracking-[0.2em] !text-slate-400'>
              {t('\u5c40\u90e8\u63d0\u793a\u8bcd')}
            </Typography.Text>
            <p className='mt-2 text-sm text-slate-500'>
              {t('\u6bcf\u753b\u4e00\u7b14\u4f1a\u751f\u6210\u4e00\u4e2a\u533a\u57df\uff0c\u5199\u6e05\u695a\u8fd9\u4e2a\u533a\u57df\u8981\u600e\u4e48\u6539')}
            </p>
            <div className='mt-4 flex flex-col gap-3'>
              {annotations.length > 0 ? (
                annotations.map((annotation, index) => (
                  <div key={annotation.id} className='rounded-[18px] border border-slate-100 bg-slate-50 p-3'>
                    <div className='mb-2 flex items-center justify-between gap-2'>
                      <div className='flex min-w-0 items-center gap-2'>
                        <span
                          className='h-3 w-3 shrink-0 rounded-full'
                          style={{ backgroundColor: annotation.color }}
                        />
                        <span className='truncate text-sm font-semibold text-slate-700'>
                          {getAnnotationLabel(annotation, index)}
                        </span>
                      </div>
                      <Button
                        theme='borderless'
                        type='danger'
                        size='small'
                        icon={<X size={14} />}
                        onClick={() => removeAnnotation(annotation.id)}
                      />
                    </div>
                    <TextArea
                      value={annotation.prompt}
                      onChange={(value) => updateAnnotationPrompt(annotation.id, value)}
                      autosize={{ minRows: 2, maxRows: 5 }}
                      placeholder={t('\u4f8b\u5982\uff1a\u628a\u8fd9\u91cc\u6539\u6210\u514d\u8d39\u8bd5\u7528\uff0c\u5b57\u4f53\u66f4\u9192\u76ee')}
                    />
                  </div>
                ))
              ) : (
                <Empty
                  image={<Brush size={34} className='text-slate-400' />}
                  title={t('\u8fd8\u6ca1\u6709\u6807\u6ce8\u533a\u57df')}
                  description={t('\u76f4\u63a5\u5728\u5de6\u4fa7\u56fe\u7247\u4e0a\u753b\u5708\u5373\u53ef\u5f00\u59cb')}
                />
              )}
            </div>
          </div>
        </div>
      </div>
    </Modal>
  );
};

const openImageInNewTab = (imageUrl) => {
  if (!imageUrl) return;
  window.open(imageUrl, '_blank', 'noopener,noreferrer');
};

const createPromptFavoriteTitle = (value) => {
  const normalized = String(value || '').replace(/\s+/g, ' ').trim();
  if (!normalized) return '';
  return normalized.length > 36 ? `${normalized.slice(0, 36)}...` : normalized;
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
  const [openaiImageSize, setOpenaiImageSize] = useState('1024x1024');
  const [batchCount, setBatchCount] = useState(1);
  const [isOptimizingPrompt, setIsOptimizingPrompt] = useState(false);
  const [optimizedPromptDraft, setOptimizedPromptDraft] = useState('');
  const [originalPrompt, setOriginalPrompt] = useState('');
  const [activeImageIndex, setActiveImageIndex] = useState(0);
  const [remoteTasks, setRemoteTasks] = useState([]);
  const [remotePage, setRemotePage] = useState(1);
  const [remoteTotal, setRemoteTotal] = useState(0);
  const [favoritesVisible, setFavoritesVisible] = useState(false);
  const [promptFavorites, setPromptFavorites] = useState([]);
  const [favoritesLoading, setFavoritesLoading] = useState(false);
  const [favoriteSearch, setFavoriteSearch] = useState('');
  const [favoriteDraftTitle, setFavoriteDraftTitle] = useState('');
  const [favoriteDraftPrompt, setFavoriteDraftPrompt] = useState('');
  const [editingFavoriteId, setEditingFavoriteId] = useState(null);
  const [annotationSource, setAnnotationSource] = useState(null);
  const [isPreparingAnnotation, setIsPreparingAnnotation] = useState(false);
  const previousDraftImagesRef = useRef([]);
  const selectedModelRef = useRef('');
  const promptOptimizerModelRef = useRef('');
  const stableTaskUrlRef = useRef(new Map());

  const pendingRemoteTasks = useMemo(
    () => remoteTasks.filter((task) => task.status === 'PENDING' || task.status === 'PROCESSING'),
    [remoteTasks],
  );

  const completedRemoteTasks = useMemo(
    () => remoteTasks.filter((task) => task.status === 'SUCCEEDED' || task.status === 'FAILED'),
    [remoteTasks],
  );

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
  const annotatedPromptCount = useMemo(
    () => getAnnotatedPromptCount(draftImages),
    [draftImages],
  );

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
      const remaining = MAX_REFERENCE_IMAGES - draftImages.length;
      if (remaining <= 0) {
        showError(t('最多支持 {{count}} 张参考图', { count: MAX_REFERENCE_IMAGES }));
        return;
      }
      const acceptedFiles = nextFiles.slice(0, remaining);
      setDraftImages((previous) => [
        ...previous,
        ...acceptedFiles.map(createDraftImageEntry),
      ]);
      if (acceptedFiles.length < nextFiles.length) {
        showError(t('最多支持 {{count}} 张参考图', { count: MAX_REFERENCE_IMAGES }));
      }
    },
    [draftImages.length, setDraftImages, t],
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

  const openDraftAnnotation = React.useCallback((index) => {
    const image = draftImages[index];
    if (!image) return;
    setAnnotationSource({ type: 'draft', index, image });
  }, [draftImages]);

  const openRecordAnnotation = React.useCallback(
    async (record, imageIndex = 0) => {
      if (!record || record.status === 'FAILED') {
        showError(t('\u8be5\u56fe\u7247\u65e0\u6cd5\u7f16\u8f91'));
        return;
      }
      const imageUrl = record?.images?.[imageIndex] || record?.images?.[0];
      if (!imageUrl && !record?.taskId) {
        showError(t('\u6ca1\u6709\u53ef\u7f16\u8f91\u7684\u56fe\u7247'));
        return;
      }

      setIsPreparingAnnotation(true);
      try {
        let draftImage;
        if (record.isRemote && record.taskId) {
          const response = await fetch(`/api/ai-image/proxy/${record.taskId}`, {
            headers: { 'New-Api-User': getUserIdFromLocalStorage() },
          });
          if (!response.ok) throw new Error(`proxy fetch failed: ${response.status}`);
          const blob = await response.blob();
          const extension = blob.type?.split('/')[1] || 'png';
          const file = new File([blob], `editable-${Date.now()}.${extension}`, { type: blob.type || 'image/png' });
          const dataUrl = await blobToDataUrl(blob);
          draftImage = createDraftImageEntry(file, { sourceDataUrl: dataUrl });
        } else {
          draftImage = await createDraftImageFromImageUrl(imageUrl, 'editable');
        }
        setAnnotationSource({ type: 'new', image: draftImage });
      } catch {
        showError(t('\u52a0\u8f7d\u56fe\u7247\u5931\u8d25\uff0c\u8bf7\u5148\u4e0b\u8f7d\u540e\u624b\u52a8\u4e0a\u4f20'));
      } finally {
        setIsPreparingAnnotation(false);
      }
    },
    [t],
  );

  const handleSaveAnnotation = React.useCallback(
    (nextImage) => {
      if (!annotationSource || !nextImage) return;
      if (annotationSource.type !== 'draft' && draftImages.length >= MAX_REFERENCE_IMAGES) {
        showError(t('\u6700\u591a\u652f\u6301 {{count}} \u5f20\u53c2\u8003\u56fe', { count: MAX_REFERENCE_IMAGES }));
        return;
      }
      if (annotationSource.image?.previewUrl !== nextImage.previewUrl) {
        revokeDraftImage(annotationSource.image);
      }
      setDraftImages((previous) => {
        if (annotationSource.type === 'draft') {
          return previous.map((image, index) =>
            index === annotationSource.index ? nextImage : image,
          );
        }
        return [...previous, nextImage];
      });
      setAnnotationSource(null);
      showSuccess(t('\u5df2\u6dfb\u52a0\u5c40\u90e8\u4fee\u6539\u8981\u6c42'));
    },
    [annotationSource, draftImages.length, setDraftImages, t],
  );

  const closeAnnotationModal = React.useCallback(() => {
    if (annotationSource?.type === 'new') {
      revokeDraftImage(annotationSource.image);
    }
    setAnnotationSource(null);
  }, [annotationSource]);

  const serializeDraftImagesForRequest = React.useCallback(async () => {
    const images = draftImages.filter(Boolean).slice(0, MAX_REFERENCE_IMAGES);
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

  const mergeStableTaskUrls = React.useCallback((items) => {
    const urlCache = stableTaskUrlRef.current;
    return items.map((task) => {
      if (task?.status !== 'SUCCEEDED') {
        return task;
      }
      const cacheKey = task.task_id || task.id;
      if (!cacheKey) {
        return task;
      }
      const cachedURL = urlCache.get(cacheKey);
      const resultURL = cachedURL || task.result_url || '';
      if (resultURL) {
        urlCache.set(cacheKey, resultURL);
      }
      return resultURL === task.result_url ? task : { ...task, result_url: resultURL };
    });
  }, []);

  const loadRemoteTasks = React.useCallback(async (nextPage = remotePage) => {
    const headers = {
      Accept: 'application/json',
      'New-Api-User': getUserIdFromLocalStorage(),
    };

    const page = Math.max(1, Number(nextPage) || 1);
    const response = await fetch(`/api/ai-image/tasks?p=${page}&page_size=${REMOTE_TASK_PAGE_SIZE}`, { headers });
    if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
    const result = await response.json();
    if (!result?.success) throw new Error(result?.message || 'failed to load image tasks');
    const data = result?.data || {};
    const items = Array.isArray(data.items) ? mergeStableTaskUrls(data.items) : [];
    setRemotePage(Number(data.page) || page);
    setRemoteTotal(Number(data.total) || 0);
    setRemoteTasks(items);
    try { localStorage.setItem(TASKS_CACHE_KEY, JSON.stringify(items)); } catch { /* ignore */ }
  }, [mergeStableTaskUrls, remotePage]);

  const loadPromptFavorites = React.useCallback(async (keyword = favoriteSearch, options = {}) => {
    setFavoritesLoading(true);
    try {
      const params = new URLSearchParams({
        p: '1',
        page_size: String(PROMPT_FAVORITES_PAGE_SIZE),
      });
      const trimmedKeyword = keyword.trim();
      if (trimmedKeyword) params.set('q', trimmedKeyword);
      const response = await fetch(`${PROMPT_FAVORITES_ENDPOINT}?${params.toString()}`, {
        headers: { 'New-Api-User': getUserIdFromLocalStorage() },
      });
      const result = await response.json();
      if (!result?.success) throw new Error(result?.message || 'failed to load favorites');
      setPromptFavorites(Array.isArray(result?.data?.items) ? result.data.items : []);
    } catch (error) {
      if (!options.silent) {
        showError(error.message || t('加载收藏夹失败'));
      }
    } finally {
      setFavoritesLoading(false);
    }
  }, [favoriteSearch, t]);

  const resetFavoriteDraft = React.useCallback((nextPrompt = '') => {
    setEditingFavoriteId(null);
    setFavoriteDraftPrompt(nextPrompt);
    setFavoriteDraftTitle(createPromptFavoriteTitle(nextPrompt));
  }, []);

  const openPromptFavorites = React.useCallback(() => {
    setFavoritesVisible(true);
    resetFavoriteDraft(prompt.trim());
    loadPromptFavorites(favoriteSearch, { silent: true });
  }, [favoriteSearch, loadPromptFavorites, prompt, resetFavoriteDraft]);

  const buildFavoritePayload = React.useCallback((sourcePrompt, sourceTitle = '') => ({
    title: sourceTitle || createPromptFavoriteTitle(sourcePrompt),
    prompt: sourcePrompt,
    model: selectedModelRef.current || selectedModel || '',
    group: selectedGroup || '',
    size: openaiImageSize || '',
    resolution: resolution || '',
    aspect_ratio: aspectRatio || '',
  }), [aspectRatio, openaiImageSize, resolution, selectedGroup, selectedModel]);

  const createPromptFavorite = React.useCallback(async (sourcePrompt, sourceTitle = '') => {
    const trimmedPrompt = String(sourcePrompt || '').trim();
    if (!trimmedPrompt) {
      showError(t('请先输入提示词'));
      return false;
    }
    try {
      const response = await fetch(PROMPT_FAVORITES_ENDPOINT, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'New-Api-User': getUserIdFromLocalStorage(),
        },
        body: JSON.stringify(buildFavoritePayload(trimmedPrompt, sourceTitle)),
      });
      const result = await response.json();
      if (!result?.success) throw new Error(result?.message || 'failed to save favorite');
      setPromptFavorites((previous) => [result.data, ...previous.filter((item) => item.id !== result.data?.id)]);
      showSuccess(t('已收藏提示词'));
      return true;
    } catch (error) {
      showError(error.message || t('收藏失败'));
      return false;
    }
  }, [buildFavoritePayload, t]);

  const saveFavoriteDraft = React.useCallback(async () => {
    const trimmedPrompt = favoriteDraftPrompt.trim();
    if (!trimmedPrompt) {
      showError(t('请先输入提示词'));
      return;
    }
    const payload = buildFavoritePayload(trimmedPrompt, favoriteDraftTitle.trim());
    try {
      const response = await fetch(
        editingFavoriteId ? `${PROMPT_FAVORITES_ENDPOINT}/${editingFavoriteId}` : PROMPT_FAVORITES_ENDPOINT,
        {
          method: editingFavoriteId ? 'PUT' : 'POST',
          headers: {
            'Content-Type': 'application/json',
            'New-Api-User': getUserIdFromLocalStorage(),
          },
          body: JSON.stringify(payload),
        },
      );
      const result = await response.json();
      if (!result?.success) throw new Error(result?.message || 'failed to save favorite');
      setPromptFavorites((previous) => [
        result.data,
        ...previous.filter((item) => item.id !== result.data?.id),
      ]);
      resetFavoriteDraft('');
      showSuccess(editingFavoriteId ? t('已更新收藏') : t('已创建收藏'));
    } catch (error) {
      showError(error.message || t('保存失败'));
    }
  }, [buildFavoritePayload, editingFavoriteId, favoriteDraftPrompt, favoriteDraftTitle, resetFavoriteDraft, t]);

  const usePromptFavorite = React.useCallback(async (favorite) => {
    if (!favorite?.prompt) return;
    setPrompt(favorite.prompt);
    if (favorite.model) {
      selectedModelRef.current = favorite.model;
      setSelectedModel(favorite.model);
    }
    if (favorite.group) setSelectedGroup(favorite.group);
    if (favorite.size) setOpenaiImageSize(favorite.size);
    if (favorite.resolution) setResolution(favorite.resolution);
    if (favorite.aspect_ratio) setAspectRatio(favorite.aspect_ratio);
    setOptimizedPromptDraft('');
    setOriginalPrompt('');
    setFavoritesVisible(false);
    showSuccess(t('已填入收藏提示词'));
    try {
      await fetch(`${PROMPT_FAVORITES_ENDPOINT}/${favorite.id}/use`, {
        method: 'POST',
        headers: { 'New-Api-User': getUserIdFromLocalStorage() },
      });
    } catch {
      // Last-used time is best-effort only.
    }
  }, [setSelectedGroup, setSelectedModel, t]);

  const editPromptFavorite = React.useCallback((favorite) => {
    setEditingFavoriteId(favorite.id);
    setFavoriteDraftTitle(favorite.title || createPromptFavoriteTitle(favorite.prompt));
    setFavoriteDraftPrompt(favorite.prompt || '');
  }, []);

  const deletePromptFavorite = React.useCallback(async (favorite) => {
    if (!favorite?.id) return;
    try {
      const response = await fetch(`${PROMPT_FAVORITES_ENDPOINT}/${favorite.id}`, {
        method: 'DELETE',
        headers: { 'New-Api-User': getUserIdFromLocalStorage() },
      });
      const result = await response.json();
      if (!result?.success) throw new Error(result?.message || 'failed to delete favorite');
      setPromptFavorites((previous) => previous.filter((item) => item.id !== favorite.id));
      if (editingFavoriteId === favorite.id) resetFavoriteDraft('');
      showSuccess(t('已删除收藏'));
    } catch (error) {
      showError(error.message || t('删除失败'));
    }
  }, [editingFavoriteId, resetFavoriteDraft, t]);

  React.useEffect(() => {
    if (!ready) {
      return;
    }
    loadRemoteTasks(remotePage).catch(() => {
      // Ignore background load failures.
    });
  }, [loadRemoteTasks, ready, remotePage]);

  React.useEffect(() => {
    if (!ready || pendingRemoteTasks.length === 0) {
      return undefined;
    }
    const timer = window.setInterval(() => {
      if (document.visibilityState === 'visible') {
        loadRemoteTasks(remotePage).catch(() => {
          // Ignore background polling failures.
        });
      }
    }, 3000);
    return () => window.clearInterval(timer);
  }, [loadRemoteTasks, pendingRemoteTasks.length, ready, remotePage]);

  const submitImageTask = React.useCallback(
    async (itemPrompt) => {
      const effectiveModel = selectedModelRef.current || modelOptions[0]?.value || '';
      if (!effectiveModel) {
        throw new Error('image model is required');
      }
      let referenceImages = [];
      if (draftImages.length > 0) {
        const dataUrls = await Promise.all(
          draftImages.filter(Boolean).slice(0, MAX_REFERENCE_IMAGES).map(async (image) => {
            if (typeof image?.sourceDataUrl === 'string' && image.sourceDataUrl.startsWith('data:image/')) {
              return image.sourceDataUrl;
            }
            if (image?.file) {
              return readFileAsDataUrl(image.file);
            }
            return '';
          }),
        );
        referenceImages = dataUrls.filter(Boolean);
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
          size: openaiImageSize,
          n: 1,
          reference_images: referenceImages,
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
    [draftImages, modelOptions, openaiImageSize, selectedGroup],
  );

  const requestOneImage = React.useCallback(
    async (itemPrompt) => {
      const effectiveModel = selectedModelRef.current || modelOptions[0]?.value || '';
      if (!effectiveModel) {
        throw new Error('image model is required');
      }

      const activeDraftImages = draftImages.slice(0, MAX_REFERENCE_IMAGES);

      if (isOpenAIImageModel(effectiveModel)) {
        const response = activeDraftImages.length > 0
          ? await postOpenAIImageEditPayload({
              model: effectiveModel,
              prompt: itemPrompt,
              images: activeDraftImages,
              selectedGroup,
            })
          : await postOpenAIImagePayload(
              {
                model: effectiveModel,
                prompt: itemPrompt,
                size: openaiImageSize,
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
    [aspectRatio, draftImages, modelOptions, openaiImageSize, resolution, selectedGroup, serializeDraftImagesForRequest],
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
      const effectivePrompt = buildPromptWithAnnotations(trimmedPrompt, draftImages);
      if (isOpenAIImageModel(fallbackImageModel)) {
        await submitImageTask(effectivePrompt);
        setRemotePage(1);
        await loadRemoteTasks(1);
        setPrompt('');
        setOptimizedPromptDraft('');
        setOriginalPrompt('');
        clearDraftImages();
        showSuccess(t('已加入生成队列'));
        return;
      }
      const parsed = await requestOneImage(effectivePrompt);
      if (parsed.imageUrls.length === 0) {
        showError(t('未返回可展示的图片，请检查模型返回格式'));
        return;
      }
      appendGeneration([{ itemPrompt: effectivePrompt, imageUrls: parsed.imageUrls }]);
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
    draftImages,
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

    const effectiveBatchCount = normalizeBatchCount(batchCount);
    const tasks = Array.from({ length: effectiveBatchCount }, (_, index) =>
      buildPromptWithAnnotations(
        prompts.length > 1 ? prompts[index % prompts.length] : prompts[0],
        draftImages,
      ),
    );

    setIsGenerating(true);
    try {
      if (isOpenAIImageModel(fallbackImageModel)) {
        await Promise.all(tasks.map((itemPrompt) => submitImageTask(itemPrompt)));
        setRemotePage(1);
        await loadRemoteTasks(1);
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
    draftImages,
    fallbackImageModel,
    loadRemoteTasks,
    prompt,
    requestOneImage,
    setSelectedModel,
    submitImageTask,
    t,
  ]);

  const handleRemotePageChange = React.useCallback((page) => {
    const nextPage = Math.max(1, Number(page) || 1);
    setRemotePage(nextPage);
    setActiveRecordId(null);
    setActiveImageIndex(0);
  }, []);

  const handleDeleteHistory = React.useCallback(() => {
    const localCount = localRecords.length;
    const remoteCount = completedRemoteTasks.length;
    if (localCount === 0 && remoteCount === 0) {
      showError(t('暂无可删除历史'));
      return;
    }

    Modal.confirm({
      title: t('确认一键删除历史记录'),
      content: t('将删除所有已完成的历史记录，排队中和生成中的任务不会删除。此操作不可恢复。'),
      okText: t('确认删除'),
      cancelText: t('取消'),
      okType: 'danger',
      onOk: async () => {
        try {
          if (remoteCount > 0) {
            const response = await fetch('/api/ai-image/tasks/history', {
              method: 'DELETE',
              headers: { 'New-Api-User': getUserIdFromLocalStorage() },
            });
            const result = await response.json();
            if (!result?.success) {
              showError(result?.message || t('删除失败'));
              return;
            }
          }

          if (localCount > 0) {
            clearCurrentSession();
          }
          const nextRemoteTasks = remoteTasks.filter(
            (task) => task.status === 'PENDING' || task.status === 'PROCESSING',
          );

          setRemoteTasks(nextRemoteTasks);
          try {
            localStorage.setItem(TASKS_CACHE_KEY, JSON.stringify(nextRemoteTasks));
          } catch {
            // Ignore storage sync errors.
          }
          setActiveRecordId(null);
          showSuccess(t('已删除历史记录'));
        } catch {
          showError(t('删除失败'));
        }
      },
    });
  }, [clearCurrentSession, completedRemoteTasks.length, localRecords.length, remoteTasks, t]);

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
    async (record) => {
      if (record?.status === 'FAILED') {
        showError(t('该任务生成失败，无法下载'));
        return;
      }
      const imageUrl = record?.images?.[0];
      if (!imageUrl && !record?.taskId) {
        showError(t('没有可下载的图片'));
        return;
      }

      if (record.isRemote && record.taskId) {
        try {
          const response = await fetch(`/api/ai-image/download/${record.taskId}`, {
            headers: { 'New-Api-User': getUserIdFromLocalStorage() },
          });
          if (!response.ok) throw new Error(`download failed: ${response.status}`);
          const blob = await response.blob();
          const blobUrl = URL.createObjectURL(blob);
          const anchor = document.createElement('a');
          anchor.href = blobUrl;
          anchor.download = `ai-image-${record.taskId}.png`;
          document.body.appendChild(anchor);
          anchor.click();
          document.body.removeChild(anchor);
          URL.revokeObjectURL(blobUrl);
          showSuccess(t('开始下载图片'));
          return;
        } catch {
          // fallback
        }
      }

      if (imageUrl) {
        try {
          const response = await fetch(imageUrl);
          const blob = await response.blob();
          const blobUrl = URL.createObjectURL(blob);
          const anchor = document.createElement('a');
          anchor.href = blobUrl;
          anchor.download = `ai-image-${record.id || Date.now()}.png`;
          document.body.appendChild(anchor);
          anchor.click();
          document.body.removeChild(anchor);
          URL.revokeObjectURL(blobUrl);
          showSuccess(t('开始下载图片'));
        } catch {
          window.open(imageUrl, '_blank', 'noopener,noreferrer');
        }
      }
    },
    [t],
  );

  const handleDeleteRecord = React.useCallback(
    async (record) => {
      if (!record) return;

      if (record.isRemote && record.taskId) {
        try {
          const response = await fetch(`/api/ai-image/tasks/${record.taskId}`, {
            method: 'DELETE',
            headers: { 'New-Api-User': getUserIdFromLocalStorage() },
          });
          const result = await response.json();
          if (!result?.success) {
            showError(result?.message || t('删除失败'));
            return;
          }
          setRemoteTasks((previous) => previous.filter((task) => task.task_id !== record.taskId));
          setRemoteTotal((previous) => Math.max(0, previous - 1));
        } catch (error) {
          showError(t('删除失败'));
          return;
        }
      } else {
        setMessages((previous) =>
          previous.filter(
            (message) =>
              message.id !== record.userMessageId &&
              message.id !== record.assistantMessageId,
          ),
        );
      }

      if (activeRecordId === record.id) {
        setActiveRecordId(null);
      }
      showSuccess(t('已删除该记录'));
    },
    [activeRecordId, setMessages, t],
  );

  const handleReuseImage = React.useCallback(
    async (record) => {
      const imageUrl = record?.images?.[0];
      if (!imageUrl && !record?.taskId) {
        showError(t('没有可引用的图片'));
        return;
      }
      if (record?.status === 'FAILED') {
        showError(t('该任务生成失败，无法引用'));
        return;
      }
      if (draftImages.length >= MAX_REFERENCE_IMAGES) {
        showError(t('最多支持 {{count}} 张参考图', { count: MAX_REFERENCE_IMAGES }));
        return;
      }

      try {
        if (imageUrl && imageUrl.startsWith('data:image/')) {
          const [meta, base64Data] = imageUrl.split(',', 2);
          const mimeType = meta.match(/^data:(.+?);base64$/)?.[1] || 'image/png';
          const binary = atob(base64Data || '');
          const bytes = new Uint8Array(binary.length);
          for (let index = 0; index < binary.length; index += 1) {
            bytes[index] = binary.charCodeAt(index);
          }
          const extension = mimeType.split('/')[1] || 'png';
          const file = new File([bytes], `referenced-${Date.now()}.${extension}`, { type: mimeType });
          setDraftImages((previous) => [...previous, createDraftImageEntry(file, { sourceDataUrl: imageUrl })]);
          showSuccess(t('已引用到参考图'));
          return;
        }

        let blob;
        if (record.isRemote && record.taskId) {
          const response = await fetch(`/api/ai-image/proxy/${record.taskId}`, {
            headers: { 'New-Api-User': getUserIdFromLocalStorage() },
          });
          if (!response.ok) throw new Error(`proxy fetch failed: ${response.status}`);
          blob = await response.blob();
        } else if (imageUrl) {
          const response = await fetch(imageUrl);
          if (!response.ok) throw new Error(`fetch failed: ${response.status}`);
          blob = await response.blob();
        } else {
          showError(t('没有可引用的图片'));
          return;
        }
        const extension = blob.type?.split('/')[1] || 'png';
        const file = new File([blob], `referenced-${Date.now()}.${extension}`, { type: blob.type || 'image/png' });
        setDraftImages((previous) => [...previous, createDraftImageEntry(file)]);
        showSuccess(t('已引用到参考图'));
      } catch {
        showError(t('引用图片失败，请右键保存后手动上传'));
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
                    {t('选择 gpt-image-2 图片模型，输入提示词后直接查看生成结果')}
                  </Typography.Text>
                </div>
              </div>

              <div className='rounded-[28px] border border-slate-200 bg-slate-50/70 p-4'>
                <div className='mb-3 flex items-center justify-between gap-3'>
                  <div>
                    <Typography.Text className='!text-xs !font-semibold !uppercase !tracking-[0.2em] !text-slate-400'>
                      {t('基础设置')}
                    </Typography.Text>
                    <p className='mt-1 text-sm text-slate-500'>{t('选择模型、分组和生成规格')}</p>
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
              </div>

              <div className='rounded-[28px] border border-slate-200 bg-slate-50/70 p-4'>
                <div className='mb-3 flex flex-wrap items-center justify-between gap-3'>
                  <div>
                    <Typography.Text className='!text-xs !font-semibold !uppercase !tracking-[0.2em] !text-slate-400'>
                      {t('提示词与参考')}
                    </Typography.Text>
                    <p className='mt-1 text-sm text-slate-500'>{t('编辑提示词、收藏常用灵感，并上传参考图')}</p>
                  </div>
                  <div className='flex flex-wrap gap-2'>
                    <Button
                      theme='light'
                      type='primary'
                      icon={<Bookmark size={15} />}
                      className='!rounded-full'
                      onClick={() => createPromptFavorite(prompt)}
                    >
                      {t('收藏当前')}
                    </Button>
                  </div>
                </div>
                <div className='mb-3 grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
                  {isOpenAIImageModel(fallbackImageModel) ? (
                    <label className='flex flex-col gap-2 md:col-span-2'>
                      <span className='text-xs font-medium uppercase tracking-[0.18em] text-slate-400'>
                        {t('图片尺寸')}
                      </span>
                      <select
                        value={openaiImageSize}
                        onChange={(event) => setOpenaiImageSize(event.target.value)}
                        className='h-11 rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'
                      >
                        {OPENAI_IMAGE_SIZE_OPTIONS.map((item) => (
                          <option key={item.value} value={item.value}>
                            {item.label}
                          </option>
                        ))}
                      </select>
                    </label>
                  ) : (
                    <>
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
                    </>
                  )}

                  <label className='flex flex-col gap-2'>
                    <span className='text-xs font-medium uppercase tracking-[0.18em] text-slate-400'>
                      {t('批量张数')}
                    </span>
                    <div className='flex items-center gap-3'>
                      <div className='flex items-center gap-2'>
                        {QUICK_BATCH_COUNT_OPTIONS.map((item) => (
                          <Button
                            key={item}
                            theme={batchCount === item ? 'solid' : 'light'}
                            type='primary'
                            className='!rounded-full'
                            onClick={() => setBatchCount(item)}
                          >
                            {item}
                          </Button>
                        ))}
                      </div>
                      <InputNumber
                        min={1}
                        max={MAX_BATCH_COUNT}
                        value={batchCount}
                        onChange={(value) => setBatchCount(normalizeBatchCount(value))}
                        className='!w-[112px] shrink-0'
                        placeholder='1-10'
                      />
                    </div>
                    <Typography.Text className='!text-xs !text-slate-500'>
                      {t('快捷选择 1 / 2 / 4，也可自定义输入，最高 10 张')}
                    </Typography.Text>
                  </label>
                </div>

                <TextArea
                  value={prompt}
                  onChange={(value) => {
                    if (value.length <= MAX_PROMPT_LENGTH) {
                      setPrompt(value);
                    }
                    if (optimizedPromptDraft) {
                      setOptimizedPromptDraft('');
                      setOriginalPrompt('');
                    }
                  }}
                  maxLength={MAX_PROMPT_LENGTH}
                  showClear
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
                        className={`group relative overflow-hidden rounded-[18px] border bg-white shadow-sm transition ${
                          getImageAnnotationCount(image) > 0
                            ? 'border-sky-300 ring-2 ring-sky-100'
                            : 'border-white/70 hover:border-sky-200'
                        }`}
                      >
                        <img
                          src={image.previewUrl}
                          alt={`${t('参考图')} ${index + 1}`}
                          className='h-24 w-24 object-cover'
                        />
                        <button
                          type='button'
                          onClick={() => openDraftAnnotation(index)}
                          className='absolute inset-x-1.5 bottom-1.5 flex h-7 items-center justify-center gap-1 rounded-full bg-slate-950/80 px-2 text-[11px] font-semibold text-white shadow-lg transition hover:bg-sky-600'
                          title={t('圈选参考图局部并填写修改要求')}
                        >
                          <Brush size={12} />
                          <span>
                            {getImageAnnotationCount(image) > 0
                              ? t('已标注 {{count}} 处', { count: getImageAnnotationCount(image) })
                              : t('标注修改')}
                          </span>
                        </button>
                        <span className='absolute left-1.5 top-1.5 rounded-full bg-emerald-500 px-1.5 py-0.5 text-[10px] font-semibold text-white shadow'>
                          {t('可编辑')}
                        </span>
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

                {annotatedPromptCount > 0 && (
                  <div className='mt-3 rounded-[18px] border border-sky-100 bg-sky-50/80 p-3'>
                    <div className='mb-2 flex items-center gap-2 text-sm font-semibold text-sky-700'>
                      <Brush size={15} />
                      <span>{t('已添加 {{count}} 个局部修改要求', { count: annotatedPromptCount })}</span>
                    </div>
                    <div className='flex flex-wrap gap-2'>
                      {draftImages.flatMap((image, imageIndex) =>
                        (Array.isArray(image.annotations) ? image.annotations : [])
                          .filter((annotation) => annotation?.prompt?.trim())
                          .map((annotation, annotationIndex) => (
                            <span
                              key={`${image.id}-${annotation.id}`}
                              className='inline-flex max-w-full items-center gap-1 rounded-full bg-white px-2.5 py-1 text-xs text-slate-700 shadow-sm'
                              title={annotation.prompt}
                            >
                              <span
                                className='h-2 w-2 shrink-0 rounded-full'
                                style={{ backgroundColor: annotation.color }}
                              />
                              <span className='shrink-0 text-slate-400'>
                                {t('图{{index}}', { index: imageIndex + 1 })}
                              </span>
                              <span className='max-w-[220px] truncate'>
                                {getAnnotationLabel(annotation, annotationIndex)}：{annotation.prompt}
                              </span>
                            </span>
                          )),
                      )}
                    </div>
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

                  <Typography.Text className='!text-xs !text-slate-500'>
                    {t('最多 {{count}} 张参考图，上传后可圈选局部修改', { count: MAX_REFERENCE_IMAGES })}
                  </Typography.Text>

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
                    onClick={normalizeBatchCount(batchCount) > 1 ? handleBatchGenerate : handleGenerate}
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
                    theme='solid'
                    type='tertiary'
                    icon={<Search size={15} />}
                    className='!rounded-full !bg-slate-900 !text-white'
                    onClick={openPromptFavorites}
                  >
                    {t('收藏夹')}
                  </Button>

                </div>

                <Typography.Text className='mt-3 block text-xs text-slate-500'>
                  {t(
                    '异步生成的图片保留 7 天后自动清理（含对象存储），本地同步图片仅保存在浏览器中。',
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
                                loading='lazy'
                                decoding='async'
                                className='h-16 w-16 object-cover sm:h-20 sm:w-20'
                              />
                            </button>
                          ))}
                        </div>
                      ) : null}
                      <div className='mt-4'>
                        <Button
                          theme='solid'
                          type='primary'
                          icon={
                            isPreparingAnnotation ? (
                              <Loader2 size={17} className='animate-spin' />
                            ) : (
                              <Brush size={17} />
                            )
                          }
                          loading={isPreparingAnnotation}
                          className='!h-11 !rounded-full !bg-slate-950 !px-5 !text-white shadow-[0_14px_34px_rgba(15,23,42,0.22)]'
                          onClick={() => openRecordAnnotation(activeRecord, activeImageIndex)}
                          disabled={!activeRecord || activeRecord?.status === 'FAILED'}
                        >
                          {t('画圈修改图片')}
                        </Button>
                        <Typography.Text className='!ml-3 !text-xs !text-slate-500'>
                          {t('圈选区域，写修改要求，再次生成')}
                        </Typography.Text>
                      </div>
                      <div className='mt-3 flex flex-wrap gap-2'>
                        <Button
                          theme='light'
                          type='primary'
                          icon={<Bookmark size={15} />}
                          className='!rounded-full'
                          onClick={() => createPromptFavorite(activeRecord?.prompt || '')}
                          disabled={!activeRecord?.prompt}
                        >
                          {t('收藏提示词')}
                        </Button>
                        <Button
                          theme='light'
                          type='primary'
                          className='!rounded-full'
                          onClick={() => handleReuseImage(activeRecord)}
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
              <Typography.Text className='!ml-2 !text-sm !font-medium !text-slate-500'>
                {t('点击下方缩略图即可切换查看历史结果，图片保留 7 天后自动清理')}
              </Typography.Text>
            </div>
            <div className='flex items-center gap-3'>
              <Typography.Text className='!text-sm !text-slate-400'>
                {t('{{count}} 条记录', { count: records.length })}
              </Typography.Text>
              <Button
                theme='light'
                type='danger'
                className='!rounded-full'
                disabled={localRecords.length === 0 && completedRemoteTasks.length === 0}
                onClick={handleDeleteHistory}
              >
                {t('一键删除历史')}
              </Button>
            </div>
          </div>

          {pendingRemoteTasks.length > 0 && (
            <div className='mb-3 rounded-2xl border border-blue-100 bg-blue-50/80 p-3'>
              <div className='mb-2 text-xs font-medium uppercase tracking-wider text-blue-500'>
                {t('排队 / 生成中')} ({pendingRemoteTasks.length})
              </div>
              <div className='flex flex-col gap-2'>
                {pendingRemoteTasks.map((task) => (
                  <div key={task.task_id} className='flex items-center gap-2 rounded-xl bg-white px-3 py-2 text-sm'>
                    <Loader2 size={14} className='animate-spin text-blue-500' />
                    <span className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${
                      task.status === 'PROCESSING'
                        ? 'bg-blue-100 text-blue-700'
                        : 'bg-amber-100 text-amber-700'
                    }`}>
                      {task.status === 'PROCESSING' ? t('生成中') : t('排队中')}
                    </span>
                    <span className='flex-1 truncate text-slate-600'>
                      {task.prompt || t('未命名任务')}
                    </span>
                    <span className='shrink-0 text-xs text-slate-400'>
                      {task.model}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {records.length > 0 ? (
            <>
              <div className='flex gap-3 overflow-x-auto pb-1'>
                {records.map((record) => (
                  <div key={record.id} className='w-[168px] shrink-0 space-y-2'>
                    {record.status === 'FAILED' ? (
                      <div
                        className='flex h-24 w-24 items-center justify-center rounded-[22px] border border-red-200 bg-red-50 sm:h-28 sm:w-28'
                        title={record.errorMessage || t('生成失败')}
                      >
                        <div className='px-2 text-center'>
                          <X size={20} className='mx-auto mb-1 text-red-400' />
                          <p className='text-xs leading-tight text-red-500 line-clamp-3'>
                            {record.errorMessage || t('生成失败')}
                          </p>
                        </div>
                      </div>
                    ) : (
                      <GalleryImage
                        src={record.images[0]}
                        alt={record.prompt || 'generated image'}
                        active={record.id === activeRecord?.id}
                        onClick={() => {
                          setActiveRecordId(record.id);
                          setActiveImageIndex(0);
                        }}
                      />
                    )}
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
                      <div className='mt-2 grid grid-cols-2 gap-1.5'>
                        <Button
                          theme='solid'
                          type='primary'
                          size='small'
                          icon={<Brush size={13} />}
                          className='!col-span-2 !h-7 !rounded-full !bg-slate-950 !px-2 !text-xs !text-white'
                          onClick={() => openRecordAnnotation(record, 0)}
                          disabled={record.status === 'FAILED' || isPreparingAnnotation}
                        >
                          {t('画圈编辑')}
                        </Button>
                        <Button
                          theme='light'
                          type='primary'
                          size='small'
                          className='!h-7 !rounded-full !px-2 !text-xs'
                          onClick={() => createPromptFavorite(record.prompt || '')}
                          disabled={!record.prompt}
                        >
                          {t('收藏提示词')}
                        </Button>
                        <Button
                          theme='light'
                          type='tertiary'
                          size='small'
                          className='!h-7 !rounded-full !px-2 !text-xs'
                          onClick={() => handleDownloadImage(record)}
                          disabled={record.status === 'FAILED'}
                        >
                          {t('下载')}
                        </Button>
                        <Button
                          theme='light'
                          type='primary'
                          size='small'
                          className='!h-7 !rounded-full !px-2 !text-xs'
                          onClick={() => handleReuseImage(record)}
                          disabled={record.status === 'FAILED'}
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
              {remoteTotal > REMOTE_TASK_PAGE_SIZE ? (
                <div className='mt-4 flex justify-end'>
                  <Pagination
                    currentPage={remotePage}
                    pageSize={REMOTE_TASK_PAGE_SIZE}
                    total={remoteTotal}
                    onPageChange={handleRemotePageChange}
                    showTotal
                  />
                </div>
              ) : null}
            </>
          ) : (
            <Empty
              image={<Sparkles size={36} className='text-slate-400' />}
              title={t('暂无历史')}
              description={t('成功生成的图片会保留在这里，方便翻动和重新选择')}
            />
          )}
        </section>
      </div>
      <ImageAnnotationModal
        visible={Boolean(annotationSource)}
        source={annotationSource}
        onCancel={closeAnnotationModal}
        onSave={handleSaveAnnotation}
      />
      <Modal
        visible={favoritesVisible}
        title={null}
        footer={null}
        width={980}
        onCancel={() => setFavoritesVisible(false)}
        className='!rounded-[28px]'
        bodyStyle={{ padding: 0, background: 'transparent' }}
      >
        <div className='overflow-hidden rounded-[28px] border border-white/70 bg-white/70 shadow-[0_24px_80px_rgba(15,23,42,0.16)] backdrop-blur-2xl'>
          <div className='border-b border-white/70 bg-white/45 px-5 py-4'>
            <div className='flex flex-wrap items-center justify-between gap-3'>
              <div>
                <Typography.Title heading={5} className='!mb-1 !text-slate-900'>
                  {t('提示词收藏夹')}
                </Typography.Title>
                <Typography.Text className='!text-sm !text-slate-500'>
                  {t('保存常用提示词，之后可一键填入并继续创作')}
                </Typography.Text>
              </div>
            </div>
          </div>

          <div className='grid max-h-[72vh] gap-4 overflow-y-auto p-4 lg:grid-cols-[minmax(0,1fr)_340px]'>
            <div className='min-w-0 overflow-hidden rounded-[24px] border border-white/70 bg-white/45 p-3 backdrop-blur-xl'>
              <div className='mb-3 flex gap-2'>
                <input
                  value={favoriteSearch}
                  onChange={(event) => setFavoriteSearch(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') loadPromptFavorites(event.currentTarget.value);
                  }}
                  placeholder={t('搜索标题或提示词')}
                  className='h-10 min-w-0 flex-1 rounded-2xl border border-white/70 bg-white/70 px-3 text-sm text-slate-900 shadow-sm outline-none backdrop-blur focus:border-sky-400'
                />
                <Button
                  theme='solid'
                  type='primary'
                  icon={<Search size={15} />}
                  className='!rounded-full'
                  onClick={() => loadPromptFavorites(favoriteSearch)}
                >
                  {t('搜索')}
                </Button>
              </div>

              <Spin spinning={favoritesLoading}>
                {promptFavorites.length > 0 ? (
                  <div className='grid gap-2'>
                    {promptFavorites.map((favorite) => (
                      <div key={favorite.id} className='min-w-0 overflow-hidden rounded-[20px] border border-white/70 bg-white/60 p-3 shadow-sm backdrop-blur'>
                        <div className='mb-2 flex items-start justify-between gap-3'>
                          <div className='min-w-0'>
                            <div className='truncate text-sm font-semibold text-slate-900' title={favorite.title || favorite.prompt}>
                              {favorite.title || createPromptFavoriteTitle(favorite.prompt) || t('未命名收藏')}
                            </div>
                            <div className='mt-1 flex min-w-0 flex-wrap gap-2 text-xs text-slate-400'>
                              {favorite.model ? <span>{favorite.model}</span> : null}
                              {favorite.size ? <span>{favorite.size}</span> : null}
                              {favorite.aspect_ratio ? <span>{favorite.aspect_ratio}</span> : null}
                            </div>
                          </div>
                          <div className='flex shrink-0 gap-1'>
                            <Button
                              theme='borderless'
                              type='tertiary'
                              size='small'
                              icon={<Edit3 size={14} />}
                              onClick={() => editPromptFavorite(favorite)}
                            />
                            <Button
                              theme='borderless'
                              type='danger'
                              size='small'
                              icon={<Trash2 size={14} />}
                              onClick={() => deletePromptFavorite(favorite)}
                            />
                          </div>
                        </div>
                        <p className='mb-3 max-w-full break-all text-sm leading-6 text-slate-600 line-clamp-3'>
                          {favorite.prompt}
                        </p>
                        <Button
                          theme='solid'
                          type='primary'
                          className='!rounded-full'
                          onClick={() => usePromptFavorite(favorite)}
                        >
                          {t('使用这个提示词')}
                        </Button>
                      </div>
                    ))}
                  </div>
                ) : (
                  <Empty
                    image={<Bookmark size={34} className='text-slate-400' />}
                    title={t('暂无收藏')}
                    description={t('可以从当前提示词或历史记录中收藏常用提示词')}
                  />
                )}
              </Spin>
            </div>

            <div className='min-w-0 rounded-[24px] border border-white/70 bg-white/55 p-4 backdrop-blur-xl'>
              <Typography.Text className='!text-xs !font-semibold !uppercase !tracking-[0.2em] !text-slate-400'>
                {editingFavoriteId ? t('编辑收藏') : t('保存当前提示词')}
              </Typography.Text>
              <label className='mt-3 flex flex-col gap-2'>
                <span className='text-xs font-medium text-slate-500'>{t('标题')}</span>
                <input
                  value={favoriteDraftTitle}
                  onChange={(event) => setFavoriteDraftTitle(event.target.value)}
                  placeholder={t('给这个提示词起个名字')}
                  className='h-10 rounded-2xl border border-white/70 bg-white/70 px-3 text-sm text-slate-900 shadow-sm outline-none backdrop-blur focus:border-sky-400'
                />
              </label>
              <label className='mt-3 flex flex-col gap-2'>
                <span className='text-xs font-medium text-slate-500'>{t('提示词')}</span>
                <TextArea
                  value={favoriteDraftPrompt}
                  onChange={setFavoriteDraftPrompt}
                  autosize={{ minRows: 9, maxRows: 14 }}
                  placeholder={t('输入要收藏的提示词')}
                />
              </label>
              <div className='mt-4 flex flex-wrap gap-2'>
                <Button
                  theme='solid'
                  type='primary'
                  icon={<Bookmark size={15} />}
                  className='!rounded-full'
                  onClick={saveFavoriteDraft}
                >
                  {editingFavoriteId ? t('保存修改') : t('保存到收藏夹')}
                </Button>
                <Button
                  theme='light'
                  type='tertiary'
                  className='!rounded-full'
                  onClick={() => resetFavoriteDraft('')}
                >
                  {t('清空')}
                </Button>
              </div>
            </div>
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default AIImage;
