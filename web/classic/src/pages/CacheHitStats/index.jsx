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

import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, DatePicker, Spin, Tabs, TabPane, Empty, Typography } from '@douyinfe/semi-ui';
import { BarChart2, RefreshCw } from 'lucide-react';
import { VChart } from '@visactor/react-vchart';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import { useActualTheme } from '../../context/Theme';
import { API, showError } from '../../helpers';

const { Title, Text } = Typography;
const CHART_CONFIG = { mode: 'desktop-browser' };

function fmtRate(r) {
  return r == null ? 0 : Number(Number(r).toFixed(2));
}

function MiniChart({ title, spec, mono, theme }) {
  return (
    <div style={{ 
      border: '1px solid var(--semi-color-border)', 
      borderRadius: 8, 
      padding: '8px 4px',
      overflow: 'hidden',
      width: '100%',
    }}>
      <div style={{
        fontSize: 12, fontWeight: 600, padding: '0 8px 4px',
        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
        fontFamily: mono ? 'monospace' : undefined,
      }} title={title}>{title}</div>
      <div style={{ 
        height: 260,
        width: '100%',
        overflow: 'hidden',
      }}>
        <VChart key={theme} spec={spec} option={CHART_CONFIG} />
      </div>
    </div>
  );
}

function makeBarSpec(values, xField, yField, isDark) {
  const themeColors = isDark
    ? ['#3b82f6', '#06b6d4', '#14b8a6', '#8b5cf6', '#ec4899', '#f59e0b']
    : ['#3b82f6', '#06b6d4', '#14b8a6', '#8b5cf6', '#ec4899', '#f59e0b'];
  
  // 截断X轴标签文本
  const truncateText = (text, maxLength = 10) => {
    if (!text) return text;
    if (text.length <= maxLength) return text;
    return text.substring(0, maxLength) + '...';
  };
  
  return {
    type: 'bar',
    data: [{ values }],
    xField,
    yField,
    seriesField: xField,
    padding: { left: 40, right: 20, top: 20, bottom: 40 },
    axes: [
      { 
        orient: 'left', 
        range: { min: 0, max: 100 },
        label: { style: { fill: isDark ? '#cbd5e1' : '#64748b' } },
        line: { style: { stroke: isDark ? '#334155' : '#e2e8f0' } },
        tickLine: { style: { stroke: isDark ? '#334155' : '#e2e8f0' } },
      },
      { 
        orient: 'bottom', 
        label: { 
          autoRotate: true,
          autoHide: true,
          style: { fill: isDark ? '#cbd5e1' : '#64748b', fontSize: 10 },
          formatMethod: (value) => truncateText(value),
        },
        line: { style: { stroke: isDark ? '#334155' : '#e2e8f0' } },
        tickLine: { style: { stroke: isDark ? '#334155' : '#e2e8f0' } },
      },
    ],
    tooltip: { 
      mark: { 
        content: [{ key: d => d[xField], value: d => `${d[yField]}%` }],
        style: { 
          backgroundColor: isDark ? '#1e293b' : '#ffffff',
          borderColor: isDark ? '#334155' : '#e2e8f0',
          color: isDark ? '#f8fafc' : '#0f172a',
        },
      },
    },
    legends: { visible: false },
    color: { type: 'ordinal', range: themeColors },
  };
}

