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
  Avatar,
  Button,
  Card,
  Col,
  Form,
  Row,
  SideSheet,
  Space,
  Spin,
  Tag,
  Typography,
  Select,
} from '@douyinfe/semi-ui';
import {
  IconBell,
  IconClose,
  IconSave,
  IconAlertTriangle,
  IconActivity,
} from '@douyinfe/semi-icons';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const EditRuleModal = ({
  visible,
  form,
  setForm,
  channels = [],
  tags = [],
  onCancel,
  onSubmit,
  t,
}) => {
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);
  const [submitting, setSubmitting] = useState(false);
  const isEdit = !!form?.id;

  const timeWindows = [
    { label: '5分钟', value: '5m' },
    { label: '15分钟', value: '15m' },
    { label: '30分钟', value: '30m' },
    { label: '1小时', value: '1h' },
    { label: '6小时', value: '6h' },
  ];

  const scopes = [
    { label: t('全局'), value: 'global' },
    { label: t('渠道'), value: 'channel' },
    { label: t('标签组'), value: 'tag' },
  ];

  const tagOptions = tags.map((tg) => ({ value: tg, label: tg }));

  // 同步外部 form state 到内部 Semi Form
  useEffect(() => {
    if (visible && formApiRef.current) {
      formApiRef.current.setValues(
        {
          name: form.name || '',
          scope: form.scope || 'global',
          scope_value: form.scope_value || '',
          time_window: form.time_window || '5m',
          threshold: form.threshold || 10,
          min_request_count: form.min_request_count || 0,
          webhook_url: form.webhook_url || '',
          webhook_secret: form.webhook_secret || '',
          remark: form.remark || '',
          enabled: form.enabled !== false,
        },
        { isOverride: true },
      );
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible, form.id]);

  const handleSubmit = async (values) => {
    setSubmitting(true);
    try {
      await onSubmit(values);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <SideSheet
      placement={isEdit ? 'right' : 'left'}
      title={
        <Space>
          {isEdit ? (
            <Tag color='blue' shape='circle'>
              {t('更新')}
            </Tag>
          ) : (
            <Tag color='green' shape='circle'>
              {t('新建')}
            </Tag>
          )}
          <Title heading={4} className='m-0'>
            {isEdit ? t('更新错误监控规则') : t('创建错误监控规则')}
          </Title>
        </Space>
      }
      bodyStyle={{ padding: '0' }}
      visible={visible}
      width={isMobile ? '100%' : 600}
      footer={
        <div className='flex justify-end bg-white'>
          <Space>
            <Button
              theme='solid'
              className='!rounded-lg'
              onClick={() => formApiRef.current?.submitForm()}
              icon={<IconSave />}
              loading={submitting}
            >
              {t('提交')}
            </Button>
            <Button
              theme='light'
              className='!rounded-lg'
              type='primary'
              onClick={onCancel}
              icon={<IconClose />}
            >
              {t('取消')}
            </Button>
          </Space>
        </div>
      }
      closeIcon={null}
      onCancel={onCancel}
    >
      <Spin spinning={submitting}>
        <Form
          key={isEdit ? `edit-${form.id}` : 'new'}
          initValues={{
            name: form.name || '',
            scope: form.scope || 'global',
            scope_value: form.scope_value || '',
            time_window: form.time_window || '5m',
            threshold: form.threshold || 10,
            min_request_count: form.min_request_count || 0,
            webhook_url: form.webhook_url || '',
            webhook_secret: form.webhook_secret || '',
            remark: form.remark || '',
            enabled: form.enabled !== false,
          }}
          getFormApi={(api) => (formApiRef.current = api)}
          onSubmit={handleSubmit}
        >
          {({ formState, formApi }) => (
            <div className='p-2'>
              {/* 基本信息 */}
              <Card className='!rounded-2xl shadow-sm border-0'>
                <div className='flex items-center mb-2'>
                  <Avatar size='small' color='red' className='mr-2 shadow-md'>
                    <IconAlertTriangle size={16} />
                  </Avatar>
                  <div>
                    <Text className='text-lg font-medium'>
                      {t('基本信息')}
                    </Text>
                    <div className='text-xs text-gray-600'>
                      {t('设置规则名称与监控范围')}
                    </div>
                  </div>
                </div>
                <Row gutter={12}>
                  <Col span={24}>
                    <Form.Input
                      field='name'
                      label={t('规则名称')}
                      placeholder={t('例如：全局错误监控')}
                      showClear
                      rules={[
                        { required: true, message: t('请输入规则名称') },
                      ]}
                      style={{ width: '100%' }}
                    />
                  </Col>
                  <Col span={24}>
                    <Form.Select
                      field='scope'
                      label={t('监控范围')}
                      optionList={scopes}
                      rules={[{ required: true }]}
                      style={{ width: '100%' }}
                      onChange={(value) => {
                        formApi.setValue('scope_value', '');
                      }}
                    />
                  </Col>
                  {formState.values.scope === 'channel' && (
                    <Col span={24}>
                      <Form.Select
                        field='scope_value'
                        label={t('选择渠道')}
                        placeholder={t('请选择渠道')}
                        filter
                        showClear
                        rules={[{ required: true, message: t('请选择渠道') }]}
                        style={{ width: '100%' }}
                      >
                        {channels.map((ch) => (
                          <Select.Option key={ch.id} value={String(ch.id)}>
                            #{ch.id} {ch.name}
                          </Select.Option>
                        ))}
                      </Form.Select>
                    </Col>
                  )}
                  {formState.values.scope === 'tag' && (
                    <Col span={24}>
                      <Form.Select
                        field='scope_value'
                        label={t('选择标签')}
                        placeholder={t('请选择标签')}
                        optionList={tagOptions}
                        filter
                        showClear
                        rules={[{ required: true, message: t('请选择标签') }]}
                        style={{ width: '100%' }}
                      />
                    </Col>
                  )}
                  <Col span={24}>
                    <Form.Input
                      field='remark'
                      label={t('备注 (可选)')}
                      placeholder={t('可选')}
                      showClear
                    />
                  </Col>
                  <Col span={24}>
                    <Form.Switch
                      field='enabled'
                      label={t('启用规则')}
                    />
                  </Col>
                </Row>
              </Card>

              {/* 阈值配置 */}
              <Card className='!rounded-2xl shadow-sm border-0 mt-2'>
                <div className='flex items-center mb-2'>
                  <Avatar size='small' color='green' className='mr-2 shadow-md'>
                    <IconActivity size={16} />
                  </Avatar>
                  <div>
                    <Text className='text-lg font-medium'>
                      {t('阈值配置')}
                    </Text>
                    <div className='text-xs text-gray-600'>
                      {t('设置时间窗口与错误率阈值')}
                    </div>
                  </div>
                </div>
                <Row gutter={12}>
                  <Col span={12}>
                    <Form.Select
                      field='time_window'
                      label={t('时间窗口')}
                      optionList={timeWindows}
                      rules={[{ required: true }]}
                      style={{ width: '100%' }}
                      extraText={t('统计最近 N 时间的错误率')}
                    />
                  </Col>
                  <Col span={12}>
                    <Form.InputNumber
                      field='threshold'
                      label={t('错误率阈值') + ' (%)'}
                      placeholder='10.0'
                      min={0}
                      max={100}
                      step={0.1}
                      precision={2}
                      style={{ width: '100%' }}
                      rules={[
                        { required: true, message: t('请输入阈值') },
                        {
                          type: 'number',
                          min: 0,
                          max: 100,
                          message: t('阈值范围: 0-100'),
                        },
                      ]}
                      extraText={t('超过此值触发告警')}
                    />
                  </Col>
                  <Col span={12}>
                    <Form.InputNumber
                      field='min_request_count'
                      label={t('最低请求数')}
                      placeholder='0'
                      min={0}
                      step={1}
                      precision={0}
                      style={{ width: '100%' }}
                      extraText={t('请求数低于此值不告警，0 表示不限制')}
                    />
                  </Col>
                </Row>
              </Card>

              {/* Webhook 配置 */}
              <Card className='!rounded-2xl shadow-sm border-0 mt-2'>
                <div className='flex items-center mb-2'>
                  <Avatar
                    size='small'
                    color='orange'
                    className='mr-2 shadow-md'
                  >
                    <IconBell size={16} />
                  </Avatar>
                  <div>
                    <Text className='text-lg font-medium'>
                      {t('通知配置 (可选)')}
                    </Text>
                    <div className='text-xs text-gray-600'>
                      {t('留空则使用系统默认 Webhook')}
                    </div>
                  </div>
                </div>
                <Row gutter={12}>
                  <Col span={24}>
                    <Form.Input
                      field='webhook_url'
                      label={t('Webhook URL')}
                      placeholder='https://your-webhook.example/path'
                      showClear
                    />
                  </Col>
                  <Col span={24}>
                    <Form.Input
                      field='webhook_secret'
                      label={t('Webhook Secret')}
                      placeholder={t('可选')}
                      showClear
                    />
                  </Col>
                </Row>
              </Card>
            </div>
          )}
        </Form>
      </Spin>
    </SideSheet>
  );
};

export default EditRuleModal;
