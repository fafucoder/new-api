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

import React, { useEffect, useRef, useState } from 'react';
import {
  Button,
  Card,
  Empty,
  Input,
  Modal,
  Space,
  Spin,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  ExternalLink,
  FileAudio,
  FileVideo,
  Image as ImageIcon,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
  Upload,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, showWarning } from '../../helpers';

const { Title, Text } = Typography;

const statusColor = (status) => {
  if (status === 'Active') return 'green';
  if (status === 'Failed') return 'red';
  return 'orange';
};

const hasFailedChannel = (response) =>
  response?.data?.data?.results?.some((result) => !result.success) === true;

const AssetPreview = ({ asset }) => {
  const url = asset.mappings?.find((mapping) => mapping.asset_url)?.asset_url;
  if (url && asset.asset_type === 'Image') {
    return (
      <img
        src={url}
        alt={asset.name}
        className='h-12 w-12 rounded object-cover'
      />
    );
  }
  if (url && asset.asset_type === 'Video') {
    return (
      <video
        src={url}
        muted
        preload='metadata'
        className='h-12 w-12 rounded object-cover'
      />
    );
  }
  const Icon =
    asset.asset_type === 'Image'
      ? ImageIcon
      : asset.asset_type === 'Audio'
        ? FileAudio
        : FileVideo;
  return (
    <span className='flex h-12 w-12 items-center justify-center rounded bg-gray-100'>
      <Icon size={20} />
    </span>
  );
};

