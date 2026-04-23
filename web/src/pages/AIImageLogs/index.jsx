import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Input, InputNumber, Table, Tag, Typography } from '@douyinfe/semi-ui';
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
  const [savingConfig, setSavingConfig] = useState(false);
  const [config, setConfig] = useState({
    enabled: false,
    workerConcurrency: 1,
    pollIntervalSec: 3,
    queueLimit: 50,
    s3Enabled: false,
    s3Endpoint: '',
    s3Region: 'auto',
    s3Bucket: '',
    s3AccessKey: '',
    s3SecretKey: '',
    s3PathPrefix: 'ai-image',
    s3UseSSL: true,
  });

  const loadData = useCallback(async (nextPage = page, nextPageSize = pageSize) => {
    setLoading(true);
    try {
      const [taskRes, statsRes] = await Promise.all([
        API.get(`/api/admin/ai-image/tasks?p=${nextPage}&page_size=${nextPageSize}`),
        API.get('/api/admin/ai-image/stats'),
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
    } catch (error) {
      showError(error?.message || '加载 AI 绘图日志失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize]);

  useEffect(() => {
    loadData(1, pageSize);
  }, [loadData, pageSize]);

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
        pollIntervalSec: parseInt(optionMap['ai_image_async_setting.poll_interval_sec'] || '3', 10),
        queueLimit: parseInt(optionMap['ai_image_async_setting.queue_limit'] || '50', 10),
        s3Enabled: toBoolean(optionMap['ai_image_async_setting.s3_enabled']),
        s3Endpoint: optionMap['ai_image_async_setting.s3_endpoint'] || '',
        s3Region: optionMap['ai_image_async_setting.s3_region'] || 'auto',
        s3Bucket: optionMap['ai_image_async_setting.s3_bucket'] || '',
        s3AccessKey: optionMap['ai_image_async_setting.s3_access_key'] || '',
        s3SecretKey: optionMap['ai_image_async_setting.s3_secret_key'] || '',
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
        ['ai_image_async_setting.poll_interval_sec', String(config.pollIntervalSec || 3)],
        ['ai_image_async_setting.queue_limit', String(config.queueLimit || 50)],
        ['ai_image_async_setting.s3_enabled', String(config.s3Enabled)],
        ['ai_image_async_setting.s3_endpoint', config.s3Endpoint || ''],
        ['ai_image_async_setting.s3_region', config.s3Region || 'auto'],
        ['ai_image_async_setting.s3_bucket', config.s3Bucket || ''],
        ['ai_image_async_setting.s3_access_key', config.s3AccessKey || ''],
        ['ai_image_async_setting.s3_secret_key', config.s3SecretKey || ''],
        ['ai_image_async_setting.s3_path_prefix', config.s3PathPrefix || 'ai-image'],
        ['ai_image_async_setting.s3_use_ssl', String(config.s3UseSSL)],
      ];
      await Promise.all(
        updates.map(([key, value]) => API.put('/api/option/', { key, value })),
      );
      showSuccess('AI 绘图异步配置已保存');
      await loadData(1, pageSize);
    } catch (error) {
      showError(error?.message || '保存 AI 绘图异步配置失败');
    } finally {
      setSavingConfig(false);
    }
  }, [config, loadData, page, pageSize]);

  const columns = useMemo(
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
        render: (value) => (
          <span className='line-clamp-2 block max-w-[320px] break-all'>
            {value || '-'}
          </span>
        ),
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
                max={10}
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
          <div className='mt-4 flex justify-end'>
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
            查看异步 AI 绘图任务状态、结果与失败信息。
          </Typography.Text>
        </div>
        <div className='flex flex-wrap gap-2 text-sm text-slate-500'>
          <span>Pending: {stats?.pending ?? '-'}</span>
          <span>Processing: {stats?.processing ?? '-'}</span>
          <span>Succeeded: {stats?.succeeded ?? '-'}</span>
          <span>Failed: {stats?.failed ?? '-'}</span>
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
    </div>
  );
};

export default AIImageLogs;
