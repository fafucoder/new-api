import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Toast } from '@douyinfe/semi-ui';
import {
  listProxies,
  deleteProxy,
  testProxy,
  updateProxy,
} from '../../services/proxy';

export const useProxiesData = () => {
  const { t } = useTranslation();

  const [proxies, setProxies] = useState([]);
  const [total, setTotal] = useState(0);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [keyword, setKeyword] = useState('');
  const [statusFilter, setStatusFilter] = useState(0);
  const [loading, setLoading] = useState(false);
  const [searching, setSearching] = useState(false);
  const [compactMode, setCompactMode] = useState(false);

  const [showEdit, setShowEdit] = useState(false);
  const [editingProxy, setEditingProxy] = useState(null);

  const [showReferences, setShowReferences] = useState(false);
  const [referencesProxy, setReferencesProxy] = useState(null);

  const [testingIds, setTestingIds] = useState({});

  const load = useCallback(
    async ({ page = activePage, size = pageSize } = {}) => {
      setLoading(true);
      try {
        const res = await listProxies({
          page,
          size,
          keyword,
          status: statusFilter,
        });
        if (res.data?.success) {
          setProxies(res.data.data?.items || []);
          setTotal(res.data.data?.total || 0);
        } else {
          Toast.error(res.data?.message || t('加载失败'));
        }
      } catch (e) {
        Toast.error(e?.message || t('加载失败'));
      } finally {
        setLoading(false);
        setSearching(false);
      }
    },
    [activePage, pageSize, keyword, statusFilter, t],
  );

  useEffect(() => {
    load({ page: activePage, size: pageSize });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activePage, pageSize]);

  const refresh = useCallback(() => load(), [load]);

  const searchProxies = useCallback(() => {
    setSearching(true);
    setActivePage(1);
    load({ page: 1, size: pageSize });
  }, [load, pageSize]);

  const handlePageChange = (page) => setActivePage(page);
  const handlePageSizeChange = (size) => {
    setPageSize(size);
    setActivePage(1);
  };

  const openEdit = (record = null) => {
    setEditingProxy(record);
    setShowEdit(true);
  };
  const closeEdit = () => setShowEdit(false);

  const openReferences = (record) => {
    setReferencesProxy(record);
    setShowReferences(true);
  };
  const closeReferences = () => setShowReferences(false);

  const handleTest = async (id) => {
    setTestingIds((prev) => ({ ...prev, [id]: true }));
    try {
      const res = await testProxy(id);
      if (res.data?.success) {
        const { ok, latency_ms, msg } = res.data.data || {};
        if (ok) {
          Toast.success(`${t('测试成功')} (${latency_ms}ms) ${msg}`);
        } else {
          Toast.error(`${t('测试失败')}: ${msg}`);
        }
        refresh();
      } else {
        Toast.error(res.data?.message || t('测试失败'));
      }
    } catch (e) {
      Toast.error(e?.message || t('测试失败'));
    } finally {
      setTestingIds((prev) => {
        const next = { ...prev };
        delete next[id];
        return next;
      });
    }
  };

  const handleDelete = async (record) => {
    try {
      const res = await deleteProxy(record.id);
      if (res.data?.success) {
        Toast.success(t('删除成功'));
        refresh();
      } else {
        const refs = res.data?.referenced_channels;
        if (Array.isArray(refs) && refs.length > 0) {
          const names = refs
            .slice(0, 5)
            .map((r) => `#${r.id} ${r.name}`)
            .join(', ');
          Toast.error(
            `${t('该代理仍被以下渠道引用')}: ${names}${
              refs.length > 5 ? ` ... (${refs.length})` : ''
            }`,
          );
        } else {
          Toast.error(res.data?.message || t('删除失败'));
        }
      }
    } catch (e) {
      Toast.error(e?.message || t('删除失败'));
    }
  };

  const handleToggleStatus = async (record, checked) => {
    try {
      const payload = { ...record, status: checked ? 1 : 2 };
      const res = await updateProxy(payload);
      if (res.data?.success) {
        Toast.success(t('已更新'));
        refresh();
      } else {
        Toast.error(res.data?.message || t('更新失败'));
      }
    } catch (e) {
      Toast.error(e?.message || t('更新失败'));
    }
  };

  return {
    t,
    proxies,
    total,
    activePage,
    pageSize,
    keyword,
    setKeyword,
    statusFilter,
    setStatusFilter,
    loading,
    searching,
    compactMode,
    setCompactMode,
    handlePageChange,
    handlePageSizeChange,
    searchProxies,
    refresh,
    showEdit,
    editingProxy,
    openEdit,
    closeEdit,
    showReferences,
    referencesProxy,
    openReferences,
    closeReferences,
    handleTest,
    handleDelete,
    handleToggleStatus,
    testingIds,
  };
};