const AssetLibrary = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [groups, setGroups] = useState([]);
  const [channels, setChannels] = useState([]);
  const [uploadState, setUploadState] = useState(null);
  const [editingGroup, setEditingGroup] = useState(null);
  const [displayName, setDisplayName] = useState('');
  const [files, setFiles] = useState([]);
  const [submitting, setSubmitting] = useState(false);
  const fileInputRef = useRef(null);

  const loadData = async () => {
    setLoading(true);
    try {
      const [groupsResponse, channelsResponse] = await Promise.all([
        API.get('/api/asset-library/groups'),
        API.get('/api/asset-library/channels'),
      ]);
      setGroups(groupsResponse.data.data || []);
      setChannels(channelsResponse.data.data || []);
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const submitUpload = async () => {
    if (!uploadState || files.length === 0) return;
    const form = new FormData();
    files.forEach((file) => form.append('files', file));
    if (uploadState.mode === 'create' && displayName.trim()) {
      form.append('display_name', displayName.trim());
    }
    const endpoint =
      uploadState.mode === 'append'
        ? `/api/asset-library/groups/${uploadState.group.id}/assets`
        : '/api/asset-library/groups';
    setSubmitting(true);
    try {
      const response = await API.post(endpoint, form);
      if (hasFailedChannel(response)) {
        showWarning(t('部分渠道同步失败'));
      } else {
        showSuccess(t('素材已上传'));
      }
      setUploadState(null);
      setFiles([]);
      if (fileInputRef.current) fileInputRef.current.value = '';
      setDisplayName('');
      await loadData();
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const renameGroup = async () => {
    if (!editingGroup || !displayName.trim()) return;
    setSubmitting(true);
    try {
      const response = await API.patch(
        `/api/asset-library/groups/${editingGroup.id}`,
        {
          display_name: displayName.trim(),
        },
      );
      if (hasFailedChannel(response)) {
        showWarning(t('部分渠道同步失败'));
      } else {
        showSuccess(t('素材组已更新'));
      }
      setEditingGroup(null);
      setDisplayName('');
      await loadData();
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const refreshGroup = async (group) => {
    try {
      const response = await API.post(
        `/api/asset-library/groups/${group.id}/refresh`,
      );
      if (hasFailedChannel(response)) {
        showWarning(t('部分渠道同步失败'));
      } else {
        showSuccess(t('素材状态已刷新'));
      }
      await loadData();
    } catch (error) {
      showError(error);
    }
  };

  const confirmDeleteGroup = (group) => {
    Modal.confirm({
      title: t('确认删除'),
      content: t('此操作会从每个已映射渠道中删除该素材组'),
      onOk: async () => {
        await API.delete(`/api/asset-library/groups/${group.id}`);
        showSuccess(t('素材组已删除'));
        await loadData();
      },
    });
  };

  const confirmDeleteAsset = (group, asset) => {
    Modal.confirm({
      title: t('确认删除'),
      content: t('此操作会从每个已映射渠道中删除该素材'),
      onOk: async () => {
        await API.delete(
          `/api/asset-library/groups/${group.id}/assets/${asset.id}`,
        );
        showSuccess(t('素材已删除'));
        await loadData();
      },
    });
  };

  return (
    <div className='mt-[60px] px-2 pb-8'>
      <div className='mb-4 flex flex-wrap items-center justify-between gap-3'>
        <div>
          <Title heading={3}>{t('素材库')}</Title>
          <Text type='tertiary'>
            {t('{{count}} 个映射渠道', { count: channels.length })}
          </Text>
        </div>
        <Button
          theme='solid'
          type='primary'
          icon={<Upload size={16} />}
          disabled={loading || channels.length === 0}
          onClick={() => {
            if (fileInputRef.current) fileInputRef.current.value = '';
            setFiles([]);
            setUploadState({ mode: 'create', group: null });
          }}
        >
          {t('上传素材')}
        </Button>
      </div>

      <Spin spinning={loading}>
        {groups.length === 0 ? (
          <div className='flex min-h-64 items-center justify-center border-y'>
            <Empty description={t('暂无素材')} />
          </div>
        ) : (
          <div className='space-y-4'>
            {groups.map((group) => (
              <Card
                key={group.id}
                title={
                  <div className='flex flex-wrap items-center gap-2'>
                    <span>{group.display_name}</span>
                    <Tag>
                      {t('{{count}} 个素材', {
                        count: group.assets?.length || 0,
                      })}
                    </Tag>
                  </div>
                }
                headerExtraContent={
                  <Space spacing={4}>
                    <Tooltip content={t('追加素材')}>
                      <Button
                        type='tertiary'
                        icon={<Plus size={16} />}
                        onClick={() => {
                          if (fileInputRef.current) {
                            fileInputRef.current.value = '';
                          }
                          setFiles([]);
                          setUploadState({ mode: 'append', group });
                        }}
                      />
                    </Tooltip>
                    <Tooltip content={t('刷新状态')}>
                      <Button
                        type='tertiary'
                        icon={<RefreshCw size={16} />}
                        onClick={() => refreshGroup(group)}
                      />
                    </Tooltip>
                    <Tooltip content={t('重命名')}>
                      <Button
                        type='tertiary'
                        icon={<Pencil size={16} />}
                        onClick={() => {
                          setEditingGroup(group);
                          setDisplayName(group.display_name);
                        }}
                      />
                    </Tooltip>
                    <Tooltip content={t('删除')}>
                      <Button
                        type='danger'
                        icon={<Trash2 size={16} />}
                        onClick={() => confirmDeleteGroup(group)}
                      />
                    </Tooltip>
                  </Space>
                }
              >
                <div className='mb-3 flex flex-wrap gap-2'>
                  {group.mappings?.map((mapping) => (
                    <Tag key={mapping.id} color={statusColor(mapping.status)}>
                      {mapping.channel_name || `#${mapping.channel_id}`} ·{' '}
                      {t(mapping.status)}
                    </Tag>
                  ))}
                </div>
                <div className='divide-y'>
                  {group.assets?.map((asset) => (
                    <div
                      key={asset.id}
                      className='grid gap-3 py-3 md:grid-cols-[minmax(180px,0.7fr)_minmax(0,1.3fr)_36px]'
                    >
                      <div className='flex min-w-0 items-center gap-3'>
                        <AssetPreview asset={asset} />
                        <div className='min-w-0'>
                          <div className='truncate font-medium'>
                            {asset.name}
                          </div>
                          <Text type='tertiary' size='small'>
                            {t(asset.asset_type)}
                          </Text>
                        </div>
                      </div>
                      <div className='grid gap-2 lg:grid-cols-2'>
                        {asset.mappings?.map((mapping) => (
                          <div
                            key={mapping.id}
                            className='flex min-w-0 items-center justify-between gap-2 rounded border px-2 py-1.5'
                          >
                            <div className='min-w-0'>
                              <div className='truncate text-xs font-medium'>
                                {mapping.channel_name ||
                                  `#${mapping.channel_id}`}
                              </div>
                              <div className='truncate font-mono text-xs text-gray-500'>
                                {mapping.error_message ||
                                  mapping.upstream_asset_id ||
                                  '-'}
                              </div>
                            </div>
                            <Space spacing={4}>
                              <Tag color={statusColor(mapping.status)}>
                                {t(mapping.status)}
                              </Tag>
                              {mapping.asset_url && (
                                <a
                                  href={mapping.asset_url}
                                  target='_blank'
                                  rel='noreferrer'
                                  title={t('打开素材')}
                                >
                                  <ExternalLink size={14} />
                                </a>
                              )}
                            </Space>
                          </div>
                        ))}
                      </div>
                      <Button
                        type='danger'
                        icon={<Trash2 size={16} />}
                        onClick={() => confirmDeleteAsset(group, asset)}
                      />
                    </div>
                  ))}
                </div>
              </Card>
            ))}
          </div>
        )}
      </Spin>

      <Modal
        title={uploadState?.mode === 'append' ? t('追加素材') : t('上传素材')}
        visible={uploadState !== null}
        confirmLoading={submitting}
        onCancel={() => setUploadState(null)}
        onOk={submitUpload}
        okButtonProps={{ disabled: files.length === 0 }}
      >
        <div className='space-y-3'>
          {uploadState?.mode === 'create' && (
            <Input
              value={displayName}
              maxLength={64}
              placeholder={t('素材组名称')}
              onChange={setDisplayName}
            />
          )}
          <input
            ref={fileInputRef}
            type='file'
            multiple
            accept='image/*,video/*,audio/*'
            onChange={(event) =>
              setFiles(Array.from(event.target.files || []).slice(0, 20))
            }
          />
          {files.length > 0 && (
            <Text type='tertiary'>
              {t('已选择 {{count}} 个文件', { count: files.length })}
            </Text>
          )}
        </div>
      </Modal>

      <Modal
        title={t('重命名素材组')}
        visible={editingGroup !== null}
        confirmLoading={submitting}
        onCancel={() => setEditingGroup(null)}
        onOk={renameGroup}
        okButtonProps={{ disabled: !displayName.trim() }}
      >
        <Input value={displayName} maxLength={64} onChange={setDisplayName} />
      </Modal>
    </div>
  );
};

export default AssetLibrary;
