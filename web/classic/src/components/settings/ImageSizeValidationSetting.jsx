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
import {
  Form,
  Button,
  Spin,
  Banner,
  Typography,
  Card,
  Row,
  Col,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';

const { Text } = Typography;

const defaultSettings = {
  enabled: false,
  models: ['gpt-image-2'],
  multiple_of: 16,
  max_edge: 3840,
  max_aspect_ratio: 3,
  min_pixels: 655360,
  max_pixels: 8294400,
};

const ImageSizeValidationSetting = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [settings, setSettings] = useState(defaultSettings);
  const formApiRef = useRef(null);

  const getSettings = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/setting/image-size-validation');
      const { success, message, data } = res.data;
      if (success && data) {
        const next = {
          enabled: data.enabled || false,
          models: Array.isArray(data.models) ? data.models : [],
          multiple_of: data.multiple_of || defaultSettings.multiple_of,
          max_edge: data.max_edge || defaultSettings.max_edge,
          max_aspect_ratio:
            data.max_aspect_ratio || defaultSettings.max_aspect_ratio,
          min_pixels: data.min_pixels || defaultSettings.min_pixels,
          max_pixels: data.max_pixels || defaultSettings.max_pixels,
        };
        setSettings(next);
        // Semi Form 的 initValues 仅在首次挂载生效，后续必须通过 formApi 同步
        if (formApiRef.current) {
          formApiRef.current.setValues(next);
        }
      } else {
        showError(message || t('获取设置失败'));
      }
    } catch (error) {
      showError(t('获取设置失败'));
    } finally {
      setLoading(false);
    }
  };

  const saveSettings = async (values) => {
    if (values.multiple_of < 1) {
      showError(t('倍数必须大于等于1'));
      return;
    }
    if (values.max_edge < 1) {
      showError(t('单边最大值必须大于0'));
      return;
    }
    if (values.max_aspect_ratio < 1) {
      showError(t('长宽比上限必须大于等于1'));
      return;
    }
    if (
      values.min_pixels < 0 ||
      values.max_pixels < 0 ||
      (values.max_pixels > 0 && values.max_pixels < values.min_pixels)
    ) {
      showError(t('总像素范围不合法'));
      return;
    }

    setLoading(true);
    try {
      const payload = {
        enabled: !!values.enabled,
        models: Array.isArray(values.models) ? values.models : [],
        multiple_of: Number(values.multiple_of),
        max_edge: Number(values.max_edge),
        max_aspect_ratio: Number(values.max_aspect_ratio),
        min_pixels: Number(values.min_pixels),
        max_pixels: Number(values.max_pixels),
      };

      const res = await API.put('/api/setting/image-size-validation', payload);
      if (!res.data.success) {
        showError(res.data.message || t('保存失败'));
        return;
      }

      showSuccess(t('保存成功'));
      await getSettings();
    } catch (error) {
      showError(t('保存失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    getSettings();
  }, []);

  return (
    <Card title={t('图片尺寸校验')} style={{ marginTop: '10px' }}>
      <Spin spinning={loading}>
        <Banner
          type='info'
          description={t(
            '对指定模型（如 gpt-image-2）的 size 参数做硬性尺寸校验，校验失败将直接返回 400 并记录到日志，不再转发上游',
          )}
          style={{ marginBottom: 20 }}
        />

        <Form
          initValues={settings}
          getFormApi={(api) => (formApiRef.current = api)}
          onSubmit={saveSettings}
          labelPosition='left'
          labelAlign='right'
          labelWidth={150}
        >
          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.Switch
                field='enabled'
                label={t('启用尺寸校验')}
                onChange={(value) => {
                  setSettings({ ...settings, enabled: value });
                }}
              />
            </Col>
          </Row>

          <Row gutter={16}>
            <Col xs={24}>
              <Form.TagInput
                field='models'
                label={t('需校验的模型')}
                placeholder={t('输入模型名后回车，如 gpt-image-2')}
                style={{ width: '100%' }}
                extraText={t('仅对列表内的模型执行尺寸校验，其他模型不受影响')}
              />
            </Col>
          </Row>

          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='multiple_of'
                label={t('宽高倍数')}
                min={1}
                style={{ width: '100%' }}
                extraText={t('宽、高均需为该值的整数倍')}
              />
            </Col>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='max_edge'
                label={t('单边最大值')}
                min={1}
                style={{ width: '100%' }}
                suffix={
                  <Text type='tertiary' size='small'>
                    px
                  </Text>
                }
                extraText={t('长边不得超过该值')}
              />
            </Col>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='max_aspect_ratio'
                label={t('长宽比上限')}
                min={1}
                step={0.1}
                style={{ width: '100%' }}
                extraText={t('长边/短边 不得超过该比例，如 3 表示 3:1')}
              />
            </Col>
          </Row>

          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='min_pixels'
                label={t('总像素下限')}
                min={0}
                style={{ width: '100%' }}
                extraText={t('宽×高 不得低于该值')}
              />
            </Col>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='max_pixels'
                label={t('总像素上限')}
                min={0}
                style={{ width: '100%' }}
                extraText={t('宽×高 不得高于该值')}
              />
            </Col>
          </Row>

          <Row style={{ marginTop: 16 }}>
            <Button type='primary' htmlType='submit' loading={loading}>
              {t('保存校验设置')}
            </Button>
          </Row>
        </Form>
      </Spin>
    </Card>
  );
};

export default ImageSizeValidationSetting;
