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

import { useMemo } from 'react';
import { Wallet, Activity, Zap, Gauge, BarChart2 } from 'lucide-react';
import {
  IconMoneyExchangeStroked,
  IconHistogram,
  IconCoinMoneyStroked,
  IconTextStroked,
  IconPulse,
  IconStopwatchStroked,
  IconTypograph,
  IconSend,
} from '@douyinfe/semi-icons';
import { renderQuota } from '../../helpers';
import { createSectionTitle } from '../../helpers/dashboard';
import { useActualTheme } from '../../context/Theme';
import i18n from '../../i18n/i18n';

export const useDashboardStats = (
  userState,
  consumeQuota,
  consumeTokens,
  times,
  trendData,
  performanceMetrics,
  navigate,
  t,
  cacheHitStats,
  adminQueryActive,
  adminQueryResult,
) => {
  const theme = useActualTheme();
  
  const fmtRate = (r) => (r == null ? '—' : `${r.toFixed(2)}%`);
  const fmtNum = (n) => {
    if (n == null) return '0';
    const locale = i18n.language?.startsWith('zh') ? 'zh' : 'en';
    return new Intl.NumberFormat(locale, { notation: 'compact', maximumFractionDigits: 2 }).format(n);
  };

  const cacheTooltip = (stats) => {
    if (!stats) return null;
    const total = (stats.cache_read || 0) + (stats.input || 0) + (stats.cache_write || 0);
    const hitRate = total > 0 ? ((stats.cache_read || 0) / total * 100).toFixed(2) : '0.00';
    const isDark = theme === 'dark';

    const colors = {
      title: isDark ? '#fff' : '#1a1a1a',
      secondary: isDark ? 'rgba(255,255,255,0.8)' : 'rgba(0,0,0,0.7)',
      muted: isDark ? 'rgba(255,255,255,0.6)' : 'rgba(0,0,0,0.5)',
      value: isDark ? '#fff' : '#1a1a1a',
      border: isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.1)',
      highlightBg: isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.05)',
    };

    const statRow = (label, value) => (
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
        <span style={{ color: colors.muted }}>{label}</span>
        <span style={{ color: colors.value }}>{value}</span>
      </div>
    );

    const formulaItems = [
      { label: t('缓存读取'), value: fmtNum(stats.cache_read) },
      { label: t('输入 Token'), value: fmtNum(stats.input) },
      { label: t('缓存创建 Token'), value: fmtNum(stats.cache_write) },
    ];

    return (
      <div style={{ fontSize: 12, lineHeight: 1.8, padding: 4, minWidth: 220, backgroundColor: 'transparent' }}>
        <div style={{ fontWeight: 600, marginBottom: 10, color: colors.title }}>{t('缓存命中率计算公式')}</div>
        <div style={{ backgroundColor: colors.highlightBg, padding: 10, borderRadius: 6, marginBottom: 12, wordBreak: 'break-all' }}>
          <div style={{ color: colors.secondary }}>
            {t('缓存命中率')} = {t('缓存读取')} / ({t('缓存读取')} + {t('输入')} + {t('缓存写入')}) × 100%
          </div>
        </div>
        {formulaItems.map(item => statRow(item.label, item.value))}
        <div style={{ borderTop: `1px solid ${colors.border}`, marginTop: 8, paddingTop: 8 }}>
          {statRow(t('总计'), fmtNum(total))}
          {statRow(t('命中率'), `${hitRate}%`)}
        </div>
      </div>
    );
  };

  const getThemeBgColor = () => {
    return theme === 'dark' ? 'bg-gray-800' : 'bg-blue-50';
  };

  const groupedStatsData = useMemo(
    () => {
      const accountQuota = adminQueryActive && adminQueryResult?.user
        ? adminQueryResult.user.quota
        : userState?.user?.quota;
      const accountUsedQuota = adminQueryActive && adminQueryResult?.user
        ? adminQueryResult.user.used_quota
        : userState?.user?.used_quota;
      const accountRequestCount = adminQueryActive && adminQueryResult?.user
        ? adminQueryResult.user.request_count
        : userState?.user?.request_count;

      const cacheStats = adminQueryActive && adminQueryResult?.cache_hit_stats
        ? {
            today: adminQueryResult.cache_hit_stats.range,
            lifetime: adminQueryResult.cache_hit_stats.lifetime,
          }
        : cacheHitStats;

      const cacheItems = [
        {
          title: adminQueryActive ? t('所选范围缓存命中率') : t('今日缓存命中率'),
          value: fmtRate(cacheStats?.today?.hit_rate),
          icon: <IconPulse />,
          avatarColor: 'teal',
          trendData: [],
          trendColor: '#14b8a6',
          tooltip: cacheTooltip(cacheStats?.today),
        },
        {
          title: t('历史缓存命中率'),
          value: fmtRate(cacheStats?.lifetime?.hit_rate),
          icon: <IconHistogram />,
          avatarColor: 'cyan',
          trendData: [],
          trendColor: '#06b6d4',
          tooltip: cacheTooltip(cacheStats?.lifetime),
        },
      ];

      return [
        {
          title: createSectionTitle(Wallet, t('账户数据')),
          color: getThemeBgColor(),
          items: [
            {
              title: t('当前余额'),
              value: renderQuota(accountQuota),
              icon: <IconMoneyExchangeStroked />,
              avatarColor: 'blue',
              trendData: [],
              trendColor: '#3b82f6',
            },
            {
              title: adminQueryActive ? t('所选范围消耗') : t('历史消耗'),
              value: renderQuota(accountUsedQuota),
              icon: <IconHistogram />,
              avatarColor: 'purple',
              trendData: [],
              trendColor: '#8b5cf6',
            },
          ],
        },
        {
          title: createSectionTitle(BarChart2, t('缓存数据')),
          color: getThemeBgColor(),
          items: cacheItems,
        },
        {
          title: createSectionTitle(Activity, t('使用统计')),
          color: getThemeBgColor(),
          items: [
            {
              title: adminQueryActive ? t('所选范围请求次数') : t('请求次数'),
              value: accountRequestCount,
              icon: <IconSend />,
              avatarColor: 'green',
              trendData: [],
              trendColor: '#10b981',
            },
            {
              title: t('统计次数'),
              value: times,
              icon: <IconPulse />,
              avatarColor: 'cyan',
              trendData: trendData.times,
              trendColor: '#06b6d4',
            },
          ],
        },
        {
          title: createSectionTitle(Zap, t('资源消耗')),
          color: getThemeBgColor(),
          items: [
            {
              title: t('统计额度'),
              value: renderQuota(consumeQuota),
              icon: <IconCoinMoneyStroked />,
              avatarColor: 'yellow',
              trendData: trendData.consumeQuota,
              trendColor: '#f59e0b',
            },
            {
              title: t('统计Tokens'),
              value: isNaN(consumeTokens) ? 0 : consumeTokens.toLocaleString(),
              icon: <IconTextStroked />,
              avatarColor: 'pink',
              trendData: trendData.tokens,
              trendColor: '#ec4899',
            },
          ],
        },
        {
          title: createSectionTitle(Gauge, t('性能指标')),
          color: getThemeBgColor(),
          items: [
            {
              title: t('平均RPM'),
              value: performanceMetrics.avgRPM,
              icon: <IconStopwatchStroked />,
              avatarColor: 'indigo',
              trendData: trendData.rpm,
              trendColor: '#6366f1',
            },
            {
              title: t('平均TPM'),
              value: performanceMetrics.avgTPM,
              icon: <IconTypograph />,
              avatarColor: 'orange',
              trendData: trendData.tpm,
              trendColor: '#f97316',
            },
          ],
        },
      ];
    },
    [
      userState?.user?.quota,
      userState?.user?.used_quota,
      userState?.user?.request_count,
      times,
      consumeQuota,
      consumeTokens,
      trendData,
      performanceMetrics,
      navigate,
      t,
      cacheHitStats,
      theme,
      adminQueryActive,
      adminQueryResult,
    ],
  );

  return {
    groupedStatsData,
  };
};
