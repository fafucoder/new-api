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
  Col
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';

const { Text } = Typography;

const ImageSizeSetting = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [settings, setSettings] = useState({
    mode: 'total_pixels',
    threshold_value: 3686400,
  });
  const formApiRef = useRef(null);

  const getSettings = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/setting/image-size');
      const { success, message, data } = res.data;
      if (success && data) {
        const next = {
          mode: data.mode || 'total_pixels',
          threshold_value: data.threshold_value || 3686400,
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
    if (values.threshold_value < 256) {
      showError(t('阈值不能小于256'));
      return;
    }

    setLoading(true);
    try {
      // 使用批量保存接口，与其他设置页面保持一致
      const updates = [
        {
          key: 'image_size_setting.mode',
          value: values.mode,
        },
        {
          key: 'image_size_setting.threshold_value',
          value: String(values.threshold_value),
        },
      ];

      const requestQueue = updates.map((item) =>
        API.put('/api/option/', item)
      );

      const results = await Promise.all(requestQueue);

      // 检查是否有失败的请求
      if (results.some(res => !res.data.success)) {
        showError(t('保存失败'));
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
    <Card title={t('图片生成尺寸设置')} style={{ marginTop: '10px' }}>
      <Spin spinning={loading}>
        <Banner
          type='info'
          description={t('用于区分普通分辨率和高分辨率图片生成请求')}
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
              <Form.Select
                field='mode'
                label={t('判断模式')}
                style={{ width: '100%' }}
                rules={[{ required: true, message: t('请选择判断模式') }]}
                onChange={(value) => {
                  setSettings({ ...settings, mode: value });
                }}
              >
                <Form.Select.Option value='total_pixels'>
                  {t('总像素数')}
                </Form.Select.Option>
                <Form.Select.Option value='min_edge'>
                  {t('最小边长')}
                </Form.Select.Option>
                <Form.Select.Option value='max_edge'>
                  {t('最大边长')}
                </Form.Select.Option>
              </Form.Select>
            </Col>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='threshold_value'
                label={t('高分辨率阈值')}
                placeholder={t('请输入阈值')}
                min={256}
                style={{ width: '100%' }}
                rules={[
                  { required: true, message: t('请输入阈值') },
                  { type: 'number', min: 256, message: t('阈值不能小于256') },
                ]}
                suffix={
                  settings.mode !== 'total_pixels' && (
                    <Text type='tertiary' size='small'>
                      px
                    </Text>
                  )
                }
                extraText={t('≤阈值为1K，>阈值为4K')}
              />
            </Col>
          </Row>

          <Card
            bodyStyle={{
              padding: '12px',
            }}
            style={{ marginTop: 16 }}
          >
            <Text strong>{t('示例配置：')}</Text>
            <div style={{ marginTop: 8, lineHeight: '1.8' }}>
              <div>• {t('OpenAI 标准：模式=总像素数，阈值=3686400 (2560x1440)')}</div>
              <div>• {t('最小边模式：模式=最小边长，阈值=1024')}</div>
              <div>• {t('最大边模式：模式=最大边长，阈值=2048')}</div>
            </div>
            <div style={{ marginTop: 12, fontSize: '12px', opacity: 0.6 }}>
              <div>• {t('如 1024x1024 = 1,048,576 像素')}</div>
              <div>• {t('如 1024x1536 的最小边是 1024')}</div>
              <div>• {t('如 1024x1536 的最大边是 1536')}</div>
            </div>
          </Card>

          <Row style={{ marginTop: 16 }}>
            <Button type='primary' htmlType='submit' loading={loading}>
              {t('保存尺寸设置')}
            </Button>
          </Row>
        </Form>
      </Spin>
    </Card>
  );
};

export default ImageSizeSetting;
