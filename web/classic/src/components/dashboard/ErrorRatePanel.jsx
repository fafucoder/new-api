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
import {
  Card,
  Button,
  Select,
  DatePicker,
  Spin,
  Empty,
  Typography,
} from '@douyinfe/semi-ui';
import { Activity } from 'lucide-react';
import { VChart } from '@visactor/react-vchart';
import { API, showError } from '../../helpers';
import { isAdmin } from '../../helpers/utils';
import { useActualTheme } from '../../context/Theme';

const { Text } = Typography;

const WINDOWS = [
  { key: '5m', label: '最近5分钟' },
  { key: '30m', label: '最近30分钟' },
  { key: '1h', label: '最近1小时' },
  { key: '2h', label: '最近2小时' },
  { key: '1d', label: '最近1天' },
];

function fmtTime(unixSec, win) {
  const d = new Date(unixSec * 1000);
  const pad = (n) => String(n).padStart(2, '0');
  if (win === '1d' || win === 'custom')
    return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function makeChartSpec(values, isDark) {
  // 双轴混合图：柱状=该桶请求总数(成功+失败)，折线=该桶错误率%
  return {
    type: 'common',
    data: [{ id: 'trend', values }],
    series: [
      {
        type: 'bar',
        id: 'requests',
        dataId: 'trend',
        xField: 'time',
        yField: 'total',
        bar: {
          style: {
            fill: isDark ? '#60a5fa' : '#3b82f6',
            cornerRadius: 2,
          },
        },
        tooltip: {
          mark: {
            title: { value: '请求数' },
            content: [{ key: (d) => d.time, value: (d) => d.total }],
          },
        },
      },
      {
        type: 'line',
        id: 'errorRate',
        dataId: 'trend',
        xField: 'time',
        yField: 'error_rate',
        point: { visible: true, size: 4 },
        line: { style: { stroke: isDark ? '#f87171' : '#ef4444', lineWidth: 2 } },
        tooltip: {
          mark: {
            title: { value: '错误率' },
            content: [{ key: (d) => d.time, value: (d) => `${d.error_rate}%` }],
          },
        },
      },
    ],
    axes: [
      {
        orient: 'left',
        seriesId: ['requests'],
        min: 0,
        label: {
          style: { fill: isDark ? '#cbd5e1' : '#64748b', fontSize: 12 },
        },
        title: {
          visible: true,
          text: '请求数',
          style: { fill: isDark ? '#94a3b8' : '#64748b', fontSize: 11 },
        },
        line: { style: { stroke: isDark ? '#334155' : '#e2e8f0' } },
        grid: { style: { stroke: isDark ? '#1e293b' : '#f1f5f9' } },
      },
      {
        orient: 'right',
        seriesId: ['errorRate'],
        min: 0,
        max: 100,
        label: {
          formatMethod: (v) => `${v}%`,
          style: { fill: isDark ? '#cbd5e1' : '#64748b', fontSize: 12 },
        },
        title: {
          visible: true,
          text: '错误率',
          style: { fill: isDark ? '#94a3b8' : '#64748b', fontSize: 11 },
        },
        line: { style: { stroke: isDark ? '#334155' : '#e2e8f0' } },
        grid: { visible: false },
      },
      {
        orient: 'bottom',
        label: {
          autoHide: true,
          autoRotate: true,
          style: { fill: isDark ? '#cbd5e1' : '#64748b', fontSize: 10 },
        },
        line: { style: { stroke: isDark ? '#334155' : '#e2e8f0' } },
      },
    ],
    legends: {
      visible: true,
      orient: 'top',
      position: 'end',
      item: {
        label: { style: { fill: isDark ? '#cbd5e1' : '#64748b' } },
      },
    },
  };
}

const ErrorRatePanel = ({ CARD_PROPS, CHART_CONFIG, FLEX_CENTER_GAP2, t }) => {
  const actualTheme = useActualTheme();
  const admin = isAdmin();

  const [timeWindow, setTimeWindow] = useState('1h');
  const [customRange, setCustomRange] = useState(null); // [startMs, endMs]
  const [channelId, setChannelId] = useState(null);
  const [tag, setTag] = useState(null);
  const [channels, setChannels] = useState([]);
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(false);

  const isCustom = !!(customRange && customRange[0] && customRange[1]);

  // 管理员：加载渠道列表用于筛选下拉
  useEffect(() => {
    if (!admin) return;
    (async () => {
      try {
        const res = await API.get('/api/channel/?page=1&page_size=100');
        if (res.data.success) setChannels(res.data.data.items || []);
      } catch (e) {
        // 下拉数据失败不阻塞主流程
      }
    })();
  }, [admin]);

  const channelOptions = useMemo(
    () => channels.map((ch) => ({ label: ch.name || `#${ch.id}`, value: ch.id })),
    [channels],
  );

  const tagOptions = useMemo(() => {
    const set = new Set();
    channels.forEach((ch) => {
      if (ch.tag) set.add(ch.tag);
    });
    return [...set].map((tg) => ({ label: tg, value: tg }));
  }, [channels]);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (isCustom) {
        params.append('start_time', Math.floor(customRange[0] / 1000));
        params.append('end_time', Math.floor(customRange[1] / 1000));
      } else {
        params.append('window', timeWindow);
      }
      if (admin) {
        if (channelId) params.append('channel_id', channelId);
        if (tag) params.append('tag', tag);
      }
      const base = admin
        ? '/api/usage_stats/error_rate'
        : '/api/usage_stats/error_rate/me';
      const res = await API.get(`${base}?${params.toString()}`);
      if (res.data.success) setData(res.data.data);
      else showError(res.data.message);
    } catch (e) {
      showError(e.message);
    } finally {
      setLoading(false);
    }
  }, [admin, timeWindow, customRange, channelId, tag, isCustom]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const isDark = actualTheme === 'dark';
  const trend = data?.trend || [];
  const hasData = trend.some((b) => b.error_count > 0 || b.success_count > 0);
  const xWin = isCustom ? 'custom' : timeWindow;
  const spec = useMemo(
    () =>
      makeChartSpec(
        trend.map((b) => ({
          time: fmtTime(b.time, xWin),
          total: (b.error_count || 0) + (b.success_count || 0),
          error_rate: b.error_rate,
        })),
        isDark,
      ),
    [trend, xWin, isDark],
  );

  return (
    <Card
      {...CARD_PROPS}
      className='!rounded-2xl'
      title={
        <div className='flex flex-col lg:flex-row lg:items-center lg:justify-between w-full gap-3'>
          <div className={FLEX_CENTER_GAP2}>
            <Activity size={16} />
            {t('错误率统计')}
          </div>
          {/* 筛选工具栏 */}
          <div className='flex items-center gap-2 flex-wrap'>
            {WINDOWS.map(({ key, label }) => (
              <Button
                key={key}
                size='small'
                theme={timeWindow === key && !isCustom ? 'solid' : 'borderless'}
                type={timeWindow === key && !isCustom ? 'primary' : 'tertiary'}
                onClick={() => {
                  setTimeWindow(key);
                  setCustomRange(null);
                }}
              >
                {t(label)}
              </Button>
            ))}
            <DatePicker
              type='dateTimeRange'
              size='small'
              placeholder={[t('开始时间'), t('结束时间')]}
              value={customRange}
              onChange={(v) => setCustomRange(v)}
              style={{ width: 300 }}
            />
            {admin && (
              <>
                <Select
                  placeholder={t('全部渠道')}
                  size='small'
                  style={{ width: 150 }}
                  showClear
                  filter
                  optionList={channelOptions}
                  value={channelId}
                  onChange={setChannelId}
                />
                <Select
                  placeholder={t('全部标签')}
                  size='small'
                  style={{ width: 130 }}
                  showClear
                  filter
                  optionList={tagOptions}
                  value={tag}
                  onChange={setTag}
                />
              </>
            )}
          </div>
        </div>
      }
      bodyStyle={{ padding: 0 }}
    >
      <div className='p-4'>

        {/* 错误率数字 */}
        <div className='flex items-end gap-6 mb-3'>
          <div className='flex flex-col'>
            <Text type='secondary' className='text-xs'>
              {t('错误率')}
            </Text>
            <span className='text-2xl font-semibold text-[var(--semi-color-primary)] tabular-nums'>
              {data ? `${data.error_rate}%` : '—'}
            </span>
          </div>
          <div className='flex items-end gap-5 pb-1'>
            {[
              { label: t('错误请求'), value: data?.error_count ?? 0 },
              { label: t('成功请求'), value: data?.success_count ?? 0 },
              { label: t('总请求'), value: data?.total ?? 0 },
            ].map(({ label, value }) => (
              <div key={label} className='flex flex-col'>
                <Text type='secondary' className='text-xs'>
                  {label}
                </Text>
                <span className='text-base font-medium tabular-nums'>
                  {value}
                </span>
              </div>
            ))}
          </div>
        </div>

        {/* 趋势图 */}
        <Spin spinning={loading}>
          <div className='h-72 p-2'>
            {!loading && !hasData ? (
              <Empty
                description={<Text type='secondary'>{t('暂无数据')}</Text>}
                style={{ paddingTop: '70px' }}
              />
            ) : (
              <VChart key={actualTheme} spec={spec} option={CHART_CONFIG} />
            )}
          </div>
        </Spin>
      </div>
    </Card>
  );
};

export default ErrorRatePanel;
