import React, { useEffect, useState } from 'react';
import {
  Button,
  Card,
  Col,
  Descriptions,
  Empty,
  Progress,
  Row,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import { renderQuota } from '../../helpers/render';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Title, Text } = Typography;

const statusColor = {
  checked: 'orange',
  fixed: 'green',
};

export default function SelfService() {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [info, setInfo] = useState(null);
  const [checkResult, setCheckResult] = useState(null);
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(false);
  const [upgrading, setUpgrading] = useState(false);

  const loadInfo = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/self-service/self/');
      if (res.data.success) {
        setInfo(res.data.data);
      } else {
        showError(res.data.message || t('加载失败'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  const runCheck = async () => {
    setChecking(true);
    try {
      const res = await API.post('/api/self-service/self/check');
      if (res.data.success) {
        setCheckResult(res.data.data);
        setInfo(res.data.data);
        showSuccess(res.data.data.message || t('检测完成'));
      } else {
        showError(res.data.message || t('检测失败'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setChecking(false);
    }
  };

  const runUpgrade = async () => {
    setUpgrading(true);
    try {
      const res = await API.post('/api/self-service/self/upgrade');
      if (res.data.success) {
        showSuccess(t('升级成功'));
        setCheckResult(null);
        await loadInfo();
      } else {
        showError(res.data.message || t('升级失败'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setUpgrading(false);
    }
  };

  useEffect(() => {
    loadInfo();
  }, []);

  const currentInfo = checkResult || info;
  const enabled = currentInfo?.enabled !== false;
  const offer = currentInfo?.upgrade_offer;
  const records = checkResult?.records || [];
  const upgradeRules = (currentInfo?.upgrade_rules || []).filter(
    (rule) => rule.enabled,
  );
  const nextRule = upgradeRules
    .filter((rule) => rule.target_group !== currentInfo?.current_group)
    .sort((a, b) => a.threshold_quota - b.threshold_quota)[0];
  const nextTarget = offer || nextRule;
  const quotaGap = nextTarget
    ? Math.max(0, nextTarget.threshold_quota - (currentInfo?.total_quota || 0))
    : 0;
  const progressValue = nextTarget
    ? Math.min(
        100,
        Math.round(
          ((currentInfo?.total_quota || 0) / nextTarget.threshold_quota) * 100,
        ),
      )
    : 0;
  const totalRefundedQuota =
    checkResult?.total_refunded_quota ?? currentInfo?.total_refunded_quota ?? 0;

  const statCardStyle = {
    border: '1px solid var(--semi-color-border)',
    borderRadius: 8,
    padding: isMobile ? 10 : 12,
    minWidth: 0,
    background: 'var(--semi-color-fill-0)',
  };

  const statLabelStyle = {
    color: 'var(--semi-color-text-2)',
    fontSize: 12,
    lineHeight: '18px',
  };

  const statValueStyle = {
    marginTop: 4,
    color: 'var(--semi-color-text-0)',
    fontSize: isMobile ? 15 : 17,
    fontWeight: 600,
    lineHeight: '22px',
    wordBreak: 'break-word',
  };

  const renderStatGrid = (items) => (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: isMobile
          ? 'repeat(2, minmax(0, 1fr))'
          : 'repeat(5, minmax(0, 1fr))',
        gap: isMobile ? 8 : 12,
        width: '100%',
      }}
    >
      {items.map((item) => (
        <div key={item.label} style={statCardStyle}>
          <div style={statLabelStyle}>{item.label}</div>
          <div style={statValueStyle}>{item.value}</div>
        </div>
      ))}
    </div>
  );

  const columns = [
    { title: t('日志ID'), dataIndex: 'log_id', width: 90 },
    {
      title: t('时间'),
      dataIndex: 'created_at',
      render: (value) => timestamp2string(value),
    },
    {
      title: t('模型'),
      dataIndex: 'model_name',
      render: (value) => (
        <span style={{ wordBreak: 'break-word', overflowWrap: 'anywhere' }}>
          {value || '-'}
        </span>
      ),
    },
    {
      title: t('扣费'),
      dataIndex: 'quota',
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
        <Tag color={statusColor[value] || 'grey'}>
          {value === 'fixed' ? t('已处理') : t('已检测')}
        </Tag>
      ),
    },
  ];

  const ruleColumns = [
    {
      title: t('目标分组'),
      dataIndex: 'target_group',
      render: (value) => (
        <Space wrap>
          <Tag color={value === currentInfo?.current_group ? 'green' : 'blue'}>
            {value}
          </Tag>
          {offer?.target_group === value && (
            <Tag color='orange'>{t('当前可升级')}</Tag>
          )}
        </Space>
      ),
    },
    {
      title: t('累计额度门槛'),
      dataIndex: 'threshold_quota',
      render: (value) => renderQuota(value),
    },
    {
      title: t('还差额度'),
      dataIndex: 'threshold_quota',
      render: (value) =>
        renderQuota(Math.max(0, value - (currentInfo?.total_quota || 0))),
    },
    {
      title: t('说明'),
      dataIndex: 'description',
      render: (value) => value || '-',
    },
  ];

  const renderMobileRecords = () => (
    <Space vertical spacing='small' style={{ width: '100%' }}>
      {records.map((record) => (
        <Card
          key={record.log_id}
          bodyStyle={{ padding: 12 }}
          style={{ width: '100%', borderRadius: 8 }}
        >
          <Space vertical spacing='small' style={{ width: '100%' }}>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: 8,
              }}
            >
              <Text type='secondary' size='small'>
                {timestamp2string(record.created_at)}
              </Text>
              <Tag color={statusColor[record.status] || 'grey'}>
                {record.status === 'fixed' ? t('已处理') : t('已检测')}
              </Tag>
            </div>
            <div style={{ minWidth: 0 }}>
              <div style={statLabelStyle}>{t('模型')}</div>
              <Text
                style={{
                  display: 'block',
                  marginTop: 4,
                  fontSize: 13,
                  lineHeight: '18px',
                  whiteSpace: 'normal',
                  wordBreak: 'break-word',
                  overflowWrap: 'anywhere',
                }}
              >
                {record.model_name || '-'}
              </Text>
            </div>
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(3, minmax(0, 1fr))',
                gap: 8,
              }}
            >
              <div>
                <div style={statLabelStyle}>{t('日志ID')}</div>
                <div style={statValueStyle}>{record.log_id}</div>
              </div>
              <div>
                <div style={statLabelStyle}>{t('扣费')}</div>
                <div style={statValueStyle}>{renderQuota(record.quota || 0)}</div>
              </div>
              <div>
                <div style={statLabelStyle}>{t('返还')}</div>
                <div style={statValueStyle}>
                  {renderQuota(record.refunded_quota || 0)}
                </div>
              </div>
            </div>
          </Space>
        </Card>
      ))}
    </Space>
  );

  return (
    <div className='mt-[60px] px-2'>
      <Spin spinning={loading}>
        <Space vertical align='start' spacing='medium' style={{ width: '100%' }}>
          <Card
            style={{
              width: '100%',
              borderColor:
                enabled && offer ? 'var(--semi-color-success-light-default)' : undefined,
              overflow: 'hidden',
            }}
          >
            <Row gutter={[16, 16]} type='flex' align='middle'>
              <Col xs={24} sm={18}>
                <Title heading={4} style={{ marginBottom: 8 }}>
                  {!enabled
                    ? t('自助平台已关闭')
                    : offer
                      ? t('你已达到自助升级条件')
                      : t('自助升级进度')}
                </Title>
                <Text style={{ wordBreak: 'break-word' }}>
                  {!enabled
                    ? t('管理员暂未开启自助检测和自助升级。')
                    : offer
                      ? t('你已消费累计达到 {{total}}，当前可升级到 {{group}}。', {
                          total: renderQuota(currentInfo?.total_quota || 0),
                          group: offer.target_group,
                        })
                      : nextTarget
                        ? t(
                            '你已消费累计达到 {{total}}，距离升级到 {{group}} 还差 {{gap}}。',
                            {
                              total: renderQuota(currentInfo?.total_quota || 0),
                              group: nextTarget.target_group,
                              gap: renderQuota(quotaGap),
                            },
                          )
                        : t('当前没有管理员配置的可升级分组。')}
                </Text>
                {nextTarget && (
                  <Progress
                    percent={progressValue}
                    showInfo
                    style={{ marginTop: 14, maxWidth: 520 }}
                  />
                )}
              </Col>
              <Col xs={24} sm={6}>
                <Button
                  type='primary'
                  theme='solid'
                  className='btn-anime-gradient'
                  disabled={!enabled || !offer}
                  loading={upgrading}
                  onClick={runUpgrade}
                  style={{ width: '100%' }}
                >
                  {offer
                    ? t('立即升级到 {{group}}', { group: offer.target_group })
                    : t('暂无可升级分组')}
                </Button>
              </Col>
            </Row>
          </Card>

          <Card style={{ width: '100%' }} bodyStyle={{ padding: isMobile ? 14 : 24 }}>
            <Space vertical align='start' spacing='medium' style={{ width: '100%' }}>
              <div>
                <Title heading={4}>{t('自助平台')}</Title>
                <Text type='secondary'>
                  {t('检测真实扣费但输出为 0 的调用记录，并按规则进行自助升级')}
                </Text>
              </div>
              {renderStatGrid([
                { label: t('当前分组'), value: currentInfo?.current_group || '-' },
                { label: t('剩余额度'), value: renderQuota(currentInfo?.quota || 0) },
                {
                  label: t('已消费额度'),
                  value: renderQuota(currentInfo?.used_quota || 0),
                },
                {
                  label: t('累计总额度'),
                  value: renderQuota(currentInfo?.total_quota || 0),
                },
                {
                  label: t('历史总返还额度'),
                  value: renderQuota(totalRefundedQuota),
                },
              ])}
              <Button
                type='primary'
                className='btn-anime-gradient'
                loading={checking}
                disabled={!enabled}
                onClick={runCheck}
                style={isMobile ? { width: '100%' } : undefined}
              >
                {t('检测空输出记录')}
              </Button>
            </Space>
          </Card>

          <Card style={{ width: '100%' }}>
            <Space vertical align='start' spacing='medium' style={{ width: '100%' }}>
              <div>
                <Title heading={5}>{t('可升级分组')}</Title>
                <Text type='secondary'>
                  {offer
                    ? t('系统已匹配到你当前最优的可升级分组。')
                    : t('达到管理员设置的累计额度后，会在这里出现升级按钮。')}
                </Text>
              </div>
              {offer ? (
                <Descriptions
                  row
                  data={[
                    { key: t('目标分组'), value: offer.target_group },
                    { key: t('升级门槛'), value: renderQuota(offer.threshold_quota) },
                    {
                      key: t('当前累计'),
                      value: renderQuota(currentInfo?.total_quota || 0),
                    },
                    { key: t('说明'), value: offer.description || '-' },
                  ]}
                />
              ) : (
                <Empty description={t('当前没有可升级分组')} />
              )}
            </Space>
          </Card>

          <Card style={{ width: '100%' }}>
            <Space vertical align='start' spacing='medium' style={{ width: '100%' }}>
              <div>
                <Title heading={5}>{t('升级规则表')}</Title>
                <Text type='secondary'>
                  {t('以下为管理员后台设置的自助升级规则。')}
                </Text>
              </div>
              {upgradeRules.length > 0 ? (
                <Table
                  rowKey='id'
                  columns={ruleColumns}
                  dataSource={upgradeRules}
                  pagination={false}
                />
              ) : (
                <Empty description={t('管理员尚未配置升级规则')} />
              )}
            </Space>
          </Card>

          {checkResult && (
            <Card style={{ width: '100%' }} bodyStyle={{ padding: isMobile ? 14 : 24 }}>
              <Space vertical align='start' spacing='medium' style={{ width: '100%' }}>
                <div>
                  <Title heading={5}>{t('扫描记录')}</Title>
                  <Text type='secondary'>
                    {t('本次检测的处理结果和累计返还额度')}
                  </Text>
                </div>
                {renderStatGrid([
                  { label: t('扫描记录'), value: checkResult.scanned_count },
                  { label: t('命中空单'), value: checkResult.candidate_count },
                  { label: t('新处理'), value: checkResult.refunded_count },
                  {
                    label: t('本次返还额度'),
                    value: renderQuota(checkResult.refunded_quota || 0),
                  },
                  {
                    label: t('历史总返还额度'),
                    value: renderQuota(totalRefundedQuota),
                  },
                ])}
                {records.length > 0 ? (
                  isMobile ? (
                    renderMobileRecords()
                  ) : (
                    <Table
                      rowKey='log_id'
                      columns={columns}
                      dataSource={records}
                      pagination={false}
                    />
                  )
                ) : (
                  <Empty description={t('没有检测到空输出记录')} />
                )}
              </Space>
            </Card>
          )}
        </Space>
      </Spin>
    </div>
  );
}
