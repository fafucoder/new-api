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
import { Button, Col, Form, InputNumber, Row, Select, Spin, Typography } from '@douyinfe/semi-ui';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

export default function SettingsInvoice(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    'invoice_setting.enabled': false,
    'invoice_setting.minimum_amount': 50.0,
    'invoice_setting.require_manual_review': true,
    'invoice_setting.provider': 'stub',
    'invoice_setting.topup_source': 'top_ups',
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

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('发票设置')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('配置发票申请功能，包括最低开票额度、是否需要人工审核等')}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'invoice_setting.enabled'}
                  label={t('启用发票功能')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange('invoice_setting.enabled')}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'invoice_setting.minimum_amount'}
                  label={t('最低开票额度')}
                  placeholder={t('最低开票额度 (USD)')}
                  onChange={handleFieldChange('invoice_setting.minimum_amount')}
                  min={0}
                  precision={2}
                  disabled={!inputs['invoice_setting.enabled']}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'invoice_setting.require_manual_review'}
                  label={t('需要人工审核')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange('invoice_setting.require_manual_review')}
                  disabled={!inputs['invoice_setting.enabled']}
                />
              </Col>
            </Row>
            <Row gutter={16} style={{ marginTop: 16 }}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Select
                  field={'invoice_setting.provider'}
                  label={t('开票服务商')}
                  value={inputs['invoice_setting.provider']}
                  onChange={handleFieldChange('invoice_setting.provider')}
                  disabled={!inputs['invoice_setting.enabled']}
                >
                  <Select.Option value='stub'>{t('Stub (测试用)')}</Select.Option>
                </Form.Select>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Select
                  field={'invoice_setting.topup_source'}
                  label={t('充值总额来源')}
                  value={inputs['invoice_setting.topup_source']}
                  onChange={handleFieldChange('invoice_setting.topup_source')}
                  disabled={!inputs['invoice_setting.enabled']}
                >
                  <Select.Option value='top_ups'>{t('充值记录 (top_ups)')}</Select.Option>
                  <Select.Option value='users'>{t('用户额度 (users)')}</Select.Option>
                </Form.Select>
              </Col>
            </Row>
            <Row>
              <Button size='default' type='primary' onClick={onSubmit} style={{ marginTop: 16 }}>
                {t('保存发票设置')}
              </Button>
            </Row>
          </Form.Section>
        </Form>
      </Spin>
    </>
  );
}