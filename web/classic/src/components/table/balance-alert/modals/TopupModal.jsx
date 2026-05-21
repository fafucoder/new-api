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
  IconCreditCard,
  IconSave,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import {
  convertUSDToCurrency,
  getCurrencyConfig,
} from '../../../../helpers/render';

const { Text, Title } = Typography;

const formatBalance = (n) => {
  if (n === undefined || n === null || isNaN(n)) return '—';
  return convertUSDToCurrency(Number(n), 4);
};

const TopupModal = ({
  visible,
  rule,
  topupForm,
  setTopupForm,
  submitting,
  onCancel,
  onSubmit,
}) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);

  const { symbol, rate } = getCurrencyConfig();
  const usdToDisplay = (usd) => {
    const v = Number(usd || 0);
    if (!Number.isFinite(v)) return '';
    return Number((v * (rate || 1)).toFixed(4));
  };
  const displayToUsd = (display) => {
    const v = Number(display || 0);
    if (!Number.isFinite(v)) return 0;
    return Number((v / (rate || 1)).toFixed(6));
  };

  useEffect(() => {
    if (visible && formApiRef.current) {
      const initial =
        topupForm.amount !== '' && topupForm.amount != null
          ? usdToDisplay(topupForm.amount)
          : '';
      formApiRef.current.setValues(
        { amount: initial },
        { isOverride: true },
      );
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible, topupForm.ruleId]);

  if (!rule) return null;
  const totalQuota = Number(rule.total_quota) || 0;
  const used = Number(rule.summary?.used_usd) || 0;
  const remaining = Math.max(totalQuota - used, 0);

  const handleSubmit = async (values) => {
    await onSubmit({ amount: displayToUsd(values.amount) });
  };

  return (
    <SideSheet
      placement='right'
      title={
        <Space>
          <Tag color='violet' shape='circle'>
            {t('充值')}
          </Tag>
          <Title heading={4} className='m-0'>
            {t('为规则累加额度')}
          </Title>
        </Space>
      }
      bodyStyle={{ padding: '0' }}
      visible={visible}
      width={isMobile ? '100%' : 520}
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
              {t('确认充值')}
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
              <Avatar size='small' color='violet' className='mr-2 shadow-md'>
                <IconCreditCard size={16} />
              </Avatar>
              <div>
                <Text className='text-lg font-medium'>
                  {t('当前规则')}
                </Text>
                <div className='text-xs text-gray-600'>
                  {t('累加金额会自动加到总额度,并清除当前告警状态')}
                </div>
              </div>
            </div>
            <Descriptions
              size='small'
              data={[
                {
                  key: t('Tag'),
                  value: <Text strong>{rule.tag}</Text>,
                },
                {
                  key: t('总额度'),
                  value: <Text strong>{formatBalance(totalQuota)}</Text>,
                },
                {
                  key: t('已使用'),
                  value: (
                    <Text type='tertiary'>
                      {formatBalance(used)} (
                      {totalQuota > 0
                        ? ((used / totalQuota) * 100).toFixed(0)
                        : 0}
                      %)
                    </Text>
                  ),
                },
                {
                  key: t('当前剩余'),
                  value: <Text strong>{formatBalance(remaining)}</Text>,
                },
              ]}
            />
          </Card>

          <Card className='!rounded-2xl shadow-sm border-0 mt-2'>
            <Form
              key={`topup-${topupForm.ruleId}`}
              initValues={{
                amount:
                  topupForm.amount !== '' && topupForm.amount != null
                    ? usdToDisplay(topupForm.amount)
                    : '',
              }}
              getFormApi={(api) => (formApiRef.current = api)}
              onSubmit={handleSubmit}
            >
              <Row gutter={12}>
                <Col span={24}>
                  <Form.InputNumber
                    field='amount'
                    label={t('充值金额') + ` (${symbol})`}
                    placeholder='100.00'
                    min={0}
                    step={0.01}
                    precision={4}
                    style={{ width: '100%' }}
                    autoFocus
                    rules={[
                      { required: true, message: t('请输入有效的充值金额') },
                      {
                        validator: (_rule, value) =>
                          Number(value) > 0
                            ? Promise.resolve()
                            : Promise.reject(t('充值金额必须大于 0')),
                      },
                    ]}
                    extraText={t(
                      '示例:当前 {{cur}},充值 {{add}} 后总额度变为 {{total}}',
                      {
                        cur: formatBalance(100),
                        add: formatBalance(50),
                        total: formatBalance(150),
                      },
                    )}
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

export default TopupModal;
