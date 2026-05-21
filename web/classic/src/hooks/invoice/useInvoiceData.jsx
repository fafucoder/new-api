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

// 用户发票管理数据钩子 — 与 useBalanceAlertData 同构,只是分页改用 server-side。
import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { useTableCompactMode } from '../common/useTableCompactMode';

const initialApplyForm = () => ({
  applicant_type: 'personal',
  title: '',
  tax_id: '',
  email: '',
  invoice_type: 'vat_normal',
});

export const useInvoiceData = () => {
  const { t } = useTranslation();

  const [summary, setSummary] = useState(null);
  const [invoices, setInvoices] = useState([]);
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [total, setTotal] = useState(0);

  // Filter form (status select + keyword)
  const [formApi, setFormApi] = useState(null);
  const [statusFilter, setStatusFilter] = useState('');
  const [keyword, setKeyword] = useState('');
  const formInitValues = { searchKeyword: '', status: '' };

  // Apply SideSheet state
  const [applyVisible, setApplyVisible] = useState(false);
  const [applyForm, setApplyForm] = useState(initialApplyForm());
  const [applySubmitting, setApplySubmitting] = useState(false);

  const [compactMode, setCompactMode] = useTableCompactMode('invoice');

  const loadSummary = useCallback(async () => {
    try {
      const res = await API.get('/api/invoice/summary');
      const { success, data, message } = res.data || {};
      if (success) setSummary(data);
      else if (message) showError(message);
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    }
  }, []);

  const loadInvoices = useCallback(
    async (page = activePage, size = pageSize, status = statusFilter) => {
      setLoading(true);
      try {
        const params = { page, page_size: size };
        if (status) params.status = status;
        const res = await API.get('/api/invoice/list', { params });
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
    },
    [activePage, pageSize, statusFilter],
  );

  const refresh = useCallback(async () => {
    await loadSummary();
    await loadInvoices(activePage, pageSize, statusFilter);
  }, [loadSummary, loadInvoices, activePage, pageSize, statusFilter]);

  useEffect(() => {
    loadSummary();
    loadInvoices(1, pageSize, '');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const getFormValues = () => {
    const v = formApi ? formApi.getValues() : {};
    return {
      searchKeyword: v.searchKeyword || '',
      status: v.status || '',
    };
  };

  const searchInvoices = async (page = 1) => {
    setSearching(true);
    const { searchKeyword, status } = getFormValues();
    setKeyword((searchKeyword || '').trim().toLowerCase());
    setStatusFilter(status || '');
    setActivePage(page);
    try {
      await loadInvoices(page, pageSize, status || '');
    } finally {
      setSearching(false);
    }
  };

  // 关键字仅作前端模糊过滤(发票号/抬头/邮箱),状态走后端 filter。
  const filteredInvoices = (() => {
    if (!keyword) return invoices;
    return invoices.filter((r) => {
      const fields = [
        r.title,
        r.tax_id,
        r.email,
        r.provider_invoice_no,
      ];
      return fields.some(
        (f) => f && f.toString().toLowerCase().includes(keyword),
      );
    });
  })();

  const handlePageChange = async (page) => {
    setActivePage(page);
    await loadInvoices(page, pageSize, statusFilter);
  };

  const handlePageSizeChange = async (size) => {
    setPageSize(size);
    setActivePage(1);
    await loadInvoices(1, size, statusFilter);
  };

  // Apply SideSheet
  const openApply = () => {
    if (summary?.enabled === false) {
      showError(t('发票功能已关闭'));
      return;
    }
    if (summary?.has_in_flight) {
      showError(t('您有待处理的发票申请'));
      return;
    }
    if ((summary?.billable || 0) < (summary?.minimum_amount || 0)) {
      showError(t('可开票余额低于最低额度'));
      return;
    }
    setApplyForm(initialApplyForm());
    setApplyVisible(true);
  };

  const closeApply = () => setApplyVisible(false);

  const submitApply = async (values) => {
    const payload = values || applyForm;
    const title = (payload.title || '').trim();
    const email = (payload.email || '').trim();
    if (!title) {
      showError(t('请填写发票抬头'));
      return;
    }
    if (!email) {
      showError(t('请填写接收邮箱'));
      return;
    }
    if (payload.applicant_type === 'enterprise' && !(payload.tax_id || '').trim()) {
      showError(t('企业申请人请填写税号'));
      return;
    }
    setApplySubmitting(true);
    try {
      const res = await API.post('/api/invoice/apply', {
        applicant_type: payload.applicant_type,
        title,
        tax_id: (payload.tax_id || '').trim(),
        email,
        invoice_type: payload.invoice_type,
      });
      const { success, message } = res.data || {};
      if (success) {
        showSuccess(t('申请已提交'));
        closeApply();
        await refresh();
      } else {
        showError(message || t('申请失败'));
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    } finally {
      setApplySubmitting(false);
    }
  };

  const canApply =
    !!summary?.enabled &&
    !summary?.has_in_flight &&
    (summary?.billable || 0) >= (summary?.minimum_amount || 0);

  return {
    t,
    // Data
    summary,
    invoices: filteredInvoices,
    totalCount: total,
    loading,
    searching,
    activePage,
    pageSize,
    handlePageChange,
    handlePageSizeChange,
    // Filters
    formInitValues,
    setFormApi,
    searchInvoices,
    // Compact
    compactMode,
    setCompactMode,
    // Refresh
    refresh,
    // Apply
    canApply,
    openApply,
    applyVisible,
    applyForm,
    setApplyForm,
    applySubmitting,
    closeApply,
    submitApply,
  };
};
