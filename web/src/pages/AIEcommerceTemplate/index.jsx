import React, { useMemo, useState } from 'react';
import { Button, Empty, Input, Spin, Tag, TextArea, Typography } from '@douyinfe/semi-ui';
import {
  Check,
  Download,
  ExternalLink,
  ImagePlus,
  Layers,
  Loader2,
  RefreshCw,
  Scissors,
  Sparkles,
  Trash2,
  Wand2,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import useAiImageState from '../../hooks/ai-image/useAiImageState';
import { API_ENDPOINTS } from '../../constants/playground.constants';
import {
  DEFAULT_ECOMMERCE_TEMPLATE_KEY,
  ECOMMERCE_DETAIL_SEGMENTS,
  ECOMMERCE_IMAGE_TEMPLATES,
} from '../../constants/ai-ecommerce-template.constants';
import {
  getUserIdFromLocalStorage,
  handleApiError,
  showError,
  showSuccess,
} from '../../helpers';

const MAX_REFERENCE_IMAGES = 5;
const DEFAULT_OPENAI_SIZE = '1024x1024';
const SEGMENT_OPENAI_SIZE = '1024x1536';
const RESOLUTION_OPTIONS = ['1K', '2K', '4K'];
const ASPECT_RATIO_OPTIONS = ['1:1', '3:2', '4:3', '16:9', '9:16'];
const OPENAI_SIZE_OPTIONS = [
  { value: '1024x1024', label: '1024x1024 母版' },
  { value: '1536x1024', label: '1536x1024 横版' },
  { value: '1024x1536', label: '1024x1536 竖版' },
  { value: '2048x2048', label: '2048x2048 2K 方图' },
];

const isOpenAIImageModel = (modelName) => {
  const lower = String(modelName || '').toLowerCase();
  return lower.includes('gpt-image') || lower.includes('dall-e');
};

const normalizeImageSource = (value, mimeType = 'image/png') => {
  if (typeof value !== 'string' || value.trim() === '') return '';
  const source = value.trim();
  if (source.startsWith('data:image/') || source.startsWith('http')) return source;
  return `data:${mimeType || 'image/png'};base64,${source}`;
};

const extractImageUrlsFromMarkdown = (content) => {
  if (typeof content !== 'string') return [];
  return [...content.matchAll(/!\[[^\]]*\]\((data:image\/[^)]+|https?:\/\/[^)]+)\)/g)]
    .map((match) => match[1])
    .filter(Boolean);
};

const extractImageUrlsFromParts = (parts = []) =>
  parts
    .map((part) => {
      if (part?.inlineData?.data) {
        return normalizeImageSource(part.inlineData.data, part.inlineData.mimeType || 'image/png');
      }
      if (part?.inline_data?.data) {
        return normalizeImageSource(part.inline_data.data, part.inline_data.mime_type || 'image/png');
      }
      if (part?.image_url?.url) return normalizeImageSource(part.image_url.url);
      if (part?.imageUrl?.url) return normalizeImageSource(part.imageUrl.url);
      if (typeof part?.text === 'string') return extractImageUrlsFromMarkdown(part.text)[0] || '';
      return '';
    })
    .filter(Boolean);

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
          if (item?.url) return [normalizeImageSource(item.url)];
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

  return [...new Set(imageUrls)];
};

const readFileAsDataUrl = (file) =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '');
    reader.onerror = () => reject(reader.error || new Error('failed to read file'));
    reader.readAsDataURL(file);
  });

const fetchImageAsDataUrl = async (src) => {
  if (typeof src !== 'string' || !src) return '';
  if (src.startsWith('data:image/')) return src;
  const response = await fetch(src);
  if (!response.ok) throw new Error(`failed to load image: ${response.status}`);
  const blob = await response.blob();
  return readFileAsDataUrl(blob);
};

