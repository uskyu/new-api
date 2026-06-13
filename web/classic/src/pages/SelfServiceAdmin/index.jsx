import React, { useEffect, useState } from 'react';
import {
  Button,
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Table,
  Tabs,
  TabPane,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import { displayAmountToQuota, quotaToDisplayAmount } from '../../helpers/quota';
import { renderQuota } from '../../helpers/render';

const { Title, Text } = Typography;

const blankRule = () => ({
  threshold_amount: 0,
  target_group: '',
  description: '',
  enabled: true,
});

export default function SelfServiceAdmin() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [config, setConfig] = useState(null);
  const [rules, setRules] = useState([]);
  const [groups, setGroups] = useState([]);
  const [stats, setStats] = useState({});
  const [attempts, setAttempts] = useState([]);
  const [attemptTotal, setAttemptTotal] = useState(0);
  const [refunds, setRefunds] = useState([]);
  const [refundTotal, setRefundTotal] = useState(0);
  const [upgrades, setUpgrades] = useState([]);
  const [upgradeTotal, setUpgradeTotal] = useState(0);
  const [attemptPage, setAttemptPage] = useState(1);
  const [refundPage, setRefundPage] = useState(1);
  const [upgradePage, setUpgradePage] = useState(1);

  const loadConfig = async () => {
    setLoading(true);
    try {
      const [configRes, groupRes] = await Promise.all([
        API.get('/api/self-service/admin/config'),
        API.get('/api/group/'),
      ]);
      if (configRes.data.success) {
        const data = configRes.data.data;
        setConfig(data.config);
        setStats(data.stats || {});
        setRules(
          (data.rules || []).map((rule) => ({
            ...rule,
            threshold_amount: quotaToDisplayAmount(rule.threshold_quota),
          })),
        );
      } else {
        showError(configRes.data.message || t('加载失败'));
      }
      if (groupRes.data.success) {
        setGroups(groupRes.data.data || []);
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  const loadAttempts = async (page = attemptPage) => {
    try {
      const res = await API.get('/api/self-service/admin/claim-attempts', {
        params: { p: page, page_size: 10 },
      });
      if (res.data.success) {
        setAttempts(res.data.data.items || []);
        setAttemptTotal(res.data.data.total || 0);
      }
    } catch (error) {
      showError(error);
    }
  };

  const loadRefunds = async (page = refundPage) => {
    try {
      const res = await API.get('/api/self-service/admin/refund-histories', {
        params: { p: page, page_size: 10 },
      });
      if (res.data.success) {
        setRefunds(res.data.data.items || []);
        setRefundTotal(res.data.data.total || 0);
      }
    } catch (error) {
      showError(error);
    }
  };

  const loadUpgrades = async (page = upgradePage) => {
    try {
      const res = await API.get('/api/self-service/admin/upgrade-histories', {
        params: { p: page, page_size: 10 },
      });
      if (res.data.success) {
        setUpgrades(res.data.data.items || []);
        setUpgradeTotal(res.data.data.total || 0);
      }
    } catch (error) {
      showError(error);
    }
  };

  useEffect(() => {
    loadConfig();
    loadAttempts(1);
    loadRefunds(1);
    loadUpgrades(1);
  }, []);

  const saveConfig = async () => {
    setSaving(true);
    try {
      const res = await API.put('/api/self-service/admin/config', config);
      if (res.data.success) {
        setConfig(res.data.data);
        showSuccess(t('保存成功'));
      } else {
        showError(res.data.message || t('保存失败'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  const saveRules = async () => {
    setSaving(true);
    try {
      const payload = rules
        .filter((rule) => rule.target_group && rule.enabled !== false)
        .map((rule) => ({
          id: rule.id || 0,
          threshold_quota: displayAmountToQuota(rule.threshold_amount),
          target_group: rule.target_group,
          description: rule.description || '',
          enabled: rule.enabled !== false,
        }));
      const res = await API.put('/api/self-service/admin/upgrade-rules', {
        rules: payload,
      });
      if (res.data.success) {
        setRules(
          (res.data.data || []).map((rule) => ({
            ...rule,
            threshold_amount: quotaToDisplayAmount(rule.threshold_quota),
          })),
        );
        showSuccess(t('保存成功'));
      } else {
        showError(res.data.message || t('保存失败'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  const updateRule = (index, patch) => {
    const next = [...rules];
    next[index] = { ...next[index], ...patch };
    setRules(next);
  };

  const attemptColumns = [
    { title: t('ID'), dataIndex: 'id', width: 80 },
    { title: t('用户'), dataIndex: 'username' },
    { title: t('扫描记录'), dataIndex: 'scanned_count' },
    { title: t('命中空单'), dataIndex: 'candidate_count' },
    { title: t('新处理'), dataIndex: 'new_candidate_count' },
    { title: t('已返还'), dataIndex: 'refunded_count' },
    {
      title: t('返还额度'),
      dataIndex: 'refunded_quota',
      render: (value) => renderQuota(value),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (value) => (
        <Tag color={value === 'success' ? 'green' : 'red'}>{value}</Tag>
      ),
    },
    { title: t('消息'), dataIndex: 'message', render: (value) => value || '-' },
    { title: t('时间'), dataIndex: 'created_at', render: timestamp2string },
  ];

  const refundColumns = [
    { title: t('ID'), dataIndex: 'id', width: 80 },
    { title: t('用户'), dataIndex: 'username' },
    { title: t('日志ID'), dataIndex: 'log_id' },
    { title: t('模型'), dataIndex: 'model_name' },
    {
      title: t('扣费'),
      dataIndex: 'original_quota',
      render: (value) => renderQuota(value),
    },
    {
      title: t('返还'),
      dataIndex: 'refunded_quota',
      render: (value) => renderQuota(value),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (value) => (
        <Tag color={value === 'success' ? 'green' : 'red'}>{value}</Tag>
      ),
    },
    { title: t('时间'), dataIndex: 'created_at', render: timestamp2string },
  ];

  const upgradeColumns = [
    { title: t('ID'), dataIndex: 'id', width: 80 },
    { title: t('用户'), dataIndex: 'username' },
    { title: t('原分组'), dataIndex: 'from_group' },
    { title: t('新分组'), dataIndex: 'to_group' },
    {
      title: t('累计总额度'),
      dataIndex: 'total_quota',
      render: (value) => renderQuota(value),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (value) => (
        <Tag color={value === 'success' ? 'green' : 'red'}>{value}</Tag>
      ),
    },
    { title: t('时间'), dataIndex: 'created_at', render: timestamp2string },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <Spin spinning={loading}>
        <Card>
          <Title heading={4}>{t('自助平台管理')}</Title>
          <Row gutter={[16, 16]} style={{ marginTop: 12 }}>
            <Col xs={12} sm={6}>
              <Text type='secondary'>{t('今日空单')}</Text>
              <div>{stats.today_refunds || 0}</div>
            </Col>
            <Col xs={12} sm={6}>
              <Text type='secondary'>{t('今日返还')}</Text>
              <div>{renderQuota(stats.today_refund_quota || 0)}</div>
            </Col>
            <Col xs={12} sm={6}>
              <Text type='secondary'>{t('累计空单')}</Text>
              <div>{stats.total_refunds || 0}</div>
            </Col>
            <Col xs={12} sm={6}>
              <Text type='secondary'>{t('累计升级')}</Text>
              <div>{stats.total_upgrades || 0}</div>
            </Col>
          </Row>
        </Card>

        <Card style={{ marginTop: 12 }}>
          <Tabs type='card'>
            <TabPane itemKey='config' tab={t('检测配置')}>
              {config && (
                <Form labelPosition='left' labelWidth={180}>
                  <Form.Slot label={t('启用自助平台')}>
                    <Switch
                      checked={config.enabled}
                      onChange={(enabled) => setConfig({ ...config, enabled })}
                    />
                  </Form.Slot>
                  <Form.InputNumber
                    label={t('回溯小时')}
                    field='lookback_hours'
                    initValue={config.lookback_hours}
                    onChange={(value) =>
                      setConfig({ ...config, lookback_hours: Number(value || 0) })
                    }
                  />
                  <Form.InputNumber
                    label={t('单次最大处理数')}
                    field='max_refunds_per_claim'
                    initValue={config.max_refunds_per_claim}
                    onChange={(value) =>
                      setConfig({
                        ...config,
                        max_refunds_per_claim: Number(value || 0),
                      })
                    }
                  />
                  <Form.InputNumber
                    label={t('每日处理限制')}
                    field='daily_refund_limit'
                    initValue={config.daily_refund_limit}
                    onChange={(value) =>
                      setConfig({
                        ...config,
                        daily_refund_limit: Number(value || 0),
                      })
                    }
                  />
                  <Form.InputNumber
                    label={t('用户显示记录数')}
                    field='display_limit'
                    initValue={config.display_limit}
                    onChange={(value) =>
                      setConfig({ ...config, display_limit: Number(value || 0) })
                    }
                  />
                  <Form.InputNumber
                    label={t('返还比例')}
                    field='refund_percent'
                    initValue={config.refund_percent}
                    suffix='%'
                    onChange={(value) =>
                      setConfig({ ...config, refund_percent: Number(value || 0) })
                    }
                  />
                  <Form.Input
                    label={t('排除模型关键词')}
                    field='exclude_models'
                    initValue={config.exclude_models}
                    onChange={(value) =>
                      setConfig({ ...config, exclude_models: value })
                    }
                  />
                  <Form.InputNumber
                    label={t('扫描上限')}
                    field='scan_limit'
                    initValue={config.scan_limit}
                    onChange={(value) =>
                      setConfig({ ...config, scan_limit: Number(value || 0) })
                    }
                  />
                  <Button type='primary' loading={saving} onClick={saveConfig}>
                    {t('保存配置')}
                  </Button>
                </Form>
              )}
            </TabPane>

            <TabPane itemKey='rules' tab={t('升级规则')}>
              <Space vertical align='start' spacing='medium' style={{ width: '100%' }}>
                {rules.map((rule, index) => (
                  <Card key={rule.id || index} style={{ width: '100%' }}>
                    <Row gutter={[12, 12]}>
                      <Col xs={24} sm={6}>
                        <InputNumber
                          prefix={t('额度')}
                          value={rule.threshold_amount || 0}
                          onChange={(value) =>
                            updateRule(index, {
                              threshold_amount: Number(value || 0),
                            })
                          }
                          style={{ width: '100%' }}
                        />
                      </Col>
                      <Col xs={24} sm={6}>
                        <Select
                          value={rule.target_group}
                          placeholder={t('目标分组')}
                          optionList={groups.map((group) => ({
                            label: group,
                            value: group,
                          }))}
                          onChange={(value) =>
                            updateRule(index, { target_group: value })
                          }
                          style={{ width: '100%' }}
                        />
                      </Col>
                      <Col xs={24} sm={8}>
                        <Input
                          value={rule.description}
                          placeholder={t('说明')}
                          onChange={(value) =>
                            updateRule(index, { description: value })
                          }
                        />
                      </Col>
                      <Col xs={24} sm={4}>
                        <Button
                          type='danger'
                          theme='borderless'
                          onClick={() =>
                            setRules(rules.filter((_, i) => i !== index))
                          }
                        >
                          {t('删除')}
                        </Button>
                      </Col>
                    </Row>
                  </Card>
                ))}
                <Space>
                  <Button onClick={() => setRules([...rules, blankRule()])}>
                    {t('新增规则')}
                  </Button>
                  <Button type='primary' loading={saving} onClick={saveRules}>
                    {t('保存规则')}
                  </Button>
                </Space>
              </Space>
            </TabPane>

            <TabPane itemKey='attempts' tab={t('检测记录')}>
              <Table
                rowKey='id'
                columns={attemptColumns}
                dataSource={attempts}
                pagination={{
                  currentPage: attemptPage,
                  pageSize: 10,
                  total: attemptTotal,
                  onPageChange: (page) => {
                    setAttemptPage(page);
                    loadAttempts(page);
                  },
                }}
              />
            </TabPane>

            <TabPane itemKey='refunds' tab={t('空单记录')}>
              <Table
                rowKey='id'
                columns={refundColumns}
                dataSource={refunds}
                pagination={{
                  currentPage: refundPage,
                  pageSize: 10,
                  total: refundTotal,
                  onPageChange: (page) => {
                    setRefundPage(page);
                    loadRefunds(page);
                  },
                }}
              />
            </TabPane>

            <TabPane itemKey='upgrades' tab={t('升级记录')}>
              <Table
                rowKey='id'
                columns={upgradeColumns}
                dataSource={upgrades}
                pagination={{
                  currentPage: upgradePage,
                  pageSize: 10,
                  total: upgradeTotal,
                  onPageChange: (page) => {
                    setUpgradePage(page);
                    loadUpgrades(page);
                  },
                }}
              />
            </TabPane>
          </Tabs>
        </Card>
      </Spin>
    </div>
  );
}
