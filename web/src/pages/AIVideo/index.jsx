import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Empty,
  Progress,
  Spin,
  Table,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Clapperboard,
  ImagePlus,
  Play,
  RefreshCw,
  Sparkles,
  Trash2,
  UploadCloud,
  Video,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';

const VIDEO_PREF_KEY = 'new-api-ai-video-preferences';

const SIZE_OPTIONS = [
  { value: '720x1280', label: '竖屏 720x1280' },
  { value: '1280x720', label: '横屏 1280x720' },
];

const SECOND_OPTIONS = [
  { value: '5', label: '5 秒' },
  { value: '10', label: '10 秒' },
  { value: '15', label: '15 秒' },
];

const statusColorMap = {
  queued: 'light-blue',
  pending: 'light-blue',
  processing: 'blue',
  in_progress: 'blue',
  completed: 'green',
  succeeded: 'green',
  failed: 'red',
  cancelled: 'grey',
  SUCCESS: 'green',
  FAILURE: 'red',
  QUEUED: 'light-blue',
  SUBMITTED: 'light-blue',
  IN_PROGRESS: 'blue',
};

const loadVideoPreferences = () => {
  if (typeof window === 'undefined') return {};
  try {
    return JSON.parse(window.localStorage.getItem(VIDEO_PREF_KEY) || '{}');
  } catch {
    return {};
  }
};

const saveVideoPreferences = (preferences) => {
  if (typeof window === 'undefined') return;
  window.localStorage.setItem(VIDEO_PREF_KEY, JSON.stringify(preferences));
};

const normalizeVideoStatus = (status) => {
  switch (status) {
    case 'SUCCESS':
      return 'completed';
    case 'FAILURE':
      return 'failed';
    case 'IN_PROGRESS':
      return 'processing';
    case 'SUBMITTED':
    case 'QUEUED':
      return 'queued';
    default:
      return status || 'queued';
  }
};

const readProgress = (value) => {
  if (typeof value === 'number') return value;
  const parsed = Number(String(value || '').replace('%', ''));
  return Number.isFinite(parsed) ? parsed : 0;
};

const unwrapTaskPayload = (payload) => {
  if (!payload) return null;
  if ((payload.success !== undefined || payload.code === 'success') && payload.data) {
    return payload.data;
  }
  return payload;
};

const getTaskData = (task) =>
  task?.data && typeof task.data === 'object' && !Array.isArray(task.data)
    ? task.data
    : {};

const getVideoUrl = (task) => {
  if (!task) return '';
  const data = getTaskData(task);
  return (
    task.video_url ||
    task.url ||
    task.result_url ||
    task.metadata?.url ||
    task.meta_data?.url ||
    task.ResultURL ||
    data.video_url ||
    data.url ||
    data.result_url ||
    data.metadata?.url ||
    data.meta_data?.url ||
    ''
  );
};

const normalizeTask = (payload) => {
  const source = unwrapTaskPayload(payload);
  if (!source) return null;
  const data = getTaskData(source);
  const properties = source.properties || {};
  const taskId =
    source.task_id ||
    (typeof source.id === 'string' ? source.id : '') ||
    data.task_id ||
    data.id ||
    source.TaskID;
  if (!taskId) return null;
  const status = normalizeVideoStatus(source.status || data.status || source.Status);
  return {
    id: taskId,
    task_id: taskId,
    model:
      properties.origin_model_name ||
      properties.upstream_model_name ||
      source.model ||
      data.model ||
      source.Model ||
      '',
    prompt:
      properties.input ||
      source.prompt ||
      data.prompt ||
      source.input ||
      data.input ||
      '',
    size: source.size || properties.size || data.size || '',
    seconds: String(source.seconds || properties.seconds || data.seconds || ''),
    status,
    progress: readProgress(source.progress || data.progress || source.Progress),
    created_at:
      source.submit_time ||
      source.created_at ||
      data.created_at ||
      source.createdAt ||
      source.CreatedAt ||
      0,
    completed_at:
      source.finish_time ||
      source.completed_at ||
      data.completed_at ||
      source.CompletedAt ||
      0,
    video_url: getVideoUrl(source),
    error:
      source.error ||
      data.error ||
      (source.fail_reason ? { message: source.fail_reason } : null),
  };
};

