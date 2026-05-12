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

import React, { useState, useEffect, useMemo, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, Typography, Button, Tag, Empty, Popover } from '@douyinfe/semi-ui';
import {
  Activity,
  Box,
  CheckCircle,
  AlertTriangle,
  RefreshCw,
  Sparkles,
  Cpu,
  Cloud,
} from 'lucide-react';
import { API } from '../../helpers';
import { CHANNEL_OPTIONS } from '../../constants';

const { Title, Text } = Typography;

const REFRESH_INTERVAL_MS = 30 * 1000;

// Resolved colours for the history squares. Set inline so Semi UI's Popover
// cannot strip them via cloneElement.
const SQUARE_COLORS = {
  success: 'var(--semi-color-success)',
  error: 'var(--semi-color-danger)',
  unknown: 'var(--semi-color-fill-2)',
};

const squareStyle = (cls) => ({
  display: 'inline-block',
  width: 10,
  height: 10,
  borderRadius: 2,
  backgroundColor: SQUARE_COLORS[cls] || SQUARE_COLORS.unknown,
  verticalAlign: 'top',
});

// Mapping of channel_type → display label used in the small tag next to the
// channel name (e.g. the lowercase "claude" / "openai" chip in the design).
const channelTypeLabelMap = CHANNEL_OPTIONS.reduce((acc, opt) => {
  acc[opt.value] = opt.label;
  return acc;
}, {});

// Provider categories shown as group headers. The first match wins; any
// channel_type not listed falls into the trailing 'other' bucket.
const PROVIDER_CATEGORIES = [
  { key: 'claude', titleKey: 'Claude / Anthropic', chip: 'claude', icon: Box, types: [14, 33] },
  { key: 'openai', titleKey: 'OpenAI', chip: 'openai', icon: Sparkles, types: [1, 3, 6, 7, 50, 51] },
  { key: 'gemini', titleKey: 'Google Gemini', chip: 'gemini', icon: Cloud, types: [24, 41] },
  { key: 'deepseek', titleKey: 'DeepSeek', chip: 'deepseek', icon: Cpu, types: [43] },
  { key: 'qwen', titleKey: 'Qwen', chip: 'qwen', icon: Cpu, types: [17] },
  { key: 'moonshot', titleKey: 'Moonshot', chip: 'moonshot', icon: Cpu, types: [25] },
  { key: 'other', titleKey: 'Other', chip: 'other', icon: Activity, types: [] },
];

const findCategory = (channelType) => {
  for (const cat of PROVIDER_CATEGORIES) {
    if (cat.types.includes(channelType)) return cat;
  }
  return PROVIDER_CATEGORIES[PROVIDER_CATEGORIES.length - 1];
};

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

