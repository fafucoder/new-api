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

import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Empty,
  Input,
  Modal,
  Select,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { FileText, RefreshCw, CheckCircle, XCircle } from 'lucide-react';
import { API, showError, showSuccess } from '../../helpers';
import CardTable from '../../components/common/ui/CardTable';
import CompactModeToggle from '../../components/common/ui/CompactModeToggle';
import { useTableCompactMode } from '../../hooks/common/useTableCompactMode';

const { Title, Text } = Typography;

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

const STATUS_COLORS = {
  pending: 'blue',
  issuing: 'yellow',
  issued: 'green',
  rejected: 'red',
};

const STATUS_MAP = {
  pending: '待审核',
  issuing: '开票中',
  issued: '已开具',
  rejected: '已拒绝',
};

const APPLICANT_TYPE_MAP = {
  personal: '个人',
  enterprise: '企业',
};

const INVOICE_TYPE_MAP = {
  vat_normal: '增值税普通发票',
  vat_special: '增值税专用发票',
};

const InvoiceAdminPage = () => {
  const { t } = useTranslation();
  const [compactMode, setCompactMode] = useTableCompactMode('invoiceAdmin');
  const [invoices, setInvoices] = useState([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [filterStatus, setFilterStatus] = useState('');
  const [filterUserID, setFilterUserID] = useState('');
  const [rejectVisible, setRejectVisible] = useState(false);
  const [rejectingID, setRejectingID] = useState(null);
  const [rejectReason, setRejectReason] = useState('');
  const [operating, setOperating] = useState(false);

  const loadInvoices = useCallback(async () => {
    setLoading(true);
    try {
      const params = { page, page_size: pageSize };
      if (filterStatus) {
        params.status = filterStatus;
      }
      if (filterUserID) {
        params.user_id = filterUserID;
      }
      const res = await API.get('/api/invoice/admin/list', { params });
      const { success, data, message } = res.data || {};
      if (success) {
        setInvoices(data?.items || []);
        setTotal(data?.total || 0);
      } else if (message) {
        showError(message);
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, filterStatus, filterUserID]);

  useEffect(() => {
    loadInvoices();
  }, [loadInvoices]);

  const handleIssue = async (id) => {
    setOperating(true);
    try {
      const res = await API.post(`/api/invoice/admin/${id}/issue`);
      const { success, message } = res.data || {};
      if (success) {
        showSuccess(t('开票成功'));
        await loadInvoices();
      } else {
        showError(message || t('开票失败'));
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    } finally {
      setOperating(false);
    }
  };

  const openRejectModal = (id) => {
    setRejectingID(id);
    setRejectReason('');
    setRejectVisible(true);
  };

  const handleReject = async () => {
    if (!rejectingID) return;
    setOperating(true);
    try {
      const res = await API.post(`/api/invoice/admin/${rejectingID}/reject`, {
        reason: rejectReason,
      });
      const { success, message } = res.data || {};
      if (success) {
        showSuccess(t('已拒绝'));
        setRejectVisible(false);
        await loadInvoices();
      } else {
        showError(message || t('操作失败'));
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    } finally {
      setOperating(false);
    }
  };

  const columns = [
    {
      title: t('ID'),
      dataIndex: 'id',
      width: 70,
    },
    {
      title: t('用户ID'),
      dataIndex: 'user_id',
      width: 80,
    },
    {
      title: t('申请时间'),
      dataIndex: 'applied_at',
      width: 160,
      render: (v) => formatTimestamp(v),
    },
    {
      title: t('申请人类型'),
      dataIndex: 'applicant_type',
      width: 100,
      render: (v) => APPLICANT_TYPE_MAP[v] || v,
    },
    {
      title: t('发票抬头'),
      dataIndex: 'title',
      width: 180,
      render: (v) => <Text ellipsis={{ showTooltip: true }}>{v}</Text>,
    },
    {
      title: t('税号'),
      dataIndex: 'tax_id',
      width: 120,
      render: (v) => v || '—',
    },
    {
      title: t('邮箱'),
      dataIndex: 'email',
      width: 180,
      render: (v) => <Text ellipsis={{ showTooltip: true }}>{v}</Text>,
    },
    {
      title: t('发票类型'),
      dataIndex: 'invoice_type',
      width: 140,
      render: (v) => INVOICE_TYPE_MAP[v] || v,
    },
    {
      title: t('申请金额'),
      dataIndex: 'amount',
      width: 120,
      render: (v) => formatMoney(v),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 100,
      render: (s) => (
        <Tag color={STATUS_COLORS[s] || 'grey'}>{t(STATUS_MAP[s] || s)}</Tag>
      ),
    },
    {
      title: t('发票号'),
      dataIndex: 'provider_invoice_no',
      width: 140,
      render: (v) => v || '—',
    },
    {
      title: t('PDF'),
      dataIndex: 'provider_pdf_url',
      width: 80,
      render: (url) => {
        if (!url) return '—';
        return (
          <a href={url} target='_blank' rel='noopener noreferrer'>
            {t('下载')}
          </a>
        );
      },
    },
    {
      title: t('拒绝原因'),
      dataIndex: 'reject_reason',
      width: 140,
      render: (v) => v || '—',
    },
    {
      title: t('操作'),
      width: 140,
      fixed: 'right',
      render: (_, record) => {
        if (record.status !== 'pending') {
          return <Text type='tertiary' size='small'>—</Text>;
        }
        return (
          <div style={{ display: 'flex', gap: 4 }}>
            <Button
              size='small'
              theme='borderless'
              type='success'
              icon={<CheckCircle size={14} />}
              onClick={() => handleIssue(record.id)}
              disabled={operating}
            >
              {t('开具')}
            </Button>
            <Button
              size='small'
              theme='borderless'
              type='danger'
              icon={<XCircle size={14} />}
              onClick={() => openRejectModal(record.id)}
              disabled={operating}
            >
              {t('拒绝')}
            </Button>
          </div>
        );
      },
    },
  ];

  return (
    <div
      className='mt-[60px] px-2 invoice-admin-page'
      style={{ width: '100%', overflow: 'hidden', boxSizing: 'border-box' }}
    >
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 24, marginBottom: 20 }}>
        <div style={{ flex: 1 }}>
          <Title heading={2} style={{ marginBottom: 8 }}>
            <FileText size={20} style={{ verticalAlign: '-3px', marginRight: 6 }} />
            {t('发票审核')}
          </Title>
          <Text type='secondary' style={{ fontSize: 13 }}>
            {t('审核用户发票申请并进行开具或拒绝操作')}
          </Text>
        </div>
        <Button
          icon={<RefreshCw size={14} />}
          onClick={() => loadInvoices()}
          loading={loading}
        >
          {t('立即刷新')}
        </Button>
      </div>

      <Card bordered style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'center' }}>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <Text type='tertiary' size='small'>{t('状态筛选')}</Text>
            <Select
              style={{ width: 120 }}
              value={filterStatus}
              onChange={(v) => { setFilterStatus(v || ''); setPage(1); }}
              placeholder={t('全部状态')}
              clear
            >
              <Select.Option value='pending'>{t('待审核')}</Select.Option>
              <Select.Option value='issuing'>{t('开票中')}</Select.Option>
              <Select.Option value='issued'>{t('已开具')}</Select.Option>
              <Select.Option value='rejected'>{t('已拒绝')}</Select.Option>
            </Select>
          </div>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <Text type='tertiary' size='small'>{t('用户ID')}</Text>
            <Input
              style={{ width: 100 }}
              value={filterUserID}
              onChange={(v) => { setFilterUserID(v); setPage(1); }}
              placeholder={t('输入用户ID')}
            />
          </div>
        </div>
      </Card>

      <Card bordered>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
          <Text strong>
            {t('发票申请列表')}
            <Text type='tertiary' size='small' style={{ marginLeft: 8 }}>
              ({total})
            </Text>
          </Text>
          <CompactModeToggle
            compactMode={compactMode}
            setCompactMode={setCompactMode}
            t={t}
          />
        </div>
        <CardTable
          columns={columns}
          dataSource={invoices}
          rowKey='id'
          pagination={{
            currentPage: page,
            pageSize: pageSize,
            total: total,
            showSizeChanger: true,
            pageSizeOptions: [10, 20, 50, 100],
            onPageSizeChange: (size) => { setPageSize(size); },
            onPageChange: (p) => { setPage(p); },
          }}
          loading={loading}
          empty={
            <Empty
              image={<FileText size={48} color='var(--semi-color-text-3)' />}
              title={t('暂无申请记录')}
              description={t('暂时没有用户的发票申请')}
            />
          }
          scroll={compactMode ? undefined : { x: 1600 }}
        />
      </Card>

      <Modal
        title={t('拒绝申请')}
        visible={rejectVisible}
        onCancel={() => setRejectVisible(false)}
        onOk={handleReject}
        okText={t('确认拒绝')}
        cancelText={t('取消')}
        width={400}
        confirmLoading={operating}
      >
        <Text type='tertiary' style={{ marginBottom: 12, display: 'block' }}>
          {t('请填写拒绝原因（可选）')}
        </Text>
        <Input
          style={{ width: '100%' }}
          value={rejectReason}
          onChange={(v) => setRejectReason(v)}
          placeholder={t('拒绝原因')}
        />
      </Modal>
    </div>
  );
};

export default InvoiceAdminPage;