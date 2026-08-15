import React from 'react';
import { Button, Popconfirm, Space, Switch, Tag, Tooltip } from '@douyinfe/semi-ui';

const maskProxyURL = (raw) => {
  if (!raw) return '';
  try {
    const u = new URL(raw);
    if (u.username || u.password) {
      u.username = '***';
      u.password = '***';
    }
    return u.toString();
  } catch {
    return raw;
  }
};

const formatTime = (ts) => {
  if (!ts) return '-';
  return new Date(ts * 1000).toLocaleString();
};

export const getProxiesColumns = ({
  t,
  openEdit,
  openReferences,
  handleTest,
  handleDelete,
  handleToggleStatus,
  testingIds,
}) => [
  {
    key: 'id',
    title: 'ID',
    dataIndex: 'id',
    width: 80,
  },
  {
    key: 'name',
    title: t('代理名称'),
    dataIndex: 'name',
    width: 180,
  },
  {
    key: 'type',
    title: t('代理类型'),
    dataIndex: 'type',
    width: 100,
    render: (v) => <Tag color='blue'>{String(v || '').toUpperCase()}</Tag>,
  },
  {
    key: 'url',
    title: t('代理地址'),
    dataIndex: 'url',
    render: (v) => (
      <code style={{ fontSize: 12, wordBreak: 'break-all' }}>
        {maskProxyURL(v)}
      </code>
    ),
  },
  {
    key: 'status',
    title: t('状态'),
    dataIndex: 'status',
    width: 90,
    render: (v, r) => (
      <Switch
        checked={v === 1}
        onChange={(checked) => handleToggleStatus(r, checked)}
      />
    ),
  },
  {
    key: 'last_test',
    title: t('上次测试'),
    dataIndex: 'last_test_time',
    width: 220,
    render: (_, r) => {
      if (!r.last_test_time) return <span>-</span>;
      return (
        <Tooltip content={r.last_test_msg || ''}>
          <Space>
            <Tag color={r.last_test_ok ? 'green' : 'red'}>
              {r.last_test_ok ? t('成功') : t('失败')}
            </Tag>
            <span style={{ fontSize: 12 }}>{formatTime(r.last_test_time)}</span>
          </Space>
        </Tooltip>
      );
    },
  },
  {
    key: 'description',
    title: t('备注'),
    dataIndex: 'description',
    render: (v) => <span style={{ fontSize: 12 }}>{v || '-'}</span>,
  },
  {
    key: 'actions',
    title: t('操作'),
    width: 300,
    fixed: 'right',
    render: (_, r) => (
      <Space>
        <Button size='small' theme='light' onClick={() => openEdit(r)}>
          {t('编辑')}
        </Button>
        <Button
          size='small'
          theme='light'
          loading={!!testingIds[r.id]}
          onClick={() => handleTest(r.id)}
        >
          {t('测试')}
        </Button>
        <Button size='small' theme='light' onClick={() => openReferences(r)}>
          {t('查看引用')}
        </Button>
        <Popconfirm
          title={t('确认删除该代理？')}
          onConfirm={() => handleDelete(r)}
        >
          <Button size='small' type='danger'>
            {t('删除')}
          </Button>
        </Popconfirm>
      </Space>
    ),
  },
];
