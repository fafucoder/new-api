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
} from '@douyinfe/semi-ui';
import {
  IconBell,
  IconClose,
  IconCreditCard,
  IconPriceTag,
  IconSave,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import { getCurrencyConfig } from '../../../../helpers/render';

const { Text, Title } = Typography;

const EditRuleModal = ({
  visible,
  form,
  setForm,
  tags = [],
  onCancel,
  onSubmit,
}) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);
  const [submitting, setSubmitting] = useState(false);
  const isEdit = !!form?.id;

  // 显示货币转换:USD <-> 当前显示币种(¥/$/自定义)
  const { symbol, rate } = getCurrencyConfig();
  const usdToDisplay = (usd) => {
    const v = Number(usd || 0);
    if (!Number.isFinite(v)) return 0;
    return Number((v * (rate || 1)).toFixed(4));
  };
  const displayToUsd = (display) => {
    const v = Number(display || 0);
    if (!Number.isFinite(v)) return 0;
    return Number((v / (rate || 1)).toFixed(6));
  };

  const tagOptions = tags.map((tg) => ({ value: tg, label: tg }));

  // 把外部 form state(USD)同步到内部 Semi Form(显示币种)
  useEffect(() => {
    if (visible && formApiRef.current) {
      formApiRef.current.setValues(
        {
          tag: form.tag || '',
          total_quota: usdToDisplay(form.total_quota),
          threshold: usdToDisplay(form.threshold ?? 5),
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
      await onSubmit({
        tag: values.tag || '',
        total_quota: displayToUsd(values.total_quota),
        threshold: displayToUsd(values.threshold),
        webhook_url: values.webhook_url || '',
        webhook_secret: values.webhook_secret || '',
        remark: values.remark || '',
        enabled: !!values.enabled,
      });
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
            {isEdit ? t('更新余额监控规则') : t('创建余额监控规则')}
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
            tag: form.tag || '',
            total_quota: usdToDisplay(form.total_quota),
            threshold: usdToDisplay(form.threshold ?? 5),
            webhook_url: form.webhook_url || '',
            webhook_secret: form.webhook_secret || '',
            remark: form.remark || '',
            enabled: form.enabled !== false,
          }}
          getFormApi={(api) => (formApiRef.current = api)}
          onSubmit={handleSubmit}
        >
          {() => (
            <div className='p-2'>
              {/* 基本信息 */}
              <Card className='!rounded-2xl shadow-sm border-0'>
                <div className='flex items-center mb-2'>
                  <Avatar size='small' color='blue' className='mr-2 shadow-md'>
                    <IconPriceTag size={16} />
                  </Avatar>
                  <div>
                    <Text className='text-lg font-medium'>
                      {t('基本信息')}
                    </Text>
                    <div className='text-xs text-gray-600'>
                      {t('设置规则所对应的上游标签与备注')}
                    </div>
                  </div>
                </div>
                <Row gutter={12}>
                  <Col span={24}>
                    <Form.Select
                      field='tag'
                      label={t('Tag (上游标签)')}
                      placeholder={t('从已有 Tag 选择,或输入新 Tag')}
                      optionList={tagOptions}
                      filter
                      allowCreate
                      showClear
                      disabled={isEdit}
                      rules={[
                        { required: true, message: t('请填写或选择 Tag') },
                      ]}
                      style={{ width: '100%' }}
                      extraText={t(
                        '对应 channels.tag 字段,同一上游账户的多个渠道请打同一个 Tag。规则创建后 Tag 不可改',
                      )}
                    />
                  </Col>
                  <Col span={24}>
                    <Form.Input
                      field='remark'
                      label={t('备注 (可选)')}
                      placeholder={t('例如:OpenRouter 主账户')}
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

              {/* 额度配置 */}
              <Card className='!rounded-2xl shadow-sm border-0 mt-2'>
                <div className='flex items-center mb-2'>
                  <Avatar size='small' color='green' className='mr-2 shadow-md'>
                    <IconCreditCard size={16} />
                  </Avatar>
                  <div>
                    <Text className='text-lg font-medium'>
                      {t('额度配置')}
                    </Text>
                    <div className='text-xs text-gray-600'>
                      {t('设置上游总额度与告警阈值,系统按"总额度 - 已用"判断')}
                    </div>
                  </div>
                </div>
                <Row gutter={12}>
                  <Col span={12}>
                    <Form.InputNumber
                      field='total_quota'
                      label={t('总额度') + ` (${symbol})`}
                      placeholder='100.00'
                      min={0}
                      step={0.01}
                      precision={4}
                      style={{ width: '100%' }}
                      rules={[
                        { required: true, message: t('请填写总额度') },
                      ]}
                      extraText={t(
                        '上游账户当前已充值的总额度。后续充值请用列表"充值"按钮累加',
                      )}
                    />
                  </Col>
                  <Col span={12}>
                    <Form.InputNumber
                      field='threshold'
                      label={t('告警阈值') + ` (${symbol})`}
                      placeholder='5.0'
                      min={0}
                      step={0.01}
                      precision={4}
                      style={{ width: '100%' }}
                      rules={[
                        { required: true, message: t('请填写阈值') },
                      ]}
                      extraText={t('剩余低于此值触发告警')}
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
                      {t('留空则使用 root 用户的默认通知方式')}
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
                      label={t('Webhook 签名密钥')}
                      placeholder={t('用于 HMAC-SHA256 校验')}
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
