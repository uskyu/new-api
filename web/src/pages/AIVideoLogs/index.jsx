import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Checkbox,
  Empty,
  Input,
  Modal,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { RefreshCw, Save, Search, Video } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';

const statusColorMap = {
  SUCCESS: 'green',
  FAILURE: 'red',
  IN_PROGRESS: 'blue',
  QUEUED: 'light-blue',
  SUBMITTED: 'light-blue',
  NOT_START: 'grey',
};

const VIDEO_SECOND_OPTIONS = ['5', '10', '15'];
const DEFAULT_VIDEO_SECONDS = ['10', '15'];
const normalizeSeconds = (seconds) =>
  (Array.isArray(seconds) ? seconds.map(String) : [])
    .filter((second) => VIDEO_SECOND_OPTIONS.includes(second))
    .sort(
      (a, b) =>
        VIDEO_SECOND_OPTIONS.indexOf(a) - VIDEO_SECOND_OPTIONS.indexOf(b),
    );

const isProbablyVideoModel = (model) => {
  const value = String(model || '').toLowerCase();
  return (
    value.includes('sora') ||
    value.includes('veo') ||
    value.includes('video') ||
    value.includes('kling') ||
    value.includes('vidu') ||
    value.includes('hailuo')
  );
};

const getTaskModel = (task) =>
  task?.properties?.origin_model_name ||
  task?.properties?.upstream_model_name ||
  task?.model ||
  '';

