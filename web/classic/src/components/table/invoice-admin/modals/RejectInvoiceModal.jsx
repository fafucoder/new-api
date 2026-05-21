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

import React, { useEffect, useRef } from 'react';
import {
  Avatar,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Row,
  SideSheet,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconClose,
  IconDelete,
  IconSave,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const formatMoney = (n) => {
  if (n === undefined || n === null || isNaN(n)) return '—';
  return `$${Number(n).toFixed(4)}`;
};

const APPLICANT_TYPE_MAP = (t) => ({
  personal: t('个人'),
  enterprise: t('企业'),
});

const INVOICE_TYPE_MAP = (t) => ({
  vat_normal: t('增值税普通发票'),
  vat_special: t('增值税专用发票'),
});

const RejectInvoiceModal = ({
  visible,
  rule,
  reason,
  setReason,
  submitting,
  onCancel,
  onSubmit,
}) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);

  useEffect(() => {
    if (visible && formApiRef.current) {
      formApiRef.current.setValues(
        { reason: reason || '' },
        { isOverride: true },
      );
    }
  }, [visible, rule?.id]);

  if (!rule) return null;

  const handleSubmit = async (values) => {
    setReason(values.reason || '');
    await onSubmit({ reason: values.reason || '' });
  };

  return (
    <SideSheet
      placement='right'
      title={
        <Space>
          <Tag color='red' shape='circle'>
            {t('拒绝')}
          </Tag>
          <Title heading={4} className='m-0'>
            {t('拒绝发票申请')}
          </Title>
        </Space>
      }
      bodyStyle={{ padding: '0' }}
      visible={visible}
      width={isMobile ? '100%' : 560}
      footer={
        <div className='flex justify-end bg-white'>
          <Space>
            <Button
              theme='solid'
              type='danger'
              className='!rounded-lg'
              onClick={() => formApiRef.current?.submitForm()}
              icon={<IconDelete />}
              loading={submitting}
            >
              {t('确认拒绝')}
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
        <div className='p-2'>
          <Card className='!rounded-2xl shadow-sm border-0'>
            <div className='flex items-center mb-2'>
              <Avatar size='small' color='red' className='mr-2 shadow-md'>
                <IconDelete size={16} />
              </Avatar>
              <div>
                <Text className='text-lg font-medium'>
                  {t('申请详情')}
                </Text>
                <div className='text-xs text-gray-600'>
                  {t('确认信息无误后,在下方填写拒绝原因')}
                </div>
              </div>
            </div>
            <Descriptions
              size='small'
              data={[
                {
                  key: t('用户'),
                  value: (
                    <Text strong>{`#${rule.user_id} (ID: ${rule.id})`}</Text>
                  ),
                },
                {
                  key: t('发票抬头'),
                  value: <Text strong>{rule.title || '—'}</Text>,
                },
                {
                  key: t('申请人类型'),
                  value:
                    APPLICANT_TYPE_MAP(t)[rule.applicant_type] ||
                    rule.applicant_type,
                },
                {
                  key: t('发票类型'),
                  value:
                    INVOICE_TYPE_MAP(t)[rule.invoice_type] || rule.invoice_type,
                },
                {
                  key: t('税号'),
                  value: rule.tax_id || '—',
                },
                {
                  key: t('接收邮箱'),
                  value: rule.email || '—',
                },
                {
                  key: t('申请金额'),
                  value: (
                    <Text
                      strong
                      style={{ color: 'var(--semi-color-success)' }}
                    >
                      {formatMoney(rule.amount)}
                    </Text>
                  ),
                },
              ]}
            />
          </Card>

          <Card className='!rounded-2xl shadow-sm border-0 mt-2'>
            <Form
              key={`reject-${rule.id}`}
              initValues={{ reason: reason || '' }}
              getFormApi={(api) => (formApiRef.current = api)}
              onSubmit={handleSubmit}
            >
              <Row gutter={12}>
                <Col span={24}>
                  <Form.TextArea
                    field='reason'
                    label={t('拒绝原因 (可选)')}
                    placeholder={t(
                      '简要说明拒绝原因,该原因将展示给用户',
                    )}
                    autosize={{ minRows: 3, maxRows: 6 }}
                    maxLength={200}
                    showClear
                  />
                </Col>
              </Row>
            </Form>
          </Card>
        </div>
      </Spin>
    </SideSheet>
  );
};

export default RejectInvoiceModal;