const dataUrlToFile = (dataUrl, filename = 'reference.png') => {
  const matched = String(dataUrl || '').match(/^data:(.+?);base64,(.+)$/);
  if (!matched) return null;
  const mimeType = matched[1] || 'image/png';
  const binary = atob(matched[2] || '');
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return new File([bytes], filename, { type: mimeType });
};

const toGeminiInlinePart = (dataUrl) => {
  const matched = String(dataUrl || '').match(/^data:(.+?);base64,(.+)$/);
  if (!matched) return null;
  return {
    inlineData: {
      mimeType: matched[1] || 'image/png',
      data: matched[2] || '',
    },
  };
};

const loadImageElement = (src) =>
  new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error('failed to decode image'));
    image.src = src;
  });

const cropMotherIntoSlices = async (sourceUrl, segments = ECOMMERCE_DETAIL_SEGMENTS) => {
  const dataUrl = await fetchImageAsDataUrl(sourceUrl);
  const image = await loadImageElement(dataUrl);
  const sliceHeight = Math.floor(image.naturalHeight / segments.length);

  return segments.map((segment, index) => {
    const y = index * sliceHeight;
    const height = index === segments.length - 1 ? image.naturalHeight - y : sliceHeight;
    const canvas = document.createElement('canvas');
    canvas.width = image.naturalWidth;
    canvas.height = height;
    const context = canvas.getContext('2d');
    context.drawImage(image, 0, y, image.naturalWidth, height, 0, 0, image.naturalWidth, height);
    return {
      ...segment,
      index,
      dataUrl: canvas.toDataURL('image/png'),
    };
  });
};

const stitchImagesVertically = async (items) => {
  const dataUrls = await Promise.all(items.map((item) => fetchImageAsDataUrl(item.imageUrl)));
  const images = await Promise.all(dataUrls.map((src) => loadImageElement(src)));
  const width = Math.max(...images.map((image) => image.naturalWidth));
  const scaledHeights = images.map((image) => Math.round((image.naturalHeight * width) / image.naturalWidth));
  const height = scaledHeights.reduce((total, itemHeight) => total + itemHeight, 0);
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext('2d');
  let offsetY = 0;
  images.forEach((image, index) => {
    context.drawImage(image, 0, offsetY, width, scaledHeights[index]);
    offsetY += scaledHeights[index];
  });
  return canvas.toDataURL('image/png');
};

const prepareReferenceImages = async (referenceImages) =>
  Promise.all(
    referenceImages.filter(Boolean).slice(0, MAX_REFERENCE_IMAGES).map(async (image, index) => {
      if (image.file && image.dataUrl) return image;
      if (image.file) {
        return { ...image, dataUrl: await readFileAsDataUrl(image.file) };
      }
      if (image.dataUrl || image.imageUrl) {
        const dataUrl = await fetchImageAsDataUrl(image.dataUrl || image.imageUrl);
        return {
          ...image,
          dataUrl,
          file: dataUrlToFile(dataUrl, image.name || `reference-${index + 1}.png`),
        };
      }
      return image;
    }),
  );

