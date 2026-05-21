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

import React from 'react';
import {
  Button,
  Modal,
  Popover,
  Space,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  AlertTriangle,
  CheckCircle,
  Clock,
  Download,
  FileSignature,
  XCircle,
} from 'lucide-react';

const { Text, Paragraph } = Typography;

const formatTimestamp = (ts) => {
  if (!ts) return '—';
  const d = new Date(ts * 1000);
  if (isNaN(d.getTime())) return '—';
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

const formatMoney = (n) => {
  if (n === undefined || n === null || isNaN(n)) return '—';
  return `$${Number(n).toFixed(4)}`;
};

const STATUS_CONFIG = (t) => ({
  pending: { color: 'blue', text: t('待审核'), icon: <Clock size={12} /> },
  issuing: {
    color: 'orange',
    text: t('开票中'),
    icon: <FileSignature size={12} />,
  },
  issued: { color: 'green', text: t('已开具'), icon: <CheckCircle size={12} /> },
  rejected: {
    color: 'red',
    text: t('已拒绝'),
    icon: <AlertTriangle size={12} />,
  },
});

const APPLICANT_TYPE_MAP = (t) => ({
  personal: t('个人'),
  enterprise: t('企业'),
});

const INVOICE_TYPE_MAP = (t) => ({
  vat_normal: t('增值税普通发票'),
  vat_special: t('增值税专用发票'),
});

const renderStatus = (status, t) => {
  const cfg = STATUS_CONFIG(t)[status] || {
    color: 'grey',
    text: status || '—',
  };
  return (
    <Tag size='small' color={cfg.color} shape='circle' prefixIcon={cfg.icon}>
      {cfg.text}
    </Tag>
  );
};

const renderTitle = (text, record, t) => (
  <div>
    <Text strong>{text || '—'}</Text>
    {record.tax_id ? (
      <div>
        <Text type='tertiary' size='small'>
          {t('税号')}: {record.tax_id}
        </Text>
      </div>
    ) : null}
  </div>
);

const renderUser = (record) => (
  <div>
    <Text strong>{`#${record.user_id}`}</Text>
    <div>
      <Text type='tertiary' size='small'>
        {`ID: ${record.id}`}
      </Text>
    </div>
  </div>
);

const renderApplicant = (record, t) => (
  <Tag size='small' color='white' shape='circle'>
    {APPLICANT_TYPE_MAP(t)[record.applicant_type] || record.applicant_type}
  </Tag>
);

const renderInvoiceType = (record, t) => (
  <Tag size='small' color='white' shape='circle'>
    {INVOICE_TYPE_MAP(t)[record.invoice_type] || record.invoice_type}
  </Tag>
);

const renderAmount = (amount, record, t) => {
  const popoverContent = (
    <div className='text-xs p-2'>
      <Paragraph copyable={{ content: String(Number(amount || 0).toFixed(4)) }}>
        {t('申请金额')}: {formatMoney(amount)}
      </Paragraph>
      {record.email ? (
        <Paragraph copyable={{ content: record.email }}>
          {t('接收邮箱')}: {record.email}
        </Paragraph>
      ) : null}
      {record.reject_reason ? (
        <Paragraph copyable={{ content: record.reject_reason }}>
          {t('拒绝原因')}: {record.reject_reason}
        </Paragraph>
      ) : null}
    </div>
  );
  return (
    <Popover content={popoverContent} position='top'>
      <Tag color='white' shape='circle'>
        <Text strong size='small'>
          {formatMoney(amount)}
        </Text>
      </Tag>
    </Popover>
  );
};

const renderInvoiceNo = (no, record, t) => {
  if (!no) {
    return (
      <Text type='tertiary' size='small'>
        —
      </Text>
    );
  }
  return (
    <Tooltip content={no} position='top'>
      <Text
        ellipsis={{ showTooltip: false }}
        style={{ maxWidth: 160 }}
        size='small'
      >
        {no}
      </Text>
    </Tooltip>
  );
};

const renderOperations = (
  record,
  { openReject, handleIssue, operating, t },
) => {
  // Non-pending rows: show download PDF only if available
  if (record.status !== 'pending') {
    if (record.provider_pdf_url) {
      return (
        <Space wrap>
          <Button
            type='tertiary'
            size='small'
            icon={<Download size={14} />}
            onClick={() => window.open(record.provider_pdf_url, '_blank')}
          >
            {t('下载 PDF')}
          </Button>
        </Space>
      );
    }
    return (
      <Text type='tertiary' size='small'>
        —
      </Text>
    );
  }
  return (
    <Space wrap>
      <Button
        type='primary'
        size='small'
        icon={<CheckCircle size={14} />}
        loading={operating}
        onClick={() => {
          Modal.confirm({
            title: t('确认为该申请开具发票?'),
            content: t('此修改将不可逆'),
            onOk: () => handleIssue(record.id),
          });
        }}
      >
        {t('开具')}
      </Button>
      <Button
        type='danger'
        size='small'
        icon={<XCircle size={14} />}
        onClick={() => openReject(record)}
      >
        {t('拒绝')}
      </Button>
    </Space>
  );
};

export const getInvoiceAdminColumns = ({
  t,
  openReject,
  handleIssue,
  operating,
}) => {
  return [
    {
      title: t('用户'),
      dataIndex: 'user_id',
      render: (_text, record) => renderUser(record),
    },
    {
      title: t('发票抬头'),
      dataIndex: 'title',
      render: (text, record) => renderTitle(text, record, t),
    },
    {
      title: t('申请人类型'),
      dataIndex: 'applicant_type',
      render: (_text, record) => renderApplicant(record, t),
    },
    {
      title: t('发票类型'),
      dataIndex: 'invoice_type',
      render: (_text, record) => renderInvoiceType(record, t),
    },
    {
      title: t('申请金额'),
      dataIndex: 'amount',
      render: (amount, record) => renderAmount(amount, record, t),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (s) => renderStatus(s, t),
    },
    {
      title: t('发票号'),
      dataIndex: 'provider_invoice_no',
      render: (no, record) => renderInvoiceNo(no, record, t),
    },
    {
      title: t('申请时间'),
      dataIndex: 'applied_at',
      render: (v) => <Text size='small'>{formatTimestamp(v)}</Text>,
    },
    {
      title: '',
      dataIndex: 'operate',
      fixed: 'right',
      render: (_text, record) =>
        renderOperations(record, {
          openReject,
          handleIssue,
          operating,
          t,
        }),
    },
  ];
};
