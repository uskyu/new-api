import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Button, Empty, Input, Modal, Spin, Tag, TextArea, Typography } from '@douyinfe/semi-ui';
import {
  Brush,
  Check,
  Download,
  ExternalLink,
  ImagePlus,
  Layers,
  Loader2,
  RefreshCw,
  Sparkles,
  Trash2,
  Wand2,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import useAiImageState from '../../hooks/ai-image/useAiImageState';
import {
  DEFAULT_ECOMMERCE_TEMPLATE_KEY,
  ECOMMERCE_IMAGE_TEMPLATES,
} from '../../constants/ai-ecommerce-template.constants';
import { API, getUserIdFromLocalStorage, showError, showSuccess, timestamp2string } from '../../helpers';

const MAX_REFERENCE_IMAGES = 5;
const ANNOTATION_CANVAS_MAX_SIDE = 1400;
const DEFAULT_OPENAI_SIZE = '1024x1024';
const OPENAI_SIZE_OPTIONS = [
  { value: '1024x1024', label: '1024x1024 母版' },
  { value: '1536x1024', label: '1536x1024 横版' },
  { value: '1024x1536', label: '1024x1536 竖版' },
  { value: '2048x2048', label: '2048x2048 2K 方图' },
];

const terminalStatuses = new Set(['SUCCEEDED', 'FAILED', 'MOTHER_FAILED']);
const runningStatuses = new Set([
  'MOTHER_PENDING',
  'MOTHER_PROCESSING',
  'SEGMENTS_PENDING',
  'SEGMENTS_PROCESSING',
]);

const statusColorMap = {
  MOTHER_PENDING: 'light-blue',
  MOTHER_PROCESSING: 'blue',
  WAITING_CONFIRM: 'orange',
  MOTHER_FAILED: 'red',
  SEGMENTS_PENDING: 'light-blue',
  SEGMENTS_PROCESSING: 'blue',
  SUCCEEDED: 'green',
  FAILED: 'red',
  PENDING: 'light-blue',
  PROCESSING: 'blue',
};

const statusLabelMap = {
  MOTHER_PENDING: '\u6bcd\u7248\u6392\u961f\u4e2d',
  MOTHER_PROCESSING: '\u6bcd\u7248\u751f\u6210\u4e2d',
  WAITING_CONFIRM: '\u7b49\u5f85\u786e\u8ba4',
  MOTHER_FAILED: '\u6bcd\u7248\u751f\u6210\u5931\u8d25',
  SEGMENTS_PENDING: '\u8be6\u60c5\u6bb5\u6392\u961f\u4e2d',
  SEGMENTS_PROCESSING: '\u8be6\u60c5\u6bb5\u751f\u6210\u4e2d',
  SUCCEEDED: '\u751f\u6210\u6210\u529f',
  FAILED: '\u751f\u6210\u5931\u8d25',
  PENDING: '\u6392\u961f\u4e2d',
  PROCESSING: '\u5904\u7406\u4e2d',
};

const ANNOTATION_COLORS = [
  { value: '#ef4444', label: '\u7ea2\u8272' },
  { value: '#2563eb', label: '\u84dd\u8272' },
  { value: '#16a34a', label: '\u7eff\u8272' },
  { value: '#f59e0b', label: '\u9ec4\u8272' },
  { value: '#8b5cf6', label: '\u7d2b\u8272' },
  { value: '#f97316', label: '\u6a59\u8272' },
];

const getStatusLabel = (status, t) => {
  if (!status) return '-';
  const normalizedStatus = String(status).trim().toUpperCase();
  return t(statusLabelMap[normalizedStatus] || normalizedStatus);
};

const readFileAsDataUrl = (file) =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '');
    reader.onerror = () => reject(reader.error || new Error('failed to read file'));
    reader.readAsDataURL(file);
  });

const downloadImage = (imageUrl, filename) => {
  if (!imageUrl) return;
  const anchor = document.createElement('a');
  anchor.href = imageUrl;
  anchor.download = filename;
  anchor.click();
};

const openImage = (imageUrl) => {
  if (!imageUrl) return;
  window.open(imageUrl, '_blank', 'noopener,noreferrer');
};

const createAnnotationId = () =>
  `annotation-${Date.now()}-${Math.random().toString(16).slice(2)}`;

const getAnnotationColorLabel = (color) =>
  ANNOTATION_COLORS.find((item) => item.value === color)?.label || '\u6807\u6ce8';

const getAnnotationLabel = (annotation, index) =>
  annotation?.label || `${getAnnotationColorLabel(annotation?.color)}\u533a\u57df ${index + 1}`;

const getAnnotationPromptCount = (annotations = []) =>
  annotations.filter((annotation) => String(annotation?.prompt || '').trim()).length;

const buildRedrawPromptWithAnnotations = (basePrompt, annotations = []) => {
  const normalizedBasePrompt = String(basePrompt || '').trim();
  const annotationLines = annotations
    .map((annotation, index) => ({
      label: getAnnotationLabel(annotation, index),
      prompt: String(annotation?.prompt || '').trim(),
    }))
    .filter((item) => item.prompt)
    .map((item) => `${item.label}\uff1a${item.prompt}`);

  if (annotationLines.length === 0) {
    return normalizedBasePrompt;
  }

  return [
    normalizedBasePrompt || '\u8bf7\u6839\u636e\u5f69\u8272\u6807\u6ce8\u56fe\u5bf9\u5f53\u524d\u8be6\u60c5\u6bb5\u8fdb\u884c\u5c40\u90e8\u91cd\u7ed8\u3002',
    '',
    '\u5c40\u90e8\u91cd\u7ed8\u8981\u6c42\uff1a',
    ...annotationLines.map((line, index) => `${index + 1}. ${line}`),
    '\u8bf7\u4f18\u5148\u6839\u636e\u672c\u6b21\u4e0a\u4f20\u7684\u5f69\u8272\u5708\u9009\u6807\u6ce8\u56fe\u8bc6\u522b\u4fee\u6539\u8303\u56f4\uff0c\u4fdd\u7559\u672a\u6807\u6ce8\u533a\u57df\u7684\u7248\u5f0f\u548c\u5546\u54c1\u4fe1\u606f\u3002',
  ].join('\n');
};

const blobToDataUrl = (blob) =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '');
    reader.onerror = () => reject(reader.error || new Error('failed to read blob'));
    reader.readAsDataURL(blob);
  });

