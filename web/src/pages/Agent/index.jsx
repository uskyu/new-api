import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Checkbox,
  Descriptions,
  Empty,
  Input,
  InputNumber,
  Modal,
  Pagination,
  Select,
  Space,
  Table,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import {
  API,
  can,
  copy,
  isAdmin,
  isRoot,
  isSupportConsole,
  PERMISSIONS,
  renderQuotaWithAmount,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import { IconSearch } from '@douyinfe/semi-icons';

const { Text, Title } = Typography;

const DEFAULT_PAGE_SIZE = 10;

function formatRate(rate) {
  return `${(Number(rate || 0) / 100).toFixed(2)}%`;
}

function formatAmount(amount) {
  return renderQuotaWithAmount(Number(amount || 0) / 100);
}

function getStatusTag(status, t) {
  return status === 1 ? (
    <Tag color='green'>{t('启用')}</Tag>
  ) : (
    <Tag color='grey'>{t('禁用')}</Tag>
  );
}

function getWithdrawStatusTag(status, t) {
  if (status === 'pending') {
    return <Tag color='orange'>{t('待处理')}</Tag>;
  }
  if (status === 'exported') {
    return <Tag color='blue'>{t('已导出')}</Tag>;
  }
  if (status === 'paid') {
    return <Tag color='green'>{t('已打款')}</Tag>;
  }
  return <Tag>{status || '-'}</Tag>;
}

export default function Agent() {
  const { t } = useTranslation();
  const supportMode = isSupportConsole();
  const canManageAgentAdmin = isAdmin() && !supportMode;
  const canAssignDownlines =
    supportMode || can(PERMISSIONS.AGENT_DOWNLINE_ASSIGN);
  const canTransferDownlines =
    supportMode || can(PERMISSIONS.AGENT_DOWNLINE_TRANSFER);
  const canViewDownlines = canManageAgentAdmin || canTransferDownlines;
  const [status, setStatus] = useState(null);
  const [statusLoading, setStatusLoading] = useState(false);
  const [overview, setOverview] = useState(null);
  const [overviewLoading, setOverviewLoading] = useState(false);
  const [groups, setGroups] = useState([]);
  const [groupModalVisible, setGroupModalVisible] = useState(false);
  const [groupSubmitting, setGroupSubmitting] = useState(false);
  const [groupForm, setGroupForm] = useState({
    id: 0,
    name: '',
    rebateRatePercent: 0,
    status: 1,
    remark: '',
  });
  const [profiles, setProfiles] = useState([]);
  const [profilesTotal, setProfilesTotal] = useState(0);
  const [profilesPage, setProfilesPage] = useState(1);
  const [profilesLoading, setProfilesLoading] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [keywordInput, setKeywordInput] = useState('');
  const [adjustments, setAdjustments] = useState([]);
  const [adjustmentsLoading, setAdjustmentsLoading] = useState(false);
  const [promoLinks, setPromoLinks] = useState([]);
  const [promoLinksTotal, setPromoLinksTotal] = useState(0);
  const [promoLinksPage, setPromoLinksPage] = useState(1);
  const [promoLinksLoading, setPromoLinksLoading] = useState(false);
  const [promoLinkStats, setPromoLinkStats] = useState([]);
  const [promoLinkStatsLoading, setPromoLinkStatsLoading] = useState(false);
  const [downlines, setDownlines] = useState([]);
  const [downlinesTotal, setDownlinesTotal] = useState(0);
  const [downlinesPage, setDownlinesPage] = useState(1);
  const [downlinesLoading, setDownlinesLoading] = useState(false);
  const [downlineKeyword, setDownlineKeyword] = useState('');
  const [downlineKeywordInput, setDownlineKeywordInput] = useState('');
  const [activeAgentScope, setActiveAgentScope] = useState(null);
  const [inspectModalVisible, setInspectModalVisible] = useState(false);
  const [initModalVisible, setInitModalVisible] = useState(false);
  const [initRatePercent, setInitRatePercent] = useState(10);
  const [profileModalVisible, setProfileModalVisible] = useState(false);
  const [profileSubmitting, setProfileSubmitting] = useState(false);
  const [profileForm, setProfileForm] = useState({
    userId: '',
    status: 1,
    rebateGroupId: 0,
    customRateEnabled: false,
    customRatePercent: 0,
    remark: '',
  });
  const [adjustModalVisible, setAdjustModalVisible] = useState(false);
  const [adjustSubmitting, setAdjustSubmitting] = useState(false);
  const [adjustForm, setAdjustForm] = useState({
    agentUserId: 0,
    username: '',
    amount: '',
    reason: '',
  });
  const [promoLinkModalVisible, setPromoLinkModalVisible] = useState(false);
  const [promoLinkSubmitting, setPromoLinkSubmitting] = useState(false);
  const [promoLinkForm, setPromoLinkForm] = useState({
    id: 0,
    agentUserId: '',
    name: '',
    code: '',
    status: 1,
    landingPage: '',
    remark: '',
  });
  const [transferModalVisible, setTransferModalVisible] = useState(false);
  const [transferSubmitting, setTransferSubmitting] = useState(false);
  const [transferAgents, setTransferAgents] = useState([]);
  const [transferAgentsLoading, setTransferAgentsLoading] = useState(false);
  const [transferAgentKeyword, setTransferAgentKeyword] = useState('');
  const [transferPromoLinks, setTransferPromoLinks] = useState([]);
  const [transferPromoLinksLoading, setTransferPromoLinksLoading] =
    useState(false);
  const [transferForm, setTransferForm] = useState({
    sourceAgentUserId: 0,
    sourceAgentUsername: '',
    downlineUserId: 0,
    downlineUsername: '',
    targetAgentUserId: 0,
    promoLinkId: 0,
    remark: '',
  });
  const [assignModalVisible, setAssignModalVisible] = useState(false);
  const [assignSubmitting, setAssignSubmitting] = useState(false);
  const [assignAgentKeyword, setAssignAgentKeyword] = useState('');
  const [assignForm, setAssignForm] = useState({
    downlineUserId: '',
    targetAgentUserId: 0,
    targetAgentUsername: '',
    remark: '',
  });
  const [withdrawRequests, setWithdrawRequests] = useState([]);
  const [withdrawRequestsTotal, setWithdrawRequestsTotal] = useState(0);
  const [withdrawRequestsPage, setWithdrawRequestsPage] = useState(1);
  const [withdrawRequestsLoading, setWithdrawRequestsLoading] = useState(false);
  const [withdrawStatusFilter, setWithdrawStatusFilter] = useState('');
  const [withdrawStartDate, setWithdrawStartDate] = useState('');
  const [withdrawEndDate, setWithdrawEndDate] = useState('');
  const withdrawImportRef = useRef(null);

  const showConflictModal = useCallback(
    (message, conflicts = []) => {
      Modal.error({
        title: t('代理比例冲突'),
        content: (
          <Space vertical align='start' style={{ width: '100%' }}>
            <Text>{message}</Text>
            {conflicts.map((conflict) => (
              <Text
                key={`${conflict.parent_agent_user_id}-${conflict.agent_user_id}`}
              >
                {conflict.agent_username} ({formatRate(conflict.agent_rate)}) /{' '}
                {conflict.parent_agent_name} {t('上限')}{' '}
                {formatRate(conflict.parent_allowed_rate)}
              </Text>
            ))}
          </Space>
        ),
      });
    },
    [t],
  );

  const loadStatus = useCallback(async () => {
    setStatusLoading(true);
    try {
      const res = await API.get('/api/agent/status');
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setStatus(data);
      setInitRatePercent(Number(data?.default_rate || 0) / 100 || 10);
    } catch (error) {
      showError(error.message || t('获取代理状态失败'));
    } finally {
      setStatusLoading(false);
    }
  }, [t]);

  const loadGroups = useCallback(async () => {
    try {
      const res = await API.get('/api/agent/groups');
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      setGroups(Array.isArray(res.data.data) ? res.data.data : []);
    } catch (error) {
      showError(error.message || t('获取代理分组失败'));
    }
  }, [t]);

  const loadOverview = useCallback(async () => {
    if (!status?.initialized) {
      setOverview(null);
      return;
    }
    setOverviewLoading(true);
    try {
      const res = await API.get('/api/agent/overview');
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      setOverview(res.data.data);
    } catch (error) {
      showError(error.message || t('获取代理总览失败'));
    } finally {
      setOverviewLoading(false);
    }
  }, [status?.initialized, t]);

  const loadProfiles = useCallback(
    async (page = profilesPage, currentKeyword = keyword) => {
      if (!status?.initialized) {
        setProfiles([]);
        setProfilesTotal(0);
        return;
      }
      setProfilesLoading(true);
      try {
        const res = await API.get('/api/agent/profiles', {
          params: {
            p: page,
            page_size: DEFAULT_PAGE_SIZE,
            keyword: currentKeyword,
          },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        setProfiles(res.data.data?.items || []);
        setProfilesTotal(res.data.data?.total || 0);
      } catch (error) {
        showError(error.message || t('获取代理列表失败'));
      } finally {
        setProfilesLoading(false);
      }
    },
    [keyword, profilesPage, status?.initialized, t],
  );

  const loadAdjustments = useCallback(async () => {
    if (!status?.initialized) {
      setAdjustments([]);
      return;
    }
    setAdjustmentsLoading(true);
    try {
      const res = await API.get('/api/agent/adjustments', {
        params: {
          p: 1,
          page_size: 10,
        },
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      setAdjustments(res.data.data?.items || []);
    } catch (error) {
      showError(error.message || t('获取调账记录失败'));
    } finally {
      setAdjustmentsLoading(false);
    }
  }, [status?.initialized, t]);

  const loadPromoLinks = useCallback(
    async (page = promoLinksPage) => {
      if (!status?.initialized) {
        setPromoLinks([]);
        setPromoLinksTotal(0);
        return;
      }
      setPromoLinksLoading(true);
      try {
        const res = await API.get('/api/agent/promo-links', {
          params: {
            p: page,
            page_size: DEFAULT_PAGE_SIZE,
          },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        setPromoLinks(res.data.data?.items || []);
        setPromoLinksTotal(res.data.data?.total || 0);
      } catch (error) {
        showError(error.message || t('获取推广链接失败'));
      } finally {
        setPromoLinksLoading(false);
      }
    },
    [promoLinksPage, status?.initialized, t],
  );

  const loadWithdrawRequests = useCallback(
    async (page = withdrawRequestsPage) => {
      if (!status?.initialized) {
        setWithdrawRequests([]);
        setWithdrawRequestsTotal(0);
        return;
      }
      setWithdrawRequestsLoading(true);
      try {
        const res = await API.get('/api/agent/withdraw-requests', {
          params: {
            p: page,
            page_size: DEFAULT_PAGE_SIZE,
            status: withdrawStatusFilter,
            start_date: withdrawStartDate,
            end_date: withdrawEndDate,
          },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        setWithdrawRequests(res.data.data?.items || []);
        setWithdrawRequestsTotal(res.data.data?.total || 0);
      } catch (error) {
        showError(error.message || t('获取提现申请失败'));
      } finally {
        setWithdrawRequestsLoading(false);
      }
    },
    [
      status?.initialized,
      t,
      withdrawEndDate,
      withdrawRequestsPage,
      withdrawStartDate,
      withdrawStatusFilter,
    ],
  );

  const loadPromoLinkStats = useCallback(
    async (agentUserId = activeAgentScope?.userId || 0) => {
      if (!status?.initialized || !agentUserId) {
        setPromoLinkStats([]);
        return;
      }
      setPromoLinkStatsLoading(true);
      try {
        const res = await API.get('/api/agent/promo-link-stats', {
          params: { agent_user_id: agentUserId },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        setPromoLinkStats(res.data.data || []);
      } catch (error) {
        showError(error.message || t('获取推广链接统计失败'));
      } finally {
        setPromoLinkStatsLoading(false);
      }
    },
    [activeAgentScope?.userId, status?.initialized, t],
  );

  const loadDownlines = useCallback(
    async (
      agentUserId = activeAgentScope?.userId || 0,
      page = downlinesPage,
      currentKeyword = downlineKeyword,
    ) => {
      if (!status?.initialized || !agentUserId) {
        setDownlines([]);
        setDownlinesTotal(0);
        return;
      }
      setDownlinesLoading(true);
      try {
        const res = await API.get('/api/agent/downlines', {
          params: {
            agent_user_id: agentUserId,
            p: page,
            page_size: DEFAULT_PAGE_SIZE,
            keyword: currentKeyword,
          },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        setDownlines(res.data.data?.items || []);
        setDownlinesTotal(res.data.data?.total || 0);
      } catch (error) {
        showError(error.message || t('获取下级用户失败'));
      } finally {
        setDownlinesLoading(false);
      }
    },
    [
      activeAgentScope?.userId,
      downlineKeyword,
      downlinesPage,
      status?.initialized,
      t,
    ],
  );

  useEffect(() => {
    loadStatus();
  }, [loadStatus]);

  useEffect(() => {
    if (status?.migration_ready) {
      if (canManageAgentAdmin) {
        loadGroups();
      }
    }
    if (status?.initialized) {
      if (canManageAgentAdmin) {
        loadOverview();
        loadAdjustments();
        loadPromoLinks(1);
        loadWithdrawRequests();
      }
      loadProfiles(1, keyword);
    }
  }, [canManageAgentAdmin, status?.migration_ready, status?.initialized]);

  const refreshAll = async () => {
    await loadStatus();
    if (canManageAgentAdmin) {
      await loadOverview();
      await loadGroups();
      await loadAdjustments();
      await loadPromoLinks(promoLinksPage);
      await loadWithdrawRequests();
      await loadPromoLinkStats(activeAgentScope?.userId || 0);
    }
    await loadDownlines(
      activeAgentScope?.userId || 0,
      downlinesPage,
      downlineKeyword,
    );
    await loadProfiles(profilesPage, keyword);
  };

  const handleOpenCreateGroup = () => {
    setGroupForm({
      id: 0,
      name: '',
      rebateRatePercent: 0,
      status: 1,
      remark: '',
    });
    setGroupModalVisible(true);
  };

  const handleOpenEditGroup = (group) => {
    setGroupForm({
      id: group.id,
      name: group.name,
      rebateRatePercent: Number(group.rebate_rate || 0) / 100,
      status: group.status,
      remark: group.remark || '',
    });
    setGroupModalVisible(true);
  };

  const handleSubmitGroup = async () => {
    setGroupSubmitting(true);
    try {
      const res = await API.post('/api/agent/group', {
        id: groupForm.id,
        name: groupForm.name,
        rebate_rate: Math.round(Number(groupForm.rebateRatePercent || 0) * 100),
        status: Number(groupForm.status),
        remark: groupForm.remark,
      });
      if (!res.data.success) {
        if (res.data.data?.conflicts?.length) {
          showConflictModal(res.data.message, res.data.data.conflicts);
          return;
        }
        showError(res.data.message);
        return;
      }
      showSuccess(t('代理分组已保存'));
      setGroupModalVisible(false);
      await loadGroups();
    } catch (error) {
      showError(error.message || t('保存代理分组失败'));
    } finally {
      setGroupSubmitting(false);
    }
  };

  const handleDeleteGroup = (group) => {
    Modal.confirm({
      title: t('确认删除代理分组？'),
      content: `${group.name} (${formatRate(group.rebate_rate)})`,
      onOk: async () => {
        const res = await API.delete(`/api/agent/group/${group.id}`);
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        showSuccess(t('代理分组已删除'));
        await loadGroups();
      },
    });
  };

  const handleInitialize = async () => {
    setProfileSubmitting(true);
    try {
      const res = await API.post('/api/agent/init', {
        default_rebate_rate: Math.round(Number(initRatePercent || 0) * 100),
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('代理功能初始化成功'));
      setInitModalVisible(false);
      await refreshAll();
    } catch (error) {
      showError(error.message || t('代理功能初始化失败'));
    } finally {
      setProfileSubmitting(false);
    }
  };

  const groupOptions = useMemo(
    () =>
      groups.map((group) => ({
        label: `${group.name} (${formatRate(group.rebate_rate)})`,
        value: group.id,
      })),
    [groups],
  );

  const transferAgentOptions = useMemo(() => {
    const normalizedKeyword = transferAgentKeyword.trim().toLowerCase();
    return transferAgents
      .filter((agent) => {
        if (!normalizedKeyword) return true;
        return [
          agent.user_id,
          agent.username,
          agent.display_name,
          agent.rebate_group_name,
        ]
          .filter((value) => value !== undefined && value !== null)
          .some((value) =>
            String(value).toLowerCase().includes(normalizedKeyword),
          );
      })
      .map((agent) => ({
        label: supportMode
          ? `${agent.username || agent.user_id}`
          : `${agent.username || agent.user_id} (${formatRate(agent.effective_rate)})`,
        value: agent.user_id,
      }));
  }, [supportMode, transferAgentKeyword, transferAgents]);

  const assignAgentOptions = useMemo(() => {
    const normalizedKeyword = assignAgentKeyword.trim().toLowerCase();
    return transferAgents
      .filter((agent) => {
        if (!normalizedKeyword) return true;
        return [
          agent.user_id,
          agent.username,
          agent.display_name,
          agent.rebate_group_name,
        ]
          .filter((value) => value !== undefined && value !== null)
          .some((value) =>
            String(value).toLowerCase().includes(normalizedKeyword),
          );
      })
      .map((agent) => ({
        label: supportMode
          ? `${agent.username || agent.user_id}`
          : `${agent.username || agent.user_id} (${formatRate(agent.effective_rate)})`,
        value: agent.user_id,
      }));
  }, [assignAgentKeyword, supportMode, transferAgents]);

  const transferPromoLinkOptions = useMemo(
    () => [
      { label: t('自动使用目标代理默认推广链接'), value: 0 },
      ...transferPromoLinks.map((link) => ({
        label: `${link.name || link.code} (${link.code})`,
        value: link.id,
      })),
    ],
    [t, transferPromoLinks],
  );

  const loadTransferAgents = useCallback(
    async (excludedUserIds = []) => {
      setTransferAgentsLoading(true);
      try {
        const res = await API.get('/api/agent/profiles', {
          params: {
            p: 1,
            page_size: 1000,
          },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        const excluded = new Set(excludedUserIds);
        const agents = (res.data.data?.items || []).filter(
          (agent) => agent.status === 1 && !excluded.has(agent.user_id),
        );
        setTransferAgents(agents);
      } catch (error) {
        showError(error.message || t('加载目标代理失败'));
      } finally {
        setTransferAgentsLoading(false);
      }
    },
    [t],
  );

  const loadTransferPromoLinks = useCallback(
    async (agentUserId = 0) => {
      if (!agentUserId) {
        setTransferPromoLinks([]);
        return [];
      }
      setTransferPromoLinksLoading(true);
      try {
        const res = await API.get('/api/agent/promo-links', {
          params: {
            agent_user_id: agentUserId,
            p: 1,
            page_size: 1000,
          },
        });
        if (!res.data.success) {
          showError(res.data.message);
          return [];
        }
        const links = (res.data.data?.items || []).filter(
          (link) => link.status === 1,
        );
        setTransferPromoLinks(links);
        return links;
      } catch (error) {
        showError(error.message || t('加载推广链接失败'));
        return [];
      } finally {
        setTransferPromoLinksLoading(false);
      }
    },
    [t],
  );

  const handleOpenCreateProfile = () => {
    setProfileForm({
      userId: '',
      status: 1,
      rebateGroupId: status?.default_group_id || groups?.[0]?.id || 0,
      customRateEnabled: false,
      customRatePercent: 0,
      remark: '',
    });
    setProfileModalVisible(true);
  };

  const handleOpenEditProfile = (record) => {
    setProfileForm({
      userId: record.user_id,
      status: record.status,
      rebateGroupId: record.rebate_group_id || status?.default_group_id || 0,
      customRateEnabled: Number(record.custom_rate || 0) > 0,
      customRatePercent: Number(record.custom_rate || 0) / 100,
      remark: record.remark || '',
    });
    setProfileModalVisible(true);
  };

  const handleInspectAgent = async (record) => {
    const scope = { userId: record.user_id, username: record.username };
    setActiveAgentScope(scope);
    setInspectModalVisible(true);
    setDownlinesPage(1);
    setDownlineKeyword('');
    setDownlineKeywordInput('');
    if (canManageAgentAdmin) {
      await loadPromoLinkStats(scope.userId);
    }
    await loadDownlines(scope.userId, 1, '');
  };

  const handleOpenTransferDownline = async (record) => {
    if (!activeAgentScope?.userId) {
      showError(t('请选择当前代理'));
      return;
    }
    setTransferForm({
      sourceAgentUserId: activeAgentScope.userId,
      sourceAgentUsername: activeAgentScope.username,
      downlineUserId: record.user_id,
      downlineUsername: record.username,
      targetAgentUserId: 0,
      promoLinkId: 0,
      remark: '',
    });
    setTransferPromoLinks([]);
    setTransferAgentKeyword('');
    setTransferModalVisible(true);
    await loadTransferAgents([activeAgentScope.userId, record.user_id]);
  };

  const handleChangeTransferTarget = async (targetAgentUserId) => {
    setTransferForm((prev) => ({
      ...prev,
      targetAgentUserId,
      promoLinkId: 0,
    }));
    if (!canManageAgentAdmin) {
      setTransferPromoLinks([]);
      return;
    }
    const links = await loadTransferPromoLinks(targetAgentUserId);
    if (links.length > 0) {
      setTransferForm((prev) => ({
        ...prev,
        promoLinkId: links[0].id,
      }));
    }
  };

  const handleSubmitTransfer = async () => {
    if (!transferForm.targetAgentUserId) {
      showError(t('请选择目标代理'));
      return;
    }
    setTransferSubmitting(true);
    try {
      const res = await API.post('/api/agent/downline/transfer', {
        source_agent_user_id: transferForm.sourceAgentUserId,
        target_agent_user_id: transferForm.targetAgentUserId,
        downline_user_id: transferForm.downlineUserId,
        promo_link_id: canManageAgentAdmin ? transferForm.promoLinkId : 0,
        remark: transferForm.remark,
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('用户已转移'));
      setTransferModalVisible(false);
      if (activeAgentScope?.userId) {
        await loadDownlines(
          activeAgentScope.userId,
          downlinesPage,
          downlineKeyword,
        );
        if (canManageAgentAdmin) {
          await loadPromoLinkStats(activeAgentScope.userId);
        }
      }
      if (canManageAgentAdmin) {
        await loadOverview();
      }
    } catch (error) {
      showError(error.message || t('转移用户失败'));
    } finally {
      setTransferSubmitting(false);
    }
  };

  const handleOpenAssignDownline = async (record = null) => {
    setAssignForm({
      downlineUserId: '',
      targetAgentUserId: record?.user_id || 0,
      targetAgentUsername: record?.username || '',
      remark: '',
    });
    setAssignAgentKeyword('');
    setAssignModalVisible(true);
    await loadTransferAgents([]);
  };

  const handleSubmitAssign = async () => {
    if (!assignForm.downlineUserId) {
      showError(t('请输入用户ID'));
      return;
    }
    if (!assignForm.targetAgentUserId) {
      showError(t('请选择目标代理'));
      return;
    }
    setAssignSubmitting(true);
    try {
      const res = await API.post('/api/agent/downline/assign', {
        target_agent_user_id: Number(assignForm.targetAgentUserId),
        downline_user_id: Number(assignForm.downlineUserId),
        promo_link_id: 0,
        remark: assignForm.remark,
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('用户已分配给目标代理'));
      setAssignModalVisible(false);
      await loadProfiles(profilesPage, keyword);
    } catch (error) {
      showError(error.message || t('分配用户失败'));
    } finally {
      setAssignSubmitting(false);
    }
  };

  const handleSubmitProfile = async () => {
    setProfileSubmitting(true);
    try {
      const res = await API.post('/api/agent/profile', {
        user_id: Number(profileForm.userId),
        status: Number(profileForm.status),
        rebate_group_id: Number(profileForm.rebateGroupId),
        custom_rate: profileForm.customRateEnabled
          ? Math.round(Number(profileForm.customRatePercent || 0) * 100)
          : 0,
        remark: profileForm.remark,
      });
      if (!res.data.success) {
        if (res.data.data?.conflicts?.length) {
          showConflictModal(res.data.message, res.data.data.conflicts);
          return;
        }
        showError(res.data.message);
        return;
      }
      showSuccess(t('代理资料已保存'));
      setProfileModalVisible(false);
      await loadProfiles(profilesPage, keyword);
    } catch (error) {
      showError(error.message || t('保存代理资料失败'));
    } finally {
      setProfileSubmitting(false);
    }
  };

  const handleOpenAdjust = (record) => {
    setAdjustForm({
      agentUserId: record.user_id,
      username: record.username,
      amount: '',
      reason: '',
    });
    setAdjustModalVisible(true);
  };

  const handleSubmitAdjust = async () => {
    setAdjustSubmitting(true);
    try {
      const res = await API.post('/api/agent/adjust', {
        agent_user_id: adjustForm.agentUserId,
        amount: adjustForm.amount,
        reason: adjustForm.reason,
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('代理返利余额已调整'));
      setAdjustModalVisible(false);
      await loadProfiles(profilesPage, keyword);
      await loadAdjustments();
    } catch (error) {
      showError(error.message || t('代理返利余额调整失败'));
    } finally {
      setAdjustSubmitting(false);
    }
  };

  const handleOpenCreatePromoLink = () => {
    setPromoLinkForm({
      id: 0,
      agentUserId: '',
      name: '',
      code: '',
      status: 1,
      landingPage: '',
      remark: '',
    });
    setPromoLinkModalVisible(true);
  };

  const handleOpenEditPromoLink = (record) => {
    setPromoLinkForm({
      id: record.id,
      agentUserId: record.agent_user_id,
      name: record.name,
      code: record.code,
      status: record.status,
      landingPage: record.landing_page || '',
      remark: record.remark || '',
    });
    setPromoLinkModalVisible(true);
  };

  const handleSubmitPromoLink = async () => {
    setPromoLinkSubmitting(true);
    try {
      const res = await API.post('/api/agent/promo-link', {
        id: promoLinkForm.id,
        agent_user_id: Number(promoLinkForm.agentUserId),
        name: promoLinkForm.name,
        code: promoLinkForm.code,
        status: Number(promoLinkForm.status),
        landing_page: promoLinkForm.landingPage,
        remark: promoLinkForm.remark,
      });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('推广链接已保存'));
      setPromoLinkModalVisible(false);
      await loadPromoLinks(promoLinksPage);
    } catch (error) {
      showError(error.message || t('保存推广链接失败'));
    } finally {
      setPromoLinkSubmitting(false);
    }
  };

  const handleDeletePromoLink = (record) => {
    Modal.confirm({
      title: t('确认删除推广链接？'),
      content: `${record.name} (${record.code})`,
      onOk: async () => {
        const res = await API.delete(`/api/agent/promo-link/${record.id}`);
        if (!res.data.success) {
          showError(res.data.message);
          return;
        }
        showSuccess(t('推广链接已删除'));
        await loadPromoLinks(promoLinksPage);
      },
    });
  };

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

  const handleExportWithdrawRequests = async () => {
    try {
      const res = await API.get('/api/agent/withdraw-requests/export', {
        params: {
          status: withdrawStatusFilter,
          start_date: withdrawStartDate,
          end_date: withdrawEndDate,
        },
        responseType: 'blob',
      });
      const blob = new Blob([res.data], { type: 'text/csv;charset=utf-8' });
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `agent-withdraw-${Date.now()}.csv`;
      a.click();
      window.URL.revokeObjectURL(url);
      showSuccess(t('提现数据已导出'));
      await loadWithdrawRequests();
    } catch (error) {
      showError(error.message || t('导出提现数据失败'));
    }
  };

  const handleImportWithdrawResults = async (event) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }
    const formData = new FormData();
    formData.append('file', file);
    try {
      const res = await API.post(
        '/api/agent/withdraw-requests/import',
        formData,
        {
          headers: { 'Content-Type': 'multipart/form-data' },
        },
      );
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      const processed = res.data.data?.processed || 0;
      showSuccess(`${t('提现回执已导入')} (${processed})`);
      await refreshAll();
    } catch (error) {
      showError(error.message || t('导入提现回执失败'));
    } finally {
      event.target.value = '';
    }
  };

  const handleSearchWithdrawRequests = async () => {
    setWithdrawRequestsPage(1);
    await loadWithdrawRequests(1);
  };

  const profileColumns = [
    { title: t('用户ID'), dataIndex: 'user_id' },
    { title: t('用户名'), dataIndex: 'username' },
    { title: t('显示名称'), dataIndex: 'display_name' },
    {
      title: t('代理级别'),
      dataIndex: 'agent_level',
      render: (_, record) =>
        record.agent_level === 2 ? t('二级代理') : t('一级代理'),
    },
    {
      title: t('直属上级'),
      dataIndex: 'parent_agent_username',
      render: (_, record) => record.parent_agent_username || '-',
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (_, record) => getStatusTag(record.status, t),
    },
    {
      title: t('分组'),
      dataIndex: 'rebate_group_name',
      render: (_, record) => record.rebate_group_name || '-',
    },
    {
      title: t('生效比例'),
      dataIndex: 'effective_rate',
      render: (_, record) => formatRate(record.effective_rate),
    },
    {
      title: t('比例来源'),
      dataIndex: 'effective_rate_source',
      render: (_, record) =>
        record.effective_rate_source === 'custom' ? t('自定义覆盖') : t('分组'),
    },
    {
      title: t('上级上限'),
      dataIndex: 'parent_max_rate',
      render: (_, record) =>
        record.parent_max_rate > 0 ? formatRate(record.parent_max_rate) : '-',
    },
    ...(canManageAgentAdmin
      ? [
          {
            title: t('返利余额'),
            dataIndex: 'rebate_balance_amount',
            render: (_, record) => formatAmount(record.rebate_balance_amount),
          },
          {
            title: t('累计返利'),
            dataIndex: 'rebate_total_amount',
            render: (_, record) => formatAmount(record.rebate_total_amount),
          },
        ]
      : []),
    {
      title: t('操作'),
      dataIndex: 'operate',
      render: (_, record) => (
        <Space>
          {canManageAgentAdmin && (
            <Button
              size='small'
              type='tertiary'
              onClick={() => handleOpenEditProfile(record)}
            >
              {t('编辑')}
            </Button>
          )}
          {canManageAgentAdmin && (
            <Button
              size='small'
              type='secondary'
              onClick={() => handleOpenAdjust(record)}
            >
              {t('调账')}
            </Button>
          )}
          {canViewDownlines && (
            <Button
              size='small'
              type='primary'
              onClick={() => handleInspectAgent(record)}
            >
              {t('查看下级')}
            </Button>
          )}
          {canAssignDownlines && (
            <Button
              size='small'
              type='warning'
              onClick={() => handleOpenAssignDownline(record)}
            >
              {t('分配用户')}
            </Button>
          )}
        </Space>
      ),
    },
  ];

  const visibleProfileColumns = supportMode
    ? profileColumns.filter((column) =>
        ['user_id', 'username', 'display_name', 'status', 'operate'].includes(
          column.dataIndex,
        ),
      )
    : profileColumns;

  const groupColumns = [
    { title: t('分组ID'), dataIndex: 'id' },
    { title: t('分组名称'), dataIndex: 'name' },
    {
      title: t('默认比例'),
      dataIndex: 'rebate_rate',
      render: (_, record) => formatRate(record.rebate_rate),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (_, record) => getStatusTag(record.status, t),
    },
    {
      title: t('默认组'),
      dataIndex: 'is_default',
      render: (_, record) =>
        record.is_default ? <Tag color='blue'>{t('是')}</Tag> : '-',
    },
    { title: t('备注'), dataIndex: 'remark' },
    {
      title: t('操作'),
      dataIndex: 'operate',
      render: (_, record) => (
        <Space>
          <Button
            size='small'
            type='tertiary'
            onClick={() => handleOpenEditGroup(record)}
          >
            {t('编辑')}
          </Button>
          {!record.is_default && (
            <Button
              size='small'
              type='danger'
              onClick={() => handleDeleteGroup(record)}
            >
              {t('删除')}
            </Button>
          )}
        </Space>
      ),
    },
  ];

  const adjustmentColumns = [
    { title: t('代理用户ID'), dataIndex: 'agent_user_id' },
    { title: t('操作人ID'), dataIndex: 'operator_user_id' },
    {
      title: t('调整类型'),
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
  ];

  const promoLinkColumns = [
    { title: t('链接ID'), dataIndex: 'id' },
    { title: t('代理用户ID'), dataIndex: 'agent_user_id' },
    { title: t('用户名'), dataIndex: 'username' },
    { title: t('渠道名称'), dataIndex: 'name' },
    { title: t('推广码'), dataIndex: 'code' },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (_, record) => getStatusTag(record.status, t),
    },
    {
      title: t('落地页'),
      dataIndex: 'landing_page',
      render: (_, record) => record.landing_page || '/',
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      render: (_, record) => (
        <Space>
          <Button
            size='small'
            type='tertiary'
            onClick={() => handleCopyPromoLink(record)}
          >
            {t('复制')}
          </Button>
          <Button
            size='small'
            type='tertiary'
            onClick={() => handleOpenEditPromoLink(record)}
          >
            {t('编辑')}
          </Button>
          <Button
            size='small'
            type='danger'
            onClick={() => handleDeletePromoLink(record)}
          >
            {t('删除')}
          </Button>
        </Space>
      ),
    },
  ];

  const promoLinkStatColumns = [
    { title: t('渠道名称'), dataIndex: 'name' },
    { title: t('推广码'), dataIndex: 'code' },
    { title: t('注册人数'), dataIndex: 'invitee_count' },
    { title: t('充值笔数'), dataIndex: 'topup_count' },
    {
      title: t('充值金额'),
      dataIndex: 'topup_amount',
      render: (_, record) => formatAmount(record.topup_amount),
    },
    {
      title: t('返利金额(含兑换码)'),
      dataIndex: 'rebate_amount',
      render: (_, record) => formatAmount(record.rebate_amount),
    },
    { title: t('最近注册用户'), dataIndex: 'last_invitee_name' },
  ];

  const downlineColumns = [
    { title: t('用户ID'), dataIndex: 'user_id' },
    { title: t('用户名'), dataIndex: 'username' },
    { title: t('显示名称'), dataIndex: 'display_name' },
    { title: t('来源渠道'), dataIndex: 'promo_link_name' },
    ...(canManageAgentAdmin
      ? [
          { title: t('充值笔数'), dataIndex: 'topup_count' },
          {
            title: t('充值金额'),
            dataIndex: 'topup_amount',
            render: (_, record) => formatAmount(record.topup_amount),
          },
          {
            title: t('返利金额(含兑换码)'),
            dataIndex: 'rebate_amount',
            render: (_, record) => formatAmount(record.rebate_amount),
          },
          {
            title: t('最近充值时间'),
            dataIndex: 'latest_topup_time',
            render: (_, record) =>
              record.latest_topup_time
                ? timestamp2string(record.latest_topup_time)
                : '-',
          },
        ]
      : []),
    {
      title: t('操作'),
      dataIndex: 'operate',
      render: (_, record) => (
        <Space>
          {canTransferDownlines && !record.is_agent && (
            <Button
              size='small'
              type='warning'
              onClick={() => handleOpenTransferDownline(record)}
            >
              {t('转移')}
            </Button>
          )}
          {record.is_agent && <Tag color='blue'>{t('代理')}</Tag>}
        </Space>
      ),
    },
  ].filter(
    (column) => canManageAgentAdmin || column.dataIndex !== 'promo_link_name',
  );

  const withdrawColumns = [
    { title: t('申请单ID'), dataIndex: 'id' },
    { title: t('用户名'), dataIndex: 'username' },
    { title: t('邮箱'), dataIndex: 'email' },
    {
      title: t('提现金额'),
      dataIndex: 'amount',
      render: (_, record) => formatAmount(record.amount),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (_, record) => getWithdrawStatusTag(record.status, t),
    },
    { title: t('支付宝账号'), dataIndex: 'account_no_snapshot' },
    { title: t('姓名'), dataIndex: 'account_name_snapshot' },
    { title: t('批次号'), dataIndex: 'export_batch_no' },
    { title: t('打款订单号'), dataIndex: 'external_order_no' },
    {
      title: t('申请时间'),
      dataIndex: 'created_at',
      render: (_, record) => timestamp2string(record.created_at),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <Space vertical align='start' style={{ width: '100%' }} spacing={16}>
        <Card style={{ width: '100%' }} loading={statusLoading}>
          <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
            <div className='flex flex-col md:flex-row md:justify-between md:items-center w-full gap-3'>
              <div>
                <Title heading={4} style={{ margin: 0 }}>
                  {t('代理管理')}
                </Title>
                <Text type='secondary'>
                  {t(
                    '独立管理代理返利、初始化状态和人工调账，不与钱包余额混用。',
                  )}
                </Text>
              </div>
              <Space>
                <Button onClick={refreshAll}>{t('刷新')}</Button>
                {!status?.initialized && (
                  <Button
                    type='primary'
                    onClick={() => setInitModalVisible(true)}
                    disabled={!status?.migration_ready || !isRoot()}
                  >
                    {t('初始化代理功能')}
                  </Button>
                )}
              </Space>
            </div>

            {status && (
              <Descriptions
                data={[
                  {
                    key: t('迁移状态'),
                    value: status.migration_ready ? t('已就绪') : t('未完成'),
                  },
                  {
                    key: t('初始化状态'),
                    value: status.initialized ? t('已初始化') : t('未初始化'),
                  },
                  {
                    key: t('功能开关'),
                    value: status.enabled ? t('已启用') : t('未启用'),
                  },
                  {
                    key: t('默认比例'),
                    value: formatRate(status.default_rate),
                  },
                  {
                    key: t('默认分组ID'),
                    value: status.default_group_id || '-',
                  },
                ]}
              />
            )}

            {!!status?.missing_resources?.length && (
              <Text type='danger'>
                {t('缺失资源：')}
                {status.missing_resources.join(', ')}
              </Text>
            )}

            {!isRoot() && !status?.initialized && (
              <Text type='warning'>{t('仅 root 账号可执行首次初始化。')}</Text>
            )}
          </Space>
        </Card>

        {status?.initialized ? (
          <>
            {canManageAgentAdmin && (
              <>
                <Card style={{ width: '100%' }} loading={overviewLoading}>
                  <Space
                    vertical
                    align='start'
                    style={{ width: '100%' }}
                    spacing={12}
                  >
                    <Title heading={5} style={{ margin: 0 }}>
                      {t('代理总览')}
                    </Title>
                    <div className='grid grid-cols-1 md:grid-cols-3 xl:grid-cols-6 gap-4 w-full'>
                      <Card>
                        <Text type='secondary'>{t('代理数')}</Text>
                        <Title heading={4}>{overview?.agent_count || 0}</Title>
                      </Card>
                      <Card>
                        <Text type='secondary'>{t('分组数')}</Text>
                        <Title heading={4}>{overview?.group_count || 0}</Title>
                      </Card>
                      <Card>
                        <Text type='secondary'>{t('推广链接数')}</Text>
                        <Title heading={4}>
                          {overview?.promo_link_count || 0}
                        </Title>
                      </Card>
                      <Card>
                        <Text type='secondary'>{t('下级用户数')}</Text>
                        <Title heading={4}>
                          {overview?.downline_user_count || 0}
                        </Title>
                      </Card>
                      <Card>
                        <Text type='secondary'>{t('返利总余额')}</Text>
                        <Title heading={4}>
                          {formatAmount(overview?.rebate_balance_amount)}
                        </Title>
                      </Card>
                      <Card>
                        <Text type='secondary'>{t('累计返利')}</Text>
                        <Title heading={4}>
                          {formatAmount(overview?.rebate_total_amount)}
                        </Title>
                      </Card>
                    </div>
                  </Space>
                </Card>

                <Card style={{ width: '100%' }}>
                  <Space
                    vertical
                    align='start'
                    style={{ width: '100%' }}
                    spacing={12}
                  >
                    <div className='flex flex-col md:flex-row md:justify-between md:items-center w-full gap-3'>
                      <Title heading={5} style={{ margin: 0 }}>
                        {t('代理分组')}
                      </Title>
                      <Button type='primary' onClick={handleOpenCreateGroup}>
                        {t('新增分组')}
                      </Button>
                    </div>
                    <Table
                      rowKey='id'
                      columns={groupColumns}
                      dataSource={groups}
                      pagination={false}
                      empty={<Empty title={t('暂无代理分组')} />}
                    />
                  </Space>
                </Card>
              </>
            )}

            <Card style={{ width: '100%' }}>
              <Space
                vertical
                align='start'
                style={{ width: '100%' }}
                spacing={12}
              >
                <div className='flex flex-col md:flex-row md:justify-between md:items-center w-full gap-3'>
                  <Title heading={5} style={{ margin: 0 }}>
                    {t('代理资料')}
                  </Title>
                  <Space>
                    <Input
                      placeholder={t('搜索用户ID、用户名或显示名称')}
                      value={keywordInput}
                      onChange={setKeywordInput}
                      style={{ width: 260 }}
                    />
                    <Button
                      onClick={() => {
                        setProfilesPage(1);
                        setKeyword(keywordInput.trim());
                        loadProfiles(1, keywordInput.trim());
                      }}
                    >
                      {t('搜索')}
                    </Button>
                    {canAssignDownlines && (
                      <Button
                        type='warning'
                        onClick={() => handleOpenAssignDownline()}
                      >
                        {t('分配用户')}
                      </Button>
                    )}
                    {canManageAgentAdmin && (
                      <Button type='primary' onClick={handleOpenCreateProfile}>
                        {t('新增代理')}
                      </Button>
                    )}
                  </Space>
                </div>
                <Table
                  rowKey='id'
                  columns={visibleProfileColumns}
                  dataSource={profiles}
                  loading={profilesLoading}
                  pagination={false}
                  empty={
                    <Empty
                      title={t('暂无代理资料')}
                      description={t('先新增一个代理用户')}
                    />
                  }
                />
                <Pagination
                  total={profilesTotal}
                  currentPage={profilesPage}
                  pageSize={DEFAULT_PAGE_SIZE}
                  onPageChange={(page) => {
                    setProfilesPage(page);
                    loadProfiles(page, keyword);
                  }}
                />
              </Space>
            </Card>

            {canManageAgentAdmin && (
              <>
                <Card style={{ width: '100%' }}>
                  <Space
                    vertical
                    align='start'
                    style={{ width: '100%' }}
                    spacing={12}
                  >
                    <Title heading={5} style={{ margin: 0 }}>
                      {t('最近调账记录')}
                    </Title>
                    <Table
                      rowKey='id'
                      columns={adjustmentColumns}
                      dataSource={adjustments}
                      loading={adjustmentsLoading}
                      pagination={false}
                      empty={
                        <Empty
                          title={t('暂无调账记录')}
                          description={t('管理员调账后会显示在这里')}
                        />
                      }
                    />
                  </Space>
                </Card>

                <Card style={{ width: '100%' }}>
                  <Space
                    vertical
                    align='start'
                    style={{ width: '100%' }}
                    spacing={12}
                  >
                    <div className='flex flex-col md:flex-row md:justify-between md:items-center w-full gap-3'>
                      <Title heading={5} style={{ margin: 0 }}>
                        {t('推广链接管理')}
                      </Title>
                      <Button
                        type='primary'
                        onClick={handleOpenCreatePromoLink}
                      >
                        {t('新增推广链接')}
                      </Button>
                    </div>
                    <Table
                      rowKey='id'
                      columns={promoLinkColumns}
                      dataSource={promoLinks}
                      loading={promoLinksLoading}
                      pagination={false}
                      empty={
                        <Empty
                          title={t('暂无推广链接')}
                          description={t('创建后即可生成邀请落地链接')}
                        />
                      }
                    />
                    <Pagination
                      total={promoLinksTotal}
                      currentPage={promoLinksPage}
                      pageSize={DEFAULT_PAGE_SIZE}
                      onPageChange={(page) => {
                        setPromoLinksPage(page);
                        loadPromoLinks(page);
                      }}
                    />
                  </Space>
                </Card>
              </>
            )}

            {canManageAgentAdmin && (
              <Card style={{ width: '100%' }}>
                <Space
                  vertical
                  align='start'
                  style={{ width: '100%' }}
                  spacing={12}
                >
                  <div className='flex flex-col md:flex-row md:justify-between md:items-center w-full gap-3'>
                    <div>
                      <Title heading={5} style={{ margin: 0 }}>
                        {t('提现申请列表')}
                      </Title>
                      <Text type='secondary'>
                        {t(
                          '导出后请在 CSV 最后一列填写打款订单号，再导入回执，系统会按申请单ID匹配并标记已打款。',
                        )}
                      </Text>
                    </div>
                    <Space>
                      <input
                        type='date'
                        value={withdrawStartDate}
                        onChange={(e) => setWithdrawStartDate(e.target.value)}
                        style={{
                          width: 170,
                          height: 32,
                          padding: '0 12px',
                          border: '1px solid var(--semi-color-border)',
                          borderRadius: 6,
                        }}
                      />
                      <input
                        type='date'
                        value={withdrawEndDate}
                        onChange={(e) => setWithdrawEndDate(e.target.value)}
                        style={{
                          width: 170,
                          height: 32,
                          padding: '0 12px',
                          border: '1px solid var(--semi-color-border)',
                          borderRadius: 6,
                        }}
                      />
                      <Select
                        value={withdrawStatusFilter}
                        onChange={setWithdrawStatusFilter}
                        optionList={[
                          { label: t('全部状态'), value: '' },
                          { label: t('待处理'), value: 'pending' },
                          { label: t('已导出'), value: 'exported' },
                          { label: t('已打款'), value: 'paid' },
                        ]}
                        style={{ width: 140 }}
                      />
                      <Button onClick={handleSearchWithdrawRequests}>
                        {t('筛选')}
                      </Button>
                      <Button
                        type='secondary'
                        onClick={handleExportWithdrawRequests}
                      >
                        {t('导出')}
                      </Button>
                      <Button
                        type='primary'
                        onClick={() => withdrawImportRef.current?.click()}
                      >
                        {t('导入回执')}
                      </Button>
                      <input
                        ref={withdrawImportRef}
                        type='file'
                        accept='.csv,text/csv'
                        style={{ display: 'none' }}
                        onChange={handleImportWithdrawResults}
                      />
                    </Space>
                  </div>
                  <Table
                    rowKey='id'
                    columns={withdrawColumns}
                    dataSource={withdrawRequests}
                    loading={withdrawRequestsLoading}
                    pagination={false}
                    empty={<Empty title={t('暂无提现申请')} />}
                  />
                  <Pagination
                    total={withdrawRequestsTotal}
                    currentPage={withdrawRequestsPage}
                    pageSize={DEFAULT_PAGE_SIZE}
                    onPageChange={(page) => {
                      setWithdrawRequestsPage(page);
                      loadWithdrawRequests(page);
                    }}
                  />
                </Space>
              </Card>
            )}
          </>
        ) : (
          <Card style={{ width: '100%' }}>
            <Empty
              title={t('代理功能尚未初始化')}
              description={t(
                '完成数据库自动迁移后，由 root 账号点击初始化代理功能。',
              )}
            />
          </Card>
        )}
      </Space>

      <Modal
        title={t('初始化代理功能')}
        visible={initModalVisible}
        onCancel={() => setInitModalVisible(false)}
        onOk={handleInitialize}
        confirmLoading={profileSubmitting}
      >
        <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
          <Text>{t('初始化会创建默认代理分组并启用代理功能。')}</Text>
          <InputNumber
            min={0}
            step={0.1}
            value={initRatePercent}
            onChange={setInitRatePercent}
            suffix='%'
            style={{ width: '100%' }}
          />
        </Space>
      </Modal>

      <Modal
        title={t('转移用户')}
        visible={transferModalVisible}
        onCancel={() => setTransferModalVisible(false)}
        onOk={handleSubmitTransfer}
        confirmLoading={transferSubmitting}
      >
        <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
          <Descriptions
            data={[
              {
                key: t('源代理'),
                value:
                  transferForm.sourceAgentUsername ||
                  transferForm.sourceAgentUserId,
              },
              {
                key: t('下线用户'),
                value:
                  transferForm.downlineUsername || transferForm.downlineUserId,
              },
            ]}
            row
          />
          <Input
            prefix={<IconSearch size={14} />}
            placeholder={t('搜索目标代理ID、用户名或分组')}
            value={transferAgentKeyword}
            onChange={setTransferAgentKeyword}
          />
          <Select
            placeholder={t('目标代理')}
            value={transferForm.targetAgentUserId || undefined}
            onChange={handleChangeTransferTarget}
            optionList={transferAgentOptions}
            loading={transferAgentsLoading}
            filter
            style={{ width: '100%' }}
          />
          {canManageAgentAdmin && (
            <Select
              placeholder={t('转移后的推广链接')}
              value={transferForm.promoLinkId}
              onChange={(value) =>
                setTransferForm((prev) => ({ ...prev, promoLinkId: value }))
              }
              optionList={transferPromoLinkOptions}
              loading={transferPromoLinksLoading}
              style={{ width: '100%' }}
            />
          )}
          <Text type='secondary'>
            {t(
              '转移会更新该用户的邀请归属和推广链接归属，历史返佣记录保持不变。',
            )}
          </Text>
          <TextArea
            placeholder={t('备注')}
            value={transferForm.remark}
            onChange={(value) =>
              setTransferForm((prev) => ({ ...prev, remark: value }))
            }
            rows={3}
          />
        </Space>
      </Modal>

      <Modal
        title={t('代理分组')}
        visible={groupModalVisible}
        onCancel={() => setGroupModalVisible(false)}
        onOk={handleSubmitGroup}
        confirmLoading={groupSubmitting}
      >
        <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
          <Input
            placeholder={t('分组名称')}
            value={groupForm.name}
            onChange={(value) =>
              setGroupForm((prev) => ({ ...prev, name: value }))
            }
          />
          <InputNumber
            min={0}
            step={0.1}
            value={groupForm.rebateRatePercent}
            onChange={(value) =>
              setGroupForm((prev) => ({
                ...prev,
                rebateRatePercent: value || 0,
              }))
            }
            suffix='%'
            style={{ width: '100%' }}
          />
          <Select
            value={groupForm.status}
            onChange={(value) =>
              setGroupForm((prev) => ({ ...prev, status: value }))
            }
            optionList={[
              { label: t('启用'), value: 1 },
              { label: t('禁用'), value: 0 },
            ]}
            style={{ width: '100%' }}
          />
          <Input
            placeholder={t('备注')}
            value={groupForm.remark}
            onChange={(value) =>
              setGroupForm((prev) => ({ ...prev, remark: value }))
            }
          />
        </Space>
      </Modal>

      <Modal
        title={t('代理资料')}
        visible={profileModalVisible}
        onCancel={() => setProfileModalVisible(false)}
        onOk={handleSubmitProfile}
        confirmLoading={profileSubmitting}
      >
        <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
          <Input
            placeholder={t('用户ID')}
            value={String(profileForm.userId)}
            onChange={(value) =>
              setProfileForm((prev) => ({ ...prev, userId: value }))
            }
          />
          <Select
            value={profileForm.status}
            onChange={(value) =>
              setProfileForm((prev) => ({ ...prev, status: value }))
            }
            optionList={[
              { label: t('启用'), value: 1 },
              { label: t('禁用'), value: 0 },
            ]}
            style={{ width: '100%' }}
          />
          <Select
            value={profileForm.rebateGroupId}
            onChange={(value) =>
              setProfileForm((prev) => ({ ...prev, rebateGroupId: value }))
            }
            optionList={groupOptions}
            style={{ width: '100%' }}
          />
          <Checkbox
            checked={profileForm.customRateEnabled}
            onChange={(e) =>
              setProfileForm((prev) => ({
                ...prev,
                customRateEnabled: e.target.checked,
                customRatePercent: e.target.checked
                  ? prev.customRatePercent
                  : 0,
              }))
            }
          >
            {t('启用自定义比例覆盖')}
          </Checkbox>
          <Text type='secondary'>
            {t(
              '不启用时将完全跟随所选分组比例，后续调整分组比例会自动同步到该代理。',
            )}
          </Text>
          <InputNumber
            min={0}
            step={0.1}
            value={profileForm.customRatePercent}
            onChange={(value) =>
              setProfileForm((prev) => ({
                ...prev,
                customRatePercent: value || 0,
              }))
            }
            suffix='%'
            style={{ width: '100%' }}
            disabled={!profileForm.customRateEnabled}
          />
          <Input
            placeholder={t('备注')}
            value={profileForm.remark}
            onChange={(value) =>
              setProfileForm((prev) => ({ ...prev, remark: value }))
            }
          />
        </Space>
      </Modal>

      <Modal
        title={t('调整代理返利余额')}
        visible={adjustModalVisible}
        onCancel={() => setAdjustModalVisible(false)}
        onOk={handleSubmitAdjust}
        confirmLoading={adjustSubmitting}
      >
        <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
          <Text>
            {t('当前代理：')}
            {adjustForm.username || adjustForm.agentUserId}
          </Text>
          <Input
            placeholder={t('调整金额，例如 10.50 或 -5.00')}
            value={adjustForm.amount}
            onChange={(value) =>
              setAdjustForm((prev) => ({ ...prev, amount: value }))
            }
          />
          <TextArea
            placeholder={t('请输入调整原因')}
            value={adjustForm.reason}
            onChange={(value) =>
              setAdjustForm((prev) => ({ ...prev, reason: value }))
            }
            rows={4}
          />
        </Space>
      </Modal>

      <Modal
        title={
          activeAgentScope
            ? `${t('代理详情')} - ${activeAgentScope.username}`
            : t('代理详情')
        }
        visible={inspectModalVisible}
        onCancel={() => setInspectModalVisible(false)}
        footer={null}
        width={1200}
      >
        <Space vertical align='start' style={{ width: '100%' }} spacing={16}>
          {canManageAgentAdmin && (
            <Card style={{ width: '100%' }}>
              <Space
                vertical
                align='start'
                style={{ width: '100%' }}
                spacing={12}
              >
                <Title heading={5} style={{ margin: 0 }}>
                  {t('渠道统计')}
                </Title>
                <Table
                  rowKey='promo_link_id'
                  columns={promoLinkStatColumns}
                  dataSource={promoLinkStats}
                  loading={promoLinkStatsLoading}
                  pagination={false}
                  empty={<Empty title={t('暂无推广链接统计')} />}
                />
              </Space>
            </Card>
          )}

          <Card style={{ width: '100%' }}>
            <Space
              vertical
              align='start'
              style={{ width: '100%' }}
              spacing={12}
            >
              <div className='flex flex-col md:flex-row md:justify-between md:items-center w-full gap-3'>
                <Title heading={5} style={{ margin: 0 }}>
                  {t('下级用户概况')}
                </Title>
                <Space>
                  <Input
                    prefix={<IconSearch size={14} />}
                    placeholder={t('搜索下级用户ID、用户名或显示名称')}
                    value={downlineKeywordInput}
                    onChange={setDownlineKeywordInput}
                    style={{ width: 280 }}
                    onEnterPress={() => {
                      const nextKeyword = downlineKeywordInput.trim();
                      setDownlinesPage(1);
                      setDownlineKeyword(nextKeyword);
                      loadDownlines(
                        activeAgentScope?.userId || 0,
                        1,
                        nextKeyword,
                      );
                    }}
                  />
                  <Button
                    onClick={() => {
                      const nextKeyword = downlineKeywordInput.trim();
                      setDownlinesPage(1);
                      setDownlineKeyword(nextKeyword);
                      loadDownlines(
                        activeAgentScope?.userId || 0,
                        1,
                        nextKeyword,
                      );
                    }}
                  >
                    {t('搜索')}
                  </Button>
                  {downlineKeyword ? (
                    <Button
                      onClick={() => {
                        setDownlineKeyword('');
                        setDownlineKeywordInput('');
                        setDownlinesPage(1);
                        loadDownlines(activeAgentScope?.userId || 0, 1, '');
                      }}
                    >
                      {t('清空')}
                    </Button>
                  ) : null}
                </Space>
              </div>
              <Table
                rowKey='user_id'
                columns={downlineColumns}
                dataSource={downlines}
                loading={downlinesLoading}
                pagination={false}
                empty={<Empty title={t('暂无下级用户数据')} />}
              />
              <Pagination
                total={downlinesTotal}
                currentPage={downlinesPage}
                pageSize={DEFAULT_PAGE_SIZE}
                onPageChange={(page) => {
                  setDownlinesPage(page);
                  loadDownlines(
                    activeAgentScope?.userId || 0,
                    page,
                    downlineKeyword,
                  );
                }}
              />
            </Space>
          </Card>
        </Space>
      </Modal>

      <Modal
        title={t('推广链接')}
        visible={promoLinkModalVisible}
        onCancel={() => setPromoLinkModalVisible(false)}
        onOk={handleSubmitPromoLink}
        confirmLoading={promoLinkSubmitting}
      >
        <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
          <Input
            placeholder={t('代理用户ID')}
            value={String(promoLinkForm.agentUserId)}
            onChange={(value) =>
              setPromoLinkForm((prev) => ({ ...prev, agentUserId: value }))
            }
          />
          <Input
            placeholder={t('渠道名称')}
            value={promoLinkForm.name}
            onChange={(value) =>
              setPromoLinkForm((prev) => ({ ...prev, name: value }))
            }
          />
          <Input
            placeholder={t('推广码，留空自动生成')}
            value={promoLinkForm.code}
            onChange={(value) =>
              setPromoLinkForm((prev) => ({ ...prev, code: value }))
            }
          />
          <Select
            value={promoLinkForm.status}
            onChange={(value) =>
              setPromoLinkForm((prev) => ({ ...prev, status: value }))
            }
            optionList={[
              { label: t('启用'), value: 1 },
              { label: t('禁用'), value: 0 },
            ]}
            style={{ width: '100%' }}
          />
          <Input
            placeholder={t('落地页，默认 /')}
            value={promoLinkForm.landingPage}
            onChange={(value) =>
              setPromoLinkForm((prev) => ({ ...prev, landingPage: value }))
            }
          />
          <Input
            placeholder={t('备注')}
            value={promoLinkForm.remark}
            onChange={(value) =>
              setPromoLinkForm((prev) => ({ ...prev, remark: value }))
            }
          />
        </Space>
      </Modal>

      <Modal
        title={t('分配代理下级用户')}
        visible={assignModalVisible}
        onCancel={() => setAssignModalVisible(false)}
        onOk={handleSubmitAssign}
        confirmLoading={assignSubmitting}
      >
        <Space vertical align='start' style={{ width: '100%' }} spacing={12}>
          <Input
            placeholder={t('普通用户ID')}
            value={String(assignForm.downlineUserId)}
            onChange={(value) =>
              setAssignForm((prev) => ({ ...prev, downlineUserId: value }))
            }
          />
          <Input
            prefix={<IconSearch size={14} />}
            placeholder={t('搜索目标代理ID、用户名或分组')}
            value={assignAgentKeyword}
            onChange={setAssignAgentKeyword}
          />
          <Select
            placeholder={t('目标代理')}
            value={assignForm.targetAgentUserId || undefined}
            onChange={(value) =>
              setAssignForm((prev) => ({ ...prev, targetAgentUserId: value }))
            }
            optionList={assignAgentOptions}
            loading={transferAgentsLoading}
            filter
            style={{ width: '100%' }}
          />
          <Text type='secondary'>
            {t(
              '分配后会更新该用户的邀请归属，后续充值和兑换码返利将归到目标代理；历史返佣记录保持不变。',
            )}
          </Text>
          <TextArea
            placeholder={t('备注')}
            value={assignForm.remark}
            onChange={(value) =>
              setAssignForm((prev) => ({ ...prev, remark: value }))
            }
            rows={3}
          />
        </Space>
      </Modal>
    </div>
  );
}
