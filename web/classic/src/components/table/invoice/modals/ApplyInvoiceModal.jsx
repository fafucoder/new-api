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
  IconClose,
  IconMail,
  IconSave,
  IconUser,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const formatMoney = (n) => {
  if (n === undefined || n === null || isNaN(n)) return '—';
  return `$${Number(n).toFixed(4)}`;
};

const ApplyInvoiceModal = ({
  visible,
  applyForm,
  setApplyForm,
  summary,
  submitting,
  onCancel,
  onSubmit,
}) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);
  const [applicantType, setApplicantType] = useState(
    applyForm.applicant_type || 'personal',
  );

  useEffect(() => {
    if (visible && formApiRef.current) {
      formApiRef.current.setValues(
        {
          applicant_type: applyForm.applicant_type || 'personal',
          title: applyForm.title || '',
          tax_id: applyForm.tax_id || '',
          email: applyForm.email || '',
          invoice_type: applyForm.invoice_type || 'vat_normal',
        },
        { isOverride: true },
      );
      setApplicantType(applyForm.applicant_type || 'personal');
    }
  }, [visible]);

  const handleSubmit = async (values) => {
    setApplyForm(values);
    await onSubmit(values);
  };

  return (
    <SideSheet
      placement='left'
      title={
        <Space>
          <Tag color='green' shape='circle'>
            {t('新建')}
          </Tag>
          <Title heading={4} className='m-0'>
            {t('申请开具发票')}
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
              {t('提交申请')}
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
          key={visible ? 'apply-open' : 'apply-closed'}
          initValues={{
            applicant_type: applyForm.applicant_type || 'personal',
            title: applyForm.title || '',
            tax_id: applyForm.tax_id || '',
            email: applyForm.email || '',
            invoice_type: applyForm.invoice_type || 'vat_normal',
          }}
          getFormApi={(api) => (formApiRef.current = api)}
          onValueChange={(values) =>
            setApplicantType(values.applicant_type || 'personal')
          }
          onSubmit={handleSubmit}
        >
          {() => (
            <div className='p-2'>
              {/* 本次申请金额 */}
              {summary && (
                <Card className='!rounded-2xl shadow-sm border-0'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='green'
                      className='mr-2 shadow-md'
                    >
                      <IconMail size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('本次申请金额')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t(
                          '可开票余额 = 充值总额 - 已锁定金额。系统按当前可开票余额开具一张发票',
                        )}
                      </div>
                    </div>
                  </div>
                  <Title
                    heading={3}
                    style={{
                      color: 'var(--semi-color-success)',
                      margin: 0,
                    }}
                  >
                    {formatMoney(summary.billable)}
                  </Title>
                </Card>
              )}

              {/* 申请人信息 */}
              <Card className='!rounded-2xl shadow-sm border-0 mt-2'>
                <div className='flex items-center mb-2'>
                  <Avatar size='small' color='blue' className='mr-2 shadow-md'>
                    <IconUser size={16} />
                  </Avatar>
                  <div>
                    <Text className='text-lg font-medium'>
                      {t('申请人信息')}
                    </Text>
                    <div className='text-xs text-gray-600'>
                      {t('请准确填写,发票开具后无法修改')}
                    </div>
                  </div>
                </div>
                <Row gutter={12}>
                  <Col span={12}>
                    <Form.Select
                      field='applicant_type'
                      label={t('申请人类型')}
                      optionList={[
                        { value: 'personal', label: t('个人') },
                        { value: 'enterprise', label: t('企业') },
                      ]}
                      style={{ width: '100%' }}
                    />
                  </Col>
                  <Col span={12}>
                    <Form.Select
                      field='invoice_type'
                      label={t('发票类型')}
                      optionList={[
                        {
                          value: 'vat_normal',
                          label: t('增值税普通发票'),
                        },
                      ]}
                      style={{ width: '100%' }}
                    />
                  </Col>
                  <Col span={24}>
                    <Form.Input
                      field='title'
                      label={t('发票抬头')}
                      placeholder={
                        applicantType === 'personal'
                          ? t('请输入个人姓名')
                          : t('请输入企业名称')
                      }
                      rules={[
                        { required: true, message: t('请填写发票抬头') },
                      ]}
                      showClear
                    />
                  </Col>
                  <Col span={24}>
                    <Form.Input
                      field='tax_id'
                      label={t('税号')}
                      placeholder={
                        applicantType === 'personal'
                          ? t('个人无需填写')
                          : t('请输入统一税号/统一信用代码')
                      }
                      disabled={applicantType === 'personal'}
                      showClear
                    />
                  </Col>
                  <Col span={24}>
                    <Form.Input
                      field='email'
                      label={t('接收邮箱')}
                      placeholder={t('发票开具完成后将发送到此邮箱')}
                      rules={[
                        { required: true, message: t('请填写接收邮箱') },
                        {
                          type: 'email',
                          message: t('邮箱格式不正确'),
                        },
                      ]}
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

export default ApplyInvoiceModal;
