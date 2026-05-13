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

import React, { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Card,
  Typography,
  Button,
  Empty,
  Popover,
  Input,
  Tag,
  Table,
} from '@douyinfe/semi-ui';
import {
  Activity,
  Boxes,
  CheckCircle,
  AlertTriangle,
  RefreshCw,
  Search,
  ChevronDown,
  ChevronRight,
} from 'lucide-react';
import { API } from '../../helpers';
import { CHANNEL_OPTIONS } from '../../constants';

const { Title, Text } = Typography;

const REFRESH_INTERVAL_MS = 30 * 1000;

const SQUARE_COLORS = {
  success: 'var(--semi-color-success)',
  error: 'var(--semi-color-danger)',
  unknown: 'var(--semi-color-fill-2)',
};

const squareStyle = (cls) => ({
  display: 'inline-block',
  width: 10,
  height: 18,
  borderRadius: 2,
  backgroundColor: SQUARE_COLORS[cls] || SQUARE_COLORS.unknown,
  verticalAlign: 'top',
});

const channelTypeLabelMap = CHANNEL_OPTIONS.reduce((acc, opt) => {
  acc[opt.value] = opt.label;
  return acc;
}, {});

const STATUS_FILTER_OPTIONS = ['normal', 'degraded', 'error', 'unknown'];

const formatTimestamp = (ts) => {
  if (!ts) return '-';
  try {
    const d = new Date(ts * 1000);
    const yyyy = d.getFullYear();
    const mm = String(d.getMonth() + 1).padStart(2, '0');
    const dd = String(d.getDate()).padStart(2, '0');
    const hh = String(d.getHours()).padStart(2, '0');
    const mi = String(d.getMinutes()).padStart(2, '0');
    const ss = String(d.getSeconds()).padStart(2, '0');
    return `${yyyy}/${mm}/${dd} ${hh}:${mi}:${ss}`;
  } catch {
    return '-';
  }
};

const HistoryStrip = ({ history }) => {
  const { t } = useTranslation();
  const stripRef = useRef(null);

  useEffect(() => {
    const el = stripRef.current;
    if (el) {
      // Latest bucket is at the right edge; default scrollLeft=0 hides it
      // behind overflow. Pin to the right so users see the newest data first.
      el.scrollLeft = el.scrollWidth;
    }
  }, [history]);

  if (!Array.isArray(history) || history.length === 0) {
    return (
      <Text type='secondary' size='small'>
        {t('暂无数据')}
      </Text>
    );
  }

  return (
    <div className='history-squares' ref={stripRef}>
      {history.map((bucket, idx) => {
        const status = bucket?.status;
        let cls = 'unknown';
        let resultLabel = t('暂无数据');
        if (status === 1) {
          cls = 'success';
          resultLabel = t('正常');
        } else if (status === 0) {
          cls = 'error';
          resultLabel = t('全部失败');
        }
        const tsStart = bucket?.ts_start;
        const tsEnd = bucket?.ts_end;
        const sampleSize = bucket?.sample_size || 0;
        const popoverContent = (
          <div className='history-popover'>
            <div className='popover-row'>
              <span className='popover-label'>{t('时间')}</span>
              <span className='popover-value mono'>
                {formatTimestamp(tsStart)} - {formatTimestamp(tsEnd)}
              </span>
            </div>
            <div className='popover-row'>
              <span className='popover-label'>{t('结果')}</span>
              <span
                className={`popover-value ${status === 1 ? 'ok' : status === 0 ? 'bad' : ''}`}
              >
                {resultLabel}
              </span>
            </div>
            <div className='popover-row'>
              <span className='popover-label'>{t('样本数')}</span>
              <span className='popover-value mono'>{sampleSize}</span>
            </div>
          </div>
        );
        return (
          <Popover
            key={idx}
            content={popoverContent}
            trigger='click'
            position='top'
            showArrow
          >
            <span
              className={`history-square clickable ${cls}`}
              style={squareStyle(cls)}
              role='button'
              tabIndex={0}
            />
          </Popover>
        );
      })}
    </div>
  );
};

