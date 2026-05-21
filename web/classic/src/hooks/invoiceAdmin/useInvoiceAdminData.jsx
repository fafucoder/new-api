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

// 管理员发票审核数据钩子 — 与 useInvoiceData 同构,操作列接管 开具/拒绝。
import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { useTableCompactMode } from '../common/useTableCompactMode';

export const useInvoiceAdminData = () => {
  const { t } = useTranslation();

  const [invoices, setInvoices] = useState([]);
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [total, setTotal] = useState(0);

  // Filter form
  const [formApi, setFormApi] = useState(null);
  const [statusFilter, setStatusFilter] = useState('');
  const [userIdFilter, setUserIdFilter] = useState('');
  const [keyword, setKeyword] = useState('');
  const formInitValues = {
    status: '',
    user_id: '',
    searchKeyword: '',
  };

  // Reject SideSheet
  const [rejectingRule, setRejectingRule] = useState(null);
  const [rejectReason, setRejectReason] = useState('');
  const [rejectSubmitting, setRejectSubmitting] = useState(false);

  // Issue operation lock (per-row to avoid double-click)
  const [operating, setOperating] = useState(false);

  const [compactMode, setCompactMode] = useTableCompactMode('invoiceAdmin');

  const loadInvoices = useCallback(
    async (
      page = activePage,
      size = pageSize,
      status = statusFilter,
      userId = userIdFilter,
    ) => {
      setLoading(true);
      try {
        const params = { page, page_size: size };
        if (status) params.status = status;
        if (userId) params.user_id = userId;
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
    },
    [activePage, pageSize, statusFilter, userIdFilter],
  );

  const refresh = useCallback(async () => {
    await loadInvoices(activePage, pageSize, statusFilter, userIdFilter);
  }, [loadInvoices, activePage, pageSize, statusFilter, userIdFilter]);

  useEffect(() => {
    loadInvoices(1, pageSize, '', '');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const getFormValues = () => {
    const v = formApi ? formApi.getValues() : {};
    return {
      status: v.status || '',
      user_id: (v.user_id || '').toString().trim(),
      searchKeyword: v.searchKeyword || '',
    };
  };

  const searchInvoices = async (page = 1) => {
    setSearching(true);
    const { status, user_id, searchKeyword } = getFormValues();
    setStatusFilter(status || '');
    setUserIdFilter(user_id || '');
    setKeyword((searchKeyword || '').trim().toLowerCase());
    setActivePage(page);
    try {
      await loadInvoices(page, pageSize, status || '', user_id || '');
    } finally {
      setSearching(false);
    }
  };

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
    await loadInvoices(page, pageSize, statusFilter, userIdFilter);
  };

  const handlePageSizeChange = async (size) => {
    setPageSize(size);
    setActivePage(1);
    await loadInvoices(1, size, statusFilter, userIdFilter);
  };

  const handleIssue = async (id) => {
    setOperating(true);
    try {
      const res = await API.post(`/api/invoice/admin/${id}/issue`);
      const { success, message } = res.data || {};
      if (success) {
        showSuccess(t('开票成功'));
        await refresh();
      } else {
        showError(message || t('开票失败'));
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    } finally {
      setOperating(false);
    }
  };

  // Reject SideSheet
  const openReject = (rule) => {
    setRejectingRule(rule);
    setRejectReason('');
  };

  const closeReject = () => {
    setRejectingRule(null);
    setRejectReason('');
  };

  const submitReject = async (values) => {
    const reason = (values?.reason ?? rejectReason ?? '').trim();
    if (!rejectingRule) return;
    setRejectSubmitting(true);
    try {
      const res = await API.post(
        `/api/invoice/admin/${rejectingRule.id}/reject`,
        { reason },
      );
      const { success, message } = res.data || {};
      if (success) {
        showSuccess(t('已拒绝'));
        closeReject();
        await refresh();
      } else {
        showError(message || t('操作失败'));
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    } finally {
      setRejectSubmitting(false);
    }
  };

  return {
    t,
    invoices: filteredInvoices,
    totalCount: total,
    loading,
    searching,
    activePage,
    pageSize,
    handlePageChange,
    handlePageSizeChange,
    formInitValues,
    setFormApi,
    searchInvoices,
    compactMode,
    setCompactMode,
    refresh,
    // Operations
    operating,
    handleIssue,
    // Reject
    rejectingRule,
    rejectReason,
    setRejectReason,
    rejectSubmitting,
    openReject,
    closeReject,
    submitReject,
  };
};