const loadImageUrlAsDataUrl = async (imageUrl) => {
  if (!imageUrl) return '';
  if (imageUrl.startsWith('data:image/')) return imageUrl;
  const response = await fetch(imageUrl, {
    headers: imageUrl.startsWith('/api/')
      ? { 'New-Api-User': getUserIdFromLocalStorage() }
      : undefined,
  });
  if (!response.ok) throw new Error(`fetch failed: ${response.status}`);
  return blobToDataUrl(await response.blob());
};

const EcommerceAnnotationModal = ({
  visible,
  sourceUrl,
  sourceLabel,
  initialAnnotations,
  onCancel,
  onSave,
}) => {
  const { t } = useTranslation();
  const canvasRef = useRef(null);
  const activeAnnotationIdRef = useRef(null);
  const [sourceDataUrl, setSourceDataUrl] = useState('');
  const [annotations, setAnnotations] = useState([]);
  const [selectedColor, setSelectedColor] = useState(ANNOTATION_COLORS[0].value);
  const [isDrawing, setIsDrawing] = useState(false);
  const [loading, setLoading] = useState(false);

  const renderCanvas = useCallback((dataUrl, nextAnnotations = []) => {
    const canvas = canvasRef.current;
    if (!canvas || !dataUrl) return;
    const ctx = canvas.getContext('2d');
    const image = new Image();
    image.onload = () => {
      const scale = Math.min(
        1,
        ANNOTATION_CANVAS_MAX_SIDE / image.naturalWidth,
        ANNOTATION_CANVAS_MAX_SIDE / image.naturalHeight,
      );
      canvas.width = Math.max(1, Math.round(image.naturalWidth * scale));
      canvas.height = Math.max(1, Math.round(image.naturalHeight * scale));
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      ctx.drawImage(image, 0, 0, canvas.width, canvas.height);
      ctx.lineWidth = Math.max(6, Math.round(Math.min(canvas.width, canvas.height) * 0.008));
      ctx.lineCap = 'round';
      ctx.lineJoin = 'round';
      nextAnnotations.forEach((annotation, annotationIndex) => {
        const points = Array.isArray(annotation.points) ? annotation.points : [];
        if (points.length < 2) return;
        ctx.strokeStyle = annotation.color || ANNOTATION_COLORS[0].value;
        ctx.beginPath();
        points.forEach((point, index) => {
          const x = point.x * canvas.width;
          const y = point.y * canvas.height;
          if (index === 0) ctx.moveTo(x, y);
          else ctx.lineTo(x, y);
        });
        ctx.stroke();
        const firstPoint = points[0];
        ctx.fillStyle = annotation.color || ANNOTATION_COLORS[0].value;
        ctx.beginPath();
        ctx.arc(firstPoint.x * canvas.width, firstPoint.y * canvas.height, 18, 0, Math.PI * 2);
        ctx.fill();
        ctx.fillStyle = '#fff';
        ctx.font = 'bold 18px sans-serif';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(String(annotationIndex + 1), firstPoint.x * canvas.width, firstPoint.y * canvas.height);
      });
    };
    image.src = dataUrl;
  }, []);

  useEffect(() => {
    if (!visible) return undefined;
    let cancelled = false;
    setLoading(true);
    setSourceDataUrl('');
    setAnnotations(Array.isArray(initialAnnotations) ? initialAnnotations : []);
    setSelectedColor(ANNOTATION_COLORS[0].value);
    loadImageUrlAsDataUrl(sourceUrl)
      .then((dataUrl) => {
        if (cancelled) return;
        setSourceDataUrl(dataUrl);
        renderCanvas(dataUrl, Array.isArray(initialAnnotations) ? initialAnnotations : []);
      })
      .catch(() => {
        if (!cancelled) showError(t('\u52a0\u8f7d\u56fe\u7247\u5931\u8d25\uff0c\u8bf7\u7a0d\u540e\u91cd\u8bd5'));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [initialAnnotations, renderCanvas, sourceUrl, t, visible]);

  useEffect(() => {
    renderCanvas(sourceDataUrl, annotations);
  }, [annotations, renderCanvas, sourceDataUrl]);

  const getPoint = (event) => {
    const canvas = canvasRef.current;
    if (!canvas) return null;
    const rect = canvas.getBoundingClientRect();
    return {
      x: Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width)),
      y: Math.min(1, Math.max(0, (event.clientY - rect.top) / rect.height)),
    };
  };

  const handlePointerDown = (event) => {
    if (!sourceDataUrl || loading) return;
    const point = getPoint(event);
    if (!point) return;
    event.currentTarget.setPointerCapture?.(event.pointerId);
    const annotation = {
      id: createAnnotationId(),
      color: selectedColor,
      points: [point],
      prompt: '',
    };
    activeAnnotationIdRef.current = annotation.id;
    setAnnotations((previous) => [...previous, annotation]);
    setIsDrawing(true);
  };

  const handlePointerMove = (event) => {
    if (!isDrawing || !activeAnnotationIdRef.current) return;
    const point = getPoint(event);
    if (!point) return;
    setAnnotations((previous) =>
      previous.map((annotation) =>
        annotation.id === activeAnnotationIdRef.current
          ? { ...annotation, points: [...annotation.points, point] }
          : annotation,
      ),
    );
  };

  const handlePointerUp = () => {
    if (!isDrawing) return;
    setIsDrawing(false);
    const activeId = activeAnnotationIdRef.current;
    activeAnnotationIdRef.current = null;
    setAnnotations((previous) => {
      const activeAnnotation = previous.find((annotation) => annotation.id === activeId);
      const nextAnnotations = previous.filter(
        (annotation) => annotation.id !== activeId || annotation.points.length > 1,
      );
      if (activeAnnotation?.points?.length > 1) {
        const colorIndex = ANNOTATION_COLORS.findIndex((item) => item.value === activeAnnotation.color);
        const nextColor = ANNOTATION_COLORS[(colorIndex + 1 + ANNOTATION_COLORS.length) % ANNOTATION_COLORS.length]?.value || ANNOTATION_COLORS[0].value;
        setSelectedColor(nextColor);
      }
      return nextAnnotations;
    });
  };

  const updateAnnotationPrompt = (annotationId, value) => {
    setAnnotations((previous) =>
      previous.map((annotation) =>
        annotation.id === annotationId ? { ...annotation, prompt: value } : annotation,
      ),
    );
  };

  const createAnnotatedDataUrl = (nextAnnotations) =>
    new Promise((resolve, reject) => {
      const image = new Image();
      image.onload = () => {
        const outputCanvas = document.createElement('canvas');
        const scale = Math.min(
          1,
          ANNOTATION_CANVAS_MAX_SIDE / image.naturalWidth,
          ANNOTATION_CANVAS_MAX_SIDE / image.naturalHeight,
        );
        outputCanvas.width = Math.max(1, Math.round(image.naturalWidth * scale));
        outputCanvas.height = Math.max(1, Math.round(image.naturalHeight * scale));
        const ctx = outputCanvas.getContext('2d');
        ctx.drawImage(image, 0, 0, outputCanvas.width, outputCanvas.height);
        ctx.lineWidth = Math.max(6, Math.round(Math.min(outputCanvas.width, outputCanvas.height) * 0.008));
        ctx.lineCap = 'round';
        ctx.lineJoin = 'round';
        nextAnnotations.forEach((annotation, annotationIndex) => {
          const points = annotation.points || [];
          if (points.length < 2) return;
          ctx.strokeStyle = annotation.color;
          ctx.beginPath();
          points.forEach((point, index) => {
            const x = point.x * outputCanvas.width;
            const y = point.y * outputCanvas.height;
            if (index === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
          });
          ctx.stroke();
          const firstPoint = points[0];
          ctx.fillStyle = annotation.color;
          ctx.beginPath();
          ctx.arc(firstPoint.x * outputCanvas.width, firstPoint.y * outputCanvas.height, 18, 0, Math.PI * 2);
          ctx.fill();
          ctx.fillStyle = '#fff';
          ctx.font = 'bold 18px sans-serif';
          ctx.textAlign = 'center';
          ctx.textBaseline = 'middle';
          ctx.fillText(String(annotationIndex + 1), firstPoint.x * outputCanvas.width, firstPoint.y * outputCanvas.height);
        });
        resolve(outputCanvas.toDataURL('image/png'));
      };
      image.onerror = reject;
      image.src = sourceDataUrl;
    });

  const handleSave = async () => {
    const cleanedAnnotations = annotations
      .map((annotation, index) => ({
        ...annotation,
        label: getAnnotationLabel(annotation, index),
        prompt: String(annotation.prompt || '').trim(),
      }))
      .filter((annotation) => annotation.points?.length > 1 && annotation.prompt);
    if (cleanedAnnotations.length === 0) {
      showError(t('\u8bf7\u5148\u753b\u51fa\u9700\u8981\u4fee\u6539\u7684\u533a\u57df\uff0c\u5e76\u586b\u5199\u5bf9\u5e94\u8981\u6c42'));
      return;
    }
    try {
      const dataUrl = await createAnnotatedDataUrl(cleanedAnnotations);
      onSave?.({ dataUrl, annotations: cleanedAnnotations });
    } catch {
      showError(t('\u4fdd\u5b58\u6807\u6ce8\u56fe\u5931\u8d25\uff0c\u8bf7\u91cd\u8bd5'));
    }
  };

  return (
    <Modal
      title={t('\u753b\u5708\u6807\u6ce8\u5c40\u90e8\u91cd\u7ed8')}
      visible={visible}
      onCancel={onCancel}
      width={1080}
      footer={null}
    >
      <div className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_330px]'>
        <div className='min-h-[360px] rounded-[24px] bg-slate-950 p-3'>
          <canvas
            ref={canvasRef}
            className='mx-auto block max-h-[68vh] max-w-full touch-none rounded-[18px] bg-white'
            onPointerDown={handlePointerDown}
            onPointerMove={handlePointerMove}
            onPointerUp={handlePointerUp}
            onPointerCancel={handlePointerUp}
          />
          {loading && <div className='py-6 text-center text-sm text-slate-300'>{t('\u56fe\u7247\u52a0\u8f7d\u4e2d...')}</div>}
        </div>
        <div className='flex flex-col gap-3'>
          <div>
            <Typography.Title heading={6} style={{ margin: 0 }}>{sourceLabel || t('\u5f53\u524d\u8be6\u60c5\u6bb5')}</Typography.Title>
            <Typography.Text type='secondary'>{t('\u6bcf\u753b\u5b8c\u4e00\u5904\u4f1a\u81ea\u52a8\u5207\u6362\u989c\u8272\uff0c\u53f3\u4fa7\u586b\u5199\u8be5\u533a\u57df\u7684\u4fee\u6539\u8981\u6c42\u3002')}</Typography.Text>
          </div>
          <div className='flex flex-wrap gap-2'>
            {ANNOTATION_COLORS.map((item) => (
              <button
                key={item.value}
                type='button'
                title={t(item.label)}
                className={`h-8 w-8 rounded-full border-2 ${selectedColor === item.value ? 'border-slate-950' : 'border-white'} shadow`}
                style={{ backgroundColor: item.value }}
                onClick={() => setSelectedColor(item.value)}
              />
            ))}
          </div>
          <div className='max-h-[44vh] overflow-auto pr-1'>
            {annotations.length > 0 ? annotations.map((annotation, index) => (
              <div key={annotation.id} className='mb-3 rounded-[18px] border border-slate-100 bg-slate-50 p-3'>
                <div className='mb-2 flex items-center justify-between gap-2'>
                  <span className='inline-flex items-center gap-2 text-sm font-medium text-slate-800'>
                    <span className='h-3 w-3 rounded-full' style={{ backgroundColor: annotation.color }} />
                    {getAnnotationLabel(annotation, index)}
                  </span>
                  <Button size='small' type='danger' theme='borderless' icon={<Trash2 size={14} />} onClick={() => setAnnotations((previous) => previous.filter((item) => item.id !== annotation.id))} />
                </div>
                <TextArea
                  value={annotation.prompt}
                  onChange={(value) => updateAnnotationPrompt(annotation.id, value)}
                  autosize={{ minRows: 2, maxRows: 4 }}
                  placeholder={t('\u8fd9\u4e2a\u533a\u57df\u600e\u4e48\u6539\uff1f')}
                />
              </div>
            )) : (
              <Empty title={t('\u6682\u65e0\u6807\u6ce8')} description={t('\u5728\u56fe\u7247\u4e0a\u5708\u51fa\u8981\u6539\u7684\u4f4d\u7f6e')} />
            )}
          </div>
          <div className='flex gap-2'>
            <Button className='!rounded-full' onClick={() => setAnnotations([])}>{t('\u6e05\u7a7a\u6807\u6ce8')}</Button>
            <Button theme='solid' type='primary' className='!rounded-full' icon={<Check size={16} />} onClick={handleSave}>
              {t('\u4fdd\u5b58\u5c40\u90e8\u8981\u6c42')}
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  );
};

const AIEcommerceTemplate = () => {
  const { t } = useTranslation();
  const {
    ready,
    storageError,
    groups,
    models,
    selectedModel,
    selectedGroup,
    setSelectedModel,
    setSelectedGroup,
  } = useAiImageState();
  const [selectedTemplateKey, setSelectedTemplateKey] = useState(DEFAULT_ECOMMERCE_TEMPLATE_KEY);
  const [referenceImages, setReferenceImages] = useState([]);
  const [productName, setProductName] = useState('');
  const [productType, setProductType] = useState('');
  const [platform, setPlatform] = useState('');
  const [sellingPoints, setSellingPoints] = useState('');
  const [extraRequirements, setExtraRequirements] = useState('');
  const [openaiSize, setOpenaiSize] = useState(DEFAULT_OPENAI_SIZE);
  const [workflow, setWorkflow] = useState(null);
  const [creatingWorkflow, setCreatingWorkflow] = useState(false);
  const [confirmingWorkflow, setConfirmingWorkflow] = useState(false);
  const [polling, setPolling] = useState(false);
  const [redrawSegment, setRedrawSegment] = useState(null);
  const [redrawPrompt, setRedrawPrompt] = useState('');
  const [redrawAnnotatedImage, setRedrawAnnotatedImage] = useState('');
  const [redrawAnnotations, setRedrawAnnotations] = useState([]);
  const [redrawAnnotationVisible, setRedrawAnnotationVisible] = useState(false);
  const [redrawing, setRedrawing] = useState(false);
  const [historyItems, setHistoryItems] = useState([]);
  const [historyLoading, setHistoryLoading] = useState(false);

  const selectedTemplate = useMemo(
    () => ECOMMERCE_IMAGE_TEMPLATES.find((template) => template.key === selectedTemplateKey) || ECOMMERCE_IMAGE_TEMPLATES[0],
    [selectedTemplateKey],
  );
  const modelOptions = models.map((model) => ({ value: model.value, label: model.label }));
  const groupOptions = groups.map((group) => ({ value: group.value, label: group.fullLabel || group.label }));
  const effectiveModel = selectedModel || modelOptions[0]?.value || '';
  const workflowRunning = workflow?.status && runningStatuses.has(workflow.status);
  const canConfirm = workflow?.status === 'WAITING_CONFIRM';

  const applyTemplate = (template) => {
    setSelectedTemplateKey(template.key);
    setProductType(template.productType);
    setPlatform(template.platform);
    setSellingPoints(template.sellingPoints);
  };

  const loadHistory = async (silent = false) => {
    if (!silent) setHistoryLoading(true);
    try {
      const res = await API.get('/api/ai-ecommerce/workflows?p=1&page_size=20');
      if (!res.data?.success) throw new Error(res.data?.message || 'failed to load history');
      const items = Array.isArray(res.data?.data?.items) ? res.data.data.items : [];
      setHistoryItems(items);
    } catch (error) {
      if (!silent) showError(error?.response?.data?.message || error.message || t('加载生成历史失败'));
    } finally {
      if (!silent) setHistoryLoading(false);
    }
  };

  const applyWorkflowSnapshot = (nextWorkflow) => {
    if (!nextWorkflow) return;
    setWorkflow(nextWorkflow);
    if (nextWorkflow.template_key) setSelectedTemplateKey(nextWorkflow.template_key);
    setProductName(nextWorkflow.product_name || '');
    setProductType(nextWorkflow.product_type || '');
    setPlatform(nextWorkflow.platform || '');
    setSellingPoints(nextWorkflow.selling_points || '');
    setExtraRequirements(nextWorkflow.extra_requirements || '');
    setOpenaiSize(nextWorkflow.size || DEFAULT_OPENAI_SIZE);
    if (nextWorkflow.model) setSelectedModel(nextWorkflow.model);
    if (nextWorkflow.group !== undefined) setSelectedGroup(nextWorkflow.group || '');
  };

  const refreshWorkflow = async (workflowId = workflow?.workflow_id, silent = false) => {
    if (!workflowId) return null;
    if (!silent) setPolling(true);
    try {
      const res = await API.get(`/api/ai-ecommerce/workflows/${workflowId}`);
      if (!res.data?.success) throw new Error(res.data?.message || 'failed to load workflow');
      setWorkflow(res.data.data || null);
      loadHistory(true);
      return res.data.data || null;
    } catch (error) {
      if (!silent) showError(error?.response?.data?.message || error.message || t('加载工作流失败'));
      return null;
    } finally {
      if (!silent) setPolling(false);
    }
  };

  useEffect(() => {
    if (!workflow?.workflow_id || terminalStatuses.has(workflow.status) || workflow.status === 'WAITING_CONFIRM') {
      return undefined;
    }
    const timer = window.setInterval(() => {
      refreshWorkflow(workflow.workflow_id, true);
    }, 3000);
    return () => window.clearInterval(timer);
  }, [workflow?.workflow_id, workflow?.status]);

  useEffect(() => {
    if (ready) {
      loadHistory(true);
    }
  }, [ready]);

  const handleAddReferenceImages = async (files) => {
    const nextFiles = Array.from(files || []).filter(Boolean);
    if (nextFiles.length === 0) return;
    const remaining = MAX_REFERENCE_IMAGES - referenceImages.length;
    if (remaining <= 0) {
      showError(t('最多支持 {{count}} 张参考图', { count: MAX_REFERENCE_IMAGES }));
      return;
    }
    const acceptedFiles = nextFiles.slice(0, remaining);
    const nextImages = await Promise.all(
      acceptedFiles.map(async (file) => ({
        id: `reference-${Date.now()}-${Math.random().toString(16).slice(2)}`,
        file,
        name: file.name,
        previewUrl: URL.createObjectURL(file),
        dataUrl: await readFileAsDataUrl(file),
      })),
    );
    setReferenceImages((previous) => [...previous, ...nextImages]);
    if (acceptedFiles.length < nextFiles.length) {
      showError(t('最多支持 {{count}} 张参考图', { count: MAX_REFERENCE_IMAGES }));
    }
  };

  const removeReferenceImage = (imageId) => {
    setReferenceImages((previous) => {
      const target = previous.find((image) => image.id === imageId);
      if (target?.previewUrl?.startsWith('blob:')) URL.revokeObjectURL(target.previewUrl);
      return previous.filter((image) => image.id !== imageId);
    });
  };

  const createWorkflow = async () => {
    if (!effectiveModel) {
      showError(t('请选择模型'));
      return;
    }
    if (referenceImages.length === 0) {
      showError(t('请先上传商品参考图'));
      return;
    }
    if (!selectedModel && effectiveModel) setSelectedModel(effectiveModel);
    setCreatingWorkflow(true);
    setWorkflow(null);
    try {
      const res = await API.post('/api/ai-ecommerce/workflows', {
        model: effectiveModel,
        group: selectedGroup,
        size: openaiSize,
        template: {
          key: selectedTemplate.key,
          name: selectedTemplate.name,
          style_prompt: selectedTemplate.stylePrompt,
          modules: selectedTemplate.modules,
        },
        product_name: productName,
        product_type: productType || selectedTemplate.productType,
        platform: platform || selectedTemplate.platform,
        selling_points: sellingPoints || selectedTemplate.sellingPoints,
        extra_requirements: extraRequirements,
        reference_images: referenceImages.map((image) => image.dataUrl).filter(Boolean),
      });
      if (!res.data?.success) throw new Error(res.data?.message || 'failed to create workflow');
      setWorkflow(res.data.data || null);
      loadHistory(true);
      showSuccess(t('母版任务已提交，生成完成后可确认继续'));
    } catch (error) {
      showError(error?.response?.data?.message || error.message || t('提交工作流失败'));
    } finally {
      setCreatingWorkflow(false);
    }
  };

  const confirmWorkflow = async () => {
    if (!workflow?.workflow_id) return;
    setConfirmingWorkflow(true);
    try {
      const res = await API.post(`/api/ai-ecommerce/workflows/${workflow.workflow_id}/confirm`);
      if (!res.data?.success) throw new Error(res.data?.message || 'failed to confirm workflow');
      setWorkflow(res.data.data || null);
      loadHistory(true);
      showSuccess(t('已确认母版，开始异步生成详情段'));
    } catch (error) {
      showError(error?.response?.data?.message || error.message || t('确认工作流失败'));
    } finally {
      setConfirmingWorkflow(false);
    }
  };

  const submitRedraw = async () => {
    if (!workflow?.workflow_id || !redrawSegment?.segment_key) return;
    const prompt = buildRedrawPromptWithAnnotations(redrawPrompt, redrawAnnotations);
    if (!prompt && !redrawAnnotatedImage) {
      showError(t('请输入重绘要求，或先画圈标注局部修改要求'));
      return;
    }
    setRedrawing(true);
    try {
      const res = await API.post(
        `/api/ai-ecommerce/workflows/${workflow.workflow_id}/segments/${redrawSegment.segment_key}/redraw`,
        {
          prompt,
          annotated_image: redrawAnnotatedImage || undefined,
        },
      );
      if (!res.data?.success) throw new Error(res.data?.message || 'failed to redraw segment');
      setWorkflow(res.data.data || null);
      loadHistory(true);
      setRedrawSegment(null);
      setRedrawPrompt('');
      setRedrawAnnotatedImage('');
      setRedrawAnnotations([]);
      showSuccess(t('重绘任务已提交'));
    } catch (error) {
      showError(error?.response?.data?.message || error.message || t('提交重绘失败'));
    } finally {
      setRedrawing(false);
    }
  };

  const resetWorkflow = () => {
    setWorkflow(null);
    setRedrawSegment(null);
    setRedrawPrompt('');
    setRedrawAnnotatedImage('');
    setRedrawAnnotations([]);
  };

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
          title={t('AI 电商绘图模板初始化失败')}
          description={t('当前浏览器可能禁用了本地存储，请稍后重试')}
        />
      </div>
    );
  }

  return (
    <div className='mt-[64px] min-h-[calc(100vh-64px)] bg-[#eef2f7] px-2 py-3 sm:px-3 lg:px-4'>
      <div className='mx-auto grid min-h-[calc(100vh-88px)] w-full gap-4 xl:grid-cols-[minmax(0,1fr)_360px]'>
        <main className='flex min-w-0 flex-col gap-4 rounded-[32px] border border-white/70 bg-white/75 p-4 shadow-[0_24px_80px_rgba(15,23,42,0.08)] backdrop-blur-xl sm:p-5'>
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <div className='flex items-center gap-3'>
              <div className='flex h-12 w-12 items-center justify-center rounded-[18px] bg-amber-50 text-amber-600 shadow-[0_16px_36px_rgba(15,23,42,0.08)]'>
                <Layers size={22} />
              </div>
              <div>
                <Typography.Title heading={4} className='!mb-0 !text-slate-900'>
                  {t('AI 电商绘图模板')}
                </Typography.Title>
                <Typography.Text className='!text-sm !text-slate-500'>
                  {t('后端异步生成母版与三段详情图，支持刷新恢复和单段重绘')}
                </Typography.Text>
              </div>
            </div>
            <div className='flex flex-wrap gap-2'>
              {workflow ? <Tag color={statusColorMap[workflow.status] || 'grey'}>{getStatusLabel(workflow.status, t)}</Tag> : null}
              <Button icon={<RefreshCw size={16} />} className='!rounded-full' onClick={() => refreshWorkflow(workflow?.workflow_id)} loading={polling} disabled={!workflow?.workflow_id}>
                {t('刷新状态')}
              </Button>
              <Button icon={<RefreshCw size={16} />} className='!rounded-full' onClick={resetWorkflow}>
                {t('新建流程')}
              </Button>
              <Button
                theme='solid'
                type='primary'
                icon={creatingWorkflow ? <Loader2 className='animate-spin' size={16} /> : <Wand2 size={16} />}
                className='!rounded-full !bg-slate-900 !text-white'
                loading={creatingWorkflow}
                disabled={workflowRunning || confirmingWorkflow}
                onClick={createWorkflow}
              >
                {t('生成第一版母版')}
              </Button>
            </div>
          </div>

          <section className='grid gap-4 lg:grid-cols-[420px_minmax(0,1fr)]'>
            <div className='flex flex-col gap-4'>
              <div className='rounded-[28px] border border-slate-200 bg-slate-50/70 p-4'>
                <Typography.Text className='!text-xs !font-semibold !uppercase !tracking-[0.2em] !text-slate-400'>
                  {t('基础设置')}
                </Typography.Text>
                <div className='mt-3 grid gap-3'>
                  <label className='flex flex-col gap-2'>
                    <span className='text-xs font-medium uppercase tracking-[0.18em] text-slate-400'>{t('分组')}</span>
                    <select value={selectedGroup} onChange={(event) => setSelectedGroup(event.target.value)} className='h-11 rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'>
                      {groupOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                    </select>
                  </label>
                  <label className='flex flex-col gap-2'>
                    <span className='text-xs font-medium uppercase tracking-[0.18em] text-slate-400'>{t('绘图模型')}</span>
                    <select value={selectedModel} onChange={(event) => setSelectedModel(event.target.value)} className='h-11 rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'>
                      <option value=''>{t('选择模型')}</option>
                      {modelOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                    </select>
                  </label>
                  <label className='flex flex-col gap-2'>
                    <span className='text-xs font-medium uppercase tracking-[0.18em] text-slate-400'>{t('母版尺寸')}</span>
                    <select value={openaiSize} onChange={(event) => setOpenaiSize(event.target.value)} className='h-11 rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'>
                      {OPENAI_SIZE_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                    </select>
                  </label>
                </div>
              </div>

              <div className='rounded-[28px] border border-slate-200 bg-slate-50/70 p-4'>
                <div className='flex items-center justify-between gap-3'>
                  <div>
                    <Typography.Text className='!text-xs !font-semibold !uppercase !tracking-[0.2em] !text-slate-400'>
                      {t('商品参考图')}
                    </Typography.Text>
                    <p className='mt-1 text-sm text-slate-500'>{t('最多 5 张，提交后会上传到对象存储作为异步参考图')}</p>
                  </div>
                  <label className='inline-flex cursor-pointer items-center gap-2 rounded-full border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm'>
                    <ImagePlus size={16} />
                    <span>{t('上传')}</span>
                    <input type='file' accept='image/*' multiple className='hidden' onChange={(event) => { handleAddReferenceImages(event.target.files); event.target.value = ''; }} />
                  </label>
                </div>
                {referenceImages.length > 0 ? (
                  <div className='mt-3 flex flex-wrap gap-2'>
                    {referenceImages.map((image) => (
                      <div key={image.id} className='group relative overflow-hidden rounded-[18px] border border-white bg-white'>
                        <img src={image.previewUrl} alt={image.name} className='h-24 w-24 object-cover' />
                        <button type='button' className='absolute right-1.5 top-1.5 flex h-6 w-6 items-center justify-center rounded-full bg-black/70 text-white' onClick={() => removeReferenceImage(image.id)}>
                          <Trash2 size={14} />
                        </button>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className='mt-3 rounded-[20px] border border-dashed border-slate-300 bg-white/60 p-5 text-center text-sm text-slate-500'>
                    {t('先上传商品图，再提交异步母版任务')}
                  </div>
                )}
              </div>

              <div className='rounded-[28px] border border-slate-200 bg-slate-50/70 p-4'>
                <Typography.Text className='!text-xs !font-semibold !uppercase !tracking-[0.2em] !text-slate-400'>
                  {t('商品信息')}
                </Typography.Text>
                <div className='mt-3 grid gap-3'>
                  <Input value={productName} onChange={setProductName} placeholder={t('商品名称，例如：男士商务机械表')} />
                  <Input value={productType} onChange={setProductType} placeholder={t('商品品类')} />
                  <Input value={platform} onChange={setPlatform} placeholder={t('目标平台，例如：淘宝 / 京东 / 小红书')} />
                  <TextArea value={sellingPoints} onChange={setSellingPoints} autosize={{ minRows: 3, maxRows: 5 }} placeholder={t('核心卖点，一行或一句话都可以')} />
                  <TextArea value={extraRequirements} onChange={setExtraRequirements} autosize={{ minRows: 3, maxRows: 5 }} placeholder={t('补充要求，例如配色、禁用元素、目标人群')} />
                </div>
              </div>

              <div className='rounded-[28px] border border-slate-200 bg-slate-50/70 p-4'>
                <div className='flex items-center justify-between gap-3'>
                  <div>
                    <Typography.Text className='!text-xs !font-semibold !uppercase !tracking-[0.2em] !text-slate-400'>
                      {t('生成历史')}
                    </Typography.Text>
                    <p className='mt-1 text-sm text-slate-500'>{t('点击历史记录可恢复母版、分段和拼接长图视图')}</p>
                  </div>
                  <Button size='small' className='!rounded-full' loading={historyLoading} onClick={() => loadHistory()}>
                    {t('刷新')}
                  </Button>
                </div>
                <div className='mt-3 max-h-[360px] space-y-2 overflow-auto pr-1'>
                  {historyItems.length > 0 ? historyItems.map((item) => {
                    const active = item.workflow_id === workflow?.workflow_id;
                    return (
                      <button
                        key={item.workflow_id}
                        type='button'
                        onClick={() => applyWorkflowSnapshot(item)}
                        className={`w-full rounded-[20px] border p-3 text-left transition ${active ? 'border-slate-900 bg-white shadow-sm' : 'border-slate-200 bg-white/70 hover:border-slate-300'}`}
                      >
                        <div className='flex items-start justify-between gap-2'>
                          <div className='min-w-0'>
                            <div className='truncate text-sm font-semibold text-slate-900'>
                              {item.product_name || item.template_name || t('未命名电商工作流')}
                            </div>
                            <div className='mt-1 truncate text-xs text-slate-500'>
                              {item.template_name || item.template_key || '-'} · {timestamp2string(item.created_at)}
                            </div>
                          </div>
                          <Tag color={statusColorMap[item.status] || 'grey'}>{getStatusLabel(item.status, t)}</Tag>
                        </div>
                        <div className='mt-2 flex items-center gap-2 text-xs text-slate-500'>
                          <span>{t('母版')}: {item.mother_result_url ? t('已生成') : t('未完成')}</span>
                          <span>{t('长图')}: {item.assembled_url ? t('已生成') : t('未完成')}</span>
                        </div>
                      </button>
                    );
                  }) : (
                    <div className='rounded-[20px] border border-dashed border-slate-300 bg-white/60 p-5 text-center text-sm text-slate-500'>
                      {historyLoading ? t('正在加载历史') : t('暂无生成历史')}
                    </div>
                  )}
                </div>
              </div>
            </div>

            <div className='flex min-w-0 flex-col gap-4'>
              <div className='rounded-[28px] border border-slate-200 bg-white p-4'>
                <div className='flex items-center justify-between gap-3'>
                  <div>
                    <Typography.Title heading={6} className='!mb-0 !text-slate-900'>{t('第一阶段：母版确认')}</Typography.Title>
                    <Typography.Text className='!text-sm !text-slate-500'>{t('母版生成完成后，确认继续三段详情图异步生成')}</Typography.Text>
                  </div>
                  <Button
                    theme='solid'
                    type='primary'
                    icon={confirmingWorkflow ? <Loader2 className='animate-spin' size={16} /> : <Check size={16} />}
                    className='!rounded-full'
                    disabled={!canConfirm || confirmingWorkflow}
                    loading={confirmingWorkflow}
                    onClick={confirmWorkflow}
                  >
                    {t('确认并生成详情段')}
                  </Button>
                </div>
                <div className='mt-4 flex min-h-[360px] items-center justify-center overflow-hidden rounded-[24px] bg-slate-50'>
                  {workflow?.status === 'MOTHER_PROCESSING' || workflow?.status === 'MOTHER_PENDING' ? (
                    <div className='flex flex-col items-center gap-3 text-sm text-slate-500'>
                      <Loader2 className='animate-spin text-slate-900' size={28} />
                      {t('母版正在后端异步生成，可刷新或稍后回来查看')}
                    </div>
                  ) : workflow?.mother_result_url ? (
                    <button type='button' onClick={() => openImage(workflow.mother_result_url)} className='block h-full w-full'>
                      <img src={workflow.mother_result_url} alt={t('电商详情母版')} className='mx-auto max-h-[520px] w-auto max-w-full object-contain' />
                    </button>
                  ) : (
                    <Empty image={<Sparkles size={40} className='text-slate-900' />} title={t('等待生成母版图')} description={t('选择右侧模板并上传商品参考图后开始')} />
                  )}
                </div>
                {workflow?.mother_result_url ? (
                  <div className='mt-3 flex flex-wrap gap-2'>
                    <Button icon={<ExternalLink size={15} />} className='!rounded-full' onClick={() => openImage(workflow.mother_result_url)}>{t('查看母版')}</Button>
                    <Button icon={<Download size={15} />} className='!rounded-full' onClick={() => downloadImage(workflow.mother_result_url, 'ecommerce-mother-page.png')}>{t('下载母版')}</Button>
                  </div>
                ) : null}
              </div>

              <div className='rounded-[28px] border border-slate-200 bg-white p-4'>
                <div className='flex items-center justify-between gap-3'>
                  <div>
                    <Typography.Title heading={6} className='!mb-0 !text-slate-900'>{t('第二阶段：分段扩展')}</Typography.Title>
                    <Typography.Text className='!text-sm !text-slate-500'>{t('后端会按首屏、中段、尾段顺序生成，后段参考前段结果')}</Typography.Text>
                  </div>
                  {workflowRunning ? <Loader2 className='animate-spin text-slate-500' size={20} /> : null}
                </div>
                {workflow?.segments?.length > 0 ? (
                  <div className='mt-4 grid gap-3 md:grid-cols-3'>
                    {workflow.segments.map((segment) => (
                      <div key={segment.segment_key} className='rounded-[22px] border border-slate-100 bg-slate-50 p-2'>
                        <div className='mb-2 flex items-center justify-between gap-2'>
                          <span className='truncate text-xs font-medium text-slate-600'>{t(segment.label)}</span>
                          <Tag color={statusColorMap[segment.status] || 'grey'}>{getStatusLabel(segment.status, t)}</Tag>
                        </div>
                        {segment.result_url ? (
                          <button type='button' onClick={() => openImage(segment.result_url)} className='block w-full overflow-hidden rounded-[16px] bg-white'>
                            <img src={segment.result_url} alt={segment.label} className='h-56 w-full object-cover object-top' />
                          </button>
                        ) : (
                          <div className='flex h-56 items-center justify-center rounded-[16px] bg-white text-sm text-slate-400'>
                            {segment.status === 'PROCESSING' || segment.status === 'PENDING' ? t('生成中') : t('暂无结果')}
                          </div>
                        )}
                        <div className='mt-2 flex flex-wrap gap-2'>
                          <Button size='small' className='!rounded-full' disabled={!segment.result_url} onClick={() => { setRedrawSegment(segment); setRedrawPrompt(''); setRedrawAnnotatedImage(''); setRedrawAnnotations([]); }}>{t('重绘')}</Button>
                          <Button size='small' icon={<Download size={13} />} className='!rounded-full' disabled={!segment.result_url} onClick={() => downloadImage(segment.result_url, `ecommerce-${segment.segment_key}.png`)}>{t('下载')}</Button>
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className='mt-4 rounded-[22px] border border-dashed border-slate-300 bg-slate-50 p-5 text-center text-sm text-slate-500'>
                    {t('确认母版后，这里会显示首屏、中段、尾段三张可拼接详情图')}
                  </div>
                )}
              </div>

              <div className='rounded-[28px] border border-slate-200 bg-white p-4'>
                <div className='flex items-center justify-between gap-3'>
                  <div>
                    <Typography.Title heading={6} className='!mb-0 !text-slate-900'>{t('拼接长图')}</Typography.Title>
                    <Typography.Text className='!text-sm !text-slate-500'>{t('后端完成三段后会拼接并上传到对象存储')}</Typography.Text>
                  </div>
                  {workflow?.assembled_url ? <Button icon={<Download size={15} />} className='!rounded-full' onClick={() => downloadImage(workflow.assembled_url, 'ecommerce-long-detail.png')}>{t('下载长图')}</Button> : null}
                </div>
                <div className='mt-4 flex min-h-[220px] items-start justify-center overflow-auto rounded-[24px] bg-slate-50 p-3'>
                  {workflow?.assembled_url ? (
                    <button type='button' onClick={() => openImage(workflow.assembled_url)}>
                      <img src={workflow.assembled_url} alt={t('电商详情长图')} className='max-h-[720px] w-auto max-w-full rounded-[18px] shadow-[0_18px_45px_rgba(15,23,42,0.12)]' />
                    </button>
                  ) : (
                    <Empty image={<Layers size={40} className='text-slate-900' />} title={t('暂无拼接长图')} description={t('三段详情图生成完成后会自动拼接')} />
                  )}
                </div>
                {workflow?.error_message ? <div className='mt-3 rounded-2xl bg-red-50 p-3 text-sm text-red-600'>{workflow.error_message}</div> : null}
              </div>
            </div>
          </section>
        </main>

        <aside className='flex min-h-0 flex-col gap-4 rounded-[32px] border border-white/70 bg-white/75 p-4 shadow-[0_24px_80px_rgba(15,23,42,0.08)] backdrop-blur-xl xl:max-h-[calc(100vh-88px)] xl:overflow-auto'>
          <div>
            <Typography.Title heading={5} className='!mb-1 !text-slate-900'>{t('工作流模板')}</Typography.Title>
            <Typography.Text className='!text-sm !text-slate-500'>{t('同一后端异步逻辑，不同商品面向和详情页结构')}</Typography.Text>
          </div>
          <div className='flex flex-col gap-3'>
            {ECOMMERCE_IMAGE_TEMPLATES.map((template) => {
              const active = template.key === selectedTemplateKey;
              return (
                <button
                  key={template.key}
                  type='button'
                  onClick={() => applyTemplate(template)}
                  className={`rounded-[24px] border p-4 text-left transition ${active ? 'border-slate-900 bg-slate-900 text-white shadow-[0_18px_40px_rgba(15,23,42,0.18)]' : 'border-slate-200 bg-white hover:border-slate-300'}`}
                >
                  <div className='flex items-start justify-between gap-3'>
                    <div>
                      <div className={`text-base font-semibold ${active ? 'text-white' : 'text-slate-900'}`}>{t(template.name)}</div>
                      <p className={`mt-1 text-sm ${active ? 'text-slate-200' : 'text-slate-500'}`}>{t(template.description)}</p>
                    </div>
                    <span className={`shrink-0 whitespace-nowrap rounded-full px-2.5 py-1 text-xs font-medium ${active ? 'bg-white text-slate-900' : 'bg-blue-50 text-blue-700'}`}>
                      {t(template.tag)}
                    </span>
                  </div>
                  <div className={`mt-3 text-xs ${active ? 'text-slate-200' : 'text-slate-500'}`}>{t(template.tone)}</div>
                  <div className='mt-3 flex flex-wrap gap-1.5'>
                    {template.modules.slice(0, 4).map((module) => (
                      <span key={module} className={`rounded-full px-2 py-1 text-xs ${active ? 'bg-white/10 text-slate-100' : 'bg-slate-100 text-slate-600'}`}>{t(module)}</span>
                    ))}
                  </div>
                </button>
              );
            })}
          </div>
          <div className='rounded-[24px] border border-amber-100 bg-amber-50 p-4 text-sm text-amber-800'>
            {t('生产级异步模式依赖对象存储。提交后可刷新页面，但当前页面暂未做历史工作流选择，后续可从日志页追踪。')}
          </div>
        </aside>
      </div>

      <Modal
        title={t('重绘详情段')}
        visible={Boolean(redrawSegment)}
        onCancel={() => { setRedrawSegment(null); setRedrawAnnotationVisible(false); }}
        width={980}
        footer={null}
      >
        <div className='grid gap-4 md:grid-cols-[320px_minmax(0,1fr)]'>
          <div className='flex flex-col gap-3'>
            <Tag color={statusColorMap[redrawSegment?.status] || 'grey'}>{redrawSegment?.label}</Tag>
            <Typography.Text type='secondary'>{t('输入额外重绘要求，也可以直接在右侧图片上画圈标注局部要求。')}</Typography.Text>
            <TextArea value={redrawPrompt} onChange={setRedrawPrompt} autosize={{ minRows: 8, maxRows: 14 }} placeholder={t('例如：增强产品特写，减少文字密度，背景更统一，保留当前商品外观')} />
            {getAnnotationPromptCount(redrawAnnotations) > 0 && (
              <div className='rounded-[18px] border border-emerald-100 bg-emerald-50 p-3 text-sm text-emerald-800'>
                {t('已添加 {{count}} 处局部重绘要求，可不填写上方整体提示词直接提交。', {
                  count: getAnnotationPromptCount(redrawAnnotations),
                })}
              </div>
            )}
            <Button theme='solid' type='primary' loading={redrawing} icon={redrawing ? <Loader2 className='animate-spin' size={16} /> : <Wand2 size={16} />} className='!rounded-full' onClick={submitRedraw}>
              {t('开始重绘')}
            </Button>
          </div>
          <div className='flex max-h-[70vh] flex-col gap-3 overflow-auto rounded-[24px] bg-slate-50 p-3'>
            <Button
              theme='solid'
              type='primary'
              icon={<Brush size={16} />}
              className='!rounded-full !bg-amber-500 !text-white hover:!bg-amber-600'
              disabled={!redrawSegment?.result_url}
              onClick={() => setRedrawAnnotationVisible(true)}
            >
              {getAnnotationPromptCount(redrawAnnotations) > 0
                ? t('继续画圈修改')
                : t('画圈标注局部重绘')}
            </Button>
            {redrawSegment?.result_url ? (
              <img
                src={redrawAnnotatedImage || redrawSegment.result_url}
                alt={redrawSegment.label}
                className='mx-auto max-h-[62vh] w-auto max-w-full rounded-[18px]'
              />
            ) : (
              <Empty description={t('当前段暂无可预览结果')} />
            )}
          </div>
        </div>
      </Modal>
      <EcommerceAnnotationModal
        visible={redrawAnnotationVisible}
        sourceUrl={redrawSegment?.task_id ? `/api/ai-image/proxy/${redrawSegment.task_id}` : redrawSegment?.result_url}
        sourceLabel={redrawSegment?.label}
        initialAnnotations={redrawAnnotations}
        onCancel={() => setRedrawAnnotationVisible(false)}
        onSave={({ dataUrl, annotations }) => {
          setRedrawAnnotatedImage(dataUrl);
          setRedrawAnnotations(annotations);
          setRedrawAnnotationVisible(false);
          showSuccess(t('已保存局部重绘要求'));
        }}
      />
    </div>
  );
};

export default AIEcommerceTemplate;