const ModelStatusPage = () => {
  const { t } = useTranslation();

  const [view, setView] = useState('admin');
  const [models, setModels] = useState([]);
  const [loading, setLoading] = useState(true);
  const [loaded, setLoaded] = useState(false);
  const [lastUpdate, setLastUpdate] = useState(null);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState(new Set(STATUS_FILTER_OPTIONS));
  const [expandedModels, setExpandedModels] = useState(new Set());

  const formatLatency = useCallback(
    (ms) => {
      if (!ms || ms <= 0) return '-';
      if (ms < 1000) return `${ms} ${t('毫秒')}`;
      return `${(ms / 1000).toFixed(1)} ${t('秒')}`;
    },
    [t],
  );

  const getChannelTypeLabel = useCallback(
    (typeId) => channelTypeLabelMap[typeId] || t('未知类型'),
    [t],
  );

  const loadModels = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/model-uptime/status');
      const { success, data } = res.data || {};
      if (success && data) {
        setView(data.view || 'admin');
        setModels(Array.isArray(data.models) ? data.models : []);
        setLastUpdate(data.updated_at ? new Date(data.updated_at * 1000) : new Date());
      } else {
        setModels([]);
      }
    } catch (e) {
      setModels([]);
    } finally {
      setLoading(false);
      setLoaded(true);
    }
  }, []);

  useEffect(() => {
    loadModels();
    const interval = setInterval(loadModels, REFRESH_INTERVAL_MS);
    return () => clearInterval(interval);
  }, [loadModels]);

  const filteredModels = useMemo(() => {
    const q = search.trim().toLowerCase();
    return models.filter((m) => {
      if (!statusFilter.has(m.status || 'unknown')) return false;
      if (q && !(m.model || '').toLowerCase().includes(q)) return false;
      return true;
    });
  }, [models, search, statusFilter]);

  const stats = useMemo(() => {
    const counts = { normal: 0, degraded: 0, error: 0, unknown: 0 };
    models.forEach((m) => {
      const s = m.status || 'unknown';
      if (counts[s] !== undefined) counts[s] += 1;
    });
    return counts;
  }, [models]);

  const toggleStatusFilter = (status) => {
    setStatusFilter((prev) => {
      const next = new Set(prev);
      if (next.has(status)) {
        next.delete(status);
      } else {
        next.add(status);
      }
      // never allow all-empty; reset to all if user unchecks the last one
      if (next.size === 0) {
        return new Set(STATUS_FILTER_OPTIONS);
      }
      return next;
    });
  };

  const toggleExpand = (modelName) => {
    setExpandedModels((prev) => {
      const next = new Set(prev);
      if (next.has(modelName)) {
        next.delete(modelName);
      } else {
        next.add(modelName);
      }
      return next;
    });
  };

  const renderStatusPill = (status) => {
    if (status === 'normal') {
      return (
        <span className='status-pill normal'>
          <CheckCircle size={12} />
          {t('正常')}
        </span>
      );
    }
    if (status === 'degraded') {
      return (
        <span className='status-pill warning'>
          <AlertTriangle size={12} />
          {t('降级')}
        </span>
      );
    }
    if (status === 'error') {
      return (
        <span className='status-pill error'>
          <AlertTriangle size={12} />
          {t('故障')}
        </span>
      );
    }
    return (
      <span className='status-pill unknown'>
        <Activity size={12} />
        {t('暂无数据')}
      </span>
    );
  };

  const channelColumns = [
    {
      title: t('Channel'),
      dataIndex: 'name',
      key: 'name',
      render: (text, record) => (
        <div className='channel-name-cell'>
          <span className='channel-name-text'>{text || `#${record.id}`}</span>
          <Tag size='small' color='white' className='channel-type-chip'>
            {getChannelTypeLabel(record.type)}
          </Tag>
        </div>
      ),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      key: 'status',
      render: (status) => renderStatusPill(status),
    },
    {
      title: t('状态码'),
      dataIndex: 'status_code',
      key: 'status_code',
      render: (code) => (code ? <span className='mono'>{code}</span> : '-'),
    },
    {
      title: t('延迟'),
      dataIndex: 'latency_ms',
      key: 'latency_ms',
      render: (ms) => <span className='mono'>{formatLatency(ms)}</span>,
    },
    {
      title: t('最近检查'),
      dataIndex: 'last_check',
      key: 'last_check',
      render: (ts) => <span className='mono'>{formatTimestamp(ts)}</span>,
    },
    {
      title: t('错误'),
      dataIndex: 'error',
      key: 'error',
      render: (err) =>
        err ? (
          <Popover
            content={<div className='channel-error-popover'>{err}</div>}
            position='topLeft'
          >
            <span className='channel-error-cell'>{err}</span>
          </Popover>
        ) : (
          '-'
        ),
    },
  ];

  const renderModelRow = (m) => {
    const isExpanded = expandedModels.has(m.model);
    const isAdmin = view === 'admin';
    const status = m.status || 'unknown';
    const uptime =
      typeof m.uptime_24h === 'number' && Number.isFinite(m.uptime_24h)
        ? `${m.uptime_24h.toFixed(1)}%`
        : '-';

    return (
      <Card key={m.model} className={`model-card status-${status}`} bodyStyle={{ padding: 12 }}>
        <div
          className={`model-row ${isAdmin ? 'expandable' : ''}`}
          onClick={isAdmin ? () => toggleExpand(m.model) : undefined}
          role={isAdmin ? 'button' : undefined}
          tabIndex={isAdmin ? 0 : undefined}
        >
          <div className='model-row-left'>
            {isAdmin ? (
              isExpanded ? (
                <ChevronDown size={14} className='expand-icon' />
              ) : (
                <ChevronRight size={14} className='expand-icon' />
              )
            ) : null}
            <span className='model-name'>{m.model}</span>
            {renderStatusPill(status)}
            {isAdmin && m.channel_count > 0 && (
              <span className='channel-count-chip'>
                {m.healthy_count}/{m.channel_count} {t('个渠道')}
              </span>
            )}
          </div>
          <div className='model-row-right'>
            <div className='uptime-cell'>
              <span className='uptime-label'>{t('24小时可用率')}</span>
              <span className='uptime-value'>{uptime}</span>
            </div>
            <div className='history-cell'><HistoryStrip history={m.history} /></div>
          </div>
        </div>

        {isAdmin && isExpanded && Array.isArray(m.channels) && m.channels.length > 0 && (
          <div className='channels-table-wrap' onClick={(e) => e.stopPropagation()}>
            <Table
              columns={channelColumns}
              dataSource={m.channels}
              rowKey='id'
              pagination={false}
              size='small'
            />
          </div>
        )}
      </Card>
    );
  };

  return (
    <div className='mt-[60px] px-2'>
      <div className='model-status-page'>
        <div className='msp-header'>
          <div className='msp-header-left'>
            <Title heading={2} className='msp-title'>
              <Boxes size={20} style={{ verticalAlign: '-3px', marginRight: 6 }} />
              {t('模型状态')}
            </Title>
            <Text type='secondary' className='msp-subtitle'>
              {t('查看模型在所有承载渠道上的近 24 小时可用性')}
            </Text>
            <div className='msp-stats-row'>
              <div className='stat-cell'>
                <div className='stat-label'>{t('正常模型')}</div>
                <div className='stat-value'>
                  <span className='stat-num normal'>{stats.normal}</span>
                </div>
              </div>
              <div className='stat-cell'>
                <div className='stat-label'>{t('降级模型')}</div>
                <div className='stat-value'>
                  <span className='stat-num warning'>{stats.degraded}</span>
                </div>
              </div>
              <div className='stat-cell'>
                <div className='stat-label'>{t('故障模型')}</div>
                <div className='stat-value'>
                  <span className='stat-num error'>{stats.error}</span>
                </div>
              </div>
              <div className='stat-cell'>
                <div className='stat-label'>{t('未知模型')}</div>
                <div className='stat-value'>
                  <span className='stat-num'>{stats.unknown}</span>
                </div>
              </div>
              <div className='stat-cell wide'>
                <div className='stat-label'>{t('更新于')}</div>
                <div className='stat-value mono'>
                  {lastUpdate
                    ? formatTimestamp(Math.floor(lastUpdate.getTime() / 1000))
                    : '-'}
                </div>
              </div>
            </div>
          </div>
          <div className='msp-header-right'>
            <Button
              icon={<RefreshCw size={14} />}
              onClick={loadModels}
              loading={loading}
            >
              {t('立即刷新')}
            </Button>
          </div>
        </div>

        <div className='msp-toolbar'>
          <Input
            prefix={<Search size={14} style={{ marginRight: 6 }} />}
            placeholder={t('搜索模型')}
            value={search}
            onChange={(v) => setSearch(v)}
            showClear
            className='msp-search'
          />
          <div className='msp-filter-chips'>
            {STATUS_FILTER_OPTIONS.map((s) => {
              const labelMap = {
                normal: t('正常'),
                degraded: t('降级'),
                error: t('故障'),
                unknown: t('未知'),
              };
              const active = statusFilter.has(s);
              return (
                <Tag
                  key={s}
                  size='large'
                  className={`filter-chip ${active ? 'active' : ''} ${s}`}
                  onClick={() => toggleStatusFilter(s)}
                >
                  {labelMap[s]} ({stats[s] || 0})
                </Tag>
              );
            })}
          </div>
        </div>

        {loaded && filteredModels.length === 0 ? (
          <Card className='empty-card' bodyStyle={{ padding: 32 }}>
            <Empty description={loaded && models.length === 0 ? t('暂无模型') : t('暂无数据')} />
          </Card>
        ) : (
          <div className='model-list'>{filteredModels.map(renderModelRow)}</div>
        )}

        <style>{`
          .model-status-page { padding: 24px; }

          .msp-header {
            display: flex;
            align-items: flex-start;
            justify-content: space-between;
            gap: 24px;
            margin-bottom: 16px;
          }
          .msp-header-left { flex: 1; min-width: 0; }
          .msp-header-right { flex-shrink: 0; }
          .msp-title { margin-bottom: 8px !important; }
          .msp-subtitle { font-size: 13px; }
          .msp-stats-row {
            display: flex;
            align-items: flex-end;
            gap: 32px;
            margin-top: 16px;
            flex-wrap: wrap;
          }
          .stat-cell { display: flex; flex-direction: column; gap: 4px; }
          .stat-cell.wide { min-width: 180px; }
          .stat-label {
            font-size: 11px;
            color: var(--semi-color-text-2);
            letter-spacing: 0.04em;
          }
          .stat-value {
            font-size: 18px;
            font-weight: 600;
            color: var(--semi-color-text-0);
            font-variant-numeric: tabular-nums;
          }
          .stat-num.normal { color: var(--semi-color-success); }
          .stat-num.warning { color: var(--semi-color-warning); }
          .stat-num.error { color: var(--semi-color-danger); }
          .stat-value.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px; }

          .msp-toolbar {
            display: flex;
            align-items: center;
            gap: 12px;
            margin-bottom: 16px;
            flex-wrap: wrap;
          }
          .msp-search { width: 280px; padding: 0 5px; }
          .msp-filter-chips { display: flex; gap: 8px; flex-wrap: wrap; }
          .filter-chip { cursor: pointer; user-select: none; opacity: 0.55; transition: opacity 0.15s; }
          .filter-chip.active { opacity: 1; }
          .filter-chip.normal.active { background-color: var(--semi-color-success-light-default) !important; color: var(--semi-color-success) !important; }
          .filter-chip.degraded.active { background-color: var(--semi-color-warning-light-default) !important; color: var(--semi-color-warning) !important; }
          .filter-chip.error.active { background-color: var(--semi-color-danger-light-default) !important; color: var(--semi-color-danger) !important; }
          .filter-chip.unknown.active { background-color: var(--semi-color-fill-1) !important; }

          .model-list { display: flex; flex-direction: column; gap: 8px; }

          .model-card {
            border: 1px solid var(--semi-color-border);
            border-radius: 8px;
          }
          .model-card.status-error { background-color: var(--semi-color-danger-light-default); border-color: var(--semi-color-danger-light-active); }
          .model-card.status-degraded { background-color: var(--semi-color-warning-light-default); border-color: var(--semi-color-warning-light-active); }
          .model-card.status-unknown { background-color: var(--semi-color-fill-0); }

          .model-row {
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 16px;
          }
          .model-row.expandable {
            cursor: pointer;
          }
          .model-row-left {
            display: flex;
            align-items: center;
            gap: 12px;
            flex: 1;
            min-width: 0;
            flex-wrap: wrap;
          }
          .model-row-right {
            display: flex;
            align-items: center;
            gap: 20px;
            flex-shrink: 0;
          }
          .expand-icon {
            color: var(--semi-color-text-2);
            flex-shrink: 0;
          }
          .model-name {
            font-size: 14px;
            font-weight: 600;
            color: var(--semi-color-text-0);
            background-color: var(--semi-color-fill-0);
            padding: 2px 8px;
            border-radius: 4px;
            font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
          }
          .channel-count-chip {
            font-size: 11px;
            color: var(--semi-color-text-2);
            padding: 2px 6px;
            background-color: var(--semi-color-fill-1);
            border-radius: 4px;
          }
          .uptime-cell {
            display: flex;
            flex-direction: column;
            align-items: flex-end;
            gap: 2px;
          }
          .uptime-label {
            font-size: 10px;
            color: var(--semi-color-text-2);
            letter-spacing: 0.04em;
          }
          .uptime-value {
            font-size: 13px;
            font-weight: 600;
            font-variant-numeric: tabular-nums;
            color: var(--semi-color-text-0);
          }
          .history-cell { min-width: 0; }

          .status-pill {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            padding: 3px 10px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: 500;
            line-height: 1;
            border: 1px solid transparent;
            white-space: nowrap;
          }
          .status-pill.normal {
            background-color: var(--semi-color-success-light-default);
            color: var(--semi-color-success);
            border-color: var(--semi-color-success-light-active);
          }
          .status-pill.warning {
            background-color: var(--semi-color-warning-light-default);
            color: var(--semi-color-warning);
            border-color: var(--semi-color-warning-light-active);
          }
          .status-pill.error {
            background-color: var(--semi-color-danger-light-default);
            color: var(--semi-color-danger);
            border-color: var(--semi-color-danger-light-active);
          }
          .status-pill.unknown {
            background-color: var(--semi-color-fill-0);
            color: var(--semi-color-text-2);
          }

          .history-squares {
            display: flex;
            flex-wrap: nowrap;
            gap: 2px;
            width: 420px;
            overflow: hidden;
          }
          .history-squares::-webkit-scrollbar { display: none; }
          .history-square { flex-shrink: 0; }
          .history-square.clickable {
            cursor: pointer;
            transition: transform 0.1s ease;
          }
          .history-square.clickable:hover { transform: scaleY(1.15); }
          .history-square.success { background-color: var(--semi-color-success) !important; }
          .history-square.error { background-color: var(--semi-color-danger) !important; }
          .history-square.unknown { background-color: var(--semi-color-fill-2) !important; }

          .history-popover {
            display: flex;
            flex-direction: column;
            gap: 6px;
            min-width: 240px;
            max-width: 360px;
            padding: 4px 2px;
            font-size: 12px;
          }
          .popover-row { display: flex; align-items: flex-start; gap: 12px; }
          .popover-label {
            color: var(--semi-color-text-2);
            flex-shrink: 0;
            min-width: 56px;
          }
          .popover-value {
            color: var(--semi-color-text-0);
            word-break: break-word;
          }
          .popover-value.mono {
            font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
            font-variant-numeric: tabular-nums;
          }
          .popover-value.ok { color: var(--semi-color-success); }
          .popover-value.bad { color: var(--semi-color-danger); }

          .channels-table-wrap {
            margin-top: 12px;
            padding-top: 12px;
            border-top: 1px dashed var(--semi-color-border);
            cursor: default;
          }
          .channel-name-cell {
            display: inline-flex;
            align-items: center;
            gap: 6px;
          }
          .channel-name-text {
            font-weight: 600;
            color: var(--semi-color-text-0);
          }
          .channel-type-chip {
            text-transform: lowercase;
            font-size: 11px !important;
          }
          .channel-error-cell {
            display: inline-block;
            max-width: 260px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            color: var(--semi-color-danger);
            font-size: 11px;
            cursor: help;
          }
          .channel-error-popover {
            max-width: 420px;
            color: var(--semi-color-danger);
            font-size: 12px;
            line-height: 1.5;
            word-break: break-word;
          }
          .mono {
            font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
            font-variant-numeric: tabular-nums;
          }

          .empty-card { display: flex; justify-content: center; }
        `}</style>
      </div>
    </div>
  );
};

export default ModelStatusPage;
