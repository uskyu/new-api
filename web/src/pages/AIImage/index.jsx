import React, { useMemo, useState } from 'react';
import { Button, Empty, Spin, TextArea, Typography } from '@douyinfe/semi-ui';
import { ImagePlus, Loader2, Sparkles, Wand2, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import useAiImageState from '../../hooks/ai-image/useAiImageState';
import { API_ENDPOINTS, MESSAGE_ROLES } from '../../constants/playground.constants';
import {
  buildApiPayload,
  getUserIdFromLocalStorage,
  handleApiError,
  showError,
  showSuccess,
} from '../../helpers';

const RESOLUTION_OPTIONS = ['1K', '2K', '4K'];
const BATCH_COUNT_OPTIONS = [1, 2, 3, 4, 6, 8];

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
        return normalizeImageSource(part.inlineData.data, part.inlineData.mimeType || 'image/png');
      }
      if (part?.inline_data?.data) {
        return normalizeImageSource(part.inline_data.data, part.inline_data.mime_type || 'image/png');
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
    prompt,
    images: [...new Set(imageUrls)],
    createdAt: assistantMessage?.createAt || Date.now(),
  };
};

const createImageAssistantMessage = (imageUrls, prompt) => ({
  role: MESSAGE_ROLES.ASSISTANT,
  content: imageUrls.map((url) => `![image](${url})`).join('\n\n'),
  parts: imageUrls.map((url) => ({ image_url: { url } })),
  prompt,
  createAt: Date.now(),
  id: `assistant-${Date.now()}-${Math.random().toString(16).slice(2)}`,
  status: 'complete',
});

const buildImagePayload = ({ prompt, draftImages, model, group, resolution }) => {
  const payload = buildApiPayload(
    [
      {
        role: MESSAGE_ROLES.USER,
        content: [
          { type: 'text', text: prompt },
          ...draftImages.map((url) => ({
            type: 'image_url',
            image_url: { url },
          })),
        ],
      },
    ],
    null,
    { model, group, stream: false },
    {},
  );

  payload.extra_body = {
    google: {
      image_config: {
        image_size: resolution,
      },
    },
  };

  return payload;
};

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

