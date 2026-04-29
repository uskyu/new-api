import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Input, InputNumber, Modal, Table, Tag, Typography } from '@douyinfe/semi-ui';
import { API, isRoot, showError, showSuccess, timestamp2string, toBoolean } from '../../helpers';

const statusColorMap = {
  PENDING: 'light-blue',
  PROCESSING: 'blue',
  SUCCEEDED: 'green',
  FAILED: 'red',
};

const AIImageLogs = () => {
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState([]);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [stats, setStats] = useState(null);
  const [dailyStats, setDailyStats] = useState(null);
  const [activeLogType, setActiveLogType] = useState('ai_image');
  const [filterUserId, setFilterUserId] = useState('');
  const [filterStatus, setFilterStatus] = useState('');
  const [filterStartTime, setFilterStartTime] = useState('');
  const [filterEndTime, setFilterEndTime] = useState('');
  const [savingConfig, setSavingConfig] = useState(false);
  const [testingS3, setTestingS3] = useState(false);
  const [s3TestResult, setS3TestResult] = useState(null);
  const [promptPreview, setPromptPreview] = useState(null);
  const [config, setConfig] = useState({
    enabled: false,
    workerConcurrency: 1,
    retryCount: 1,
    maxTimeoutMin: 10,
    pollIntervalSec: 3,
    queueLimit: 50,
    s3Enabled: false,
    s3Endpoint: '',
    s3Region: 'auto',
    s3Bucket: '',
    s3AccessKey: '',
    s3SecretKey: '',
    s3PublicBaseURL: '',
    s3PathPrefix: 'ai-image',
    s3UseSSL: true,
  });

  const toTimestamp = (value) => {
    if (!value) return '';
    const time = new Date(value).getTime();
    return Number.isFinite(time) ? String(Math.floor(time / 1000)) : '';
  };

  const buildQuery = useCallback((nextPage, nextPageSize) => {
    const params = new URLSearchParams({
      p: String(nextPage),
      page_size: String(nextPageSize),
    });
    if (filterUserId.trim()) params.set('user_id', filterUserId.trim());
    if (filterStatus) params.set('status', filterStatus);
    if (filterStartTime) params.set('start_time', toTimestamp(filterStartTime));
    if (filterEndTime) params.set('end_time', toTimestamp(filterEndTime));
    return params.toString();
  }, [filterEndTime, filterStartTime, filterStatus, filterUserId]);

  const loadData = useCallback(async (nextPage = page, nextPageSize = pageSize) => {
    setLoading(true);
    try {
      const query = buildQuery(nextPage, nextPageSize);
      const listURL = activeLogType === 'ecommerce'
        ? `/api/admin/ai-ecommerce/workflows?${query}`
        : `/api/admin/ai-image/tasks?source=ai_image&${query}`;
      const [taskRes, statsRes, dailyStatsRes] = await Promise.all([
        API.get(listURL),
        API.get('/api/admin/ai-image/stats'),
        API.get('/api/admin/ai-image/daily-stats'),
      ]);
      if (taskRes.data?.success) {
        const data = taskRes.data.data || {};
        setItems(Array.isArray(data.items) ? data.items : []);
        setTotal(Number(data.total || 0));
        setPage(Number(data.page || nextPage));
        setPageSize(Number(data.page_size || nextPageSize));
      } else {
        showError(taskRes.data?.message || '加载 AI 绘图日志失败');
      }
      if (statsRes.data?.success) {
        setStats(statsRes.data.data || null);
      }
      if (dailyStatsRes.data?.success) {
        setDailyStats(dailyStatsRes.data.data || null);
      }
    } catch (error) {
      showError(error?.message || '加载 AI 绘图日志失败');
    } finally {
      setLoading(false);
    }
  }, [activeLogType, buildQuery, page, pageSize]);

  useEffect(() => {
    loadData(1, pageSize);
  }, [activeLogType]);

  const loadConfig = useCallback(async () => {
    if (!isRoot()) {
      return;
    }
    try {
      const res = await API.get('/api/option/');
      if (!res.data?.success) {
        return;
      }
      const optionMap = {};
      (res.data.data || []).forEach((item) => {
        optionMap[item.key] = item.value;
      });
      setConfig({
        enabled: toBoolean(optionMap['ai_image_async_setting.enabled']),
        workerConcurrency: parseInt(optionMap['ai_image_async_setting.worker_concurrency'] || '1', 10),
        retryCount: parseInt(optionMap['ai_image_async_setting.retry_count'] || '1', 10),
        maxTimeoutMin: parseInt(optionMap['ai_image_async_setting.max_timeout_min'] || '10', 10),
        pollIntervalSec: parseInt(optionMap['ai_image_async_setting.poll_interval_sec'] || '3', 10),
        queueLimit: parseInt(optionMap['ai_image_async_setting.queue_limit'] || '50', 10),
        s3Enabled: toBoolean(optionMap['ai_image_async_setting.s3_enabled']),
        s3Endpoint: optionMap['ai_image_async_setting.s3_endpoint'] || '',
        s3Region: optionMap['ai_image_async_setting.s3_region'] || 'auto',
        s3Bucket: optionMap['ai_image_async_setting.s3_bucket'] || '',
        s3AccessKey: optionMap['ai_image_async_setting.s3_access_key'] || '',
        s3SecretKey: optionMap['ai_image_async_setting.s3_secret_key'] || '',
        s3PublicBaseURL: optionMap['ai_image_async_setting.s3_public_base_url'] || '',
        s3PathPrefix: optionMap['ai_image_async_setting.s3_path_prefix'] || 'ai-image',
        s3UseSSL: optionMap['ai_image_async_setting.s3_use_ssl'] === '' ? true : toBoolean(optionMap['ai_image_async_setting.s3_use_ssl']),
      });
    } catch {
      // Ignore config bootstrap failures for non-root or first-run state.
    }
  }, []);

  useEffect(() => {
    loadConfig();
  }, [loadConfig]);

  const saveConfig = useCallback(async () => {
    setSavingConfig(true);
    try {
      const updates = [
        ['ai_image_async_setting.enabled', String(config.enabled)],
        ['ai_image_async_setting.worker_concurrency', String(config.workerConcurrency || 1)],
        ['ai_image_async_setting.retry_count', String(Math.max(0, Number(config.retryCount || 0)))],
        ['ai_image_async_setting.max_timeout_min', String(Math.min(120, Math.max(1, Number(config.maxTimeoutMin || 10))))],
        ['ai_image_async_setting.poll_interval_sec', String(config.pollIntervalSec || 3)],
        ['ai_image_async_setting.queue_limit', String(config.queueLimit || 50)],
        ['ai_image_async_setting.s3_enabled', String(config.s3Enabled)],
        ['ai_image_async_setting.s3_endpoint', config.s3Endpoint || ''],
        ['ai_image_async_setting.s3_region', config.s3Region || 'auto'],
        ['ai_image_async_setting.s3_bucket', config.s3Bucket || ''],
        ['ai_image_async_setting.s3_access_key', config.s3AccessKey || ''],
        ['ai_image_async_setting.s3_secret_key', config.s3SecretKey || ''],
        ['ai_image_async_setting.s3_public_base_url', config.s3PublicBaseURL || ''],
        ['ai_image_async_setting.s3_path_prefix', config.s3PathPrefix || 'ai-image'],
        ['ai_image_async_setting.s3_use_ssl', String(config.s3UseSSL)],
      ];
      await Promise.all(
        updates.map(([key, value]) => API.put('/api/option/', { key, value })),
      );
      showSuccess('AI 绘图异步配置已保存');
      setS3TestResult(null);
      await loadData(1, pageSize);
    } catch (error) {
      showError(error?.message || '保存 AI 绘图异步配置失败');
    } finally {
      setSavingConfig(false);
    }
  }, [config, loadData, page, pageSize]);

  const testS3Connection = useCallback(async () => {
    setTestingS3(true);
    setS3TestResult(null);
    try {
      const res = await API.post('/api/admin/ai-image/test-s3');
      if (res.data?.success) {
        setS3TestResult({ ok: true, message: res.data.data?.message || 'S3 连通成功' });
        showSuccess('S3 连通性检测通过');
      } else {
        setS3TestResult({ ok: false, message: res.data?.message || 'S3 连通失败' });
        showError(res.data?.message || 'S3 连通失败');
      }
    } catch (error) {
      const msg = error?.response?.data?.message || error?.message || 'S3 连通检测异常';
      setS3TestResult({ ok: false, message: msg });
      showError(msg);
    } finally {
      setTestingS3(false);
    }
  }, []);

  const imageTaskColumns = useMemo(
    () => [
      {
        title: '提交时间',
        dataIndex: 'created_at',
        render: (value) => timestamp2string(value),
      },
      {
        title: '用户',
        dataIndex: 'username',
        render: (_, record) => record.username || `#${record.user_id}`,
      },
      {
        title: '模型',
        dataIndex: 'model',
      },
      {
        title: '状态',
        dataIndex: 'status',
        render: (value) => <Tag color={statusColorMap[value] || 'grey'}>{value}</Tag>,
      },
        {
          title: '提示词',
          dataIndex: 'prompt',
          render: (value) => {
            if (!value) {
              return '-';
            }
            return (
              <div className='max-w-[320px]'>
                <div
                  style={{
                    display: '-webkit-box',
                    WebkitBoxOrient: 'vertical',
                    WebkitLineClamp: 4,
                    overflow: 'hidden',
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-word',
                  }}
                >
                  {value}
                </div>
                {value.length > 80 ? (
                  <Button
                    theme='borderless'
                    type='primary'
                    size='small'
                    className='!px-0'
                    onClick={() => setPromptPreview(value)}
                  >
                    查看全文
                  </Button>
                ) : null}
              </div>
            );
          },
          width: 360,
        },
      {
        title: '结果',
        dataIndex: 'result_url',
        render: (value) =>
          value ? (
            <Button
              theme='light'
              type='primary'
              size='small'
              onClick={() => window.open(value, '_blank', 'noopener,noreferrer')}
            >
              查看
            </Button>
          ) : (
            '-'
          ),
      },
      {
        title: '来源',
        dataIndex: 'source',
        render: (value) => value || 'ai_image',
      },
      {
        title: '阶段',
        dataIndex: 'workflow_stage',
        render: (value) => value || '-',
      },
      {
        title: '错误',
        dataIndex: 'error_message',
        render: (value) => (
          <span className='line-clamp-2 block max-w-[280px] break-all text-red-500'>
            {value || '-'}
          </span>
        ),
      },
    ],
    [],
  );

  const ecommerceColumns = useMemo(
    () => [
      {
        title: '提交时间',
        dataIndex: 'created_at',
        render: (value) => timestamp2string(value),
      },
      {
        title: '用户',
        dataIndex: 'username',
        render: (_, record) => record.username || `#${record.user_id}`,
      },
      {
        title: '模板',
        dataIndex: 'template_name',
        render: (value, record) => value || record.template_key || '-',
      },
      {
        title: '商品',
        dataIndex: 'product_name',
        render: (value) => value || '-',
      },
      {
        title: '模型',
        dataIndex: 'model',
      },
      {
        title: '状态',
        dataIndex: 'status',
        render: (value) => <Tag color={statusColorMap[value] || 'grey'}>{value}</Tag>,
      },
      {
        title: '进度',
        dataIndex: 'segments',
        render: (segments = []) => {
          if (!Array.isArray(segments) || segments.length === 0) return '-';
          return (
            <div className='flex flex-wrap gap-1'>
              {segments.map((segment) => (
                <Tag key={segment.segment_key} color={statusColorMap[segment.status] || 'grey'}>
                  {segment.label}: {segment.status}
                </Tag>
              ))}
            </div>
          );
        },
        width: 320,
      },
      {
        title: '结果',
        dataIndex: 'assembled_url',
        render: (value, record) => (
          <div className='flex flex-wrap gap-2'>
            {record.mother_result_url ? (
              <Button theme='light' type='tertiary' size='small' onClick={() => window.open(record.mother_result_url, '_blank', 'noopener,noreferrer')}>
                母版
              </Button>
            ) : null}
            {value ? (
              <Button theme='light' type='primary' size='small' onClick={() => window.open(value, '_blank', 'noopener,noreferrer')}>
                长图
              </Button>
            ) : null}
            {!record.mother_result_url && !value ? '-' : null}
          </div>
        ),
      },
      {
        title: '错误',
        dataIndex: 'error_message',
        render: (value) => (
          <span className='line-clamp-2 block max-w-[280px] break-all text-red-500'>
            {value || '-'}
          </span>
        ),
      },
    ],
    [],
  );

  const columns = activeLogType === 'ecommerce' ? ecommerceColumns : imageTaskColumns;

  return (
    <div className='mt-[60px] space-y-4 px-2'>
      {isRoot() ? (
        <div className='rounded-2xl border border-slate-200 bg-white p-4 shadow-sm'>
          <div className='mb-3'>
            <Typography.Title heading={6} className='!mb-1'>
              AI 绘图异步配置
            </Typography.Title>
            <Typography.Text type='tertiary'>
              直接在 AI 绘图区域配置异步任务、S3 对象存储与 worker 并发。
            </Typography.Text>
          </div>
          <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
            <label className='flex flex-col gap-1 text-sm'>
              <span>异步处理开关</span>
              <select
                value={config.enabled ? 'true' : 'false'}
                onChange={(event) =>
                  setConfig((previous) => ({
                    ...previous,
                    enabled: event.target.value === 'true',
                  }))
                }
                className='h-9 rounded-lg border border-slate-200 px-3'
              >
                <option value='true'>启用</option>
                <option value='false'>关闭</option>
              </select>
            </label>
            <label className='flex flex-col gap-1 text-sm'>
              <span>Worker 并发</span>
              <InputNumber
                min={1}
                max={50}
                value={config.workerConcurrency}
                onChange={(value) =>
                  setConfig((previous) => ({
                    ...previous,
                    workerConcurrency: Number(value || 1),
                  }))
                }
              />
            </label>
            <label className='flex flex-col gap-1 text-sm'>
              <span>失败重试次数</span>
              <InputNumber
                min={0}
                max={10}
                value={config.retryCount}
                onChange={(value) =>
                  setConfig((previous) => ({
                    ...previous,
                    retryCount: Number(value || 0),
                  }))
                }
              />
            </label>
            <label className='flex flex-col gap-1 text-sm'>
              <span>最大超时分钟</span>
              <InputNumber
                min={1}
                max={120}
                value={config.maxTimeoutMin}
                onChange={(value) =>
                  setConfig((previous) => ({
                    ...previous,
                    maxTimeoutMin: Math.min(120, Math.max(1, Number(value || 10))),
                  }))
                }
              />
            </label>
            <label className='flex flex-col gap-1 text-sm'>
              <span>轮询间隔（秒）</span>
              <InputNumber
                min={1}
                max={30}
                value={config.pollIntervalSec}
                onChange={(value) =>
                  setConfig((previous) => ({
                    ...previous,
                    pollIntervalSec: Number(value || 3),
                  }))
                }
              />
            </label>
            <label className='flex flex-col gap-1 text-sm'>
              <span>队列上限</span>
              <InputNumber
                min={1}
                max={1000}
                value={config.queueLimit}
                onChange={(value) =>
                  setConfig((previous) => ({
                    ...previous,
                    queueLimit: Number(value || 50),
                  }))
                }
              />
            </label>
            <label className='flex flex-col gap-1 text-sm'>
              <span>S3 开关</span>
              <select
                value={config.s3Enabled ? 'true' : 'false'}
                onChange={(event) =>
                  setConfig((previous) => ({
                    ...previous,
                    s3Enabled: event.target.value === 'true',
                  }))
                }
                className='h-9 rounded-lg border border-slate-200 px-3'
              >
                <option value='true'>启用</option>
                <option value='false'>关闭</option>
              </select>
            </label>
            <label className='flex flex-col gap-1 text-sm md:col-span-2'>
              <span>S3 Endpoint</span>
              <Input
                value={config.s3Endpoint}
                onChange={(value) =>
                  setConfig((previous) => ({ ...previous, s3Endpoint: value }))
                }
                placeholder='qweapis3.orbiai.cloud'
              />
            </label>
            <label className='flex flex-col gap-1 text-sm'>
              <span>S3 Region</span>
              <Input
                value={config.s3Region}
                onChange={(value) =>
                  setConfig((previous) => ({ ...previous, s3Region: value }))
                }
                placeholder='auto'
              />
            </label>
            <label className='flex flex-col gap-1 text-sm'>
              <span>S3 Bucket</span>
              <Input
                value={config.s3Bucket}
                onChange={(value) =>
                  setConfig((previous) => ({ ...previous, s3Bucket: value }))
                }
                placeholder='qweapi'
              />
            </label>
            <label className='flex flex-col gap-1 text-sm md:col-span-2'>
              <span>Access Key</span>
              <Input
                value={config.s3AccessKey}
                onChange={(value) =>
                  setConfig((previous) => ({ ...previous, s3AccessKey: value }))
                }
              />
            </label>
            <label className='flex flex-col gap-1 text-sm md:col-span-2'>
              <span>Secret Key</span>
              <Input
                mode='password'
                value={config.s3SecretKey}
                onChange={(value) =>
                  setConfig((previous) => ({ ...previous, s3SecretKey: value }))
                }
              />
            </label>
            <label className='flex flex-col gap-1 text-sm md:col-span-2'>
              <span>Public Base URL</span>
              <Input
                value={config.s3PublicBaseURL}
                onChange={(value) =>
                  setConfig((previous) => ({ ...previous, s3PublicBaseURL: value }))
                }
                placeholder='https://cdn.example.com/ai-image'
              />
            </label>
            <label className='flex flex-col gap-1 text-sm'>
              <span>Path Prefix</span>
              <Input
                value={config.s3PathPrefix}
                onChange={(value) =>
                  setConfig((previous) => ({ ...previous, s3PathPrefix: value }))
                }
                placeholder='ai-image'
              />
            </label>
            <label className='flex flex-col gap-1 text-sm'>
              <span>HTTPS</span>
              <select
                value={config.s3UseSSL ? 'true' : 'false'}
                onChange={(event) =>
                  setConfig((previous) => ({
                    ...previous,
                    s3UseSSL: event.target.value === 'true',
                  }))
                }
                className='h-9 rounded-lg border border-slate-200 px-3'
              >
                <option value='true'>启用</option>
                <option value='false'>关闭</option>
              </select>
            </label>
          </div>
          <div className='mt-4 flex flex-wrap items-center justify-end gap-3'>
            {s3TestResult && (
              <span className={`text-sm ${s3TestResult.ok ? 'text-green-600' : 'text-red-500'}`}>
                {s3TestResult.message}
              </span>
            )}
            <Button
              loading={testingS3}
              onClick={testS3Connection}
              theme='light'
              type='primary'
              disabled={!config.s3Enabled || !config.s3Endpoint}
            >
              检测 S3 连通性
            </Button>
            <Button loading={savingConfig} onClick={saveConfig} type='primary'>
              保存 AI 绘图配置
            </Button>
          </div>
        </div>
      ) : null}

      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div>
          <Typography.Title heading={5} className='!mb-1'>
            AI 绘图日志
          </Typography.Title>
          <Typography.Text type='tertiary'>
            查看异步 AI 绘图任务、电商模板工作流、结果与失败信息。
          </Typography.Text>
        </div>
        <div className='flex flex-wrap gap-2 text-sm text-slate-500'>
          <span>Pending: {stats?.pending ?? '-'}</span>
          <span>Processing: {stats?.processing ?? '-'}</span>
          <span>Succeeded: {stats?.succeeded ?? '-'}</span>
          <span>Failed: {stats?.failed ?? '-'}</span>
        </div>
      </div>

      <div className='grid gap-3 md:grid-cols-3'>
        <div className='rounded-2xl border border-slate-200 bg-white p-4 shadow-sm'>
          <div className='text-sm text-slate-500'>今日 AI 绘画请求</div>
          <div className='mt-2 text-2xl font-semibold text-slate-900'>{dailyStats?.ai_image_tasks ?? '-'}</div>
        </div>
        <div className='rounded-2xl border border-slate-200 bg-white p-4 shadow-sm'>
          <div className='text-sm text-slate-500'>今日电商模板子任务</div>
          <div className='mt-2 text-2xl font-semibold text-slate-900'>{dailyStats?.ai_ecommerce_tasks ?? '-'}</div>
        </div>
        <div className='rounded-2xl border border-slate-200 bg-white p-4 shadow-sm'>
          <div className='text-sm text-slate-500'>今日电商模板工作流</div>
          <div className='mt-2 text-2xl font-semibold text-slate-900'>{dailyStats?.ai_ecommerce_workflows ?? '-'}</div>
        </div>
      </div>

      <div className='rounded-2xl border border-slate-200 bg-white p-4 shadow-sm'>
        <div className='mb-3 flex flex-wrap gap-2'>
          <Button
            type='primary'
            theme={activeLogType === 'ai_image' ? 'solid' : 'light'}
            className='!rounded-full'
            onClick={() => {
              setActiveLogType('ai_image');
              setFilterStatus('');
              setPage(1);
            }}
          >
            AI 绘画日志
          </Button>
          <Button
            type='primary'
            theme={activeLogType === 'ecommerce' ? 'solid' : 'light'}
            className='!rounded-full'
            onClick={() => {
              setActiveLogType('ecommerce');
              setFilterStatus('');
              setPage(1);
            }}
          >
            AI 电商绘图模板日志
          </Button>
        </div>
        <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-5'>
          <label className='flex flex-col gap-1 text-sm'>
            <span>用户 ID</span>
            <Input value={filterUserId} onChange={setFilterUserId} placeholder='例如 1' />
          </label>
          <label className='flex flex-col gap-1 text-sm'>
            <span>状态</span>
            <select value={filterStatus} onChange={(event) => setFilterStatus(event.target.value)} className='h-9 rounded-lg border border-slate-200 px-3'>
              <option value=''>全部</option>
              {activeLogType === 'ecommerce' ? (
                <>
                  <option value='MOTHER_PROCESSING'>MOTHER_PROCESSING</option>
                  <option value='WAITING_CONFIRM'>WAITING_CONFIRM</option>
                  <option value='SEGMENTS_PROCESSING'>SEGMENTS_PROCESSING</option>
                  <option value='SUCCEEDED'>SUCCEEDED</option>
                  <option value='FAILED'>FAILED</option>
                  <option value='MOTHER_FAILED'>MOTHER_FAILED</option>
                </>
              ) : (
                <>
                  <option value='PENDING'>PENDING</option>
                  <option value='PROCESSING'>PROCESSING</option>
                  <option value='SUCCEEDED'>SUCCEEDED</option>
                  <option value='FAILED'>FAILED</option>
                </>
              )}
            </select>
          </label>
          <label className='flex flex-col gap-1 text-sm'>
            <span>开始时间</span>
            <input type='datetime-local' value={filterStartTime} onChange={(event) => setFilterStartTime(event.target.value)} className='h-9 rounded-lg border border-slate-200 px-3' />
          </label>
          <label className='flex flex-col gap-1 text-sm'>
            <span>结束时间</span>
            <input type='datetime-local' value={filterEndTime} onChange={(event) => setFilterEndTime(event.target.value)} className='h-9 rounded-lg border border-slate-200 px-3' />
          </label>
          <div className='flex items-end gap-2'>
            <Button type='primary' className='!rounded-full' onClick={() => loadData(1, pageSize)}>搜索</Button>
            <Button className='!rounded-full' onClick={() => { setFilterUserId(''); setFilterStatus(''); setFilterStartTime(''); setFilterEndTime(''); setTimeout(() => loadData(1, pageSize), 0); }}>重置</Button>
          </div>
        </div>
      </div>

      <Table
        rowKey='task_id'
        loading={loading}
        columns={columns}
        dataSource={items}
        pagination={{
          currentPage: page,
          pageSize,
          total,
          onPageChange: (nextPage) => loadData(nextPage, pageSize),
          onPageSizeChange: (nextPageSize) => loadData(1, nextPageSize),
        }}
      />

      <Modal
        title='提示词全文'
        visible={Boolean(promptPreview)}
        onCancel={() => setPromptPreview(null)}
        footer={
          <Button type='primary' onClick={() => setPromptPreview(null)}>
            关闭
          </Button>
        }
      >
        <div className='max-h-[60vh] overflow-y-auto whitespace-pre-wrap break-words text-sm leading-6'>
          {promptPreview}
        </div>
      </Modal>
    </div>
  );
};

export default AIImageLogs;
