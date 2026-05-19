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
  Form,
  Input,
  Modal,
  Select,
  Table,
  Tag,
  Typography,
  Tooltip,
} from '@douyinfe/semi-ui';
import { FileText, Plus, RefreshCw } from 'lucide-react';
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

const InvoicePage = () => {
  const { t } = useTranslation();
  const [compactMode, setCompactMode] = useTableCompactMode('invoices');
  const [summary, setSummary] = useState(null);
  const [invoices, setInvoices] = useState([]);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [applyVisible, setApplyVisible] = useState(false);
  const [form, setForm] = useState({
    applicant_type: 'personal',
    title: '',
    tax_id: '',
    email: '',
    invoice_type: 'vat_normal',
  });

  const loadSummary = useCallback(async () => {
    try {
      const res = await API.get('/api/invoice/summary');
      const { success, data, message } = res.data || {};
      if (success) {
        setSummary(data);
      } else if (message) {
        showError(message);
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    }
  }, []);

  const loadInvoices = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/invoice/list', {
        params: { page, page_size: pageSize },
      });
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
  }, [page, pageSize]);

  useEffect(() => {
    loadSummary();
    loadInvoices();
  }, [loadSummary, loadInvoices]);

  const handleApply = async () => {
    if (!form.title?.trim()) {
      showError(t('请填写发票抬头'));
      return;
    }
    if (!form.email?.trim()) {
      showError(t('请填写接收邮箱'));
      return;
    }
    if (form.applicant_type === 'enterprise' && !form.tax_id?.trim()) {
      showError(t('企业申请人请填写税号'));
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.post('/api/invoice/apply', {
        applicant_type: form.applicant_type,
        title: form.title.trim(),
        tax_id: form.tax_id?.trim() || '',
        email: form.email.trim(),
        invoice_type: form.invoice_type,
      });
      const { success, message } = res.data || {};
      if (success) {
        showSuccess(t('申请已提交'));
        setApplyVisible(false);
        setForm({
          applicant_type: 'personal',
          title: '',
          tax_id: '',
          email: '',
          invoice_type: 'vat_normal',
        });
        await loadSummary();
        await loadInvoices();
      } else {
        showError(message || t('申请失败'));
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    } finally {
      setSubmitting(false);
    }
  };

  const openApplyModal = () => {
    if (summary?.enabled === false) {
      showError(t('发票功能已关闭'));
      return;
    }
    if (summary?.has_in_flight) {
      showError(t('您有待处理的发票申请'));
      return;
    }
    if (summary?.billable < summary?.minimum_amount) {
      showError(t('可开票余额低于最低额度'));
      return;
    }
    setApplyVisible(true);
  };

  const canApply = summary?.enabled && !summary?.has_in_fflight && summary?.billable >= summary?.minimum_amount;

  const columns = [
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
      width: 100,
      render: (url) => {
        if (!url) return '—';
        return (
          <a href={url} target='_blank' rel='noopener noreferrer'>
            {t('下载')}
          </a>
        );
      },
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <div className='invoice-page'>
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 24, marginBottom: 20 }}>
          <div style={{ flex: 1 }}>
            <Title heading={2} style={{ marginBottom: 8 }}>
              <FileText size={20} style={{ verticalAlign: '-3px', marginRight: 6 }} />
              {t('发票管理')}
            </Title>
            <Text type='secondary' style={{ fontSize: 13 }}>
            {t('申请开具增值税普通发票或专用发票')}
          </Text>
        </div>
        <Button
          icon={<RefreshCw size={14} />}
          onClick={() => { loadSummary(); loadInvoices(); }}
          loading={loading}
        >
          {t('立即刷新')}
        </Button>
      </div>

      {summary && (
        <Card bordered style={{ marginBottom: 16 }}>
          <div style={{ display: 'flex', gap: 32, flexWrap: 'wrap' }}>
            <div>
              <Text type='tertiary' size='small'>{t('充值总额')}</Text>
              <div><Text strong>{formatMoney(summary.topup_total)}</Text></div>
            </div>
            <div>
              <Text type='tertiary' size='small'>{t('已锁定金额')}</Text>
              <div><Text strong>{formatMoney(summary.invoiced_total)}</Text></div>
            </div>
            <div>
              <Text type='tertiary' size='small'>{t('可开票余额')}</Text>
              <div><Text strong style={{ color: 'var(--semi-color-success)' }}>{formatMoney(summary.billable)}</Text></div>
            </div>
            <div>
              <Text type='tertiary' size='small'>{t('最低开票额度')}</Text>
              <div><Text strong>{formatMoney(summary.minimum_amount)}</Text></div>
            </div>
          </div>
          <div style={{ marginTop: 16, display: 'flex', gap: 8 }}>
            <Tooltip content={!summary.enabled ? t('发票功能已关闭') : summary.has_in_flight ? t('有待处理申请') : summary.billable < summary.minimum_amount ? t('余额低于最低额度') : ''}>
              <span>
                <Button
                  type='primary'
                  icon={<Plus size={14} />}
                  onClick={openApplyModal}
                  disabled={!canApply}
                >
                  {t('申请开票')}
                </Button>
              </span>
            </Tooltip>
          </div>
        </Card>
      )}

      <Card bordered>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
          <Text strong>
            {t('申请记录')}
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
              description={t('点击上方"申请开票"开始')}
            />
          }
          scroll={compactMode ? undefined : { x: 1200 }}
        />
      </Card>

      <Modal
        title={t('申请开票')}
        visible={applyVisible}
        onCancel={() => setApplyVisible(false)}
        onOk={handleApply}
        okText={t('提交申请')}
        cancelText={t('取消')}
        width={520}
        confirmLoading={submitting}
      >
        {summary && (
          <div style={{ marginBottom: 16, padding: 12, background: 'var(--semi-fill-grey-1)', borderRadius: 4 }}>
            <Text type='tertiary' size='small'>{t('本次申请金额')}</Text>
            <div><Text strong size='large' style={{ color: 'var(--semi-color-success)' }}>{formatMoney(summary.billable)}</Text></div>
            <Text type='tertiary' size='small' style={{ fontSize: 11 }}>
              {t('(可开票余额 = 充值总额 - 已锁定金额)')}
            </Text>
          </div>
        )}
        <Form labelPosition='top'>
          <Form.Slot label={t('申请人类型')}>
            <Select
              style={{ width: '100%' }}
              value={form.applicant_type}
              onChange={(v) => setForm((f) => ({ ...f, applicant_type: v }))}
            >
              <Select.Option value='personal'>{t('个人')}</Select.Option>
              <Select.Option value='enterprise'>{t('企业')}</Select.Option>
            </Select>
          </Form.Slot>
          <Form.Slot label={t('发票抬头')} required>
            <Input
              style={{ width: '100%' }}
              value={form.title}
              onChange={(v) => setForm((f) => ({ ...f, title: v }))}
              placeholder={form.applicant_type === 'personal' ? t('请输入个人姓名') : t('请输入企业名称')}
            />
          </Form.Slot>
          <Form.Slot label={t('税号')}>
            <Input
              style={{ width: '100%' }}
              value={form.tax_id}
              onChange={(v) => setForm((f) => ({ ...f, tax_id: v }))}
              placeholder={form.applicant_type === 'personal' ? t('个人无需填写') : t('请输入纳税人识别号')}
              disabled={form.applicant_type === 'personal'}
            />
          </Form.Slot>
          <Form.Slot label={t('接收邮箱')} required>
            <Input
              style={{ width: '100%' }}
              type='email'
              value={form.email}
              onChange={(v) => setForm((f) => ({ ...f, email: v }))}
              placeholder={t('发票开具完成后将发送到此邮箱')}
            />
          </Form.Slot>
          <Form.Slot label={t('发票类型')}>
            <Select
              style={{ width: '100%' }}
              value={form.invoice_type}
              onChange={(v) => setForm((f) => ({ ...f, invoice_type: v }))}
            >
              <Select.Option value='vat_normal'>{t('增值税普通发票')}</Select.Option>
              {form.applicant_type === 'enterprise' && (
                <Select.Option value='vat_special'>{t('增值税专用发票')}</Select.Option>
              )}
            </Select>
          </Form.Slot>
        </Form>
      </Modal>

      <style>{`
        .invoice-page { padding: 24px; }
      `}</style>
      </div>
    </div>
  );
};

export default InvoicePage;