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
import { Modal, Select, Space, Typography } from '@douyinfe/semi-ui';

const { Text } = Typography;

const text = {
  title: '\u63d0\u5347\u7528\u6237\u7b49\u7ea7',
  ok: '\u786e\u5b9a',
  cancel: '\u53d6\u6d88',
  user: '\u7528\u6237',
  placeholder: '\u8bf7\u9009\u62e9\u76ee\u6807\u7b49\u7ea7',
  help:
    '\u5ba2\u670d\u7ba1\u7406\u5458\u53ea\u80fd\u7ba1\u7406\u5151\u6362\u7801\uff0c\u5e76\u51cf\u5c11\u666e\u901a\u7528\u6237\u989d\u5ea6\uff1b\u7ba1\u7406\u5458\u62e5\u6709\u5b8c\u6574\u7ba1\u7406\u6743\u9650\u3002',
};

const PromoteUserModal = ({
  visible,
  onCancel,
  onConfirm,
  user,
  t,
  roleOptions = [],
}) => {
  const [action, setAction] = useState('');

  useEffect(() => {
    if (visible) {
      setAction(roleOptions[0]?.value || '');
    }
  }, [visible, roleOptions]);

  return (
    <Modal
      title={t(text.title)}
      visible={visible}
      onCancel={onCancel}
      onOk={() => onConfirm(action)}
      okText={t(text.ok)}
      cancelText={t(text.cancel)}
      type='warning'
      okButtonProps={{ disabled: !action }}
    >
      <Space vertical align='start' spacing='medium' style={{ width: '100%' }}>
        <Text>
          {t(text.user)}: {user?.username || user?.id || '-'}
        </Text>
        <Select
          value={action}
          optionList={roleOptions}
          onChange={setAction}
          placeholder={t(text.placeholder)}
          style={{ width: '100%' }}
        />
        <Text type='tertiary'>{t(text.help)}</Text>
      </Space>
    </Modal>
  );
};

export default PromoteUserModal;