const mergeTasks = (tasks, nextTask) => {
  if (!nextTask?.task_id) return tasks;
  const exists = tasks.some((item) => item.task_id === nextTask.task_id);
  if (exists) {
    return tasks.map((item) =>
      item.task_id === nextTask.task_id ? { ...item, ...nextTask } : item,
    );
  }
  return [nextTask, ...tasks].slice(0, 30);
};

const AIVideo = () => {
  const { t } = useTranslation();
  const fileInputRef = useRef(null);
  const initialPreferences = useMemo(() => loadVideoPreferences(), []);
  const [models, setModels] = useState([]);
  const [modelConfigs, setModelConfigs] = useState([]);
  const [model, setModel] = useState(initialPreferences.model || '');
  const [prompt, setPrompt] = useState('');
  const [size, setSize] = useState(initialPreferences.size || '720x1280');
  const [seconds, setSeconds] = useState(initialPreferences.seconds || '10');
  const [referenceFiles, setReferenceFiles] = useState([]);
  const [submitting, setSubmitting] = useState(false);
  const [loadingTasks, setLoadingTasks] = useState(false);
  const [tasks, setTasks] = useState([]);
  const activeTask = tasks[0] || null;
  const activeVideoUrl =
    activeTask?.status === 'completed'
      ? getVideoUrl(activeTask) || `/v1/videos/${activeTask.task_id}/content`
      : '';

  const loadModels = useCallback(async () => {
    try {
      const res = await API.get('/api/ai-video/models');
      const raw = res.data?.data;
      const nextConfigs = Array.isArray(raw)
        ? raw
            .map((item) =>
              typeof item === 'string'
                ? { model: item, seconds: ['10', '15'] }
                : {
                    model: item?.model || '',
                    seconds: Array.isArray(item?.seconds)
                      ? item.seconds.map(String)
                      : ['10', '15'],
                  },
            )
            .filter((item) => item.model)
        : [];
      const nextModels = nextConfigs.map((item) => item.model);
      setModelConfigs(nextConfigs);
      setModels(nextModels);
      setModel((previous) =>
        nextModels.includes(previous) ? previous : nextModels[0] || '',
      );
    } catch (error) {
      showError(error?.message || t('加载 AI 视频模型失败'));
    }
  }, [t]);

  const currentSecondOptions = useMemo(() => {
    const config = modelConfigs.find((item) => item.model === model);
    const allowed = new Set(config?.seconds?.length ? config.seconds : ['10', '15']);
    return SECOND_OPTIONS.filter((item) => allowed.has(item.value));
  }, [model, modelConfigs]);

  useEffect(() => {
    if (
      currentSecondOptions.length > 0 &&
      !currentSecondOptions.some((item) => item.value === seconds)
    ) {
      setSeconds(currentSecondOptions[0].value);
    }
  }, [currentSecondOptions, seconds]);

  useEffect(() => {
    saveVideoPreferences({ model, size, seconds });
  }, [model, size, seconds]);

  const loadTasks = useCallback(async () => {
    setLoadingTasks(true);
    try {
      const responses = await Promise.all([
        API.get('/api/task/self', {
          params: { action: 'generate', p: 1, page_size: 30 },
        }),
        API.get('/api/task/self', {
          params: { action: 'textGenerate', p: 1, page_size: 30 },
        }),
      ]);
      const items = responses.flatMap((res) =>
        res.data?.success && Array.isArray(res.data.data?.items)
          ? res.data.data.items
          : [],
      );
      const selected = new Set(models);
      const normalized = items
        .map((item) => normalizeTask(item))
        .filter(Boolean)
        .filter((item) => selected.has(item.model));
      setTasks((previous) => {
        let merged = previous;
        normalized.forEach((item) => {
          merged = mergeTasks(merged, item);
        });
        return merged;
      });
    } catch {
      // 历史加载失败不影响继续提交新的视频任务。
    } finally {
      setLoadingTasks(false);
    }
  }, [models]);

  useEffect(() => {
    loadModels();
  }, [loadModels]);

  useEffect(() => {
    if (models.length > 0) {
      loadTasks();
    }
  }, [loadTasks, models.length]);

  const refreshTask = useCallback(async (taskId) => {
    if (!taskId) return;
    try {
      const res = await API.get(`/api/ai-video/videos/${taskId}`);
      const nextTask = normalizeTask(res.data);
      if (nextTask) {
        setTasks((previous) => mergeTasks(previous, nextTask));
      }
    } catch {
      // 单个任务轮询失败时保留当前状态，避免界面跳动。
    }
  }, []);

  useEffect(() => {
    const timer = window.setInterval(() => {
      tasks
        .filter((task) => ['queued', 'processing', 'in_progress'].includes(task.status))
        .slice(0, 5)
        .forEach((task) => refreshTask(task.task_id));
    }, 5000);
    return () => window.clearInterval(timer);
  }, [refreshTask, tasks]);

  const handleReferenceChange = (event) => {
    const files = Array.from(event.target.files || []).filter((file) =>
      file.type.startsWith('image/'),
    );
    event.target.value = '';
    setReferenceFiles(files.slice(0, 5));
  };

  const submitVideo = async () => {
    const trimmedPrompt = prompt.trim();
    if (!model) {
      showError(t('请先选择视频模型'));
      return;
    }
    if (!trimmedPrompt) {
      showError(t('请先输入视频提示词'));
      return;
    }
    const formData = new FormData();
    formData.append('model', model);
    formData.append('prompt', trimmedPrompt);
    formData.append('size', size);
    formData.append('seconds', seconds);
    referenceFiles.forEach((file) => {
      formData.append('input_reference', file, file.name);
    });
    setSubmitting(true);
    try {
      const res = await API.post('/api/ai-video/videos', formData);
      if (res.data?.success === false) {
        showError(res.data?.message || t('提交 AI 视频任务失败'));
        return;
      }
      const nextTask = normalizeTask(res.data);
      if (!nextTask) {
        showError(t('视频任务返回格式异常'));
        return;
      }
      setTasks((previous) =>
        mergeTasks(previous, {
          model,
          prompt: trimmedPrompt,
          size,
          seconds,
          ...nextTask,
        }),
      );
      setPrompt('');
      setReferenceFiles([]);
      showSuccess(t('AI 视频任务已提交'));
      window.setTimeout(() => refreshTask(nextTask.task_id), 1800);
    } catch (error) {
      showError(
        error?.response?.data?.message ||
          error?.response?.data?.error?.message ||
          error?.message ||
          t('提交 AI 视频任务失败'),
      );
    } finally {
      setSubmitting(false);
    }
  };

  const columns = useMemo(
    () => [
      {
        title: t('任务'),
        dataIndex: 'task_id',
        render: (value) => <span className='font-mono text-xs'>{value}</span>,
        width: 190,
      },
      { title: t('模型'), dataIndex: 'model', width: 160 },
      {
        title: t('参数'),
        dataIndex: 'size',
        render: (_, record) => (
          <span className='text-xs text-slate-500'>
            {[record.size, record.seconds ? `${record.seconds} 秒` : '']
              .filter(Boolean)
              .join(' / ') || '-'}
          </span>
        ),
        width: 130,
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (value, record) => (
          <div className='flex flex-col gap-1'>
            <Tag color={statusColorMap[value] || 'grey'}>{value}</Tag>
            {['queued', 'processing', 'in_progress'].includes(value) ? (
              <Progress percent={record.progress || 0} size='small' />
            ) : null}
          </div>
        ),
        width: 130,
      },
      {
        title: t('提交时间'),
        dataIndex: 'created_at',
        render: (value) => (value ? timestamp2string(value) : '-'),
        width: 170,
      },
      {
        title: t('结果'),
        dataIndex: 'video_url',
        render: (_, record) =>
          record.status === 'completed' ? (
            <Button
              size='small'
              type='primary'
              theme='light'
              onClick={() =>
                window.open(
                  getVideoUrl(record) || `/v1/videos/${record.task_id}/content`,
                  '_blank',
                  'noopener,noreferrer',
                )
              }
            >
              {t('查看')}
            </Button>
          ) : (
            record.error?.message || '-'
          ),
      },
    ],
    [t],
  );

  return (
    <div className='min-h-screen bg-slate-50 px-4 py-6 lg:px-8'>
      <div className='mx-auto flex max-w-7xl flex-col gap-5'>
        <div className='flex flex-wrap items-center justify-between gap-4'>
          <div>
            <div className='mb-2 flex items-center gap-2 text-sm font-medium text-sky-600'>
              <Clapperboard size={18} />
              {t('Sora / Veo 视频生成')}
            </div>
            <Typography.Title heading={3} className='!mb-1 !text-slate-950'>
              {t('AI 视频')}
            </Typography.Title>
            <Typography.Text className='!text-slate-500'>
              {t('输入提示词，可选参考图，提交后自动轮询生成进度。')}
            </Typography.Text>
          </div>
          <Button
            icon={<RefreshCw size={16} />}
            theme='light'
            type='primary'
            onClick={loadTasks}
            loading={loadingTasks}
          >
            {t('刷新任务')}
          </Button>
        </div>

        <div className='grid gap-5 xl:grid-cols-[420px_minmax(0,1fr)]'>
          <section className='rounded-lg border border-slate-200 bg-white p-5 shadow-sm'>
            <div className='mb-4 flex items-center gap-2'>
              <Sparkles size={18} className='text-sky-600' />
              <Typography.Title heading={5} className='!mb-0'>
                {t('生成参数')}
              </Typography.Title>
            </div>

            <div className='flex flex-col gap-4'>
              <label className='flex flex-col gap-2 text-sm'>
                <span className='font-medium text-slate-600'>{t('视频模型')}</span>
                <select
                  value={model}
                  onChange={(event) => setModel(event.target.value)}
                  className='h-11 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none focus:border-sky-400'
                >
                  {models.length > 0 ? (
                    models.map((item) => (
                      <option key={item} value={item}>
                        {item}
                      </option>
                    ))
                  ) : (
                    <option value=''>{t('暂无可用视频模型')}</option>
                  )}
                </select>
                {models.length === 0 ? (
                  <span className='text-xs text-amber-600'>
                    {t('请先在 AI 视频日志中勾选可用模型')}
                  </span>
                ) : null}
              </label>

              <label className='flex flex-col gap-2 text-sm'>
                <span className='font-medium text-slate-600'>{t('视频提示词')}</span>
                <TextArea
                  value={prompt}
                  onChange={setPrompt}
                  autosize={{ minRows: 5, maxRows: 10 }}
                  placeholder={t('例如：猫咪听歌摇头晃脑，窗外下大雨，电影感镜头')}
                />
              </label>

              <div className='grid grid-cols-2 gap-3'>
                <label className='flex flex-col gap-2 text-sm'>
                  <span className='font-medium text-slate-600'>{t('画面尺寸')}</span>
                  <select
                    value={size}
                    onChange={(event) => setSize(event.target.value)}
                    className='h-10 rounded-lg border border-slate-200 bg-white px-3'
                  >
                    {SIZE_OPTIONS.map((item) => (
                      <option key={item.value} value={item.value}>
                        {item.label}
                      </option>
                    ))}
                  </select>
                </label>
                <label className='flex flex-col gap-2 text-sm'>
                  <span className='font-medium text-slate-600'>{t('视频时长')}</span>
                  <select
                    value={seconds}
                    onChange={(event) => setSeconds(event.target.value)}
                    className='h-10 rounded-lg border border-slate-200 bg-white px-3'
                  >
                    {currentSecondOptions.map((item) => (
                      <option key={item.value} value={item.value}>
                        {item.label}
                      </option>
                    ))}
                  </select>
                </label>
              </div>

              <div>
                <input
                  ref={fileInputRef}
                  type='file'
                  accept='image/*'
                  multiple
                  className='hidden'
                  onChange={handleReferenceChange}
                />
                <Button
                  icon={<UploadCloud size={16} />}
                  theme='light'
                  type='primary'
                  className='!w-full !rounded-lg'
                  onClick={() => fileInputRef.current?.click()}
                >
                  {t('上传参考图')}
                </Button>
                {referenceFiles.length > 0 ? (
                  <div className='mt-3 flex flex-wrap gap-2'>
                    {referenceFiles.map((file) => (
                      <Tag key={`${file.name}-${file.size}`} color='blue'>
                        {file.name}
                      </Tag>
                    ))}
                    <Button
                      size='small'
                      theme='borderless'
                      type='danger'
                      icon={<Trash2 size={14} />}
                      onClick={() => setReferenceFiles([])}
                    >
                      {t('清空')}
                    </Button>
                  </div>
                ) : (
                  <div className='mt-2 flex items-center gap-1 text-xs text-slate-400'>
                    <ImagePlus size={14} />
                    {t('可上传多张图片作为 input_reference')}
                  </div>
                )}
              </div>

              <Button
                icon={<Play size={17} />}
                type='primary'
                size='large'
                loading={submitting}
                disabled={!model || models.length === 0}
                onClick={submitVideo}
                className='!h-12 !rounded-lg'
              >
                {t('生成视频')}
              </Button>
            </div>
          </section>

          <section className='flex min-h-[540px] flex-col rounded-lg border border-slate-200 bg-white p-5 shadow-sm'>
            <div className='mb-4 flex items-center justify-between gap-3'>
              <div>
                <Typography.Title heading={5} className='!mb-1'>
                  {t('生成预览')}
                </Typography.Title>
                <Typography.Text className='!text-sm !text-slate-500'>
                  {activeTask
                    ? `${activeTask.model || '-'} / ${activeTask.status}`
                    : t('还没有视频任务')}
                </Typography.Text>
              </div>
              {activeTask ? (
                <Tag color={statusColorMap[activeTask.status] || 'grey'}>
                  {activeTask.status}
                </Tag>
              ) : null}
            </div>

            <div className='flex min-h-[360px] flex-1 items-center justify-center overflow-hidden rounded-lg bg-slate-950'>
              {activeVideoUrl ? (
                <video
                  src={activeVideoUrl}
                  controls
                  className='h-full max-h-[560px] w-full object-contain'
                />
              ) : activeTask ? (
                <div className='flex flex-col items-center gap-4 px-6 text-center text-white'>
                  <Spin size='large' />
                  <div>
                    <div className='text-lg font-semibold'>
                      {t('视频正在生成中')}
                    </div>
                    <div className='mt-2 text-sm text-white/60'>
                      {t('当前进度')} {activeTask.progress || 0}%
                    </div>
                  </div>
                  <Progress
                    percent={activeTask.progress || 0}
                    style={{ width: 260 }}
                  />
                </div>
              ) : (
                <Empty
                  image={<Video size={48} className='text-slate-400' />}
                  title={t('提交任务后在这里预览视频')}
                  description={t('支持 Sora / Veo 同一套视频任务格式')}
                />
              )}
            </div>
          </section>
        </div>

        <section className='rounded-lg border border-slate-200 bg-white p-5 shadow-sm'>
          <div className='mb-3 flex items-center justify-between gap-3'>
            <Typography.Title heading={5} className='!mb-0'>
              {t('最近视频任务')}
            </Typography.Title>
            <Button
              size='small'
              theme='light'
              icon={<RefreshCw size={14} />}
              onClick={loadTasks}
              loading={loadingTasks}
            >
              {t('刷新')}
            </Button>
          </div>
          <Table
            rowKey='task_id'
            columns={columns}
            dataSource={tasks}
            pagination={false}
            empty={<Empty title={t('暂无视频任务')} />}
          />
        </section>
      </div>
    </div>
  );
};

export default AIVideo;
