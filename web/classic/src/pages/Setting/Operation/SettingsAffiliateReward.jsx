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

import React, { useEffect, useRef, useState } from 'react';
import { Button, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';

const DEFAULT_INPUTS = {
  'affiliate_setting.invitee_reward_require_recharge': false,
  'affiliate_setting.inviter_withdraw_require_invitee_recharge': false,
  'affiliate_setting.unrecharged_withdraw_ratio': 100,
  'affiliate_setting.recharged_withdraw_ratio': 100,
};

const toBoolean = (value, fallback = false) => {
  if (typeof value === 'boolean') {
    return value;
  }
  if (typeof value === 'string') {
    return value === 'true' || value === '1';
  }
  if (typeof value === 'number') {
    return value !== 0;
  }
  return fallback;
};

const toRatio = (value, fallback = 100) => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return fallback;
  }
  return Math.min(100, Math.max(0, parsed));
};

export default function SettingsAffiliateReward(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState(DEFAULT_INPUTS);
  const [inputsRow, setInputsRow] = useState(DEFAULT_INPUTS);
  const refForm = useRef();

  const handleSwitchChange = (key) => (value) => {
    setInputs((prev) => ({ ...prev, [key]: Boolean(value) }));
  };

  const handleRatioChange = (key) => (value) => {
    setInputs((prev) => ({ ...prev, [key]: toRatio(value, 0) }));
  };

  function onSubmit() {
    const updateArray = compareObjects(inputsRow, inputs);
    if (!updateArray.length) {
      return showWarning(t('你似乎并没有修改什么'));
    }
    const requestQueue = updateArray.map((item) =>
      API.put('/api/option/', {
        key: item.key,
        value: String(inputs[item.key]),
      }),
    );
    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (requestQueue.length === 1) {
          if (res.includes(undefined)) return;
        } else if (requestQueue.length > 1) {
          if (res.includes(undefined)) {
            return showError(t('部分保存失败，请重试'));
          }
        }
        showSuccess(t('保存成功'));
        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  useEffect(() => {
    const currentInputs = {};
    for (const key of Object.keys(DEFAULT_INPUTS)) {
      const fallback = DEFAULT_INPUTS[key];
      const raw =
        props.options && Object.prototype.hasOwnProperty.call(props.options, key)
          ? props.options[key]
          : fallback;
      currentInputs[key] =
        typeof fallback === 'boolean'
          ? toBoolean(raw, fallback)
          : toRatio(raw, fallback);
    }
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    if (refForm.current) {
      refForm.current.setValues(currentInputs);
    }
  }, [props.options]);

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        getFormApi={(formAPI) => (refForm.current = formAPI)}
        style={{ marginBottom: 15 }}
      >
        <Form.Section text={t('邀请奖励设置')}>
          <Row gutter={16}>
            <Col xs={24} sm={12} md={12} lg={12} xl={12}>
              <Form.Switch
                field={
                  'affiliate_setting.invitee_reward_require_recharge'
                }
                label={t(
                  '开启后，被邀请人需完成一次充值才会收到邀请奖励',
                )}
                size='default'
                checkedText='开'
                uncheckedText='关'
                onChange={handleSwitchChange(
                  'affiliate_setting.invitee_reward_require_recharge',
                )}
              />
            </Col>
            <Col xs={24} sm={12} md={12} lg={12} xl={12}>
              <Form.Switch
                field={
                  'affiliate_setting.inviter_withdraw_require_invitee_recharge'
                }
                label={t(
                  '开启后，邀请人的邀请额度需被邀请人充值后才可解锁提现',
                )}
                size='default'
                checkedText='开'
                uncheckedText='关'
                onChange={handleSwitchChange(
                  'affiliate_setting.inviter_withdraw_require_invitee_recharge',
                )}
              />
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field={'affiliate_setting.unrecharged_withdraw_ratio'}
                label={t(
                  '被邀请人未充值时，邀请额度可划转的比例',
                )}
                step={1}
                min={0}
                max={100}
                suffix='%'
                onChange={handleRatioChange(
                  'affiliate_setting.unrecharged_withdraw_ratio',
                )}
              />
            </Col>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field={'affiliate_setting.recharged_withdraw_ratio'}
                label={t(
                  '被邀请人已充值时，邀请额度可划转的比例',
                )}
                step={1}
                min={0}
                max={100}
                suffix='%'
                onChange={handleRatioChange(
                  'affiliate_setting.recharged_withdraw_ratio',
                )}
              />
            </Col>
          </Row>
          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存邀请奖励设置')}
            </Button>
          </Row>
        </Form.Section>
      </Form>
    </Spin>
  );
}