const StatusPage = () => {
  const { t } = useTranslation();
  const [view, setView] = useState('admin');
  const [services, setServices] = useState([]);
  const [intervalMinutes, setIntervalMinutes] = useState(5);
  const [loading, setLoading] = useState(true);
  const [loaded, setLoaded] = useState(false);
  const [lastUpdate, setLastUpdate] = useState(null);

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

  const loadServices = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/channel-uptime/status');
      const { success, data } = res.data || {};
      if (success && data) {
        setView(data.view || 'admin');
        setServices(Array.isArray(data.services) ? data.services : []);
        if (data.interval_minutes) setIntervalMinutes(data.interval_minutes);
        setLastUpdate(data.updated_at ? new Date(data.updated_at * 1000) : new Date());
      } else {
        setServices([]);
      }
    } catch (e) {
      setServices([]);
    } finally {
      setLoading(false);
      setLoaded(true);
    }
  }, []);

  useEffect(() => {
    loadServices();
    const interval = setInterval(loadServices, REFRESH_INTERVAL_MS);
    return () => clearInterval(interval);
  }, [loadServices]);

  const groupedServices = useMemo(() => {
    if (view !== 'admin') return [];
    const buckets = new Map();
    PROVIDER_CATEGORIES.forEach((cat) => buckets.set(cat.key, { category: cat, items: [] }));
    services.forEach((s) => {
      const cat = findCategory(s.type);
      buckets.get(cat.key).items.push(s);
    });
    return Array.from(buckets.values())
      .filter((b) => b.items.length > 0)
      .map((b) => ({
        ...b,
        items: [...b.items].sort((a, b) => (a.name || '').localeCompare(b.name || '')),
      }));
  }, [services, view]);

  const stats = useMemo(() => {
    const counts = { normal: 0, error: 0, total: services.length, totalLatencyMs: 0, withLatency: 0 };
    services.forEach((s) => {
      if (s.status === 'normal') counts.normal += 1;
      if (s.status === 'error' || s.status === 'degraded') counts.error += 1;
      if (s.latency_ms && s.latency_ms > 0) {
        counts.totalLatencyMs += s.latency_ms;
        counts.withLatency += 1;
      }
    });
    counts.avgLatencyMs =
      counts.withLatency > 0 ? Math.round(counts.totalLatencyMs / counts.withLatency) : 0;
    return counts;
  }, [services]);

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
          {t('部分异常')}
        </span>
      );
    }
    if (status === 'error') {
      return (
        <span className='status-pill error'>
          <AlertTriangle size={12} />
          {t('异常')}
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

  const renderAdminHistorySquares = (history) => {
    if (!Array.isArray(history) || history.length === 0) {
      return (
        <Text type='secondary' size='small'>
          {t('暂无数据')}
        </Text>
      );
    }
    // Render oldest → newest, left to right. Backend returns newest first, so reverse a copy.
    const ordered = [...history].reverse();
    return (
      <div className='history-squares'>
        {ordered.map((entry, idx) => {
          const status = entry?.status;
          let cls = 'unknown';
          if (status === 1) cls = 'success';
          else if (status === 0) cls = 'error';
          const popoverContent = (
            <div className='history-popover'>
              <div className='popover-row'>
                <span className='popover-label'>{t('时间')}</span>
                <span className='popover-value mono'>{formatTimestamp(entry?.ts)}</span>
              </div>
              {status === 1 || status === 0 ? (
                <>
                  <div className='popover-row'>
                    <span className='popover-label'>{t('结果')}</span>
                    <span
                      className={`popover-value ${status === 1 ? 'ok' : 'bad'}`}
                    >
                      {status === 1 ? t('成功') : t('失败')}
                    </span>
                  </div>
                  {entry?.status_code > 0 && (
                    <div className='popover-row'>
                      <span className='popover-label'>{t('状态码')}</span>
                      <span className='popover-value mono'>{entry.status_code}</span>
                    </div>
                  )}
                  {entry?.latency_ms > 0 && (
                    <div className='popover-row'>
                      <span className='popover-label'>{t('延迟')}</span>
                      <span className='popover-value mono'>
                        {formatLatency(entry.latency_ms)}
                      </span>
                    </div>
                  )}
                  {status === 0 && entry?.error && (
                    <div className='popover-row error-row'>
                      <span className='popover-label'>{t('错误')}</span>
                      <span className='popover-value popover-error'>{entry.error}</span>
                    </div>
                  )}
                </>
              ) : (
                <div className='popover-row'>
                  <span className='popover-label'>{t('结果')}</span>
                  <span className='popover-value'>{t('暂无数据')}</span>
                </div>
              )}
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

  const renderPublicHistorySquares = (history) => {
    if (!Array.isArray(history) || history.length === 0) {
      return (
        <Text type='secondary' size='small'>
          {t('暂无数据')}
        </Text>
      );
    }
    const ordered = [...history];
    return (
      <div className='history-squares'>
        {ordered.map((entry, idx) => {
          const status = entry?.status;
          let cls = 'unknown';
          let resultLabel = t('暂无数据');
          if (status === 1) {
            cls = 'success';
            resultLabel = t('正常');
          } else if (status === 0) {
            cls = 'error';
            resultLabel = t('全部失败');
          }
          const tsStart = entry?.ts_start;
          const tsEnd = entry?.ts_end;
          const popoverContent = (
            <div className='history-popover'>
              <div className='popover-row'>
                <span className='popover-label'>{t('时间区间')}</span>
                <span className='popover-value mono'>
                  {formatTimestamp(tsStart)} → {formatTimestamp(tsEnd)}
                </span>
              </div>
              <div className='popover-row'>
                <span className='popover-label'>{t('结果')}</span>
                <span
                  className={`popover-value ${
                    status === 1 ? 'ok' : status === 0 ? 'bad' : ''
                  }`}
                >
                  {resultLabel}
                </span>
              </div>
              <div className='popover-row'>
                <span className='popover-label'>{t('采样次数')}</span>
                <span className='popover-value mono'>{entry?.sample_size || 0}</span>
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

  const renderServiceCard = (service) => {
    const lastCheck = service.last_check;
    const nextRun = lastCheck ? lastCheck + intervalMinutes * 60 : 0;
    const typeChip = (channelTypeLabelMap[service.type] || '').toLowerCase().split(/[\s/]/)[0] || 'other';
    return (
      <Card key={`s-${service.id}`} className='service-card' bodyStyle={{ padding: 16 }}>
        <div className='service-header'>
          <div className='service-name-block'>
            <span className='service-name'>{service.name || `#${service.id}`}</span>
            <Tag size='small' color='grey' className='service-type-chip'>
              {typeChip}
            </Tag>
          </div>
          {renderStatusPill(service.status)}
        </div>
        <div className='service-meta'>
          <span className='meta-item'>
            <span className='meta-label'>{t('上次检查')}</span>
            <span className='meta-value'>{formatTimestamp(lastCheck)}</span>
          </span>
          <span className='meta-divider'>|</span>
          <span className='meta-item'>
            <span className='meta-label'>{t('延迟')}</span>
            <span className='meta-value'>{formatLatency(service.latency_ms)}</span>
          </span>
          {service.status_code > 0 && (
            <>
              <span className='meta-divider'>|</span>
              <span className='meta-item'>
                <span className='meta-label'>{t('状态码')}</span>
                <span className='meta-value'>{service.status_code}</span>
              </span>
            </>
          )}
          <span className='meta-divider'>|</span>
          <span className='meta-item'>
            <span className='meta-label'>{t('间隔')}:</span>
            <span className='meta-value'>{intervalMinutes}{t('分钟')}</span>
          </span>
          <span className='meta-divider'>|</span>
          <span className='meta-item'>
            <span className='meta-label'>{t('下次运行')}</span>
            <span className='meta-value'>{formatTimestamp(nextRun)}</span>
          </span>
        </div>
        <div className='history-block'>
          <Text type='secondary' size='small' className='history-title'>
            {t('最近检查')}
          </Text>
          {renderAdminHistorySquares(service.history)}
        </div>
      </Card>
    );
  };

  const renderPublicCard = (service, index) => {
    return (
      <Card
        key={`pub-${service.type}-${index}`}
        className='service-card'
        bodyStyle={{ padding: 16 }}
      >
        <div className='service-header'>
          <div className='service-name-block'>
            <span className='service-name'>{getChannelTypeLabel(service.type)}</span>
          </div>
          {renderStatusPill(service.status)}
        </div>
        <div className='service-meta'>
          <span className='meta-item'>
            <span className='meta-label'>{t('24小时可用率')}</span>
            <span className='meta-value'>
              {service.uptime_24h !== null && service.uptime_24h !== undefined
                ? `${service.uptime_24h.toFixed(2)}%`
                : '-'}
            </span>
          </span>
          <span className='meta-divider'>|</span>
          <span className='meta-item'>
            <span className='meta-label'>{t('间隔')}:</span>
            <span className='meta-value'>{intervalMinutes}{t('分钟')}</span>
          </span>
        </div>
        <div className='history-block'>
          <Text type='secondary' size='small' className='history-title'>
            {t('最近检查')}
          </Text>
          {renderPublicHistorySquares(service.history)}
        </div>
      </Card>
    );
  };

  const isAdmin = view === 'admin';

  return (
    <div className='mt-[60px] px-2'>
        <div className='uptime-page'>
          <div className='uptime-header'>
        <div className='uptime-header-left'>
          <Title heading={2} className='uptime-title'>
            {t('供应商状态')}
          </Title>
          <Text type='secondary' className='uptime-subtitle'>
            {t('实时查看各供应商的可用性、延迟与最近事件。')}
          </Text>
          <div className='uptime-stats-row'>
            <div className='stat-cell'>
              <div className='stat-label'>{t('正常')}</div>
              <div className='stat-value'>
                <span className='stat-num normal'>{stats.normal}</span>
                <span className='stat-total'>/{stats.total}</span>
              </div>
            </div>
            <div className='stat-cell'>
              <div className='stat-label'>{t('异常')}</div>
              <div className='stat-value'>
                <span className='stat-num error'>{stats.error}</span>
              </div>
            </div>
            <div className='stat-cell'>
              <div className='stat-label'>{t('延迟')}</div>
              <div className='stat-value'>
                <span className='stat-num'>{stats.avgLatencyMs}</span>
                <span className='stat-unit'> ms</span>
              </div>
            </div>
            <div className='stat-cell wide'>
              <div className='stat-label'>{t('更新于')}</div>
              <div className='stat-value mono'>
                {lastUpdate ? formatTimestamp(Math.floor(lastUpdate.getTime() / 1000)) : '-'}
              </div>
            </div>
          </div>
        </div>
        <div className='uptime-header-right'>
          <Button
            icon={<RefreshCw size={14} />}
            onClick={loadServices}
            loading={loading}
          >
            {t('立即刷新')}
          </Button>
        </div>
      </div>

      {loaded && services.length === 0 ? (
        <Card className='empty-card' bodyStyle={{ padding: 32 }}>
          <Empty description={t('暂无数据')} />
        </Card>
      ) : isAdmin ? (
        groupedServices.map(({ category, items }) => {
          const Icon = category.icon;
          return (
            <div key={category.key} className='provider-group'>
              <div className='group-header'>
                <div className='group-header-left'>
                  <Icon size={14} className='group-icon' />
                  <span className='group-title'>{t(category.titleKey)}</span>
                </div>
                <span className='group-count'>
                  {items.length} {t('个供应商')}
                </span>
              </div>
              <div className='group-cards'>
                {items.map((service) => renderServiceCard(service))}
              </div>
            </div>
          );
        })
      ) : (
        <div className='group-cards'>
          {services.map((service, idx) => renderPublicCard(service, idx))}
        </div>
      )}

      <style>{`
        .uptime-page {
          padding: 24px;
        }
        .uptime-header {
          display: flex;
          align-items: flex-start;
          justify-content: space-between;
          gap: 24px;
          margin-bottom: 24px;
        }
        .uptime-header-left { flex: 1; min-width: 0; }
        .uptime-title { margin-bottom: 8px !important; }
        .uptime-subtitle { font-size: 13px; }
        .uptime-stats-row {
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
        .stat-num.error { color: var(--semi-color-danger); }
        .stat-total {
          color: var(--semi-color-text-2);
          font-weight: 400;
          font-size: 13px;
          margin-left: 2px;
        }
        .stat-unit {
          font-size: 12px;
          color: var(--semi-color-text-2);
          font-weight: 400;
          margin-left: 2px;
        }
        .stat-value.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px; }
        .uptime-header-right { flex-shrink: 0; }

        .provider-group { margin-bottom: 24px; }
        .group-header {
          display: flex;
          align-items: center;
          justify-content: space-between;
          padding: 8px 4px;
          margin-bottom: 8px;
        }
        .group-header-left {
          display: flex;
          align-items: center;
          gap: 8px;
        }
        .group-icon { color: var(--semi-color-text-2); }
        .group-title {
          font-size: 11px;
          font-weight: 600;
          letter-spacing: 0.08em;
          text-transform: uppercase;
          color: var(--semi-color-text-2);
        }
        .group-count {
          font-size: 12px;
          color: var(--semi-color-text-2);
        }

        .group-cards {
          display: flex;
          flex-direction: column;
          gap: 12px;
        }

        .service-card {
          border: 1px solid var(--semi-color-border);
          border-radius: 8px;
        }
        .service-header {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 12px;
          margin-bottom: 8px;
        }
        .service-name-block {
          display: flex;
          align-items: center;
          gap: 8px;
          min-width: 0;
        }
        .service-name {
          font-size: 14px;
          font-weight: 600;
          color: var(--semi-color-text-0);
          background-color: var(--semi-color-fill-0);
          padding: 2px 8px;
          border-radius: 4px;
        }
        .service-type-chip {
          text-transform: lowercase;
          font-size: 11px !important;
        }
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

        .service-meta {
          display: flex;
          align-items: center;
          flex-wrap: wrap;
          gap: 6px 8px;
          font-size: 12px;
          color: var(--semi-color-text-1);
          margin-bottom: 12px;
        }
        .meta-item { display: inline-flex; align-items: center; gap: 6px; }
        .meta-label { color: var(--semi-color-text-2); }
        .meta-value {
          color: var(--semi-color-text-0);
          font-variant-numeric: tabular-nums;
        }
        .meta-divider { color: var(--semi-color-text-3); }

        .history-block {
          display: flex;
          flex-direction: column;
          gap: 6px;
        }
        .history-title {
          font-size: 11px !important;
        }
        .history-squares {
          display: flex;
          flex-wrap: wrap;
          gap: 3px;
        }
        .history-square {
          flex-shrink: 0;
        }
        .history-square.clickable {
          cursor: pointer;
          transition: transform 0.1s ease;
        }
        .history-square.clickable:hover {
          transform: scale(1.4);
        }
        .history-square.success { background-color: var(--semi-color-success); }
        .history-square.error { background-color: var(--semi-color-danger); }
        .history-square.unknown { background-color: var(--semi-color-fill-2); }

        .history-popover {
          display: flex;
          flex-direction: column;
          gap: 6px;
          min-width: 220px;
          max-width: 360px;
          padding: 4px 2px;
          font-size: 12px;
        }
        .popover-row {
          display: flex;
          align-items: flex-start;
          gap: 12px;
        }
        .popover-row.error-row { align-items: flex-start; }
        .popover-label {
          color: var(--semi-color-text-2);
          flex-shrink: 0;
          min-width: 64px;
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
        .popover-error {
          color: var(--semi-color-danger);
          font-size: 11px;
          line-height: 1.4;
        }

        .empty-card {
          display: flex;
          justify-content: center;
        }
          `}</style>
        </div>
      </div>
    );
  };

  export default StatusPage;
