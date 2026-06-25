/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Input, Modal, Select, Space, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../../helpers';

const { Text } = Typography;

const text = {
  title: '\u66f4\u6539\u4e0a\u7ea7\u4ee3\u7406',
  ok: '\u786e\u8ba4\u66f4\u6539',
  cancel: '\u53d6\u6d88',
  user: '\u7528\u6237',
  currentAgent: '\u5f53\u524d\u4e0a\u7ea7\u4ee3\u7406',
  noAgent: '\u65e0\u4e0a\u7ea7\u4ee3\u7406',
  targetAgent: '\u76ee\u6807\u4ee3\u7406',
  searchPlaceholder: '\u641c\u7d22\u4ee3\u7406ID\u3001\u7528\u6237\u540d\u6216\u663e\u793a\u540d',
  reasonPlaceholder: '\u5907\u6ce8\u539f\u56e0\uff08\u53ef\u9009\uff09',
  emptyAgent: '\u8bf7\u9009\u62e9\u76ee\u6807\u4ee3\u7406',
  sameAgent: '\u76ee\u6807\u4ee3\u7406\u4e0d\u80fd\u548c\u5f53\u524d\u4e0a\u7ea7\u76f8\u540c',
  success: '\u4e0a\u7ea7\u4ee3\u7406\u5df2\u66f4\u6539',
  failed: '\u64cd\u4f5c\u5931\u8d25\uff0c\u8bf7\u91cd\u8bd5',
};

const buildAgentLabel = (agent) => {
  if (!agent) {
    return '';
  }
  const name = agent.display_name || agent.username || '';
  return name ? `${name} (#${agent.user_id})` : `#${agent.user_id}`;
};

const ChangeAgentModal = ({ visible, user, onCancel, refresh }) => {
  const { t } = useTranslation();
  const [agentOptions, setAgentOptions] = useState([]);
  const [targetAgentUserId, setTargetAgentUserId] = useState();
  const [keyword, setKeyword] = useState('');
  const [remark, setRemark] = useState('');
  const [loadingAgents, setLoadingAgents] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const loadAgents = async (searchKeyword = '') => {
    setLoadingAgents(true);
    try {
      const params = new URLSearchParams({
        p: '1',
        page_size: '20',
      });
      if (searchKeyword) {
        params.set('keyword', searchKeyword);
      }
      const res = await API.get(`/api/agent/profiles?${params.toString()}`);
      const { success, message, data } = res.data;
      if (success) {
        setAgentOptions(
          (data.items || []).map((agent) => ({
            label: buildAgentLabel(agent),
            value: agent.user_id,
            agent,
          })),
        );
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message || t(text.failed));
    } finally {
      setLoadingAgents(false);
    }
  };

  useEffect(() => {
    if (visible) {
      setTargetAgentUserId();
      setKeyword('');
      setRemark('');
      loadAgents();
    }
  }, [visible]);

  const submit = async () => {
    if (!user?.id) {
      return;
    }
    if (!targetAgentUserId) {
      showError(t(text.emptyAgent));
      return;
    }
    if (Number(targetAgentUserId) === Number(user.inviter_id || 0)) {
      showError(t(text.sameAgent));
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.post('/api/agent/downline/change', {
        target_agent_user_id: Number(targetAgentUserId),
        downline_user_id: user.id,
        remark,
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t(text.success));
        await refresh?.();
        onCancel?.();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message || t(text.failed));
    } finally {
      setSubmitting(false);
    }
  };

  const currentAgent = user?.inviter_id
    ? user.inviter_display_name || user.inviter_username
      ? `${user.inviter_display_name || user.inviter_username} (#${user.inviter_id})`
      : `#${user.inviter_id}`
    : t(text.noAgent);

  return (
    <Modal
      title={t(text.title)}
      visible={visible}
      onCancel={onCancel}
      onOk={submit}
      confirmLoading={submitting}
      okText={t(text.ok)}
      cancelText={t(text.cancel)}
      closable={null}
    >
      <Space vertical align='start' spacing='medium' style={{ width: '100%' }}>
        <Text>
          {t(text.user)}: {user?.username || user?.id}
        </Text>
        <Text>
          {t(text.currentAgent)}: {currentAgent}
        </Text>
        <Select
          style={{ width: '100%' }}
          filter
          remote
          showClear
          loading={loadingAgents}
          placeholder={t(text.searchPlaceholder)}
          value={targetAgentUserId}
          optionList={agentOptions}
          onSearch={(value) => {
            setKeyword(value);
            loadAgents(value);
          }}
          onChange={setTargetAgentUserId}
          onFocus={() => loadAgents(keyword)}
        />
        <Input
          placeholder={t(text.reasonPlaceholder)}
          value={remark}
          onChange={setRemark}
          showClear
        />
      </Space>
    </Modal>
  );
};

export default ChangeAgentModal;
