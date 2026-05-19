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

import React, { useEffect, useState, useRef } from 'react';
import { Button, Col, Form, Row, Spin, Typography } from '@douyinfe/semi-ui';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

export default function SettingsBalanceAlert(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    'monitor_setting.auto_balance_alert_enabled': false,
    'monitor_setting.auto_balance_alert_minutes': 30,
    'monitor_setting.balance_alert_cooldown_hours': 6,
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);

  function handleFieldChange(fieldName) {
    return (value) => {
      setInputs((inputs) => ({ ...inputs, [fieldName]: value }));
    };
  }

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow);
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    const requestQueue = updateArray.map((item) => {
      let value = '';
      if (typeof inputs[item.key] === 'boolean') {
        value = String(inputs[item.key]);
      } else if (typeof inputs[item.key] === 'number') {
        value = String(inputs[item.key]);
      } else {
        value = inputs[item.key];
      }
      return API.put('/api/option/', {
        key: item.key,
        value,
      });
    });
    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (requestQueue.length === 1) {
          if (res.includes(undefined)) return;
        } else if (requestQueue.length > 1) {
          if (res.includes(undefined))
            return showError(t('部分保存失败，请重试'));
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
    for (let key in props.options) {
      if (Object.keys(inputs).includes(key)) {
        currentInputs[key] = props.options[key];
      }
    }
    if (Object.keys(currentInputs).length > 0) {
      setInputs(currentInputs);
      setInputsRow(structuredClone(currentInputs));
      if (refForm.current) {
        refForm.current.setValues(currentInputs);
      }
    }
  }, [props.options]);

  const enabled = inputs['monitor_setting.auto_balance_alert_enabled'];

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('余额监控告警设置')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t(
                '后台周期性扫描渠道余额监控规则，跌破阈值通过 webhook 或站内通知告警。规则在「余额监控」页配置。',
              )}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'monitor_setting.auto_balance_alert_enabled'}
                  label={t('启用渠道余额监控告警')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange(
                    'monitor_setting.auto_balance_alert_enabled',
                  )}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'monitor_setting.auto_balance_alert_minutes'}
                  label={t('余额监控扫描间隔')}
                  step={1}
                  min={1}
                  suffix={t('分钟')}
                  extraText={t('每隔多少分钟扫描一次余额告警规则')}
                  placeholder={''}
                  disabled={!enabled}
                  onChange={(value) =>
                    handleFieldChange(
                      'monitor_setting.auto_balance_alert_minutes',
                    )(parseInt(value))
                  }
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'monitor_setting.balance_alert_cooldown_hours'}
                  label={t('余额告警冷却时间')}
                  step={1}
                  min={1}
                  suffix={t('小时')}
                  extraText={t(
                    '持续低余额时,每隔多少小时重复告警一次,避免 spam',
                  )}
                  placeholder={''}
                  disabled={!enabled}
                  onChange={(value) =>
                    handleFieldChange(
                      'monitor_setting.balance_alert_cooldown_hours',
                    )(parseInt(value))
                  }
                />
              </Col>
            </Row>
            <Row>
              <Button
                size='default'
                type='primary'
                onClick={onSubmit}
                style={{ marginTop: 16 }}
              >
                {t('保存余额监控设置')}
              </Button>
            </Row>
          </Form.Section>
        </Form>
      </Spin>
    </>
  );
}
