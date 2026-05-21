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

import React from 'react';
import {
  Button,
  Modal,
  Popover,
  Progress,
  Space,
  Switch,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  AlertTriangle,
  CheckCircle,
  CreditCard,
  Pencil,
  Send,
  Trash2,
} from 'lucide-react';
import { convertUSDToCurrency } from '../../../helpers/render';

const { Text, Paragraph } = Typography;

const formatTimestamp = (ts) => {
  if (!ts) return '—';
  const d = new Date(ts * 1000);
  if (isNaN(d.getTime())) return '—';
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

const formatBalance = (n) => {
  if (n === undefined || n === null || isNaN(n)) return '—';
  return convertUSDToCurrency(Number(n), 4);
};

const getProgressColor = (pct) => {
  if (pct >= 100) return 'var(--semi-color-danger)';
  if (pct >= 80) return 'var(--semi-color-warning)';
  return undefined;
};

// 状态(已停用 / 告警中 / 正常)与令牌"状态"列样式保持一致
const renderState = (rule, t) => {
  if (!rule.enabled) {
    return (
      <Tag size='small' color='grey' shape='circle'>
        {t('已停用')}
      </Tag>
    );
  }
  if (rule.alert_state === 'alerting') {
    return (
      <Tag
        size='small'
        color='red'
        shape='circle'
        prefixIcon={<AlertTriangle size={12} />}
      >
        {t('告警中')}
      </Tag>
    );
  }
  return (
    <Tag
      size='small'
      color='green'
      shape='circle'
      prefixIcon={<CheckCircle size={12} />}
    >
      {t('正常')}
    </Tag>
  );
};

// 额度使用列:沿用令牌"剩余/总额度 + Progress + Popover 明细"的样式
const renderQuotaUsage = (rule, t) => {
  const total = Number(rule.total_quota) || 0;
  const used = Number(rule.summary?.used_usd) || 0;
  const remaining = Math.max(total - used, 0);
  const percent = total > 0 ? (used / total) * 100 : 0;

  if (total <= 0) {
    const popoverContent = (
      <div className='text-xs p-2'>
        <Paragraph>{t('尚未配置总额度,请编辑规则填写。')}</Paragraph>
        <Paragraph copyable={{ content: String(used.toFixed(4)) }}>
          {t('已使用')}: {formatBalance(used)}
        </Paragraph>
      </div>
    );
    return (
      <Popover content={popoverContent} position='top'>
        <Tag color='white' shape='circle'>
          {t('未配置')}
        </Tag>
      </Popover>
    );
  }

  const popoverContent = (
    <div className='text-xs p-2'>
      <Paragraph copyable={{ content: String(total.toFixed(4)) }}>
        {t('总额度')}: {formatBalance(total)}
      </Paragraph>
      <Paragraph copyable={{ content: String(used.toFixed(4)) }}>
        {t('已使用')}: {formatBalance(used)} ({percent.toFixed(0)}%)
      </Paragraph>
      <Paragraph copyable={{ content: String(remaining.toFixed(4)) }}>
        {t('剩余')}: {formatBalance(remaining)}
      </Paragraph>
    </div>
  );

  return (
    <Popover content={popoverContent} position='top'>
      <Tag color='white' shape='circle'>
        <div className='flex flex-col items-end'>
          <span className='text-xs leading-none'>{`${formatBalance(remaining)} / ${formatBalance(total)}`}</span>
          <Progress
            percent={Math.min(percent, 100)}
            stroke={getProgressColor(percent)}
            aria-label='balance usage'
            format={() => `${percent.toFixed(0)}%`}
            style={{ width: '100%', marginTop: '1px', marginBottom: 0 }}
          />
        </div>
      </Tag>
    </Popover>
  );
};

const renderTag = (text, record) => (
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
);

const renderWebhook = (url, t) => {
  if (!url) {
    return (
      <Tag size='small' color='blue' shape='circle'>
        {t('默认通知')}
      </Tag>
    );
  }
  return (
    <Tooltip content={url} position='top'>
      <Text
        ellipsis={{ showTooltip: false }}
        style={{ maxWidth: 160 }}
        size='small'
      >
        {url}
      </Text>
    </Tooltip>
  );
};

const renderChannels = (summary) => {
  if (!summary) return '—';
  return `${summary.enabled_count}/${summary.channel_count}`;
};

const renderOperations = (
  record,
  { openTopup, openEdit, handleTest, handleDelete, t },
) => {
  return (
    <Space wrap>
      <Button
        type='primary'
        size='small'
        icon={<CreditCard size={14} />}
        onClick={() => openTopup(record)}
      >
        {t('充值')}
      </Button>

      <Button
        type='tertiary'
        size='small'
        icon={<Send size={14} />}
        onClick={() => handleTest(record.id)}
      >
        {t('测试')}
      </Button>

      <Button
        type='tertiary'
        size='small'
        icon={<Pencil size={14} />}
        onClick={() => openEdit(record)}
      >
        {t('编辑')}
      </Button>

      <Button
        type='danger'
        size='small'
        icon={<Trash2 size={14} />}
        onClick={() => {
          Modal.confirm({
            title: t('确定删除该规则?'),
            content: t('此修改将不可逆'),
            onOk: () => handleDelete(record.id),
          });
        }}
      >
        {t('删除')}
      </Button>
    </Space>
  );
};

export const getBalanceAlertColumns = ({
  t,
  openEdit,
  openTopup,
  handleTest,
  handleDelete,
  handleToggleEnabled,
}) => {
  return [
    {
      title: t('Tag'),
      dataIndex: 'tag',
      render: renderTag,
    },
    {
      title: t('状态'),
      dataIndex: 'state',
      render: (_text, record) => renderState(record, t),
    },
    {
      title: t('剩余/总额度'),
      key: 'quota_usage',
      render: (_text, record) => renderQuotaUsage(record, t),
    },
    {
      title: t('告警阈值'),
      dataIndex: 'threshold',
      render: (v) => formatBalance(v),
    },
    {
      title: t('渠道数'),
      dataIndex: 'summary',
      render: (summary) => renderChannels(summary),
    },
    {
      title: t('Webhook'),
      dataIndex: 'webhook_url',
      render: (url) => renderWebhook(url, t),
    },
    {
      title: t('上次告警'),
      dataIndex: 'last_alerted_at',
      render: (v) => formatTimestamp(v),
    },
    {
      title: t('启用'),
      dataIndex: 'enabled',
      render: (_text, record) => (
        <Switch
          size='small'
          checked={record.enabled}
          onChange={(v) => handleToggleEnabled(record, v)}
        />
      ),
    },
    {
      title: '',
      dataIndex: 'operate',
      fixed: 'right',
      render: (_text, record) =>
        renderOperations(record, {
          openTopup,
          openEdit,
          handleTest,
          handleDelete,
          t,
        }),
    },
  ];
};
