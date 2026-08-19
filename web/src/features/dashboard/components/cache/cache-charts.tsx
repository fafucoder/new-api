/*
Copyright (C) 2023-2026 QuantumNous

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
import { VChart } from '@visactor/react-vchart'
import { Database } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { useThemeCustomization } from '@/context/theme-customization-provider'
import { useTheme } from '@/context/theme-provider'
import { getCacheQuotaDates } from '@/features/dashboard/api'
import {
  CACHE_HIT_RATE_CHART_OPTIONS,
  DEFAULT_TIME_GRANULARITY,
  TIME_RANGE_BY_GRANULARITY,
} from '@/features/dashboard/constants'
import { processCacheChartData } from '@/features/dashboard/lib'
import type {
  CacheFilters,
  CacheHitRateChartTab,
  QuotaDataItem,
} from '@/features/dashboard/types'
import { getRollingDateRange, type TimeGranularity } from '@/lib/time'
import { VCHART_OPTION } from '@/lib/vchart'

import { CacheFilterDialog } from './cache-filter-dialog'

let themeManagerPromise: Promise<
  (typeof import('@visactor/vchart'))['ThemeManager']
> | null = null

type ChartSpecKey = 'spec_request_hit_rate' | 'spec_token_hit_rate'

const CHART_SPEC_KEYS: Record<CacheHitRateChartTab, ChartSpecKey> = {
  request: 'spec_request_hit_rate',
  token: 'spec_token_hit_rate',
}

function buildDefaultCacheFilters(): CacheFilters {
  const granularity = DEFAULT_TIME_GRANULARITY
  const days = TIME_RANGE_BY_GRANULARITY[granularity]
  const { start, end } = getRollingDateRange(days)
  return {
    start_timestamp: start,
    end_timestamp: end,
    time_granularity: granularity,
    model_name: '',
    channel_id: undefined,
  }
}

export function CacheCharts() {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const { customization } = useThemeCustomization()
  const [activeTab, setActiveTab] = useState<CacheHitRateChartTab>('request')
  const [themeReady, setThemeReady] = useState(false)
  const themeManagerRef = useRef<
    (typeof import('@visactor/vchart'))['ThemeManager'] | null
  >(null)

  const [filters, setFilters] = useState<CacheFilters>(buildDefaultCacheFilters)
  const [data, setData] = useState<QuotaDataItem[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    const updateTheme = async () => {
      setThemeReady(false)
      if (!themeManagerPromise) {
        themeManagerPromise = import('@visactor/vchart').then(
          (m) => m.ThemeManager
        )
      }
      const ThemeManager = await themeManagerPromise
      themeManagerRef.current = ThemeManager
      ThemeManager.setCurrentTheme(resolvedTheme === 'dark' ? 'dark' : 'light')
      setThemeReady(true)
    }
    updateTheme()
  }, [resolvedTheme])

  // Fetch data when filters change
  useEffect(() => {
    const fetchData = async () => {
      if (!filters.start_timestamp || !filters.end_timestamp) return
      setLoading(true)
      try {
        const startTs = Math.floor(filters.start_timestamp.getTime() / 1000)
        const endTs = Math.floor(filters.end_timestamp.getTime() / 1000)
        const params: {
          start_timestamp: number
          end_timestamp: number
          model_name?: string
          channel_id?: number
        } = { start_timestamp: startTs, end_timestamp: endTs }
        if (filters.model_name) params.model_name = filters.model_name
        if (filters.channel_id) params.channel_id = filters.channel_id
        const res = await getCacheQuotaDates(params)
        if (res.success) {
          setData(res.data || [])
        }
      } catch {
        // silently fail
      } finally {
        setLoading(false)
      }
    }
    fetchData()
  }, [filters])

  const timeGranularity: TimeGranularity =
    filters.time_granularity ?? DEFAULT_TIME_GRANULARITY

  const chartData = useMemo(
    () => processCacheChartData(loading ? [] : data, timeGranularity, t),
    [data, loading, timeGranularity, t]
  )

  const handleFilterChange = useCallback((newFilters: CacheFilters) => {
    setFilters(newFilters)
  }, [])

  const handleResetFilters = useCallback(() => {
    setFilters(buildDefaultCacheFilters())
  }, [])

  const spec = chartData[CHART_SPEC_KEYS[activeTab]]
  const specType = typeof spec?.type === 'string' ? spec.type : activeTab
  const chartKey = [
    activeTab,
    specType,
    loading ? 'loading' : 'ready',
    data.length,
    resolvedTheme,
    customization.preset,
  ].join('-')

  return (
    <div className='space-y-3 sm:space-y-4'>
      <div className='flex items-center justify-end'>
        <CacheFilterDialog
          currentFilters={filters}
          onFilterChange={handleFilterChange}
          onReset={handleResetFilters}
        />
      </div>
      <div className='overflow-hidden rounded-lg border'>
        <div className='flex w-full flex-col gap-1.5 border-b px-3 py-2 sm:gap-3 sm:px-5 sm:py-3 lg:flex-row lg:items-center lg:justify-between'>
          <div className='flex items-center gap-2'>
            <IconBadge tone='chart-4' size='sm'>
              <Database />
            </IconBadge>
            <div className='text-sm font-semibold'>
              {t('Cache Hit Rate')}
            </div>
          </div>

          <div className='bg-muted/60 inline-flex h-7 w-full overflow-x-auto rounded-lg border p-0.5 sm:h-8 sm:w-auto'>
            {CACHE_HIT_RATE_CHART_OPTIONS.map((tab) => (
              <button
                key={tab.value}
                type='button'
                onClick={() => setActiveTab(tab.value)}
                className={`shrink-0 rounded-md px-3 text-xs font-medium transition-colors ${
                  activeTab === tab.value
                    ? 'bg-background text-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                {t(tab.labelKey)}
              </button>
            ))}
          </div>
        </div>

        <div className='h-[300px] p-1.5 sm:h-96 sm:p-2'>
          {loading ? (
            <Skeleton className='h-full w-full' />
          ) : (
            themeReady &&
            spec && (
              <VChart
                key={chartKey}
                spec={{
                  ...spec,
                  theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                  background: 'transparent',
                }}
                option={VCHART_OPTION}
              />
            )
          )}
        </div>
      </div>
    </div>
  )
}
