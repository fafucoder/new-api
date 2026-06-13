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
  Space,
  Switch,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  AlertTriangle,
  CheckCircle,
  Pencil,
  Send,
  Trash2,
  Clock,
} from 'lucide-react';

const { Text } = Typography;

const formatTimestamp = (ts) => {
  if (!ts) return '—';
  const d = new Date(ts * 1000);
  if (isNaN(d.getTime())) return '—';
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

const timeWindowLabels = {
  '5m': '5分钟',
  '15m': '15分钟',
  '30m': '30分钟',
  '1h': '1小时',
  '6h': '6小时',
};

// 状态列
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

// 规则名称列
const renderName = (text, record) => (
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

// 监控范围列
const renderScope = (rule, channels, t) => {
  const { scope, scope_value } = rule;

  if (scope === 'global') {
    return (
      <Tag size='small' color='blue' shape='circle'>
        {t('全局')}
      </Tag>
    );
  }

  if (scope === 'channel') {
    const channel = channels.find((c) => c.id === parseInt(scope_value));
    const text = channel ? `#${channel.id} ${channel.name}` : `#${scope_value}`;
    return (
      <Tooltip content={text} position='top'>
        <Tag size='small' color='cyan' shape='circle'>
          {t('渠道')} #{scope_value}
        </Tag>
      </Tooltip>
    );
  }

  if (scope === 'tag') {
    return (
      <Tooltip content={scope_value} position='top'>
        <Tag size='small' color='purple' shape='circle'>
          {t('标签组')} "{scope_value}"
        </Tag>
      </Tooltip>
    );
  }

  return scope;
};

// 时间窗口列
const renderTimeWindow = (text) => {
  return (
    <Tag size='small' color='white' shape='circle' prefixIcon={<Clock size={12} />}>
      {timeWindowLabels[text] || text}
    </Tag>
  );
};

// 错误率列
const renderErrorRate = (rule, t) => {
  if (!rule.last_checked_at) {
    return (
      <Tag size='small' color='grey' shape='circle'>
        {t('未检查')}
      </Tag>
    );
  }

  const rate = rule.last_error_rate || 0;
  const threshold = rule.threshold || 0;
  const color = rate >= threshold ? 'red' : 'green';

  return (
    <Tag size='small' color={color} shape='circle'>
      {rate.toFixed(2)}%
    </Tag>
  );
};

// Webhook 列
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

// 操作列
const renderOperations = (
  record,
  { openEdit, handleTest, handleDelete, t },
) => {
  return (
    <Space wrap>
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
            title: t('确定删除此规则？'),
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

export const getErrorRateAlertColumns = ({
  t,
  channels,
  openEdit,
  handleTest,
  handleDelete,
  handleToggleEnabled,
}) => {
  return [
    {
      title: t('规则名称'),
      dataIndex: 'name',
      render: renderName,
    },
    {
      title: t('状态'),
      dataIndex: 'state',
      render: (_text, record) => renderState(record, t),
    },
    {
      title: t('监控范围'),
      key: 'scope',
      render: (_text, record) => renderScope(record, channels, t),
    },
    {
      title: t('时间窗口'),
      dataIndex: 'time_window',
      render: renderTimeWindow,
    },
    {
      title: t('阈值'),
      dataIndex: 'threshold',
      render: (v) => `${v}%`,
    },
    {
      title: t('最近错误率'),
      key: 'error_rate',
      render: (_text, record) => renderErrorRate(record, t),
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
          openEdit,
          handleTest,
          handleDelete,
          t,
        }),
    },
  ];
};
