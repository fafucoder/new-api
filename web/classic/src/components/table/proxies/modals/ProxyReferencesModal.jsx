import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { Modal, Table, Tag, Toast, Empty, Button } from '@douyinfe/semi-ui';
import { getProxyReferences } from '../../../../services/proxy';

const statusLabel = (t, s) => {
  switch (s) {
    case 1:
      return <Tag color='green'>{t('启用')}</Tag>;
    case 2:
      return <Tag color='red'>{t('禁用')}</Tag>;
    default:
      return <Tag>{s}</Tag>;
  }
};

const ProxyReferencesModal = ({ visible, proxy, onCancel }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!visible || !proxy?.id) return;
    setLoading(true);
    getProxyReferences(proxy.id)
      .then((res) => {
        if (res.data?.success) {
          setItems(res.data.data || []);
        } else {
          Toast.error(res.data?.message || t('加载失败'));
        }
      })
      .catch((e) => Toast.error(e?.message || t('加载失败')))
      .finally(() => setLoading(false));
  }, [visible, proxy, t]);

  const goChannel = (record) => {
    const kw = record.name || String(record.id || '');
    onCancel?.();
    navigate(`/console/channel?keyword=${encodeURIComponent(kw)}`);
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 80 },
    {
      title: t('渠道名称'),
      dataIndex: 'name',
      render: (v, r) => (
        <Button
          theme='borderless'
          type='primary'
          size='small'
          style={{ padding: 0 }}
          onClick={() => goChannel(r)}
        >
          {v}
        </Button>
      ),
    },
    { title: t('类型'), dataIndex: 'type', width: 100 },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 100,
      render: (v) => statusLabel(t, v),
    },
    {
      title: t('操作'),
      width: 120,
      render: (_, r) => (
        <Button size='small' theme='light' onClick={() => goChannel(r)}>
          {t('前往渠道')}
        </Button>
      ),
    },
  ];

  return (
    <Modal
      title={`${t('查看引用')} - ${proxy?.name || ''}`}
      visible={visible}
      onCancel={onCancel}
      footer={null}
      width={720}
    >
      <Table
        columns={columns}
        dataSource={items}
        loading={loading}
        rowKey='id'
        pagination={false}
        empty={<Empty description={t('当前无渠道引用此代理')} />}
      />
    </Modal>
  );
};

export default ProxyReferencesModal;