export default function CacheHitStats() {
  const { t } = useTranslation();
  const actualTheme = useActualTheme();
  const [range, setRange] = useState('today');
  const [customRange, setCustomRange] = useState(null);
  // statsRows: from /api/cache_hit_stats/by_channel_model
  const [statsRows, setStatsRows] = useState([]);
  // uptimeModels: from /api/model-uptime/status — [{model, channels:[{id,name},...]}]
  const [uptimeModels, setUptimeModels] = useState([]);
  const [loading, setLoading] = useState(false);

  const buildParams = useCallback(() => {
    if (customRange && customRange[0] && customRange[1]) {
      return `start_time=${Math.floor(customRange[0] / 1000)}&end_time=${Math.floor(customRange[1] / 1000)}`;
    }
    return `range=${range}`;
  }, [range, customRange]);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const params = buildParams();
      const [statsRes, uptimeRes] = await Promise.all([
        API.get(`/api/cache_hit_stats/by_channel_model?${params}`),
        API.get('/api/model-uptime/status'),
      ]);
      if (statsRes.data.success) setStatsRows(statsRes.data.data.items || []);
      else showError(statsRes.data.message);
      if (uptimeRes.data.success) setUptimeModels(uptimeRes.data.data?.models || []);
    } catch (e) {
      showError(e.message);
    } finally {
      setLoading(false);
    }
  }, [buildParams]);

  useEffect(() => { fetchData(); }, [fetchData]);

  // 初始化 VChart Semi 主题，监听主题切换
  useEffect(() => {
    initVChartSemiTheme({
      isWatchingThemeSwitch: true,
    });
  }, []);

  // index: { channel_name -> { model_name -> hit_rate } }
  const statsByChannelModel = useMemo(() => {
    const idx = {};
    statsRows.forEach(r => {
      const ch = r.channel_name || `#${r.channel_id}`;
      if (!idx[ch]) idx[ch] = {};
      idx[ch][r.model_name] = fmtRate(r.hit_rate);
    });
    return idx;
  }, [statsRows]);

  // index: { model_name -> { channel_name -> hit_rate } }
  const statsByModelChannel = useMemo(() => {
    const idx = {};
    statsRows.forEach(r => {
      const ch = r.channel_name || `#${r.channel_id}`;
      if (!idx[r.model_name]) idx[r.model_name] = {};
      idx[r.model_name][ch] = fmtRate(r.hit_rate);
    });
    return idx;
  }, [statsRows]);

  // Tab1: 按供应商 — 每个渠道一个图表，X轴=模型，Y轴=命中率
  // 渠道列表从 uptimeModels 里收集所有出现过的渠道
  const channelCharts = useMemo(() => {
    // channel id -> { name, models: Set }
    const channelMap = {};
    uptimeModels.forEach(m => {
      (m.channels || []).forEach(ch => {
        if (!channelMap[ch.id]) channelMap[ch.id] = { name: ch.name || `#${ch.id}`, models: new Set() };
        channelMap[ch.id].models.add(m.model);
      });
    });
    const isDark = actualTheme === 'dark';
    return Object.values(channelMap).map(({ name: chName, models }) => ({
      chName,
      spec: makeBarSpec(
        [...models].sort().map(m => ({ model: m, hit_rate: statsByChannelModel[chName]?.[m] ?? 0 })),
        'model', 'hit_rate', isDark
      ),
    })).sort((a, b) => a.chName.localeCompare(b.chName));
  }, [uptimeModels, statsByChannelModel, actualTheme]);

  // Tab2: 按模型 — 每个模型一个图表，X轴=渠道，Y轴=命中率
  // 模型列表和每个模型的渠道列表来自 uptimeModels（与模型状态页一致）
  const modelCharts = useMemo(() => {
    const isDark = actualTheme === 'dark';
    return uptimeModels.map(m => {
      const channelNames = (m.channels || []).map(ch => ch.name || `#${ch.id}`);
      return {
        modelName: m.model,
        spec: makeBarSpec(
          channelNames.map(ch => ({ channel: ch, hit_rate: statsByModelChannel[m.model]?.[ch] ?? 0 })),
          'channel', 'hit_rate', isDark
        ),
      };
    });
  }, [uptimeModels, statsByModelChannel, actualTheme]);

  const summary = useMemo(() => {
    const totalRead = statsRows.reduce((s, r) => s + (r.cache_read || 0), 0);
    const totalAll = statsRows.reduce((s, r) => s + (r.total || 0), 0);
    const hitRate = totalAll > 0 ? (totalRead / totalAll * 100).toFixed(2) : null;
    const channelSet = new Set();
    uptimeModels.forEach(m => (m.channels || []).forEach(ch => channelSet.add(ch.id)));
    return {
      channels: channelSet.size,
      models: uptimeModels.length,
      hitRate,
    };
  }, [statsRows, uptimeModels]);

  const rangeButtons = [
    { key: 'today', label: t('今日') },
    { key: '7d', label: t('近7天') },
    { key: '30d', label: t('近30天') },
    { key: 'all', label: t('全部') },
  ];

  return (
    <div className='mt-[60px] px-2' style={{ 
      width: '100%', 
      overflow: 'hidden', 
      boxSizing: 'border-box',
    }}>
      <div className='chs-header'>
        <div className='chs-header-left'>
          <Title heading={2} className='chs-title'>
            <BarChart2 size={20} style={{ verticalAlign: '-3px', marginRight: 6 }} />
            {t('缓存命中率统计')}
          </Title>
          <Text type='secondary' className='chs-subtitle'>{t('查看各供应商和模型的缓存命中率分布')}</Text>
          <div className='chs-stats-row'>
            {[
              { label: t('供应商数'), value: summary.channels },
              { label: t('模型数'), value: summary.models },
              { label: t('整体命中率'), value: summary.hitRate != null ? `${summary.hitRate}%` : '—', primary: true },
            ].map(({ label, value, primary }) => (
              <div key={label} className='stat-cell'>
                <div className='stat-label'>{label}</div>
                <div className={`stat-value${primary ? ' primary' : ''}`}>{value}</div>
              </div>
            ))}
          </div>
        </div>
        <div className='chs-header-right'>
          <Button icon={<RefreshCw size={14} />} onClick={fetchData} loading={loading}>{t('刷新')}</Button>
        </div>
      </div>
      <div className='chs-toolbar'>
        {rangeButtons.map(({ key, label }) => (
          <Button key={key} type={range === key && !customRange ? 'primary' : 'tertiary'}
            onClick={() => { setRange(key); setCustomRange(null); }}>{label}</Button>
        ))}
        <DatePicker type="dateTimeRange" placeholder={[t('开始时间'), t('结束时间')]}
          value={customRange} onChange={(v) => { setCustomRange(v); setRange(''); }} style={{ width: 360 }} />
      </div>

      <Spin spinning={loading}>
        {!loading && statsRows.length === 0 ? (
          <Empty description={<Text type="secondary">{t('暂无数据')}</Text>} style={{ padding: '60px 0' }} />
        ) : (
          <Tabs defaultActiveKey="model">
            <TabPane tab={t('按模型')} itemKey="model">
              <div style={{ 
                display: 'grid', 
                gridTemplateColumns: 'repeat(3, 1fr)', 
                gap: 16,
                width: '100%',
                overflow: 'hidden',
                boxSizing: 'border-box',
              }}>
                {modelCharts.map(({ modelName, spec }) => (
                  <MiniChart key={`${modelName}-${actualTheme}`} title={modelName} spec={spec} mono theme={actualTheme} />
                ))}
              </div>
            </TabPane>
            <TabPane tab={t('按供应商')} itemKey="channel">
              <div style={{ 
                display: 'grid', 
                gridTemplateColumns: 'repeat(3, 1fr)', 
                gap: 16,
                width: '100%',
                overflow: 'hidden',
                boxSizing: 'border-box',
              }}>
                {channelCharts.map(({ chName, spec }) => (
                  <MiniChart key={`${chName}-${actualTheme}`} title={chName} spec={spec} theme={actualTheme} />
                ))}
              </div>
            </TabPane>
          </Tabs>
        )}
      </Spin>
      <style>{`
        .chs-header {
          display: flex;
          align-items: flex-start;
          justify-content: space-between;
          gap: 24px;
          margin-bottom: 20px;
        }
        .chs-header-left { flex: 1; min-width: 0; }
        .chs-header-right { flex-shrink: 0; }
        .chs-title { margin-bottom: 8px !important; }
        .chs-subtitle { font-size: 13px; }
        .chs-stats-row {
          display: flex;
          align-items: flex-end;
          gap: 32px;
          margin-top: 16px;
          flex-wrap: wrap;
        }
        .stat-cell { display: flex; flex-direction: column; gap: 4px; }
        .stat-label { font-size: 11px; color: var(--semi-color-text-2); letter-spacing: 0.04em; }
        .stat-value { font-size: 18px; font-weight: 600; color: var(--semi-color-text-0); font-variant-numeric: tabular-nums; }
        .stat-value.primary { color: var(--semi-color-primary); }
        .chs-toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 16px; }
      `}</style>
    </div>
  );
}