const AIImage = () => {
  const { t } = useTranslation();
  const {
    ready,
    storageError,
    messages,
    setMessages,
    groups,
    models,
    selectedModel,
    selectedGroup,
    setSelectedModel,
    setSelectedGroup,
    draftImages,
    setDraftImages,
    clearCurrentSession,
    markSessionActivity,
  } = useAiImageState();

  const [prompt, setPrompt] = useState('');
  const [isGenerating, setIsGenerating] = useState(false);
  const [activeRecordId, setActiveRecordId] = useState(null);
  const [resolution, setResolution] = useState('1K');
  const [batchCount, setBatchCount] = useState(4);

  const groupOptions = groups.map((group) => ({
    value: group.value,
    label: group.fullLabel || group.label,
  }));

  const modelOptions = models.map((model) => ({
    value: model.value,
    label: model.label,
  }));

  const records = useMemo(() => {
    const nextRecords = [];
    for (let index = 0; index < messages.length; index += 1) {
      const current = messages[index];
      const next = messages[index + 1];
      if (current?.role !== MESSAGE_ROLES.USER || next?.role !== MESSAGE_ROLES.ASSISTANT) continue;
      const record = extractGenerationRecord(current, next);
      if (record) nextRecords.push(record);
    }
    return nextRecords.reverse();
  }, [messages]);

  const activeRecord = records.find((record) => record.id === activeRecordId) || records[0] || null;
  const activeImage = activeRecord?.images?.[0] || '';

  React.useEffect(() => {
    if (!activeRecordId && records.length > 0) {
      setActiveRecordId(records[0].id);
    } else if (activeRecordId && !records.some((record) => record.id === activeRecordId)) {
      setActiveRecordId(records[0]?.id || null);
    }
  }, [activeRecordId, records]);

  const handleAddDraftImage = React.useCallback(
    (imageDataUrl) => setDraftImages([imageDataUrl]),
    [setDraftImages],
  );

  const handleRemoveDraftImage = React.useCallback(
    (index) => {
      setDraftImages((previous) => previous.filter((_, itemIndex) => itemIndex !== index));
    },
    [setDraftImages],
  );

  const requestOneImage = React.useCallback(
    async (itemPrompt) => {
      const payload = buildImagePayload({
        prompt: itemPrompt,
        draftImages,
        model: selectedModel,
        group: selectedGroup || '',
        resolution,
      });

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

      return parseImageResponse(response);
    },
    [draftImages, resolution, selectedGroup, selectedModel],
  );

  const appendGeneration = React.useCallback(
    (items) => {
      const createdMessages = items.flatMap(({ itemPrompt, imageUrls }) => [
        {
          role: MESSAGE_ROLES.USER,
          content: [
            { type: 'text', text: itemPrompt },
            ...draftImages.map((url) => ({ type: 'image_url', image_url: { url } })),
          ],
          createAt: Date.now(),
          id: `user-${Date.now()}-${Math.random().toString(16).slice(2)}`,
        },
        createImageAssistantMessage(imageUrls, itemPrompt),
      ]);

      if (createdMessages.length === 0) return;
      markSessionActivity();
      setMessages((previous) => [...previous, ...createdMessages]);
      const lastAssistant = [...createdMessages].reverse().find(
        (message) => message.role === MESSAGE_ROLES.ASSISTANT,
      );
      setActiveRecordId(lastAssistant?.id || null);
    },
    [draftImages, markSessionActivity, setMessages],
  );

  const handleGenerate = React.useCallback(async () => {
    const trimmedPrompt = prompt.trim();
    if (!trimmedPrompt) {
      showError(t('请输入提示词'));
      return;
    }
    if (!selectedModel) {
      showError(t('请选择模型'));
      return;
    }

    setIsGenerating(true);
    try {
      const parsed = await requestOneImage(trimmedPrompt);
      if (parsed.imageUrls.length === 0) {
        showError(t('未返回可展示的图片，请检查模型返回格式'));
        return;
      }
      appendGeneration([{ itemPrompt: trimmedPrompt, imageUrls: parsed.imageUrls }]);
      setPrompt('');
      setDraftImages([]);
      showSuccess(t('图片生成成功'));
    } catch (error) {
      const errorInfo = handleApiError(error);
      showError(errorInfo.error || t('请求发生错误'));
    } finally {
      setIsGenerating(false);
    }
  }, [appendGeneration, prompt, requestOneImage, selectedModel, setDraftImages, t]);

  const handleBatchGenerate = React.useCallback(async () => {
    const prompts = prompt.split('\n').map((item) => item.trim()).filter(Boolean);
    if (!selectedModel) {
      showError(t('请选择模型'));
      return;
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
      appendGeneration(validResults);
      setPrompt('');
      setDraftImages([]);
      showSuccess(t('批量生成完成'));
    } catch (error) {
      const errorInfo = handleApiError(error);
      showError(errorInfo.error || t('请求发生错误'));
    } finally {
      setIsGenerating(false);
    }
  }, [appendGeneration, batchCount, prompt, requestOneImage, selectedModel, setDraftImages, t]);

  const handleClearHistory = React.useCallback(() => {
    clearCurrentSession();
    setActiveRecordId(null);
    showSuccess(t('已清空最近记录'));
  }, [clearCurrentSession, t]);

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
                    {t('Google 图片模型')}
                  </span>
                  <select
                    value={selectedModel}
                    onChange={(event) => setSelectedModel(event.target.value)}
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
                        <option key={item} value={item}>{item}</option>
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
                        <option key={item} value={item}>{item}</option>
                      ))}
                    </select>
                  </label>
                </div>

                <TextArea
                  value={prompt}
                  onChange={setPrompt}
                  autosize={{ minRows: 7, maxRows: 14 }}
                  placeholder={t('描述你想生成的图片；批量生成时可一行一个提示词')}
                  className='!bg-transparent'
                />

                {draftImages.length > 0 && (
                  <div className='mt-3 flex flex-wrap gap-2'>
                    {draftImages.map((image, index) => (
                      <div key={`${index}-${image.length}`} className='group relative overflow-hidden rounded-[18px] border border-white/70 bg-white'>
                        <img src={image} alt={`${t('参考图')} ${index + 1}`} className='h-24 w-24 object-cover' />
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
                      className='hidden'
                      onChange={(event) => {
                        const file = event.target.files?.[0];
                        event.target.value = '';
                        if (!file) return;
                        const reader = new FileReader();
                        reader.onload = () => handleAddDraftImage(reader.result);
                        reader.readAsDataURL(file);
                      }}
                    />
                  </label>

                  <Button
                    theme='solid'
                    type='primary'
                    icon={isGenerating ? <Loader2 size={16} className='animate-spin' /> : <Wand2 size={16} />}
                    loading={isGenerating}
                    onClick={handleGenerate}
                    className='!rounded-full'
                  >
                    {t('生成图片')}
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
                      <img
                        src={activeImage}
                        alt={activeRecord?.prompt || 'generated image'}
                        className='max-h-[72vh] w-full flex-1 rounded-[24px] object-contain shadow-[0_20px_60px_rgba(15,23,42,0.12)]'
                      />
                      {activeRecord?.prompt ? (
                        <p className='mt-3 text-sm text-slate-500'>{activeRecord.prompt}</p>
                      ) : null}
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
                <div key={record.id} className='min-w-[168px] space-y-2'>
                  <GalleryImage
                    src={record.images[0]}
                    alt={record.prompt || 'generated image'}
                    active={record.id === activeRecord?.id}
                    onClick={() => setActiveRecordId(record.id)}
                  />
                  <div className='px-1'>
                    <p className='line-clamp-2 text-sm text-slate-700'>
                      {record.prompt || t('未命名提示词')}
                    </p>
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