const requestImage = async ({
  model,
  group,
  prompt,
  referenceImages,
  openaiSize,
  resolution,
  aspectRatio,
}) => {
  const query = group ? `?group=${encodeURIComponent(group)}` : '';
  const images = await prepareReferenceImages(referenceImages);

  if (isOpenAIImageModel(model)) {
    if (images.length > 0) {
      const formData = new FormData();
      formData.append('model', model);
      formData.append('prompt', prompt);
      formData.append('n', '1');
      formData.append('size', openaiSize || DEFAULT_OPENAI_SIZE);
      images.forEach((image, index) => {
        const file = image.file || dataUrlToFile(image.dataUrl, image.name || `reference-${index + 1}.png`);
        if (file) formData.append('image', file, image.name || file.name || `reference-${index + 1}.png`);
      });

      const response = await fetch(`${API_ENDPOINTS.IMAGE_EDITS}${query}`, {
        method: 'POST',
        headers: { 'New-Api-User': getUserIdFromLocalStorage() },
        body: formData,
      });
      if (!response.ok) throw new Error(`HTTP error! status: ${response.status}, body: ${await response.text()}`);
      return parseImageResponse(response);
    }

    const response = await fetch(`${API_ENDPOINTS.IMAGE_GENERATIONS}${query}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'New-Api-User': getUserIdFromLocalStorage(),
      },
      body: JSON.stringify({ model, prompt, size: openaiSize || DEFAULT_OPENAI_SIZE, n: 1 }),
    });
    if (!response.ok) throw new Error(`HTTP error! status: ${response.status}, body: ${await response.text()}`);
    return parseImageResponse(response);
  }

  const response = await fetch(
    `${API_ENDPOINTS.GEMINI_NATIVE_MODELS}/${encodeURIComponent(model)}:generateContent${query}`,
    {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        'New-Api-User': getUserIdFromLocalStorage(),
      },
      body: JSON.stringify({
        contents: [
          {
            role: 'user',
            parts: [
              { text: prompt },
              ...images.map((image) => toGeminiInlinePart(image.dataUrl)).filter(Boolean),
            ],
          },
        ],
        generationConfig: {
          responseModalities: ['TEXT', 'IMAGE'],
          imageConfig: {
            aspectRatio,
            imageSize: resolution,
          },
        },
      }),
    },
  );
  if (!response.ok) throw new Error(`HTTP error! status: ${response.status}, body: ${await response.text()}`);
  return parseImageResponse(response);
};

const buildMotherPrompt = ({ template, productName, productType, platform, sellingPoints, extraRequirements }) =>
  [
    '请基于上传的商品参考图，生成一张 1:1 方形的中文电商详情页母版图。',
    '这张图不是最终长图，而是后续分段扩展的视觉参考母版。',
    `商品名称：${productName || '未命名商品'}`,
    `商品品类：${productType || template.productType}`,
    `目标平台：${platform || template.platform}`,
    `核心卖点：${sellingPoints || template.sellingPoints}`,
    `模板风格：${template.stylePrompt}`,
    `页面模块：${template.modules.join('、')}`,
    '画面要求：真实中国电商详情页语言，包含主视觉、卖点区、参数/规格块、细节展示、信任背书和收尾 CTA 区域。',
    '排版要求：三段纵向结构浓缩在一个方形板内，模块感清晰，留出大量中文标题和正文排版区域。',
    '文字要求：可以生成文字感占位和局部中文短句，不要求最终可商用文案准确，但必须像真实详情页。',
    '商品一致性：必须保留参考图中的商品外观、材质、颜色和核心识别特征。',
    extraRequirements ? `补充要求：${extraRequirements}` : '',
  ]
    .filter(Boolean)
    .join('\n');

const buildSegmentPrompt = ({ template, segment, productName, productType, platform, sellingPoints, extraRequirements }) =>
  [
    `请生成电商详情页长图的第 ${segment.index + 1} 段：${segment.label}。`,
    '参考图包含：原始商品图、完整母版图、当前母版切片，以及上一段结果（如果有）。',
    '请严格延续母版图的商品身份、视觉风格、模块语言、配色和电商详情页排版。',
    `本段定位：${segment.description}`,
    `商品名称：${productName || '未命名商品'}`,
    `商品品类：${productType || template.productType}`,
    `目标平台：${platform || template.platform}`,
    `核心卖点：${sellingPoints || template.sellingPoints}`,
    `模板风格：${template.stylePrompt}`,
    '输出要求：生成一张竖向详情页分段图，适合后续和其他段落上下拼接。',
    '连续性要求：背景色温、边距、模块密度、光影、商品比例要尽量承接上一段。',
    '文字要求：保留真实电商详情页的标题区、说明区和参数块感觉，不要求最终文案完全准确。',
    extraRequirements ? `补充要求：${extraRequirements}` : '',
  ]
    .filter(Boolean)
    .join('\n');

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
  const [resolution, setResolution] = useState('1K');
  const [aspectRatio, setAspectRatio] = useState('1:1');
  const [motherImage, setMotherImage] = useState('');
  const [motherPrompt, setMotherPrompt] = useState('');
  const [motherSlices, setMotherSlices] = useState([]);
  const [segments, setSegments] = useState([]);
  const [assembledImage, setAssembledImage] = useState('');
  const [isGeneratingMother, setIsGeneratingMother] = useState(false);
  const [isGeneratingSegments, setIsGeneratingSegments] = useState(false);
  const [currentSegmentLabel, setCurrentSegmentLabel] = useState('');

  const selectedTemplate = useMemo(
    () => ECOMMERCE_IMAGE_TEMPLATES.find((template) => template.key === selectedTemplateKey) || ECOMMERCE_IMAGE_TEMPLATES[0],
    [selectedTemplateKey],
  );

  const modelOptions = models.map((model) => ({ value: model.value, label: model.label }));
  const groupOptions = groups.map((group) => ({ value: group.value, label: group.fullLabel || group.label }));
  const effectiveModel = selectedModel || modelOptions[0]?.value || '';
  const isOpenAIModel = isOpenAIImageModel(effectiveModel);

  const applyTemplate = (template) => {
    setSelectedTemplateKey(template.key);
    setProductType(template.productType);
    setPlatform(template.platform);
    setSellingPoints(template.sellingPoints);
    setMotherPrompt('');
  };

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

  const resetWorkflow = () => {
    setMotherImage('');
    setMotherSlices([]);
    setSegments([]);
    setAssembledImage('');
    setCurrentSegmentLabel('');
  };

  const generateMother = async () => {
    if (!effectiveModel) {
      showError(t('请选择模型'));
      return;
    }
    if (referenceImages.length === 0) {
      showError(t('请先上传商品参考图'));
      return;
    }
    if (!selectedModel && effectiveModel) setSelectedModel(effectiveModel);

    const prompt = motherPrompt || buildMotherPrompt({
      template: selectedTemplate,
      productName,
      productType,
      platform,
      sellingPoints,
      extraRequirements,
    });

    setIsGeneratingMother(true);
    resetWorkflow();
    try {
      const imageUrls = await requestImage({
        model: effectiveModel,
        group: selectedGroup,
        prompt,
        referenceImages,
        openaiSize,
        resolution,
        aspectRatio,
      });
      if (imageUrls.length === 0) {
        showError(t('未返回可展示的图片，请检查模型返回格式'));
        return;
      }
      const normalizedMotherImage = await fetchImageAsDataUrl(imageUrls[0]);
      setMotherImage(normalizedMotherImage);
      setMotherPrompt(prompt);
      showSuccess(t('母版图生成成功，请确认后继续生成详情段'));
    } catch (error) {
      const errorInfo = handleApiError(error);
      showError(errorInfo.error || t('请求发生错误'));
    } finally {
      setIsGeneratingMother(false);
    }
  };

  const generateSegments = async () => {
    if (!motherImage) {
      showError(t('请先生成并确认母版图'));
      return;
    }
    if (!effectiveModel) {
      showError(t('请选择模型'));
      return;
    }

    setIsGeneratingSegments(true);
    setSegments([]);
    setAssembledImage('');
    try {
      const slices = await cropMotherIntoSlices(motherImage);
      setMotherSlices(slices);
      const nextSegments = [];

      for (const segment of slices) {
        setCurrentSegmentLabel(segment.label);
        const prompt = buildSegmentPrompt({
          template: selectedTemplate,
          segment,
          productName,
          productType,
          platform,
          sellingPoints,
          extraRequirements,
        });
        const previousSegment = nextSegments[nextSegments.length - 1];
        const productReferences = referenceImages.slice(0, previousSegment ? 2 : 3);
        const segmentReferences = [
          ...productReferences,
          { dataUrl: motherImage, name: 'mother-page.png' },
          { dataUrl: segment.dataUrl, name: `${segment.key}-slice.png` },
          previousSegment ? { dataUrl: previousSegment.imageUrl, name: 'previous-segment.png' } : null,
        ]
          .filter(Boolean)
          .slice(0, MAX_REFERENCE_IMAGES);

        const imageUrls = await requestImage({
          model: effectiveModel,
          group: selectedGroup,
          prompt,
          referenceImages: segmentReferences,
          openaiSize: isOpenAIModel ? SEGMENT_OPENAI_SIZE : openaiSize,
          resolution,
          aspectRatio: isOpenAIModel ? aspectRatio : '9:16',
        });
        if (imageUrls.length === 0) throw new Error('segment image response is empty');
        const nextSegment = {
          ...segment,
          prompt,
          imageUrl: await fetchImageAsDataUrl(imageUrls[0]),
        };
        nextSegments.push(nextSegment);
        setSegments([...nextSegments]);
      }

      setAssembledImage(await stitchImagesVertically(nextSegments));
      showSuccess(t('电商详情段生成完成'));
    } catch (error) {
      const errorInfo = handleApiError(error);
      showError(errorInfo.error || t('请求发生错误'));
    } finally {
      setCurrentSegmentLabel('');
      setIsGeneratingSegments(false);
    }
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
                  {t('先生成详情页母版，确认后再扩展为可拼接的电商长图分段')}
                </Typography.Text>
              </div>
            </div>
            <div className='flex flex-wrap gap-2'>
              <Button icon={<RefreshCw size={16} />} className='!rounded-full' onClick={resetWorkflow}>
                {t('重置流程')}
              </Button>
              <Button
                theme='solid'
                type='primary'
                icon={isGeneratingMother ? <Loader2 className='animate-spin' size={16} /> : <Wand2 size={16} />}
                className='!rounded-full !bg-slate-900 !text-white'
                loading={isGeneratingMother}
                onClick={generateMother}
              >
                {motherImage ? t('重新生成母版') : t('生成第一版母版')}
              </Button>
            </div>
          </div>

          <section className='grid gap-4 lg:grid-cols-[420px_minmax(0,1fr)]'>
            <div className='flex flex-col gap-4'>
              <div className='rounded-[28px] border border-slate-200 bg-slate-50/70 p-4'>
                <Typography.Text className='!text-xs !font-semibold !uppercase !tracking-[0.2em] !text-slate-400'>
                  {t('基础设置')}
                </Typography.Text>
                <div className='mt-3 grid gap-3 md:grid-cols-2 lg:grid-cols-1'>
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
                  {isOpenAIModel ? (
                    <label className='flex flex-col gap-2'>
                      <span className='text-xs font-medium uppercase tracking-[0.18em] text-slate-400'>{t('母版尺寸')}</span>
                      <select value={openaiSize} onChange={(event) => setOpenaiSize(event.target.value)} className='h-11 rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'>
                        {OPENAI_SIZE_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                      </select>
                    </label>
                  ) : (
                    <div className='grid gap-3 md:grid-cols-2 lg:grid-cols-1'>
                      <label className='flex flex-col gap-2'>
                        <span className='text-xs font-medium uppercase tracking-[0.18em] text-slate-400'>{t('分辨率')}</span>
                        <select value={resolution} onChange={(event) => setResolution(event.target.value)} className='h-11 rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'>
                          {RESOLUTION_OPTIONS.map((item) => <option key={item} value={item}>{item}</option>)}
                        </select>
                      </label>
                      <label className='flex flex-col gap-2'>
                        <span className='text-xs font-medium uppercase tracking-[0.18em] text-slate-400'>{t('图像比例')}</span>
                        <select value={aspectRatio} onChange={(event) => setAspectRatio(event.target.value)} className='h-11 rounded-2xl border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'>
                          {ASPECT_RATIO_OPTIONS.map((item) => <option key={item} value={item}>{item}</option>)}
                        </select>
                      </label>
                    </div>
                  )}
                </div>
              </div>

              <div className='rounded-[28px] border border-slate-200 bg-slate-50/70 p-4'>
                <div className='flex items-center justify-between gap-3'>
                  <div>
                    <Typography.Text className='!text-xs !font-semibold !uppercase !tracking-[0.2em] !text-slate-400'>
                      {t('商品参考图')}
                    </Typography.Text>
                    <p className='mt-1 text-sm text-slate-500'>{t('最多 5 张，用于锁定商品身份和材质细节')}</p>
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
                    {t('先上传商品图，再生成电商详情页母版')}
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
            </div>

            <div className='flex min-w-0 flex-col gap-4'>
              <div className='rounded-[28px] border border-slate-200 bg-white p-4'>
                <div className='flex items-center justify-between gap-3'>
                  <div>
                    <Typography.Title heading={6} className='!mb-0 !text-slate-900'>{t('第一阶段：母版确认')}</Typography.Title>
                    <Typography.Text className='!text-sm !text-slate-500'>{t('满意母版方向后，再确认生成后续详情段')}</Typography.Text>
                  </div>
                  <Button
                    theme='solid'
                    type='primary'
                    icon={isGeneratingSegments ? <Loader2 className='animate-spin' size={16} /> : <Check size={16} />}
                    className='!rounded-full'
                    disabled={!motherImage || isGeneratingMother || isGeneratingSegments}
                    loading={isGeneratingSegments}
                    onClick={generateSegments}
                  >
                    {t('确认并生成详情段')}
                  </Button>
                </div>
                <div className='mt-4 flex min-h-[360px] items-center justify-center overflow-hidden rounded-[24px] bg-slate-50'>
                  {isGeneratingMother ? (
                    <div className='flex flex-col items-center gap-3 text-sm text-slate-500'>
                      <Loader2 className='animate-spin text-slate-900' size={28} />
                      {t('正在生成第一版电商详情母版...')}
                    </div>
                  ) : motherImage ? (
                    <button type='button' onClick={() => openImage(motherImage)} className='block h-full w-full'>
                      <img src={motherImage} alt={t('电商详情母版')} className='mx-auto max-h-[520px] w-auto max-w-full object-contain' />
                    </button>
                  ) : (
                    <Empty image={<Sparkles size={40} className='text-slate-900' />} title={t('等待生成母版图')} description={t('选择右侧模板并上传商品参考图后开始')} />
                  )}
                </div>
                {motherImage ? (
                  <div className='mt-3 flex flex-wrap gap-2'>
                    <Button icon={<ExternalLink size={15} />} className='!rounded-full' onClick={() => openImage(motherImage)}>{t('查看母版')}</Button>
                    <Button icon={<Download size={15} />} className='!rounded-full' onClick={() => downloadImage(motherImage, 'ecommerce-mother-page.png')}>{t('下载母版')}</Button>
                  </div>
                ) : null}
              </div>

              <div className='rounded-[28px] border border-slate-200 bg-white p-4'>
                <div className='flex items-center gap-2'>
                  <Scissors size={18} className='text-slate-500' />
                  <Typography.Title heading={6} className='!mb-0 !text-slate-900'>{t('第二阶段：分段扩展')}</Typography.Title>
                </div>
                {isGeneratingSegments ? (
                  <div className='mt-4 rounded-[22px] border border-sky-100 bg-sky-50 px-4 py-3 text-sm text-sky-700'>
                    {t('正在生成：{{name}}', { name: currentSegmentLabel || t('详情段') })}
                  </div>
                ) : null}
                {motherSlices.length > 0 ? (
                  <div className='mt-4 grid gap-3 md:grid-cols-3'>
                    {motherSlices.map((slice) => (
                      <div key={slice.key} className='rounded-[20px] border border-slate-100 bg-slate-50 p-2'>
                        <img src={slice.dataUrl} alt={slice.label} className='h-28 w-full rounded-[16px] object-cover' />
                        <p className='mt-2 text-xs font-medium text-slate-600'>{t(slice.label)}</p>
                      </div>
                    ))}
                  </div>
                ) : null}
                {segments.length > 0 ? (
                  <div className='mt-4 grid gap-3 md:grid-cols-3'>
                    {segments.map((segment) => (
                      <div key={segment.key} className='rounded-[22px] border border-slate-100 bg-slate-50 p-2'>
                        <button type='button' onClick={() => openImage(segment.imageUrl)} className='block w-full overflow-hidden rounded-[16px] bg-white'>
                          <img src={segment.imageUrl} alt={segment.label} className='h-56 w-full object-cover object-top' />
                        </button>
                        <div className='mt-2 flex items-center justify-between gap-2'>
                          <span className='truncate text-xs font-medium text-slate-600'>{t(segment.label)}</span>
                          <Button size='small' icon={<Download size={13} />} className='!rounded-full' onClick={() => downloadImage(segment.imageUrl, `ecommerce-${segment.key}.png`)} />
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className='mt-4 rounded-[22px] border border-dashed border-slate-300 bg-slate-50 p-5 text-center text-sm text-slate-500'>
                    {t('确认母版后，这里会生成首屏、中段、尾段三张可拼接详情图')}
                  </div>
                )}
              </div>

              <div className='rounded-[28px] border border-slate-200 bg-white p-4'>
                <div className='flex items-center justify-between gap-3'>
                  <div>
                    <Typography.Title heading={6} className='!mb-0 !text-slate-900'>{t('拼接预览')}</Typography.Title>
                    <Typography.Text className='!text-sm !text-slate-500'>{t('浏览器本地拼接，仅用于预览和下载草案')}</Typography.Text>
                  </div>
                  {assembledImage ? (
                    <Button icon={<Download size={15} />} className='!rounded-full' onClick={() => downloadImage(assembledImage, 'ecommerce-long-detail-preview.png')}>{t('下载长图')}</Button>
                  ) : null}
                </div>
                <div className='mt-4 flex min-h-[220px] items-start justify-center overflow-auto rounded-[24px] bg-slate-50 p-3'>
                  {assembledImage ? (
                    <button type='button' onClick={() => openImage(assembledImage)}>
                      <img src={assembledImage} alt={t('电商详情长图预览')} className='max-h-[720px] w-auto max-w-full rounded-[18px] shadow-[0_18px_45px_rgba(15,23,42,0.12)]' />
                    </button>
                  ) : (
                    <Empty image={<Scissors size={40} className='text-slate-900' />} title={t('暂无拼接预览')} description={t('三段详情图生成完成后会自动拼接')} />
                  )}
                </div>
              </div>
            </div>
          </section>
        </main>

        <aside className='flex min-h-0 flex-col gap-4 rounded-[32px] border border-white/70 bg-white/75 p-4 shadow-[0_24px_80px_rgba(15,23,42,0.08)] backdrop-blur-xl xl:max-h-[calc(100vh-88px)] xl:overflow-auto'>
          <div>
            <Typography.Title heading={5} className='!mb-1 !text-slate-900'>{t('工作流模板')}</Typography.Title>
            <Typography.Text className='!text-sm !text-slate-500'>{t('同一生成逻辑，不同商品面向和详情页结构')}</Typography.Text>
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
                    <Tag color={active ? 'white' : 'blue'}>{t(template.tag)}</Tag>
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
            {t('建议：先把母版图看作详情页视觉草案，文案最终可在设计层重新覆盖，以获得更稳定的商用文字。')}
          </div>
        </aside>
      </div>
    </div>
  );
};

export default AIEcommerceTemplate;