const AIVideoLogs = () => {
  const { t } = useTranslation();
  const [availableModels, setAvailableModels] = useState([]);
  const [enabledModels, setEnabledModels] = useState([]);
  const [modelConfigs, setModelConfigs] = useState({});
  const [modelSearch, setModelSearch] = useState('');
  const [loadingModels, setLoadingModels] = useState(false);
  const [savingModels, setSavingModels] = useState(false);
  const [tasks, setTasks] = useState([]);
  const [loadingTasks, setLoadingTasks] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [filterStatus, setFilterStatus] = useState('');
  const [filterTaskId, setFilterTaskId] = useState('');
  const [promptPreview, setPromptPreview] = useState(null);
  const [modelPickerVisible, setModelPickerVisible] = useState(false);

  const loadModels = useCallback(async () => {
    setLoadingModels(true);
    try {
      const res = await API.get('/api/admin/ai-video/models');
      if (!res.data?.success) {
        showError(res.data?.message || t('加载 AI 视频模型配置失败'));
        return;
      }
      const data = res.data.data || {};
      const enabled = Array.isArray(data.enabled) ? data.enabled : [];
      const configs = Array.isArray(data.configs) ? data.configs : [];
      const nextConfigMap = {};
      configs.forEach((item) => {
        if (!item?.model) return;
        const seconds = normalizeSeconds(item.seconds);
        nextConfigMap[item.model] = seconds.length
          ? seconds
          : DEFAULT_VIDEO_SECONDS;
      });
      enabled.forEach((modelName) => {
        if (!nextConfigMap[modelName]) {
          nextConfigMap[modelName] = DEFAULT_VIDEO_SECONDS;
        }
      });
      setAvailableModels(Array.isArray(data.available) ? data.available : []);
      setEnabledModels(enabled);
      setModelConfigs(nextConfigMap);
    } catch (error) {
      showError(error?.message || t('加载 AI 视频模型配置失败'));
    } finally {
      setLoadingModels(false);
    }
  }, [t]);

  const loadTasks = useCallback(
    async (nextPage = page, nextPageSize = pageSize) => {
      setLoadingTasks(true);
      try {
        const params = {
          p: nextPage,
          page_size: nextPageSize,
          action: 'generate',
        };
        if (filterStatus) params.status = filterStatus;
        if (filterTaskId.trim()) params.task_id = filterTaskId.trim();
        const res = await API.get('/api/task/', { params });
        if (!res.data?.success) {
          showError(res.data?.message || t('加载 AI 视频日志失败'));
          return;
        }
        const data = res.data.data || {};
        const selected = new Set(enabledModels);
        const items = Array.isArray(data.items) ? data.items : [];
        setTasks(
          items.filter((item) => {
            const model = getTaskModel(item);
            return selected.has(model) || isProbablyVideoModel(model);
          }),
        );
        setTotal(Number(data.total || 0));
        setPage(Number(data.page || nextPage));
        setPageSize(Number(data.page_size || nextPageSize));
      } catch (error) {
        showError(error?.message || t('加载 AI 视频日志失败'));
      } finally {
        setLoadingTasks(false);
      }
    },
    [enabledModels, filterStatus, filterTaskId, page, pageSize, t],
  );

  useEffect(() => {
    loadModels();
  }, [loadModels]);

  useEffect(() => {
    loadTasks(1, pageSize);
  }, [enabledModels]);

  const filteredModels = useMemo(() => {
    const keyword = modelSearch.trim().toLowerCase();
    const models = keyword
      ? availableModels.filter((item) => item.toLowerCase().includes(keyword))
      : availableModels;
    const selected = new Set(enabledModels);
    return models.sort((a, b) => {
      const aSelected = selected.has(a) ? 0 : 1;
      const bSelected = selected.has(b) ? 0 : 1;
      if (aSelected !== bSelected) return aSelected - bSelected;
      return a.localeCompare(b);
    });
  }, [availableModels, enabledModels, modelSearch]);

  const toggleModel = (modelName, checked) => {
    setEnabledModels((previous) => {
      if (checked) {
        return Array.from(new Set([...previous, modelName])).sort();
      }
      return previous.filter((item) => item !== modelName);
    });
    if (checked) {
      setModelConfigs((previous) => ({
        ...previous,
        [modelName]: previous[modelName]?.length
          ? previous[modelName]
          : DEFAULT_VIDEO_SECONDS,
      }));
    }
  };

  const toggleModelSecond = (modelName, second, checked) => {
    setModelConfigs((previous) => {
      const current = previous[modelName]?.length
        ? previous[modelName]
        : DEFAULT_VIDEO_SECONDS;
      const next = checked
        ? Array.from(new Set([...current, second]))
        : current.filter((item) => item !== second);
      return {
        ...previous,
          [modelName]: next.length > 0 ? normalizeSeconds(next) : current,
      };
    });
  };

  const saveModels = async () => {
    setSavingModels(true);
    try {
      const configs = enabledModels.map((modelName) => ({
        model: modelName,
        seconds: modelConfigs[modelName]?.length
          ? normalizeSeconds(modelConfigs[modelName])
          : DEFAULT_VIDEO_SECONDS,
      }));
      const res = await API.put('/api/admin/ai-video/models', {
        models: enabledModels,
        configs,
      });
      if (res.data?.success) {
        showSuccess(t('AI 视频模型配置已保存'));
        const next = res.data.data?.enabled;
        if (Array.isArray(next)) setEnabledModels(next);
        const nextConfigs = res.data.data?.configs;
        if (Array.isArray(nextConfigs)) {
          const nextConfigMap = {};
          nextConfigs.forEach((item) => {
            if (item?.model) {
              const seconds = normalizeSeconds(item.seconds);
              nextConfigMap[item.model] = seconds.length
                ? seconds
                : DEFAULT_VIDEO_SECONDS;
            }
          });
          setModelConfigs(nextConfigMap);
        }
      } else {
        showError(res.data?.message || t('保存 AI 视频模型配置失败'));
      }
    } catch (error) {
      showError(error?.message || t('保存 AI 视频模型配置失败'));
    } finally {
      setSavingModels(false);
    }
  };

  const columns = useMemo(
    () => [
      {
        title: t('提交时间'),
        dataIndex: 'submit_time',
        render: (value, record) =>
          timestamp2string(value || record.created_at || record.CreatedAt),
        width: 170,
      },
      {
        title: t('用户'),
        dataIndex: 'username',
        render: (_, record) => record.username || `#${record.user_id}`,
        width: 120,
      },
      {
        title: t('任务 ID'),
        dataIndex: 'task_id',
        render: (value) => <span className='font-mono text-xs'>{value}</span>,
        width: 190,
      },
      {
        title: t('模型'),
        dataIndex: 'properties',
        render: (_, record) => getTaskModel(record) || '-',
        width: 160,
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (value, record) => (
          <div className='flex flex-col gap-1'>
            <Tag color={statusColorMap[value] || 'grey'}>{value}</Tag>
            <span className='text-xs text-slate-400'>{record.progress || '-'}</span>
          </div>
        ),
        width: 120,
      },
      {
        title: t('提示词'),
        dataIndex: 'properties',
        render: (_, record) => {
          const text = record?.properties?.input || '';
          if (!text) return '-';
          return (
            <div className='max-w-[360px]'>
              <div className='line-clamp-3 whitespace-pre-wrap break-words text-sm'>
                {text}
              </div>
              {text.length > 80 ? (
                <Button
                  theme='borderless'
                  type='primary'
                  size='small'
                  className='!px-0'
                  onClick={() => setPromptPreview(text)}
                >
                  {t('查看全文')}
                </Button>
              ) : null}
            </div>
          );
        },
      },
      {
        title: t('结果'),
        dataIndex: 'result_url',
        render: (_, record) => {
          const url = record.result_url || `/v1/videos/${record.task_id}/content`;
          return record.status === 'SUCCESS' ? (
            <Button
              size='small'
              theme='light'
              type='primary'
              onClick={() => window.open(url, '_blank', 'noopener,noreferrer')}
            >
              {t('查看')}
            </Button>
          ) : (
            record.fail_reason || '-'
          );
        },
      },
    ],
    [t],
  );

  return (
    <div className='mt-[60px] space-y-5 px-2'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div>
          <Typography.Title heading={4} className='!mb-1'>
            {t('AI 视频日志')}
          </Typography.Title>
          <Typography.Text type='tertiary'>
            {t('配置用户侧可见视频模型，并查看 Sora / Veo 视频生成任务。')}
          </Typography.Text>
        </div>
        <Button
          icon={<RefreshCw size={16} />}
          theme='light'
          type='primary'
          onClick={() => {
            loadModels();
            loadTasks(1, pageSize);
          }}
        >
          {t('刷新')}
        </Button>
      </div>

      <section className='rounded-2xl border border-slate-200 bg-white p-4 shadow-sm'>
        <div className='mb-4 flex flex-wrap items-center justify-between gap-3'>
          <div className='flex items-center gap-2'>
            <Video size={18} className='text-sky-600' />
            <Typography.Title heading={5} className='!mb-0'>
              {t('AI 视频模型控制板')}
            </Typography.Title>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button
              icon={<RefreshCw size={15} />}
              loading={loadingModels}
              onClick={async () => {
                await loadModels();
                setModelPickerVisible(true);
              }}
              theme='light'
              type='primary'
            >
              {t('获取模型广场模型')}
            </Button>
            <Button
              icon={<Save size={15} />}
              loading={savingModels}
              onClick={saveModels}
              type='primary'
            >
              {t('保存勾选模型')}
            </Button>
          </div>
        </div>

        <div className='rounded-lg border border-slate-100 bg-slate-50 px-3 py-3'>
          <div className='mb-2 flex items-center gap-2 text-sm text-slate-500'>
            <span>{t('当前已展示模型')}</span>
            <Tag color='blue'>{enabledModels.length}</Tag>
          </div>
          {enabledModels.length > 0 ? (
            <div className='flex flex-wrap gap-2'>
              {enabledModels.map((item) => (
                <div
                  key={item}
                  className='flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-2 py-1'
                >
                  <span className='font-mono text-xs text-slate-700'>{item}</span>
                  <span className='text-xs text-slate-400'>
                    {(modelConfigs[item]?.length
                      ? modelConfigs[item]
                      : DEFAULT_VIDEO_SECONDS
                    ).join('/')}
                    {t('秒')}
                  </span>
                </div>
              ))}
            </div>
          ) : (
            <Typography.Text type='tertiary'>
              {t('还没有勾选视频模型，用户侧不会显示视频模型。')}
            </Typography.Text>
          )}
        </div>
      </section>

      <section className='rounded-2xl border border-slate-200 bg-white p-4 shadow-sm'>
        <div className='mb-3 grid gap-3 md:grid-cols-[180px_180px_auto]'>
          <Input
            value={filterTaskId}
            onChange={setFilterTaskId}
            placeholder={t('任务 ID')}
          />
          <select
            value={filterStatus}
            onChange={(event) => setFilterStatus(event.target.value)}
            className='h-9 rounded-lg border border-slate-200 px-3'
          >
            <option value=''>{t('全部状态')}</option>
            <option value='SUBMITTED'>SUBMITTED</option>
            <option value='QUEUED'>QUEUED</option>
            <option value='IN_PROGRESS'>IN_PROGRESS</option>
            <option value='SUCCESS'>SUCCESS</option>
            <option value='FAILURE'>FAILURE</option>
          </select>
          <Button
            icon={<Search size={15} />}
            type='primary'
            className='!rounded-full'
            onClick={() => loadTasks(1, pageSize)}
          >
            {t('搜索')}
          </Button>
        </div>

        <Table
          rowKey='task_id'
          loading={loadingTasks}
          columns={columns}
          dataSource={tasks}
          empty={<Empty title={t('暂无 AI 视频任务')} />}
          pagination={{
            currentPage: page,
            pageSize,
            total,
            onPageChange: (nextPage) => loadTasks(nextPage, pageSize),
            onPageSizeChange: (nextPageSize) => loadTasks(1, nextPageSize),
          }}
        />
      </section>

      <Modal
        title={t('提示词全文')}
        visible={Boolean(promptPreview)}
        onCancel={() => setPromptPreview(null)}
        footer={
          <Button type='primary' onClick={() => setPromptPreview(null)}>
            {t('关闭')}
          </Button>
        }
      >
        <div className='max-h-[60vh] overflow-y-auto whitespace-pre-wrap break-words text-sm leading-6'>
          {promptPreview}
        </div>
      </Modal>

      <Modal
        title={t('选择 AI 视频模型')}
        visible={modelPickerVisible}
        onCancel={() => setModelPickerVisible(false)}
        footer={
          <div className='flex justify-end gap-2'>
            <Button onClick={() => setModelPickerVisible(false)}>
              {t('取消')}
            </Button>
            <Button
              type='primary'
              loading={savingModels}
              onClick={async () => {
                await saveModels();
                setModelPickerVisible(false);
              }}
            >
              {t('保存并关闭')}
            </Button>
          </div>
        }
        style={{ width: 760, maxWidth: '92vw' }}
      >
        <div className='mb-3 grid gap-3 md:grid-cols-[minmax(0,1fr)_auto]'>
          <Input
            prefix={<Search size={15} />}
            value={modelSearch}
            onChange={setModelSearch}
            placeholder={t('搜索模型名称，例如 sora、veo')}
          />
          <div className='flex items-center gap-2 text-sm text-slate-500'>
            <span>{t('已勾选')}</span>
            <Tag color='blue'>{enabledModels.length}</Tag>
          </div>
        </div>

        {filteredModels.length > 0 ? (
          <div className='grid max-h-[52vh] gap-2 overflow-y-auto rounded-lg border border-slate-100 p-3 md:grid-cols-2'>
            {filteredModels.map((item) => (
              <div
                key={item}
                className='min-w-0 rounded-lg border border-slate-100 px-3 py-2 text-sm transition hover:border-sky-200 hover:bg-sky-50'
              >
                <label className='flex min-w-0 items-center gap-2'>
                  <Checkbox
                    checked={enabledModels.includes(item)}
                    onChange={(event) => toggleModel(item, event.target.checked)}
                  />
                  <span className='truncate font-mono text-xs'>{item}</span>
                </label>
                {enabledModels.includes(item) ? (
                  <div className='mt-2 flex flex-wrap gap-2 pl-6'>
                    {VIDEO_SECOND_OPTIONS.map((second) => (
                      <Checkbox
                        key={`${item}-${second}`}
                        checked={(modelConfigs[item]?.length
                          ? modelConfigs[item]
                          : DEFAULT_VIDEO_SECONDS
                        ).includes(second)}
                        onChange={(event) =>
                          toggleModelSecond(item, second, event.target.checked)
                        }
                      >
                        {second}
                        {t('秒')}
                      </Checkbox>
                    ))}
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        ) : (
          <Empty title={t('暂无模型，请先获取模型广场模型')} />
        )}
      </Modal>
    </div>
  );
};

export default AIVideoLogs;
