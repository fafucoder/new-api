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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Dropdown,
  Empty,
  Input,
  Modal,
  Pagination,
  Select,
  SideSheet,
  Space,
  Spin,
  Tabs,
  TabPane,
  Tag,
  TextArea,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  ChevronRight,
  CloudUpload,
  Copy,
  ExternalLink,
  FileAudio,
  FileVideo,
  Folder,
  FolderPlus,
  Image as ImageIcon,
  LayoutGrid,
  Link as LinkIcon,
  MoreHorizontal,
  Pencil,
  Plus,
  RefreshCw,
  ScanFace,
  Search,
  Settings,
  Trash2,
  Upload,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, copy, isAdmin, showError, showSuccess, showWarning } from '../../helpers';

const { Title, Text } = Typography;

const GROUP_TYPES = [
  { key: 'AIGC', labelKey: '虚拟素材', icon: LayoutGrid },
  { key: 'LivenessFace', labelKey: '真人素材', icon: ScanFace, disabled: true },
];

const PAGE_SIZE = 12;

const UPSTREAM_FORMATS = [
  { value: 'volcengine', label: '火山引擎 (query-action)' },
  { value: 'openai', label: 'OpenAI Files API' },
];

const statusColor = (status) => {
  if (status === 'Active') return 'green';
  if (status === 'Failed') return 'red';
  return 'orange';
};

const statusLabel = (t, status) => {
  switch (status) {
    case 'Active':
      return t('可用');
    case 'Failed':
      return t('失败');
    case 'Processing':
      return t('处理中');
    default:
      return status ? t(status) : t('处理中');
  }
};

const assetTypeLabel = (t, type) => {
  switch (type) {
    case 'Image':
      return t('图片');
    case 'Video':
      return t('视频');
    case 'Audio':
      return t('音频');
    default:
      return type || '-';
  }
};

