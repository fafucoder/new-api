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
  Banner,
  Button,
  Card,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Switch,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  AlertTriangle,
  CheckCircle,
  Pencil,
  Plus,
  RefreshCw,
  Send,
  Trash2,
  Wallet,
} from 'lucide-react';
import { API, showError, showSuccess } from '../../helpers';

const { Title, Text } = Typography;

const formatTimestamp = (ts) => {
  if (!ts) return '—';
  const d = new Date(ts * 1000);
  if (isNaN(d.getTime())) return '—';
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

const formatBalance = (n) => {
  if (n === undefined || n === null || isNaN(n)) return '—';
  return `$${Number(n).toFixed(4)}`;
};

const initialRuleForm = () => ({
  id: 0,
  tag: '',
  threshold: 5,
  webhook_url: '',
  webhook_secret: '',
  remark: '',
  enabled: true,
});

const BalanceAlertPage = () => {
  const { t } = useTranslation();
  const [rules, setRules] = useState([]);
  const [tags, setTags] = useState([]);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState(null); // null | rule object
  const [form, setForm] = useState(initialRuleForm());

  const loadRules = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/balance-alert/rules');
      const { success, data, message } = res.data || {};
      if (success) {
        setRules(Array.isArray(data) ? data : []);
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
    } catch (e) {
      // 静默失败,新建表单还能用 Input 手填 tag
    }
  }, []);

  useEffect(() => {
    loadRules();
    loadTags();
  }, [loadRules, loadTags]);

  const openCreate = () => {
    setEditing({ id: 0 });
    setForm(initialRuleForm());
  };

  const openEdit = (rule) => {
    setEditing(rule);
    setForm({
      id: rule.id,
      tag: rule.tag || '',
      threshold: rule.threshold || 0,
      webhook_url: rule.webhook_url || '',
      webhook_secret: rule.webhook_secret || '',
      remark: rule.remark || '',
      enabled: rule.enabled !== false,
    });
  };

  const closeModal = () => {
    setEditing(null);
  };

  const submitForm = async () => {
    if (!form.tag || !form.tag.trim()) {
      showError(t('请填写或选择 Tag'));
      return;
    }
    if (form.threshold == null || isNaN(form.threshold) || form.threshold < 0) {
      showError(t('阈值需为非负数'));
      return;
    }
    try {
      const payload = {
        tag: form.tag.trim(),
        threshold: Number(form.threshold),
        webhook_url: form.webhook_url || '',
        webhook_secret: form.webhook_secret || '',
        remark: form.remark || '',
        enabled: !!form.enabled,
      };
      const res = form.id
        ? await API.put(`/api/balance-alert/rules/${form.id}`, payload)
        : await API.post('/api/balance-alert/rules', payload);
      const { success, message } = res.data || {};
      if (success) {
        showSuccess(form.id ? t('已更新') : t('已创建'));
        closeModal();
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

  const renderState = (rule) => {
    if (!rule.enabled) {
      return (
        <Tag size='small' color='grey'>
          {t('已停用')}
        </Tag>
      );
    }
    if (rule.alert_state === 'alerting') {
      return (
        <Tag size='small' color='red' prefixIcon={<AlertTriangle size={12} />}>
          {t('告警中')}
        </Tag>
      );
    }
    return (
      <Tag size='small' color='green' prefixIcon={<CheckCircle size={12} />}>
        {t('正常')}
      </Tag>
    );
  };

  const columns = [
    {
      title: t('Tag (上游标签)'),
      dataIndex: 'tag',
      width: 180,
      render: (text, record) => (
        <div>
          <Text strong>{text}</Text>
          {record.remark ? (
            <div>
              <Text type='tertiary' size='small'>
                {record.remark}
              </Text>
            </div>
          ) : null}
        </div>
      ),
    },
    {
      title: t('当前余额'),
      dataIndex: 'summary',
      width: 140,
      render: (summary, record) => {
        const balance = summary?.balance ?? record.last_balance ?? 0;
        const updated = summary?.balance_updated_time ?? 0;
        const below = balance < record.threshold;
        return (
          <div>
            <Text strong style={{ color: below ? 'var(--semi-color-danger)' : undefined }}>
              {formatBalance(balance)}
            </Text>
            <div>
              <Text type='tertiary' size='small'>
                {updated ? formatTimestamp(updated) : t('暂无数据')}
              </Text>
            </div>
          </div>
        );
      },
    },
    {
      title: t('告警阈值'),
      dataIndex: 'threshold',
      width: 120,
      render: (v) => formatBalance(v),
    },
    {
      title: t('渠道数'),
      dataIndex: 'summary',
      width: 100,
      render: (summary) => {
        if (!summary) return '—';
        return `${summary.enabled_count}/${summary.channel_count}`;
      },
    },
    {
      title: t('状态'),
      width: 100,
      render: (_, record) => renderState(record),
    },
    {
      title: t('Webhook'),
      dataIndex: 'webhook_url',
      width: 160,
      render: (url) => {
        if (!url) {
          return (
            <Tag size='small' color='blue'>
              {t('默认通知')}
            </Tag>
          );
        }
        return (
          <Text
            ellipsis={{ showTooltip: true }}
            style={{ maxWidth: 140 }}
            size='small'
          >
            {url}
          </Text>
        );
      },
    },
    {
      title: t('上次告警'),
      dataIndex: 'last_alerted_at',
      width: 150,
      render: (v) => formatTimestamp(v),
    },
    {
      title: t('启用'),
      width: 80,
      render: (_, record) => (
        <Switch
          size='small'
          checked={record.enabled}
          onChange={(v) => handleToggleEnabled(record, v)}
        />
      ),
    },
    {
      title: t('操作'),
      width: 200,
      fixed: 'right',
      render: (_, record) => (
        <div style={{ display: 'flex', gap: 4 }}>
          <Button
            size='small'
            theme='borderless'
            icon={<Send size={14} />}
            onClick={() => handleTest(record.id)}
          >
            {t('测试')}
          </Button>
          <Button
            size='small'
            theme='borderless'
            icon={<Pencil size={14} />}
            onClick={() => openEdit(record)}
          >
            {t('编辑')}
          </Button>
          <Popconfirm
            title={t('确定删除该规则?')}
            onConfirm={() => handleDelete(record.id)}
          >
            <Button
              size='small'
              theme='borderless'
              type='danger'
              icon={<Trash2 size={14} />}
            />
          </Popconfirm>
        </div>
      ),
    },
  ];

  const tagOptions = tags.map((tg) => ({ value: tg, label: tg }));

  return (
    <div
      className='mt-[60px] px-2 balance-alert-page'
      style={{ width: '100%', overflow: 'hidden', boxSizing: 'border-box' }}
    >
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 24, marginBottom: 20 }}>
        <div style={{ flex: 1 }}>
          <Title heading={2} style={{ marginBottom: 8 }}>
            <Wallet size={20} style={{ verticalAlign: '-3px', marginRight: 6 }} />
            {t('渠道余额监控')}
          </Title>
          <Text type='secondary' style={{ fontSize: 13 }}>
            {t(
              '按渠道 Tag 聚合上游账户;余额跌破阈值通过 webhook 或站内通知告警。同上游的多个渠道请打同一个 Tag。',
            )}
          </Text>
        </div>
      </div>

      <Banner
        type='info'
        description={t(
          '规则按 channels.tag 字段聚合。请先在渠道管理里给指向同一上游的渠道打同一个 Tag,再在这里为该 Tag 配置阈值。Webhook 留空则走 root 用户的默认通知方式(邮件/Bark/Gotify/Webhook)。',
        )}
        fullMode={false}
        style={{ marginBottom: 16 }}
      />

      <Card bordered>
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            marginBottom: 12,
          }}
        >
          <Text strong>
            {t('监控规则')}
            <Text type='tertiary' size='small' style={{ marginLeft: 8 }}>
              ({rules.length})
            </Text>
          </Text>
          <div style={{ display: 'flex', gap: 8 }}>
            <Button
              icon={<RefreshCw size={14} />}
              theme='borderless'
              onClick={() => {
                loadRules();
                loadTags();
              }}
              loading={loading}
            >
              {t('刷新')}
            </Button>
            <Button
              theme='solid'
              type='primary'
              icon={<Plus size={14} />}
              onClick={openCreate}
            >
              {t('新建规则')}
            </Button>
          </div>
        </div>
        <Table
          columns={columns}
          dataSource={rules}
          rowKey='id'
          pagination={false}
          loading={loading}
          empty={
            <Empty
              image={<Wallet size={48} color='var(--semi-color-text-3)' />}
              title={t('暂无规则')}
              description={t('点击右上角"新建规则"开始监控')}
            />
          }
          scroll={{ x: 1200 }}
        />
      </Card>

      <Modal
        title={form.id ? t('编辑规则') : t('新建规则')}
        visible={editing !== null}
        onCancel={closeModal}
        onOk={submitForm}
        okText={t('保存')}
        cancelText={t('取消')}
        width={560}
      >
        <Form labelPosition='top'>
          <Form.Slot label={t('Tag (上游标签)')} required>
            <Select
              style={{ width: '100%' }}
              value={form.tag || undefined}
              onChange={(v) => setForm((f) => ({ ...f, tag: v || '' }))}
              placeholder={t('从已有 Tag 选择,或输入新 Tag')}
              optionList={tagOptions}
              filter
              allowCreate
              showClear
              disabled={!!form.id}
            />
            <Text type='tertiary' size='small'>
              {t('对应 channels.tag 字段,同一上游账户的多个渠道请打同一个 Tag。规则创建后 Tag 不可改')}
            </Text>
          </Form.Slot>
          <Form.Slot label={t('告警阈值 (USD)')} required>
            <Input
              value={form.threshold}
              onChange={(v) => setForm((f) => ({ ...f, threshold: v }))}
              placeholder='5.0'
              type='number'
              step='0.01'
              min='0'
            />
            <Text type='tertiary' size='small'>
              {t('余额低于此值触发告警')}
            </Text>
          </Form.Slot>
          <Form.Slot label={t('Webhook URL (可选)')}>
            <Input
              value={form.webhook_url}
              onChange={(v) => setForm((f) => ({ ...f, webhook_url: v }))}
              placeholder='https://your-webhook.example/path'
            />
            <Text type='tertiary' size='small'>
              {t('留空使用 root 用户的默认通知方式')}
            </Text>
          </Form.Slot>
          <Form.Slot label={t('Webhook 签名密钥 (可选)')}>
            <Input
              value={form.webhook_secret}
              onChange={(v) => setForm((f) => ({ ...f, webhook_secret: v }))}
              placeholder={t('用于 HMAC-SHA256 校验')}
            />
          </Form.Slot>
          <Form.Slot label={t('备注 (可选)')}>
            <Input
              value={form.remark}
              onChange={(v) => setForm((f) => ({ ...f, remark: v }))}
              placeholder={t('例如:OpenRouter 主账户')}
            />
          </Form.Slot>
          <Form.Slot label={t('启用')}>
            <Switch
              checked={form.enabled}
              onChange={(v) => setForm((f) => ({ ...f, enabled: v }))}
            />
          </Form.Slot>
        </Form>
      </Modal>
    </div>
  );
};

export default BalanceAlertPage;
