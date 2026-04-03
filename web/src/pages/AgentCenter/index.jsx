import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Empty,
  Input,
  InputNumber,
  Modal,
  Pagination,
  Space,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  API,
  copy,
  renderQuotaWithAmount,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';

const { Title, Text } = Typography;
const DEFAULT_PAGE_SIZE = 10;

function formatAmount(amount) {
  return renderQuotaWithAmount(Number(amount || 0) / 100);
}

function formatRate(rate) {
  return `${(Number(rate || 0) / 100).toFixed(2)}%`;
}

export default function AgentCenter() {
  const { t } = useTranslation();
  const [summary, setSummary] = useState(null);
  const [summaryLoading, setSummaryLoading] = useState(false);
  const [rebates, setRebates] = useState([]);
  const [rebatesTotal, setRebatesTotal] = useState(0);
  const [rebatesPage, setRebatesPage] = useState(1);
  const [rebatesLoading, setRebatesLoading] = useState(false);
  const [adjustments, setAdjustments] = useState([]);
  const [adjustmentsTotal, setAdjustmentsTotal] = useState(0);
  const [adjustmentsPage, setAdjustmentsPage] = useState(1);
  const [adjustmentsLoading, setAdjustmentsLoading] = useState(false);
  const [promoLinks, setPromoLinks] = useState([]);
  const [promoLinksLoading, setPromoLinksLoading] = useState(false);
  const [promoLinkStats, setPromoLinkStats] = useState([]);
  const [promoLinkStatsLoading, setPromoLinkStatsLoading] = useState(false);
  const [promoLinkModalVisible, setPromoLinkModalVisible] = useState(false);
  const [promoLinkSubmitting, setPromoLinkSubmitting] = useState(false);
  const [promoLinkForm, setPromoLinkForm] = useState({
    name: '',
    code: '',
    remark: '',
  });
  const [downlines, setDownlines] = useState([]);
  const [downlinesLoading, setDownlinesLoading] = useState(false);
  const [upgradeModalVisible, setUpgradeModalVisible] = useState(false);
  const [upgradeSubmitting, setUpgradeSubmitting] = useState(false);
  const [upgradeForm, setUpgradeForm] = useState({
    targetUserId: 0,
    username: '',
    targetRatePercent: 0,
    remark: '',
  });

  const loadSummary = useCallback(async () => {
    setSummaryLoading(true);
    try {
      const res = await API.get('/api/agent/self');
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      setSummary(res.data.data);
    } catch (error) {
      showError(error.message || t('获取合作代理信息失败'));
    } finally {
      setSummaryLoading(false);
    }
  }, [t]);

  const loadRebates = useCallback(
    async (page = rebatesPage) => {
      if (!summary?.is_agent) {
        setRebates([]);
        setRebatesTotal(0);
        return;
      }
      setRebatesLoading(true);
      try {
        const res = await API.get('/api/agent/self/rebates', {
          params: {
            p: page,
            page_size: DEFAULT_PAGE_SIZE,
          },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        setRebates(res.data.data?.items || []);
        setRebatesTotal(res.data.data?.total || 0);
      } catch (error) {
        showError(error.message || t('获取返利流水失败'));
      } finally {
        setRebatesLoading(false);
      }
    },
    [rebatesPage, summary?.is_agent, t],
  );

  const loadAdjustments = useCallback(
    async (page = adjustmentsPage) => {
      if (!summary?.is_agent) {
        setAdjustments([]);
        setAdjustmentsTotal(0);
        return;
      }
      setAdjustmentsLoading(true);
      try {
        const res = await API.get('/api/agent/self/adjustments', {
          params: {
            p: page,
            page_size: DEFAULT_PAGE_SIZE,
          },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        setAdjustments(res.data.data?.items || []);
        setAdjustmentsTotal(res.data.data?.total || 0);
      } catch (error) {
        showError(error.message || t('获取调账记录失败'));
      } finally {
        setAdjustmentsLoading(false);
      }
    },
    [adjustmentsPage, summary?.is_agent, t],
  );

  const loadPromoLinks = useCallback(async () => {
    if (!summary?.is_agent) {
      setPromoLinks([]);
      return;
    }
    setPromoLinksLoading(true);
    try {
      const res = await API.get('/api/agent/self/promo-links', {
        params: {
          p: 1,
          page_size: 20,
        },
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      setPromoLinks(res.data.data?.items || []);
    } catch (error) {
      showError(error.message || t('获取推广链接失败'));
    } finally {
      setPromoLinksLoading(false);
    }
  }, [summary?.is_agent, t]);

  const loadPromoLinkStats = useCallback(async () => {
    if (!summary?.is_agent) {
      setPromoLinkStats([]);
      return;
    }
    setPromoLinkStatsLoading(true);
    try {
      const res = await API.get('/api/agent/self/promo-link-stats');
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      setPromoLinkStats(res.data.data || []);
    } catch (error) {
      showError(error.message || t('获取渠道统计失败'));
    } finally {
      setPromoLinkStatsLoading(false);
    }
  }, [summary?.is_agent, t]);

  const loadDownlines = useCallback(async () => {
    if (!summary?.is_agent) {
      setDownlines([]);
      return;
    }
    setDownlinesLoading(true);
    try {
      const res = await API.get('/api/agent/self/downlines', {
        params: {
          p: 1,
          page_size: 50,
        },
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      setDownlines(res.data.data?.items || []);
    } catch (error) {
      showError(error.message || t('获取直属下级失败'));
    } finally {
      setDownlinesLoading(false);
    }
  }, [summary?.is_agent, t]);

  useEffect(() => {
    loadSummary();
  }, [loadSummary]);

  useEffect(() => {
    if (summary?.is_agent) {
      loadRebates(1);
      loadAdjustments(1);
      loadPromoLinks();
      loadPromoLinkStats();
      loadDownlines();
    }
  }, [summary?.is_agent]);

  const refreshAll = useCallback(async () => {
    await loadSummary();
    if (summary?.is_agent) {
      await loadRebates(1);
      await loadAdjustments(1);
      await loadPromoLinks();
      await loadPromoLinkStats();
      await loadDownlines();
    }
  }, [loadAdjustments, loadDownlines, loadPromoLinkStats, loadPromoLinks, loadRebates, loadSummary, summary?.is_agent]);

  const handleCopyPromoLink = async (record) => {
    const landingPath = record.landing_page || '/';
    const url = `${window.location.origin}${landingPath.startsWith('/') ? landingPath : `/${landingPath}`}?aff=${record.code}`;
    const ok = await copy(url);
    if (ok) {
      showSuccess(t('推广链接已复制到剪贴板'));
      return;
    }
    showError(t('复制失败，请手动复制'));
  };

  const handleCreatePromoLink = async () => {
    setPromoLinkSubmitting(true);
    try {
      const res = await API.post('/api/agent/self/promo-link', {
        name: promoLinkForm.name,
        code: promoLinkForm.code,
        status: 1,
        landing_page: '/',
        remark: promoLinkForm.remark,
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('推广链接已创建'));
      setPromoLinkModalVisible(false);
      setPromoLinkForm({ name: '', code: '', remark: '' });
      await loadPromoLinks();
      await loadPromoLinkStats();
    } catch (error) {
      showError(error.message || t('创建推广链接失败'));
    } finally {
      setPromoLinkSubmitting(false);
    }
  };

  const handleDeletePromoLink = (record) => {
    Modal.confirm({
      title: t('确认删除推广链接？'),
      content: `${record.name} (${record.code})`,
      onOk: async () => {
        const res = await API.delete(`/api/agent/self/promo-link/${record.id}`);
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        showSuccess(t('推广链接已删除'));
        await loadPromoLinks();
        await loadPromoLinkStats();
      },
    });
  };

  const handleOpenUpgrade = (record) => {
    setUpgradeForm({
      targetUserId: record.user_id,
      username: record.username,
      targetRatePercent: 0,
      remark: '',
    });
    setUpgradeModalVisible(true);
  };

  const handleSubmitUpgrade = async () => {
    setUpgradeSubmitting(true);
    try {
      const res = await API.post('/api/agent/self/upgrade-request', {
        target_user_id: upgradeForm.targetUserId,
        target_rate: Math.round(Number(upgradeForm.targetRatePercent || 0) * 100),
        remark: upgradeForm.remark,
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('已成功开通代理'));
      setUpgradeModalVisible(false);
      await loadDownlines();
    } catch (error) {
      showError(error.message || t('提交代理开通申请失败'));
    } finally {
      setUpgradeSubmitting(false);
    }
  };

  const rebateColumns = useMemo(
    () => [
      { title: t('订单号'), dataIndex: 'trade_no' },
      { title: t('来源'), dataIndex: 'source_type' },
      { title: t('下级用户ID'), dataIndex: 'invitee_user_id' },
      {
        title: t('支付金额'),
        dataIndex: 'pay_amount',
        render: (_, record) => formatAmount(record.pay_amount),
      },
      {
        title: t('返利比例'),
        dataIndex: 'rebate_rate',
        render: (_, record) => formatRate(record.rebate_rate),
      },
      {
        title: t('返利金额'),
        dataIndex: 'rebate_amount',
        render: (_, record) => formatAmount(record.rebate_amount),
      },
      {
        title: t('结算时间'),
        dataIndex: 'settled_at',
        render: (_, record) => timestamp2string(record.settled_at || record.created_at),
      },
    ],
    [t],
  );

  const adjustmentColumns = useMemo(
    () => [
      {
        title: t('类型'),
        dataIndex: 'change_type',
        render: (_, record) =>
          record.change_type === 'increase' ? (
            <Tag color='green'>{t('增加')}</Tag>
          ) : (
            <Tag color='red'>{t('减少')}</Tag>
          ),
      },
      {
        title: t('调整金额'),
        dataIndex: 'delta_amount',
        render: (_, record) => formatAmount(record.delta_amount),
      },
      {
        title: t('调整后余额'),
        dataIndex: 'balance_after',
        render: (_, record) => formatAmount(record.balance_after),
      },
      { title: t('原因'), dataIndex: 'reason' },
      {
        title: t('时间'),
        dataIndex: 'created_at',
        render: (_, record) => timestamp2string(record.created_at),
      },
    ],
    [t],
  );

  const promoLinkColumns = useMemo(
    () => [
      { title: t('渠道名称'), dataIndex: 'name' },
      { title: t('推广码'), dataIndex: 'code' },
      {
        title: t('落地页'),
        dataIndex: 'landing_page',
        render: (_, record) => record.landing_page || '/',
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (_, record) =>
          record.status === 1 ? <Tag color='green'>{t('启用')}</Tag> : <Tag>{t('禁用')}</Tag>,
      },
      {
        title: t('操作'),
        dataIndex: 'operate',
        render: (_, record) => (
          <Space>
            <a onClick={() => handleCopyPromoLink(record)}>{t('复制链接')}</a>
            <a onClick={() => handleDeletePromoLink(record)}>{t('删除')}</a>
          </Space>
        ),
      },
    ],
    [t],
  );

  const promoLinkStatColumns = useMemo(
    () => [
      { title: t('渠道名称'), dataIndex: 'name' },
      { title: t('注册人数'), dataIndex: 'invitee_count' },
      { title: t('充值笔数'), dataIndex: 'topup_count' },
      {
        title: t('充值金额'),
        dataIndex: 'topup_amount',
        render: (_, record) => formatAmount(record.topup_amount),
      },
      {
        title: t('返利金额'),
        dataIndex: 'rebate_amount',
        render: (_, record) => formatAmount(record.rebate_amount),
      },
    ],
    [t],
  );

  const downlineColumns = useMemo(
    () => [
      { title: t('用户ID'), dataIndex: 'user_id' },
      { title: t('用户名'), dataIndex: 'username' },
      { title: t('显示名称'), dataIndex: 'display_name' },
      { title: t('来源渠道'), dataIndex: 'promo_link_name' },
      {
        title: t('是否代理'),
        dataIndex: 'is_agent',
        render: (_, record) =>
          record.is_agent ? <Tag color='blue'>{t('是')}</Tag> : <Tag>{t('否')}</Tag>,
      },
      {
        title: t('充值金额'),
        dataIndex: 'topup_amount',
        render: (_, record) => formatAmount(record.topup_amount),
      },
      {
        title: t('操作'),
        dataIndex: 'operate',
        render: (_, record) =>
          record.is_agent ? (
            <Text type='secondary'>{t('已是代理')}</Text>
          ) : (
            <a onClick={() => handleOpenUpgrade(record)}>{t('立即开通代理')}</a>
          ),
      },
    ],
    [t],
  );

  return (
    <div className='mt-[60px] px-2'>
      <Space vertical align='start' style={{ width: '100%' }} spacing={16}>
        <Card style={{ width: '100%' }} loading={summaryLoading}>
          <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
            <div className='flex flex-col md:flex-row md:justify-between md:items-center w-full gap-3'>
              <div>
                <Title heading={4} style={{ margin: 0 }}>
                  {t('合作代理')}
                </Title>
                <Text type='secondary'>
                  {t('这里展示独立的代理返利余额与流水，后续提现也将基于此账本。')}
                </Text>
              </div>
              <Button onClick={refreshAll}>{t('刷新')}</Button>
            </div>

            {!summary?.agent_initialized ? (
              <Empty
                title={t('代理功能尚未启用')}
                description={t('管理员完成初始化后，您才可以使用合作代理中心。')}
              />
            ) : !summary?.is_agent ? (
              <Empty
                title={t('您当前还不是代理')}
                description={t('管理员为您开通代理资料后，这里会显示返利余额和流水。')}
              />
            ) : (
              <div className='grid grid-cols-1 md:grid-cols-4 gap-4 w-full'>
                <Card>
                  <Text type='secondary'>{t('返利余额')}</Text>
                  <Title heading={3} style={{ margin: '8px 0 0' }}>
                    {formatAmount(summary?.profile?.rebate_balance_amount)}
                  </Title>
                </Card>
                <Card>
                  <Text type='secondary'>{t('累计返利')}</Text>
                  <Title heading={3} style={{ margin: '8px 0 0' }}>
                    {formatAmount(summary?.profile?.rebate_total_amount)}
                  </Title>
                </Card>
                <Card>
                  <Text type='secondary'>{t('当前比例')}</Text>
                  <Title heading={3} style={{ margin: '8px 0 0' }}>
                    {formatRate(summary?.profile?.effective_rate)}
                  </Title>
                </Card>
                <Card>
                  <Text type='secondary'>{t('返利笔数')}</Text>
                  <Title heading={3} style={{ margin: '8px 0 0' }}>
                    {summary?.recent_rebate_count || 0}
                  </Title>
                </Card>
              </div>
            )}
          </Space>
        </Card>

        {summary?.is_agent && (
          <>
            <Card style={{ width: '100%' }}>
              <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
                <div className='flex flex-col md:flex-row md:justify-between md:items-center w-full gap-3'>
                  <Title heading={5} style={{ margin: 0 }}>
                    {t('我的推广链接')}
                  </Title>
                  <Button type='primary' onClick={() => setPromoLinkModalVisible(true)}>
                    {t('新增推广链接')}
                  </Button>
                </div>
                <Table
                  rowKey='id'
                  columns={promoLinkColumns}
                  dataSource={promoLinks}
                  loading={promoLinksLoading}
                  pagination={false}
                  empty={<Empty title={t('暂无推广链接')} description={t('管理员为您创建推广链接后，会显示在这里。')} />}
                />
              </Space>
            </Card>

            <Card style={{ width: '100%' }}>
              <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
                <Title heading={5} style={{ margin: 0 }}>
                  {t('渠道统计')}
                </Title>
                <Table
                  rowKey='promo_link_id'
                  columns={promoLinkStatColumns}
                  dataSource={promoLinkStats}
                  loading={promoLinkStatsLoading}
                  pagination={false}
                  empty={<Empty title={t('暂无渠道统计')} />}
                />
              </Space>
            </Card>

            <Card style={{ width: '100%' }}>
              <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
                <Title heading={5} style={{ margin: 0 }}>
                  {t('我的直属下级')}
                </Title>
                <Table
                  rowKey='user_id'
                  columns={downlineColumns}
                  dataSource={downlines}
                  loading={downlinesLoading}
                  pagination={false}
                  empty={<Empty title={t('暂无直属下级')} />}
                />
              </Space>
            </Card>

            <Card style={{ width: '100%' }}>
              <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
                <Title heading={5} style={{ margin: 0 }}>
                  {t('返利流水')}
                </Title>
                <Table
                  rowKey='id'
                  columns={rebateColumns}
                  dataSource={rebates}
                  loading={rebatesLoading}
                  pagination={false}
                  empty={<Empty title={t('暂无返利流水')} />}
                />
                <Pagination
                  currentPage={rebatesPage}
                  pageSize={DEFAULT_PAGE_SIZE}
                  total={rebatesTotal}
                  onPageChange={(page) => {
                    setRebatesPage(page);
                    loadRebates(page);
                  }}
                  showTotal
                />
              </Space>
            </Card>

            <Card style={{ width: '100%' }}>
              <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
                <Title heading={5} style={{ margin: 0 }}>
                  {t('人工调账记录')}
                </Title>
                <Table
                  rowKey='id'
                  columns={adjustmentColumns}
                  dataSource={adjustments}
                  loading={adjustmentsLoading}
                  pagination={false}
                  empty={<Empty title={t('暂无调账记录')} />}
                />
                <Pagination
                  currentPage={adjustmentsPage}
                  pageSize={DEFAULT_PAGE_SIZE}
                  total={adjustmentsTotal}
                  onPageChange={(page) => {
                    setAdjustmentsPage(page);
                    loadAdjustments(page);
                  }}
                  showTotal
                />
              </Space>
            </Card>
          </>
        )}
      </Space>

      <Modal
        title={t('新增推广链接')}
        visible={promoLinkModalVisible}
        onCancel={() => setPromoLinkModalVisible(false)}
        onOk={handleCreatePromoLink}
        confirmLoading={promoLinkSubmitting}
      >
        <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
          <Input
            placeholder={t('渠道名称')}
            value={promoLinkForm.name}
            onChange={(value) => setPromoLinkForm((prev) => ({ ...prev, name: value }))}
          />
          <Input
            placeholder={t('推广码，留空自动生成')}
            value={promoLinkForm.code}
            onChange={(value) => setPromoLinkForm((prev) => ({ ...prev, code: value }))}
          />
          <Input
            placeholder={t('备注')}
            value={promoLinkForm.remark}
            onChange={(value) => setPromoLinkForm((prev) => ({ ...prev, remark: value }))}
          />
        </Space>
      </Modal>

      <Modal
        title={t('立即开通代理')}
        visible={upgradeModalVisible}
        onCancel={() => setUpgradeModalVisible(false)}
        onOk={handleSubmitUpgrade}
        confirmLoading={upgradeSubmitting}
      >
        <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
          <Text>
            {t('目标用户：')}
            {upgradeForm.username}
          </Text>
          <InputNumber
            min={0}
            step={0.1}
            suffix='%'
            value={upgradeForm.targetRatePercent}
            onChange={(value) =>
              setUpgradeForm((prev) => ({ ...prev, targetRatePercent: value || 0 }))
            }
            style={{ width: '100%' }}
          />
          <Input
            placeholder={t('备注')}
            value={upgradeForm.remark}
            onChange={(value) => setUpgradeForm((prev) => ({ ...prev, remark: value }))}
          />
        </Space>
      </Modal>
    </div>
  );
}
