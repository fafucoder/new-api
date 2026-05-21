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

import React, { useRef, useState } from 'react';
import {
  Avatar,
  Button,
  Card,
  Descriptions,
  SideSheet,
  Space,
  Spin,
  Tag,
  Typography,
  Upload,
} from '@douyinfe/semi-ui';
import { IconClose, IconUpload } from '@douyinfe/semi-icons';
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

const IssueUploadModal = ({ visible, rule, submitting, onCancel, onSubmit }) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [file, setFile] = useState(null);

  const r = rule || {};

  const handleSubmit = () => {
    if (!file) return;
    onSubmit(file);
  };

  const handleCancel = () => {
    setFile(null);
    onCancel();
  };

  return (
    <SideSheet
      placement='right'
      title={
        <Space>
          <Tag color='green' shape='circle'>
            {t('上传')}
          </Tag>
          <Title heading={4} className='m-0'>
            {t('上传发票文件')}
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
              type='primary'
              className='!rounded-lg'
              onClick={handleSubmit}
              icon={<IconUpload />}
              loading={submitting}
              disabled={!file}
            >
              {t('确认上传开具')}
            </Button>
            <Button
              theme='light'
              className='!rounded-lg'
              type='primary'
              onClick={handleCancel}
              icon={<IconClose />}
            >
              {t('取消')}
            </Button>
          </Space>
        </div>
      }
      closeIcon={null}
      onCancel={handleCancel}
    >
      <Spin spinning={submitting}>
        <div className='p-2'>
          <Card className='!rounded-2xl shadow-sm border-0'>
            <div className='flex items-center mb-2'>
              <Avatar size='small' color='green' className='mr-2 shadow-md'>
                <IconUpload size={16} />
              </Avatar>
              <div>
                <Text className='text-lg font-medium'>{t('申请详情')}</Text>
                <div className='text-xs text-gray-600'>
                  {t('上传发票文件后将直接发送至用户邮箱')}
                </div>
              </div>
            </div>
            <Descriptions
              size='small'
              data={[
                {
                  key: t('用户'),
                  value: <Text strong>{`#${r.user_id} (ID: ${r.id})`}</Text>,
                },
                {
                  key: t('发票抬头'),
                  value: <Text strong>{r.title || '—'}</Text>,
                },
                {
                  key: t('申请人类型'),
                  value:
                    APPLICANT_TYPE_MAP(t)[r.applicant_type] || r.applicant_type,
                },
                {
                  key: t('发票类型'),
                  value:
                    INVOICE_TYPE_MAP(t)[r.invoice_type] || r.invoice_type,
                },
                {
                  key: t('接收邮箱'),
                  value: r.email || '—',
                },
                {
                  key: t('申请金额'),
                  value: (
                    <Text strong style={{ color: 'var(--semi-color-success)' }}>
                      {formatMoney(r.amount)}
                    </Text>
                  ),
                },
              ]}
            />
          </Card>

          <Card className='!rounded-2xl shadow-sm border-0 mt-2'>
            <Upload
              action=''
              beforeUpload={({ file }) => {
                setFile(file.fileInstance);
                return { autoRemove: false, shouldUpload: false };
              }}
              onRemove={() => setFile(null)}
              accept='.pdf,.jpg,.jpeg,.png,.gif,.webp'
              limit={1}
              draggable
            >
              <div className='flex flex-col items-center py-4'>
                <IconUpload size={24} className='text-gray-400 mb-2' />
                <Text type='tertiary'>{t('点击或拖拽上传发票文件')}</Text>
                <Text type='tertiary' size='small'>
                  {t('支持 PDF、图片格式，最大 10MB')}
                </Text>
              </div>
            </Upload>
          </Card>
        </div>
      </Spin>
    </SideSheet>
  );
};

export default IssueUploadModal;