const formatBytes = (bytes) => {
  if (!bytes || bytes <= 0) return '-';
  const units = ['B', 'KB', 'MB', 'GB'];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(value >= 10 || unit === 0 ? 0 : 1)} ${units[unit]}`;
};

const formatTime = (seconds) => {
  if (!seconds) return '-';
  const d = new Date(seconds * 1000);
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}/${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
};

const assetPrimaryURL = (asset) =>
  asset?.mappings?.find((mapping) => mapping.asset_url)?.asset_url ||
  asset?.source_url ||
  '';

// Aggregate a single display status across an asset's upstream mappings.
const assetDisplayStatus = (asset) => {
  const mappings = asset?.mappings || [];
  if (mappings.length === 0) return 'Processing';
  if (mappings.some((m) => m.status === 'Failed')) return 'Failed';
  if (mappings.some((m) => m.status === 'Processing')) return 'Processing';
  if (mappings.every((m) => m.status === 'Active')) return 'Active';
  return 'Processing';
};

const AssetThumb = ({ asset, size = 'card' }) => {
  const url = assetPrimaryURL(asset);
  const processing = assetDisplayStatus(asset) === 'Processing';
  const boxClass =
    size === 'card'
      ? 'flex aspect-square w-full items-center justify-center rounded-t-lg'
      : 'flex aspect-video w-full items-center justify-center rounded-lg';
  if (url && asset.asset_type === 'Image') {
    return (
      <img
        src={url}
        alt={asset.name}
        className={
          size === 'card'
            ? 'aspect-square w-full rounded-t-lg object-cover'
            : 'aspect-video w-full rounded-lg object-cover'
        }
      />
    );
  }
  if (url && asset.asset_type === 'Video') {
    return (
      <video
        src={url}
        muted
        preload='metadata'
        className={
          size === 'card'
            ? 'aspect-square w-full rounded-t-lg object-cover'
            : 'aspect-video w-full rounded-lg object-cover'
        }
      />
    );
  }
  const Icon =
    asset.asset_type === 'Audio'
      ? FileAudio
      : asset.asset_type === 'Video'
        ? FileVideo
        : ImageIcon;
  return (
    <div
      className={boxClass}
      style={{ backgroundColor: 'var(--semi-color-fill-0)' }}
    >
      <div
        className='flex flex-col items-center gap-2'
        style={{ color: 'var(--semi-color-text-2)' }}
      >
        <Icon size={size === 'card' ? 32 : 40} />
        {processing && (
          <Text type='tertiary' size='small'>
            {size === 'card' ? '' : ''}
          </Text>
        )}
      </div>
    </div>
  );
};

const emptyUpstreamForm = () => ({
  id: null,
  name: '',
  format: 'volcengine',
  base_url: '',
  api_key: '',
  enabled: true,
  version: '2024-01-01',
  project_name: '',
  create_group_action: 'CreateAssetGroup',
  get_group_action: 'GetAssetGroup',
  update_group_action: 'UpdateAssetGroup',
  delete_group_action: 'DeleteAssetGroup',
  list_groups_action: 'ListAssetGroups',
  create_asset_action: 'CreateAsset',
  get_asset_action: 'GetAsset',
  update_asset_action: 'UpdateAsset',
  delete_asset_action: 'DeleteAsset',
  purpose: 'user_data',
});

const AssetLibrary = () => {
  const { t } = useTranslation();
  const admin = isAdmin();

  const [loading, setLoading] = useState(true);
  const [groups, setGroups] = useState([]);
  const [upstreams, setUpstreams] = useState([]);
  const [activeType, setActiveType] = useState('AIGC');
  const [groupSearch, setGroupSearch] = useState('');
  const [selectedGroupId, setSelectedGroupId] = useState(null);

  const [assetSearch, setAssetSearch] = useState('');
  const [assetTypeFilter, setAssetTypeFilter] = useState('all');
  const [assetStatusFilter, setAssetStatusFilter] = useState('all');
  const [assetSort, setAssetSort] = useState('created_desc');
  const [page, setPage] = useState(1);

  const [uploadTab, setUploadTab] = useState('url');
  const [urlValue, setUrlValue] = useState('');
  const [dragOver, setDragOver] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const fileInputRef = useRef(null);

  const [createVisible, setCreateVisible] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createDescription, setCreateDescription] = useState('');
  const [editingGroup, setEditingGroup] = useState(null);
  const [editingName, setEditingName] = useState('');
  const [editingDescription, setEditingDescription] = useState('');

  const [detailAssetId, setDetailAssetId] = useState(null);
  const [editingAsset, setEditingAsset] = useState(null);
  const [editingAssetName, setEditingAssetName] = useState('');

  // Upstream admin config
  const [upstreamSheetVisible, setUpstreamSheetVisible] = useState(false);
  const [upstreamForm, setUpstreamForm] = useState(emptyUpstreamForm());
  const [upstreamEditing, setUpstreamEditing] = useState(false);
  const [upstreamSaving, setUpstreamSaving] = useState(false);
  const [upstreamList, setUpstreamList] = useState([]);

  const loadData = async (keepSelection = true) => {
    setLoading(true);
    try {
      const [groupsResponse, channelsResponse] = await Promise.all([
        API.get('/api/asset-library/groups'),
        API.get('/api/asset-library/channels'),
      ]);
      const nextGroups = groupsResponse.data.data || [];
      setGroups(nextGroups);
      setUpstreams(channelsResponse.data.data || []);
      if (!keepSelection || !nextGroups.some((g) => g.id === selectedGroupId)) {
        const firstOfType = nextGroups.find(
          (g) => (g.group_type || 'AIGC') === activeType,
        );
        setSelectedGroupId(firstOfType ? firstOfType.id : null);
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const visibleGroups = useMemo(() => {
    const keyword = groupSearch.trim().toLowerCase();
    return groups
      .filter((group) => (group.group_type || 'AIGC') === activeType)
      .filter(
        (group) =>
          !keyword ||
          (group.display_name || '').toLowerCase().includes(keyword),
      );
  }, [groups, activeType, groupSearch]);

  const selectedGroup = useMemo(
    () => groups.find((group) => group.id === selectedGroupId) || null,
    [groups, selectedGroupId],
  );

  // Derive the detail asset from the latest groups so the drawer stays in sync
  // after a rename/refresh instead of showing a stale snapshot.
  const detailAsset = useMemo(() => {
    if (detailAssetId == null) return null;
    for (const group of groups) {
      const found = (group.assets || []).find((a) => a.id === detailAssetId);
      if (found) return found;
    }
    return null;
  }, [groups, detailAssetId]);

  const filteredAssets = useMemo(() => {
    if (!selectedGroup) return [];
    const keyword = assetSearch.trim().toLowerCase();
    let list = (selectedGroup.assets || []).filter((asset) => {
      if (keyword && !(asset.name || '').toLowerCase().includes(keyword)) {
        return false;
      }
      if (assetTypeFilter !== 'all' && asset.asset_type !== assetTypeFilter) {
        return false;
      }
      if (
        assetStatusFilter !== 'all' &&
        assetDisplayStatus(asset) !== assetStatusFilter
      ) {
        return false;
      }
      return true;
    });
    list = [...list].sort((a, b) => {
      switch (assetSort) {
        case 'created_asc':
          return (a.created_time || 0) - (b.created_time || 0);
        case 'name_asc':
          return (a.name || '').localeCompare(b.name || '');
        case 'created_desc':
        default:
          return (b.created_time || 0) - (a.created_time || 0);
      }
    });
    return list;
  }, [selectedGroup, assetSearch, assetTypeFilter, assetStatusFilter, assetSort]);

  const pagedAssets = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE;
    return filteredAssets.slice(start, start + PAGE_SIZE);
  }, [filteredAssets, page]);

  useEffect(() => {
    setPage(1);
  }, [selectedGroupId, assetSearch, assetTypeFilter, assetStatusFilter, assetSort]);

  const hasFailedChannel = (response) =>
    response?.data?.data?.results?.some((result) => !result.success) === true;

  const createGroup = async () => {
    const name = createName.trim();
    if (!name) return;
    setSubmitting(true);
    try {
      const response = await API.post('/api/asset-library/groups', {
        display_name: name,
        group_type: activeType,
        description: createDescription.trim(),
      });
      if (hasFailedChannel(response)) {
        showWarning(t('部分素材同步失败'));
      } else {
        showSuccess(t('素材组已创建'));
      }
      setCreateVisible(false);
      setCreateName('');
      setCreateDescription('');
      const created = response?.data?.data?.group;
      await loadData(true);
      if (created?.id) setSelectedGroupId(created.id);
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const renameGroup = async () => {
    if (!editingGroup || !editingName.trim()) return;
    setSubmitting(true);
    try {
      const response = await API.patch(
        `/api/asset-library/groups/${editingGroup.id}`,
        {
          display_name: editingName.trim(),
          description: editingDescription.trim(),
        },
      );
      if (hasFailedChannel(response)) {
        showWarning(t('部分素材同步失败'));
      } else {
        showSuccess(t('素材组已更新'));
      }
      setEditingGroup(null);
      setEditingName('');
      setEditingDescription('');
      await loadData(true);
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const renameAsset = async () => {
    if (!editingAsset || !editingAssetName.trim()) return;
    setSubmitting(true);
    try {
      const response = await API.patch(
        `/api/asset-library/groups/${editingAsset.group_id}/assets/${editingAsset.id}`,
        { name: editingAssetName.trim() },
      );
      if (hasFailedChannel(response)) {
        showWarning(t('部分素材同步失败'));
      } else {
        showSuccess(t('素材已更新'));
      }
      setEditingAsset(null);
      setEditingAssetName('');
      await loadData(true);
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const deleteGroupRequest = async (group, force) => {
    await API.delete(
      `/api/asset-library/groups/${group.id}${force ? '?force=true' : ''}`,
    );
    showSuccess(t('素材组已删除'));
    if (selectedGroupId === group.id) setSelectedGroupId(null);
    await loadData(false);
  };

  const confirmDeleteGroup = (group) => {
    if ((group.assets?.length || 0) > 0) {
      showWarning(t('素材组中还有素材，请先删除全部素材后再删除素材组'));
      return;
    }
    Modal.confirm({
      title: t('确认删除'),
      content: t('此操作将永久删除该素材组，删除后不可恢复'),
      onOk: async () => {
        try {
          await deleteGroupRequest(group, false);
        } catch (error) {
          Modal.confirm({
            title: t('删除失败'),
            content: t('删除时发生错误，是否强制删除？强制删除会忽略错误并直接移除该素材组。'),
            okType: 'danger',
            okText: t('强制删除'),
            onOk: async () => {
              try {
                await deleteGroupRequest(group, true);
              } catch (forceError) {
                showError(forceError);
              }
            },
          });
        }
      },
    });
  };

  const uploadFiles = async (fileList) => {
    const files = Array.from(fileList || []).slice(0, 20);
    if (!selectedGroup || files.length === 0) return;
    const form = new FormData();
    files.forEach((file) => form.append('files', file));
    setSubmitting(true);
    try {
      const response = await API.post(
        `/api/asset-library/groups/${selectedGroup.id}/assets`,
        form,
      );
      if (hasFailedChannel(response)) {
        showWarning(t('部分素材同步失败'));
      } else {
        showSuccess(t('素材已上传'));
      }
      if (fileInputRef.current) fileInputRef.current.value = '';
      await loadData(true);
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const uploadURLs = async () => {
    if (!selectedGroup) return;
    const urls = urlValue
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);
    if (urls.length === 0) return;
    setSubmitting(true);
    try {
      const response = await API.post(
        `/api/asset-library/groups/${selectedGroup.id}/assets`,
        { assets: urls.map((url) => ({ url })) },
      );
      if (hasFailedChannel(response)) {
        showWarning(t('部分素材同步失败'));
      } else {
        showSuccess(t('素材已上传'));
      }
      setUrlValue('');
      await loadData(true);
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const refreshGroup = async () => {
    if (!selectedGroup) return;
    try {
      const response = await API.post(
        `/api/asset-library/groups/${selectedGroup.id}/refresh`,
      );
      if (hasFailedChannel(response)) {
        showWarning(t('部分素材同步失败'));
      } else {
        showSuccess(t('素材状态已刷新'));
      }
      await loadData(true);
    } catch (error) {
      showError(error);
    }
  };

  const deleteAssetRequest = async (asset, force) => {
    await API.delete(
      `/api/asset-library/groups/${selectedGroup.id}/assets/${asset.id}${
        force ? '?force=true' : ''
      }`,
    );
    showSuccess(t('素材已删除'));
    if (detailAssetId === asset.id) setDetailAssetId(null);
    await loadData(true);
  };

  const confirmDeleteAsset = (asset) => {
    Modal.confirm({
      title: t('确认删除'),
      content: t('此操作将永久删除该素材，删除后不可恢复'),
      onOk: async () => {
        try {
          await deleteAssetRequest(asset, false);
        } catch (error) {
          // Upstream failure (e.g. 502): offer a force delete that removes the
          // local record regardless of upstream errors.
          Modal.confirm({
            title: t('删除失败'),
            content: t('删除时发生错误，是否强制删除？强制删除会忽略错误并直接移除该素材。'),
            okType: 'danger',
            okText: t('强制删除'),
            onOk: async () => {
              try {
                await deleteAssetRequest(asset, true);
              } catch (forceError) {
                showError(forceError);
              }
            },
          });
        }
      },
    });
  };

  const handleDrop = (event) => {
    event.preventDefault();
    setDragOver(false);
    if (event.dataTransfer?.files?.length) {
      uploadFiles(event.dataTransfer.files);
    }
  };

  // ---- upstream admin ----
  const openUpstreamSheet = async () => {
    setUpstreamSheetVisible(true);
    try {
      const response = await API.get('/api/asset-library/upstreams');
      setUpstreamList(response.data.data || []);
    } catch (error) {
      showError(error);
    }
  };

  const editUpstream = (upstream) => {
    // Merge over defaults, but let empty values (e.g. action fields added after
    // this upstream was created) fall back to their defaults instead of showing
    // blank inputs.
    const defaults = emptyUpstreamForm();
    const merged = { ...defaults };
    Object.keys(upstream || {}).forEach((key) => {
      const value = upstream[key];
      if (value !== null && value !== undefined && value !== '') {
        merged[key] = value;
      }
    });
    merged.id = upstream.id;
    merged.enabled = upstream.enabled;
    merged.api_key = '';
    setUpstreamForm(merged);
    setUpstreamEditing(true);
  };

  const newUpstream = () => {
    setUpstreamForm(emptyUpstreamForm());
    setUpstreamEditing(true);
  };

  const saveUpstream = async () => {
    if (!upstreamForm.name.trim() || !upstreamForm.base_url.trim()) {
      showWarning(t('请填写名称与基础地址'));
      return;
    }
    setUpstreamSaving(true);
    try {
      if (upstreamForm.id) {
        await API.put(
          `/api/asset-library/upstreams/${upstreamForm.id}`,
          upstreamForm,
        );
      } else {
        await API.post('/api/asset-library/upstreams', upstreamForm);
      }
      showSuccess(t('上游已保存'));
      setUpstreamEditing(false);
      const response = await API.get('/api/asset-library/upstreams');
      setUpstreamList(response.data.data || []);
      await loadData(true);
    } catch (error) {
      showError(error);
    } finally {
      setUpstreamSaving(false);
    }
  };

  const deleteUpstream = (upstream) => {
    Modal.confirm({
      title: t('确认删除'),
      content: t('删除上游后，新素材将不再同步到该上游'),
      onOk: async () => {
        try {
          await API.delete(`/api/asset-library/upstreams/${upstream.id}`);
          showSuccess(t('上游已删除'));
          const response = await API.get('/api/asset-library/upstreams');
          setUpstreamList(response.data.data || []);
          await loadData(true);
        } catch (error) {
          showError(error);
        }
      },
    });
  };

  const setForm = (key, value) =>
    setUpstreamForm((prev) => ({ ...prev, [key]: value }));

  const totalPages = Math.max(1, Math.ceil(filteredAssets.length / PAGE_SIZE));

  // Colors come from Semi Design CSS variables so the page matches the theme
  // (the project's tailwind config only exposes semi-color-* tokens, not the
  // default gray/blue palette).
  const cardStyle = {
    borderRadius: 12,
    border: '1px solid var(--semi-color-border)',
    background: 'var(--semi-color-bg-1)',
  };
  const borderColor = 'var(--semi-color-border)';
  const activeBg = 'var(--semi-color-primary-light-default)';

  return (
    <div
      className='mt-[60px] px-4 flex flex-col overflow-hidden'
      style={{ height: 'calc(100vh - 110px)' }}
    >
      {/* Header */}
      <div className='mb-4 flex items-start justify-between shrink-0'>
        <div>
          <Title heading={3}>{t('素材管理')}</Title>
          <Text type='tertiary'>
            {t('集中管理与复用图片、视频、音频及真人认证素材')}
          </Text>
        </div>
        {admin && (
          <Button
            theme='light'
            type='tertiary'
            icon={<Settings size={16} />}
            onClick={openUpstreamSheet}
          >
            {t('上游配置')}
          </Button>
        )}
      </div>

      <div
        className='flex flex-1 overflow-hidden'
        style={{ ...cardStyle, minHeight: 0 }}
      >
        {/* Left: group list */}
        <div
          className='flex w-[240px] shrink-0 flex-col'
          style={{ borderRight: `1px solid ${borderColor}`, minHeight: 0 }}
        >
          <div
            className='flex items-center justify-between px-3 py-2.5'
            style={{ borderBottom: `1px solid ${borderColor}` }}
          >
            <Text strong>{t('素材组')}</Text>
            <Button
              theme='light'
              type='primary'
              size='small'
              icon={<FolderPlus size={14} />}
              disabled={upstreams.length === 0}
              onClick={() => {
                setCreateName('');
                setCreateVisible(true);
              }}
            >
              {t('新建组')}
            </Button>
          </div>

          <div className='px-2 pt-1'>
            <Tabs
              type='line'
              size='small'
              activeKey={activeType}
              onChange={(key) => {
                const target = GROUP_TYPES.find((tp) => tp.key === key);
                if (target?.disabled) {
                  showWarning(t('开发中'));
                  return;
                }
                setActiveType(key);
                const first = groups.find(
                  (g) => (g.group_type || 'AIGC') === key,
                );
                setSelectedGroupId(first ? first.id : null);
              }}
            >
              {GROUP_TYPES.map((type) => {
                const Icon = type.icon;
                return (
                  <TabPane
                    key={type.key}
                    itemKey={type.key}
                    disabled={type.disabled}
                    tab={
                      type.disabled ? (
                        <Tooltip content={t('开发中')}>
                          <span className='flex items-center gap-1.5'>
                            <Icon size={15} />
                            {t(type.labelKey)}
                          </span>
                        </Tooltip>
                      ) : (
                        <span className='flex items-center gap-1.5'>
                          <Icon size={15} />
                          {t(type.labelKey)}
                        </span>
                      )
                    }
                  />
                );
              })}
            </Tabs>
          </div>

          <div className='px-3 py-2'>
            <Input
              prefix={<Search size={15} style={{ marginLeft: 6 }} />}
              placeholder={t('搜索素材组')}
              value={groupSearch}
              onChange={setGroupSearch}
              showClear
            />
          </div>

          <div className='flex-1 overflow-y-auto px-2 pb-3'>
            {visibleGroups.length === 0 ? (
              <div className='px-2 py-8 text-center'>
                <Text type='tertiary' size='small'>
                  {t('暂无素材组')}
                </Text>
              </div>
            ) : (
              visibleGroups.map((group) => {
                const active = group.id === selectedGroupId;
                return (
                  <button
                    key={group.id}
                    type='button'
                    onClick={() => setSelectedGroupId(group.id)}
                    className='mb-1 flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left transition-colors'
                    style={{
                      background: active ? activeBg : 'transparent',
                      border: 'none',
                      cursor: 'pointer',
                    }}
                  >
                    <Folder
                      size={18}
                      style={{
                        color: active
                          ? 'var(--semi-color-primary)'
                          : 'var(--semi-color-text-2)',
                      }}
                    />
                    <div className='min-w-0 flex-1'>
                      <div
                        className='truncate text-sm'
                        style={{
                          fontWeight: active ? 600 : 400,
                          color: active
                            ? 'var(--semi-color-primary)'
                            : 'var(--semi-color-text-0)',
                        }}
                      >
                        {group.display_name}
                      </div>
                      <div
                        className='text-xs'
                        style={{ color: 'var(--semi-color-text-2)' }}
                      >
                        {t('{{count}} 个素材', {
                          count: group.assets?.length || 0,
                        })}
                      </div>
                    </div>
                    {active && (
                      <ChevronRight
                        size={16}
                        style={{ color: 'var(--semi-color-primary)' }}
                      />
                    )}
                  </button>
                );
              })
            )}
          </div>
        </div>

        {/* Right: group detail */}
        <div
          className='relative flex flex-1 flex-col overflow-hidden'
          style={{ minWidth: 0 }}
        >
          {loading && (
            <div
              className='absolute inset-0 z-10 flex items-center justify-center'
              style={{ background: 'var(--semi-color-bg-1)', opacity: 0.6 }}
            >
              <Spin spinning />
            </div>
          )}
          {!selectedGroup ? (
            <div className='flex flex-1 items-center justify-center'>
              <Empty
                description={
                  upstreams.length === 0
                    ? admin
                      ? t('尚未配置上游，请点击右上角「上游配置」')
                      : t('素材功能尚未就绪，请联系管理员')
                    : t('请选择或新建一个素材组')
                }
              />
            </div>
          ) : (
              <div className='flex flex-1 flex-col overflow-hidden' style={{ minHeight: 0 }}>
                <div className='flex-1 overflow-y-auto p-4' style={{ minHeight: 0 }}>
                <div className='mb-4 flex items-start justify-between'>
                  <div className='min-w-0'>
                    <Title heading={5} className='truncate'>
                      {selectedGroup.display_name}
                    </Title>
                    <Text type='tertiary' size='small'>
                      {selectedGroup.description || t('暂无描述')}
                    </Text>
                  </div>
                  <Space>
                    <Tooltip content={t('重命名')}>
                      <Button
                        type='tertiary'
                        theme='borderless'
                        icon={<Pencil size={16} />}
                        onClick={() => {
                          setEditingGroup(selectedGroup);
                          setEditingName(selectedGroup.display_name);
                          setEditingDescription(selectedGroup.description || '');
                        }}
                      />
                    </Tooltip>
                    <Tooltip content={t('删除')}>
                      <Button
                        type='danger'
                        theme='borderless'
                        icon={<Trash2 size={16} />}
                        onClick={() => confirmDeleteGroup(selectedGroup)}
                      />
                    </Tooltip>
                  </Space>
                </div>

                {/* Filter row */}
                <div className='mb-3 flex flex-wrap items-center gap-2'>
                  <Input
                    prefix={<Search size={15} style={{ marginLeft: 6 }} />}
                    placeholder={t('搜索当前组素材')}
                    value={assetSearch}
                    onChange={setAssetSearch}
                    showClear
                    className='min-w-[200px] flex-1'
                  />
                  <Select
                    value={assetTypeFilter}
                    onChange={setAssetTypeFilter}
                    style={{ width: 120 }}
                    optionList={[
                      { label: t('全部类型'), value: 'all' },
                      { label: t('图片'), value: 'Image' },
                      { label: t('视频'), value: 'Video' },
                      { label: t('音频'), value: 'Audio' },
                    ]}
                  />
                  <Select
                    value={assetStatusFilter}
                    onChange={setAssetStatusFilter}
                    style={{ width: 120 }}
                    optionList={[
                      { label: t('全部状态'), value: 'all' },
                      { label: t('可用'), value: 'Active' },
                      { label: t('处理中'), value: 'Processing' },
                      { label: t('失败'), value: 'Failed' },
                    ]}
                  />
                  <Select
                    value={assetSort}
                    onChange={setAssetSort}
                    style={{ width: 130 }}
                    optionList={[
                      { label: t('最近创建'), value: 'created_desc' },
                      { label: t('最早创建'), value: 'created_asc' },
                      { label: t('名称'), value: 'name_asc' },
                    ]}
                  />
                  <Tooltip content={t('刷新状态')}>
                    <Button
                      type='tertiary'
                      icon={<RefreshCw size={16} />}
                      onClick={refreshGroup}
                    />
                  </Tooltip>
                </div>

                {/* Upload source tabs */}
                <div className='mb-3 flex gap-2'>
                  {[
                    { key: 'local', icon: Upload, label: '本地文件', disabled: true },
                    { key: 'url', icon: LinkIcon, label: 'Public URL' },
                  ].map((tab) => {
                    const active = uploadTab === tab.key;
                    const TabIcon = tab.icon;
                    const btn = (
                      <button
                        key={tab.key}
                        type='button'
                        disabled={tab.disabled}
                        onClick={() => {
                          if (tab.disabled) {
                            showWarning(t('开发中'));
                            return;
                          }
                          setUploadTab(tab.key);
                        }}
                        className='flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm'
                        style={{
                          border: 'none',
                          cursor: tab.disabled ? 'not-allowed' : 'pointer',
                          fontWeight: active ? 600 : 400,
                          background: active ? activeBg : 'transparent',
                          opacity: tab.disabled ? 0.5 : 1,
                          color: active
                            ? 'var(--semi-color-primary)'
                            : 'var(--semi-color-text-1)',
                        }}
                      >
                        <TabIcon size={14} />
                        {t(tab.label)}
                      </button>
                    );
                    return tab.disabled ? (
                      <Tooltip key={tab.key} content={t('开发中')}>
                        {btn}
                      </Tooltip>
                    ) : (
                      btn
                    );
                  })}
                </div>

                {uploadTab === 'local' ? (
                  <div
                    onClick={() => fileInputRef.current?.click()}
                    onDragOver={(e) => {
                      e.preventDefault();
                      setDragOver(true);
                    }}
                    onDragLeave={() => setDragOver(false)}
                    onDrop={handleDrop}
                    className='flex cursor-pointer items-center gap-3 rounded-lg px-4 py-3 transition-colors'
                    style={{
                      border: `1px dashed ${
                        dragOver ? 'var(--semi-color-primary)' : borderColor
                      }`,
                      background: dragOver
                        ? 'var(--semi-color-primary-light-default)'
                        : 'var(--semi-color-fill-0)',
                    }}
                  >
                    <CloudUpload
                      size={22}
                      className='shrink-0'
                      style={{ color: 'var(--semi-color-primary)' }}
                    />
                    <div className='min-w-0'>
                      <Text strong>{t('拖拽文件到这里上传')}</Text>
                      <Text type='tertiary' size='small' className='ml-2'>
                        {t(
                          '.jpg、.jpeg、.png、.webp、.mp4、.mov、.mp3、.wav · 单个不超过 200 MiB · 每次 1 个',
                        )}
                      </Text>
                    </div>
                    <input
                      ref={fileInputRef}
                      type='file'
                      multiple
                      accept='image/*,video/*,audio/*'
                      className='hidden'
                      onChange={(event) => uploadFiles(event.target.files)}
                    />
                  </div>
                ) : (
                  <div
                    className='rounded-lg p-3'
                    style={{
                      border: `1px solid ${borderColor}`,
                      background: 'var(--semi-color-fill-0)',
                    }}
                  >
                    <TextArea
                      value={urlValue}
                      onChange={setUrlValue}
                      autosize={{ minRows: 2, maxRows: 6 }}
                      placeholder={t(
                        '每行一个公共可访问 URL，例如 https://example.com/logo.png',
                      )}
                    />
                    <div className='mt-2 flex justify-end'>
                      <Button
                        theme='solid'
                        type='primary'
                        size='small'
                        icon={<Plus size={14} />}
                        loading={submitting}
                        disabled={!urlValue.trim()}
                        onClick={uploadURLs}
                      >
                        {t('通过 URL 上传')}
                      </Button>
                    </div>
                  </div>
                )}

                {/* Asset grid */}
                <div className='mt-4'>
                  {filteredAssets.length === 0 ? (
                    <div className='py-12 text-center'>
                      <Text type='tertiary'>
                        {t('暂无素材，可拖拽文件或通过 Public URL 上传')}
                      </Text>
                    </div>
                  ) : (
                    <>
                      <div className='grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4'>
                        {pagedAssets.map((asset) => {
                          const status = assetDisplayStatus(asset);
                          return (
                            <div
                              key={asset.id}
                              className='group relative cursor-pointer overflow-hidden rounded-lg transition-shadow hover:shadow-md'
                              style={{
                                border: `1px solid ${borderColor}`,
                                background: 'var(--semi-color-bg-1)',
                              }}
                              onClick={() => setDetailAssetId(asset.id)}
                            >
                              <AssetThumb asset={asset} />
                              <div
                                className='absolute right-2 top-2 flex items-center gap-1'
                                onClick={(e) => e.stopPropagation()}
                              >
                                {assetPrimaryURL(asset) && (
                                  <Tooltip content={t('复制地址')}>
                                    <span
                                      className='flex h-7 w-7 cursor-pointer items-center justify-center rounded-md shadow-sm'
                                      style={{
                                        background: 'var(--semi-color-bg-2)',
                                        color: 'var(--semi-color-text-1)',
                                      }}
                                      onClick={async () => {
                                        const ok = await copy(
                                          assetPrimaryURL(asset),
                                        );
                                        if (ok !== false)
                                          showSuccess(t('已复制素材地址'));
                                      }}
                                    >
                                      <Copy size={15} />
                                    </span>
                                  </Tooltip>
                                )}
                                <Dropdown
                                  trigger='click'
                                  position='bottomRight'
                                  render={
                                    <Dropdown.Menu>
                                      <Dropdown.Item
                                        onClick={() =>
                                          setDetailAssetId(asset.id)
                                        }
                                      >
                                        {t('查看详情')}
                                      </Dropdown.Item>
                                      <Dropdown.Item
                                        onClick={() => {
                                          setEditingAsset(asset);
                                          setEditingAssetName(asset.name);
                                        }}
                                      >
                                        {t('重命名')}
                                      </Dropdown.Item>
                                      {assetPrimaryURL(asset) && (
                                        <Dropdown.Item
                                          onClick={() =>
                                            window.open(
                                              assetPrimaryURL(asset),
                                              '_blank',
                                            )
                                          }
                                        >
                                          {t('打开素材')}
                                        </Dropdown.Item>
                                      )}
                                      <Dropdown.Item
                                        type='danger'
                                        onClick={() => confirmDeleteAsset(asset)}
                                      >
                                        {t('删除')}
                                      </Dropdown.Item>
                                    </Dropdown.Menu>
                                  }
                                >
                                  <span
                                    className='flex h-7 w-7 items-center justify-center rounded-md shadow-sm'
                                    style={{
                                      background: 'var(--semi-color-primary)',
                                      color: 'var(--semi-color-white)',
                                    }}
                                  >
                                    <MoreHorizontal size={16} />
                                  </span>
                                </Dropdown>
                              </div>
                              <div className='p-2'>
                                <div className='truncate text-sm font-medium'>
                                  {asset.name}
                                </div>
                                <div className='mt-1 flex items-center gap-1.5'>
                                  <Tag size='small' color={statusColor(status)}>
                                    {statusLabel(t, status)}
                                  </Tag>
                                  <Text type='tertiary' size='small'>
                                    {assetTypeLabel(t, asset.asset_type)}
                                  </Text>
                                  {asset.file_size > 0 && (
                                    <Text type='tertiary' size='small'>
                                      · {formatBytes(asset.file_size)}
                                    </Text>
                                  )}
                                </div>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </>
                  )}
                </div>
                </div>

                {/* Footer pagination (pinned to bottom) */}
                <div
                  className='flex items-center justify-between px-4 py-3'
                  style={{ borderTop: `1px solid ${borderColor}` }}
                >
                  <Text type='tertiary' size='small'>
                    {t('共 {{count}} 个素材，第 {{page}}/{{total}} 页', {
                      count: filteredAssets.length,
                      page,
                      total: totalPages,
                    })}
                  </Text>
                  {totalPages > 1 && (
                    <Pagination
                      total={filteredAssets.length}
                      pageSize={PAGE_SIZE}
                      currentPage={page}
                      onPageChange={setPage}
                    />
                  )}
                </div>
              </div>
            )}
        </div>
      </div>

      {/* Asset detail SideSheet */}
      <SideSheet
        title={t('素材详情')}
        visible={detailAsset !== null}
        onCancel={() => setDetailAssetId(null)}
        width={420}
      >
        {detailAsset && (
          <div className='space-y-4'>
            {assetPrimaryURL(detailAsset) &&
            detailAsset.asset_type === 'Image' ? (
              <img
                src={assetPrimaryURL(detailAsset)}
                alt={detailAsset.name}
                className='w-full rounded-lg object-contain'
              />
            ) : assetPrimaryURL(detailAsset) &&
              detailAsset.asset_type === 'Video' ? (
              <video
                src={assetPrimaryURL(detailAsset)}
                controls
                className='w-full rounded-lg'
              />
            ) : (
              <div
                className='flex aspect-video w-full flex-col items-center justify-center gap-2 rounded-lg'
                style={{
                  background: 'var(--semi-color-fill-0)',
                  color: 'var(--semi-color-text-2)',
                }}
              >
                <ImageIcon size={40} />
                <Text type='tertiary' size='small'>
                  {t('素材正在处理中')}
                </Text>
              </div>
            )}

            <div className='flex items-center justify-between'>
              <div className='min-w-0'>
                <div className='truncate text-base font-medium'>
                  {detailAsset.name}
                </div>
                <Tag
                  className='mt-1'
                  color={statusColor(assetDisplayStatus(detailAsset))}
                >
                  {statusLabel(t, assetDisplayStatus(detailAsset))}
                </Tag>
              </div>
              <Space>
                <Tooltip content={t('重命名')}>
                  <Button
                    type='tertiary'
                    theme='borderless'
                    icon={<Pencil size={16} />}
                    onClick={() => {
                      setEditingAsset(detailAsset);
                      setEditingAssetName(detailAsset.name);
                    }}
                  />
                </Tooltip>
                <Tooltip content={t('删除')}>
                  <Button
                    type='danger'
                    theme='borderless'
                    icon={<Trash2 size={16} />}
                    onClick={() => confirmDeleteAsset(detailAsset)}
                  />
                </Tooltip>
              </Space>
            </div>

            <div className='grid grid-cols-2 gap-y-3 text-sm'>
              <div>
                <Text type='tertiary' size='small'>
                  {t('素材类型')}
                </Text>
                <div>{assetTypeLabel(t, detailAsset.asset_type)}</div>
              </div>
              <div>
                <Text type='tertiary' size='small'>
                  {t('文件大小')}
                </Text>
                <div>{formatBytes(detailAsset.file_size)}</div>
              </div>
              <div>
                <Text type='tertiary' size='small'>
                  {t('文件格式')}
                </Text>
                <div>{detailAsset.mime_type || '-'}</div>
              </div>
              <div>
                <Text type='tertiary' size='small'>
                  {t('创建时间')}
                </Text>
                <div>{formatTime(detailAsset.created_time)}</div>
              </div>
              <div>
                <Text type='tertiary' size='small'>
                  {t('更新时间')}
                </Text>
                <div>{formatTime(detailAsset.updated_time)}</div>
              </div>
            </div>

            <div>
              <Text type='tertiary' size='small'>
                {t('素材地址')}
              </Text>
              {assetPrimaryURL(detailAsset) ? (
                <div className='flex items-start gap-1'>
                  <a
                    href={assetPrimaryURL(detailAsset)}
                    target='_blank'
                    rel='noreferrer'
                    className='flex flex-1 items-center gap-1 break-all'
                    style={{ color: 'var(--semi-color-primary)' }}
                  >
                    {assetPrimaryURL(detailAsset)}
                    <ExternalLink size={12} />
                  </a>
                  <Tooltip content={t('复制地址')}>
                    <Button
                      size='small'
                      type='tertiary'
                      theme='borderless'
                      icon={<Copy size={14} />}
                      onClick={async () => {
                        const ok = await copy(assetPrimaryURL(detailAsset));
                        if (ok !== false) showSuccess(t('已复制素材地址'));
                      }}
                    />
                  </Tooltip>
                </div>
              ) : (
                <div style={{ color: 'var(--semi-color-text-2)' }}>
                  {t('处理中，暂无地址')}
                </div>
              )}
            </div>

            {(() => {
              const assetId =
                detailAsset.mappings?.find((m) => m.upstream_asset_id)
                  ?.upstream_asset_id || '';
              return (
                <div>
                  <Text type='tertiary' size='small'>
                    {t('资产 ID')}
                  </Text>
                  {assetId ? (
                    <div className='flex items-start gap-1'>
                      <span
                        className='flex-1 break-all font-mono text-xs'
                        style={{ color: 'var(--semi-color-text-0)' }}
                      >
                        {assetId}
                      </span>
                      <Tooltip content={t('复制资产 ID')}>
                        <Button
                          size='small'
                          type='tertiary'
                          theme='borderless'
                          icon={<Copy size={14} />}
                          onClick={async () => {
                            const ok = await copy(assetId);
                            if (ok !== false) showSuccess(t('已复制资产 ID'));
                          }}
                        />
                      </Tooltip>
                    </div>
                  ) : (
                    <div style={{ color: 'var(--semi-color-text-2)' }}>
                      {t('处理中，暂无资产 ID')}
                    </div>
                  )}
                </div>
              );
            })()}

            {/* Sync status */}
            {detailAsset.mappings?.length > 0 && (
              <div>
                <Text type='tertiary' size='small'>
                  {t('同步状态')}
                </Text>
                <div className='mt-1 space-y-1'>
                  {detailAsset.mappings.map((mapping, index) => (
                    <div
                      key={mapping.id}
                      className='flex items-center justify-between rounded px-2 py-1.5'
                      style={{ border: `1px solid ${borderColor}` }}
                    >
                      <span className='truncate text-xs'>
                        {t('线路 {{index}}', { index: index + 1 })}
                      </span>
                      <Tag size='small' color={statusColor(mapping.status)}>
                        {statusLabel(t, mapping.status)}
                      </Tag>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </SideSheet>

      {/* Create group modal */}
      <Modal
        title={t('新建素材组')}
        visible={createVisible}
        confirmLoading={submitting}
        onCancel={() => setCreateVisible(false)}
        onOk={createGroup}
        okButtonProps={{ disabled: !createName.trim() }}
      >
        <div className='space-y-2'>
          <Text type='tertiary' size='small'>
            {t('类型')}：
            {t(GROUP_TYPES.find((tp) => tp.key === activeType)?.labelKey)}
          </Text>
          <Input
            value={createName}
            maxLength={64}
            placeholder={t('素材组名称')}
            onChange={setCreateName}
          />
          <TextArea
            value={createDescription}
            maxLength={300}
            autosize={{ minRows: 2, maxRows: 4 }}
            placeholder={t('素材组描述（选填，最长 300 字）')}
            onChange={setCreateDescription}
          />
        </div>
      </Modal>

      {/* Rename group modal */}
      <Modal
        title={t('编辑素材组')}
        visible={editingGroup !== null}
        confirmLoading={submitting}
        onCancel={() => setEditingGroup(null)}
        onOk={renameGroup}
        okButtonProps={{ disabled: !editingName.trim() }}
      >
        <div className='space-y-2'>
          <Input
            value={editingName}
            maxLength={64}
            placeholder={t('素材组名称')}
            onChange={setEditingName}
          />
          <TextArea
            value={editingDescription}
            maxLength={300}
            autosize={{ minRows: 2, maxRows: 4 }}
            placeholder={t('素材组描述（选填，最长 300 字）')}
            onChange={setEditingDescription}
          />
        </div>
      </Modal>

      {/* Rename asset modal */}
      <Modal
        title={t('重命名素材')}
        visible={editingAsset !== null}
        confirmLoading={submitting}
        onCancel={() => setEditingAsset(null)}
        onOk={renameAsset}
        okButtonProps={{ disabled: !editingAssetName.trim() }}
      >
        <Input
          value={editingAssetName}
          maxLength={64}
          onChange={setEditingAssetName}
        />
      </Modal>

      {/* Upstream config SideSheet (admin) */}
      <SideSheet
        title={t('上游配置')}
        visible={upstreamSheetVisible}
        onCancel={() => {
          setUpstreamSheetVisible(false);
          setUpstreamEditing(false);
        }}
        width={520}
      >
        {upstreamEditing ? (
          <div className='space-y-3'>
            <div>
              <Text size='small'>{t('名称')}</Text>
              <Input
                value={upstreamForm.name}
                onChange={(v) => setForm('name', v)}
                placeholder={t('上游名称')}
              />
            </div>
            <div>
              <Text size='small'>{t('格式')}</Text>
              <Select
                value={upstreamForm.format}
                onChange={(v) => setForm('format', v)}
                optionList={UPSTREAM_FORMATS.map((f) => ({
                  label: t(f.label),
                  value: f.value,
                }))}
                style={{ width: '100%' }}
              />
            </div>
            <div>
              <Text size='small'>{t('基础地址')}</Text>
              <Input
                value={upstreamForm.base_url}
                onChange={(v) => setForm('base_url', v)}
                placeholder={
                  upstreamForm.format === 'openai'
                    ? 'https://api.openai.com/v1'
                    : 'https://host/api/asset-library'
                }
              />
            </div>
            <div>
              <Text size='small'>{t('API Key')}</Text>
              <Input
                mode='password'
                value={upstreamForm.api_key}
                onChange={(v) => setForm('api_key', v)}
                placeholder={
                  upstreamForm.id ? t('留空则不修改') : t('填写 Bearer Key')
                }
              />
            </div>

            {upstreamForm.format === 'volcengine' ? (
              <>
                <div className='grid grid-cols-2 gap-2'>
                  <div>
                    <Text size='small'>{t('接口版本 (Version)')}</Text>
                    <Input
                      value={upstreamForm.version}
                      onChange={(v) => setForm('version', v)}
                    />
                  </div>
                  <div>
                    <Text size='small'>{t('项目名称 (ProjectName)')}</Text>
                    <Input
                      value={upstreamForm.project_name}
                      onChange={(v) => setForm('project_name', v)}
                      placeholder='default'
                    />
                  </div>
                </div>
                <Text type='tertiary' size='small'>
                  {t('Action 名称（不同上游可能不同）')}
                </Text>
                <div className='grid grid-cols-2 gap-2'>
                  {[
                    ['create_group_action', '创建素材组'],
                    ['get_group_action', '素材组详情'],
                    ['update_group_action', '更新素材组'],
                    ['delete_group_action', '删除素材组'],
                    ['list_groups_action', '查询素材组'],
                    ['create_asset_action', '创建素材'],
                    ['get_asset_action', '查询素材'],
                    ['update_asset_action', '更新素材'],
                    ['delete_asset_action', '删除素材'],
                  ].map(([key, label]) => (
                    <div key={key}>
                      <Text size='small'>{t(label)}</Text>
                      <Input
                        value={upstreamForm[key]}
                        onChange={(v) => setForm(key, v)}
                      />
                    </div>
                  ))}
                </div>
              </>
            ) : (
              <div>
                <Text size='small'>{t('用途 (purpose)')}</Text>
                <Input
                  value={upstreamForm.purpose}
                  onChange={(v) => setForm('purpose', v)}
                  placeholder='user_data'
                />
              </div>
            )}

            <div className='flex justify-end gap-2 pt-2'>
              <Button onClick={() => setUpstreamEditing(false)}>
                {t('取消')}
              </Button>
              <Button
                theme='solid'
                type='primary'
                loading={upstreamSaving}
                onClick={saveUpstream}
              >
                {t('保存')}
              </Button>
            </div>
          </div>
        ) : (
          <div className='space-y-3'>
            <div className='flex justify-end'>
              <Button
                theme='solid'
                type='primary'
                icon={<Plus size={14} />}
                onClick={newUpstream}
              >
                {t('新增上游')}
              </Button>
            </div>
            {upstreamList.length === 0 ? (
              <Empty description={t('暂无上游')} />
            ) : (
              upstreamList.map((upstream) => (
                <div
                  key={upstream.id}
                  className='flex items-center justify-between rounded-lg px-3 py-2.5'
                  style={{ border: `1px solid ${borderColor}` }}
                >
                  <div className='min-w-0'>
                    <div className='flex items-center gap-2'>
                      <span className='truncate font-medium'>
                        {upstream.name}
                      </span>
                      <Tag size='small'>{upstream.format}</Tag>
                      {!upstream.enabled && (
                        <Tag size='small' color='grey'>
                          {t('已禁用')}
                        </Tag>
                      )}
                    </div>
                    <div
                      className='truncate text-xs'
                      style={{ color: 'var(--semi-color-text-2)' }}
                    >
                      {upstream.base_url}
                    </div>
                  </div>
                  <Space>
                    <Button
                      type='tertiary'
                      theme='borderless'
                      icon={<Pencil size={16} />}
                      onClick={() => editUpstream(upstream)}
                    />
                    <Button
                      type='danger'
                      theme='borderless'
                      icon={<Trash2 size={16} />}
                      onClick={() => deleteUpstream(upstream)}
                    />
                  </Space>
                </div>
              ))
            )}
          </div>
        )}
      </SideSheet>
    </div>
  );
};

export default AssetLibrary;
