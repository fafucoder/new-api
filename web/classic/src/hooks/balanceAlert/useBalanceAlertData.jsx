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

// 余额监控页的数据钩子 — 拟令牌管理(hooks/tokens/useTokensData)结构。
// 后端 /api/balance-alert/rules 一次性返回所有规则(数量级小,等于上游账户数),
// 所以搜索/分页都在前端做,保持 UI 与令牌页一致。
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { useTableCompactMode } from '../common/useTableCompactMode';

const initialRuleForm = () => ({
  id: 0,
  tag: '',
  total_quota: 0,
  threshold: 5,
  webhook_url: '',
  webhook_secret: '',
  remark: '',
  enabled: true,
});

const initialTopupForm = () => ({
  ruleId: 0,
  tag: '',
  amount: '',
});

export const useBalanceAlertData = () => {
  const { t } = useTranslation();

  // Server data
  const [allRules, setAllRules] = useState([]);
  const [tags, setTags] = useState([]);

  // List state
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);

  // Filter form
  const [formApi, setFormApi] = useState(null);
  const [keyword, setKeyword] = useState('');
  const formInitValues = { searchKeyword: '' };

  // Edit modal state
  const [showEdit, setShowEdit] = useState(false);
  const [form, setForm] = useState(initialRuleForm());

  // Topup modal state
  const [topupRule, setTopupRule] = useState(null);
  const [topupForm, setTopupForm] = useState(initialTopupForm());
  const [topupSubmitting, setTopupSubmitting] = useState(false);

  // UI state
  const [compactMode, setCompactMode] = useTableCompactMode('balance-alert');

  // Load all rules from API
  const loadRules = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/balance-alert/rules');
      const { success, data, message } = res.data || {};
      if (success) {
        setAllRules(Array.isArray(data) ? data : []);
      } else if (message) {
        showError(message);
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadTags = useCallback(async () => {
    try {
      const res = await API.get('/api/balance-alert/tags');
      const { success, data } = res.data || {};
      if (success) setTags(Array.isArray(data) ? data : []);
    } catch (_e) {
      // 静默失败:新建表单还能用 Input 手填 tag
    }
  }, []);

  const refresh = useCallback(async () => {
    await loadRules();
    await loadTags();
  }, [loadRules, loadTags]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  // Client-side search (rules count is small, tens at most)
  const getFormValues = () => {
    const values = formApi ? formApi.getValues() : {};
    return { searchKeyword: values.searchKeyword || '' };
  };

  const searchRules = (page = 1) => {
    setSearching(true);
    const { searchKeyword } = getFormValues();
    setKeyword((searchKeyword || '').trim().toLowerCase());
    setActivePage(page);
    setSearching(false);
  };

  const filteredRules = useMemo(() => {
    if (!keyword) return allRules;
    return allRules.filter((r) => {
      const tag = (r.tag || '').toLowerCase();
      const remark = (r.remark || '').toLowerCase();
      return tag.includes(keyword) || remark.includes(keyword);
    });
  }, [allRules, keyword]);

  const pagedRules = useMemo(() => {
    const start = (activePage - 1) * pageSize;
    return filteredRules.slice(start, start + pageSize);
  }, [filteredRules, activePage, pageSize]);

  const handlePageChange = (page) => setActivePage(page);
  const handlePageSizeChange = (size) => {
    setPageSize(size);
    setActivePage(1);
  };

  // Edit / Create
  const openCreate = () => {
    setForm(initialRuleForm());
    setShowEdit(true);
  };

  const openEdit = (rule) => {
    setForm({
      id: rule.id,
      tag: rule.tag || '',
      total_quota: rule.total_quota || 0,
      threshold: rule.threshold || 0,
      webhook_url: rule.webhook_url || '',
      webhook_secret: rule.webhook_secret || '',
      remark: rule.remark || '',
      enabled: rule.enabled !== false,
    });
    setShowEdit(true);
  };

  const closeEdit = () => setShowEdit(false);

  // values: SideSheet Form 提交后的最新值;不传则回退读 form state
  const submitForm = async (values) => {
    const source = values || form;
    const tagVal = (source.tag || '').toString().trim();
    if (!tagVal) {
      showError(t('请填写或选择 Tag'));
      return;
    }
    const threshold = Number(source.threshold);
    if (!Number.isFinite(threshold) || threshold < 0) {
      showError(t('阈值需为非负数'));
      return;
    }
    const totalQuota = Number(source.total_quota);
    if (!Number.isFinite(totalQuota) || totalQuota < 0) {
      showError(t('总额度需为非负数'));
      return;
    }
    try {
      const payload = {
        tag: tagVal,
        total_quota: totalQuota,
        threshold,
        webhook_url: source.webhook_url || '',
        webhook_secret: source.webhook_secret || '',
        remark: source.remark || '',
        enabled: source.enabled !== false,
      };
      const res = form.id
        ? await API.put(`/api/balance-alert/rules/${form.id}`, payload)
        : await API.post('/api/balance-alert/rules', payload);
      const { success, message } = res.data || {};
      if (success) {
        showSuccess(form.id ? t('已更新') : t('已创建'));
        closeEdit();
        await loadRules();
      } else {
        showError(message || t('保存失败'));
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    }
  };

  const handleDelete = async (id) => {
    try {
      const res = await API.delete(`/api/balance-alert/rules/${id}`);
      if (res.data?.success) {
        showSuccess(t('已删除'));
        await loadRules();
      } else {
        showError(res.data?.message);
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    }
  };

  const handleTest = async (id) => {
    try {
      const res = await API.post(`/api/balance-alert/rules/${id}/test`);
      const { success, message } = res.data || {};
      if (success) {
        showSuccess(message || t('已发送测试告警'));
      } else {
        showError(message || t('测试失败'));
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    }
  };

  const handleToggleEnabled = async (rule, enabled) => {
    try {
      const res = await API.put(`/api/balance-alert/rules/${rule.id}`, {
        tag: rule.tag,
        total_quota: rule.total_quota || 0,
        threshold: rule.threshold,
        webhook_url: rule.webhook_url || '',
        webhook_secret: rule.webhook_secret || '',
        remark: rule.remark || '',
        enabled,
      });
      if (res.data?.success) {
        await loadRules();
      } else {
        showError(res.data?.message);
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    }
  };

  // Topup
  const openTopup = (rule) => {
    setTopupRule(rule);
    setTopupForm({ ruleId: rule.id, tag: rule.tag, amount: '' });
  };

  const closeTopup = () => {
    setTopupRule(null);
    setTopupForm(initialTopupForm());
  };

  const submitTopup = async (values) => {
    const amountRaw = values?.amount ?? topupForm.amount;
    const amount = Number(amountRaw);
    if (!Number.isFinite(amount) || amount <= 0) {
      showError(t('请输入有效的充值金额'));
      return;
    }
    setTopupSubmitting(true);
    try {
      const res = await API.post(`/api/balance-alert/rules/${topupForm.ruleId}/topup`, { amount });
      const { success, message } = res.data || {};
      if (success) {
        showSuccess(t('充值成功'));
        closeTopup();
        await loadRules();
      } else {
        showError(message || t('充值失败'));
      }
    } catch (e) {
      showError(e?.response?.data?.message || e?.message);
    } finally {
      setTopupSubmitting(false);
    }
  };

  return {
    // i18n
    t,
    // Lists
    rules: pagedRules,
    totalCount: filteredRules.length,
    tags,
    loading,
    searching,
    activePage,
    pageSize,
    handlePageChange,
    handlePageSizeChange,
    // Filters
    formInitValues,
    setFormApi,
    searchRules,
    // Compact
    compactMode,
    setCompactMode,
    // Actions
    refresh,
    openCreate,
    openEdit,
    openTopup,
    handleDelete,
    handleTest,
    handleToggleEnabled,
    // Edit modal
    showEdit,
    form,
    setForm,
    closeEdit,
    submitForm,
    // Topup modal
    topupRule,
    topupForm,
    setTopupForm,
    topupSubmitting,
    closeTopup,
    submitTopup,
  };
};
