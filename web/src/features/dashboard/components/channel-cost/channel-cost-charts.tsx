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
import { BarChart3 } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { IconBadge } from '@/components/ui/icon-badge'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { useThemeCustomization } from '@/context/theme-customization-provider'
import { useTheme } from '@/context/theme-provider'
import { searchChannels } from '@/features/channels/api'
import { getChannelCostDates } from '@/features/dashboard/api'
import { processChannelCostChartData } from '@/features/dashboard/lib'
import type {
  DashboardFilters,
  QuotaDataItem,
} from '@/features/dashboard/types'
import { VCHART_OPTION } from '@/lib/vchart'

let themeManagerPromise: Promise<
  (typeof import('@visactor/vchart'))['ThemeManager']
> | null = null

interface ChannelCostChartsProps {
  // Shared Model Call Analytics filters (time range drives this chart too).
  filters: DashboardFilters
}

export function ChannelCostCharts(props: ChannelCostChartsProps) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const { customization } = useThemeCustomization()
  const [themeReady, setThemeReady] = useState(false)
  const themeManagerRef = useRef<
    (typeof import('@visactor/vchart'))['ThemeManager'] | null
  >(null)

  const [channelId, setChannelId] = useState<number | undefined>(undefined)
  const [data, setData] = useState<QuotaDataItem[]>([])
  const [channelNames, setChannelNames] = useState<Map<number, string>>(
    () => new Map()
  )
  const [channelOptions, setChannelOptions] = useState<
    Array<{ id: number; name: string }>
  >([])
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

  // Load channel id -> name map for bar labels and the inline selector.
  useEffect(() => {
    searchChannels({ keyword: '', p: 1, page_size: 100 })
      .then((res) => {
        if (res.success && res.data?.items) {
          const map = new Map<number, string>()
          res.data.items.forEach((ch) => map.set(ch.id, ch.name))
          setChannelNames(map)
          setChannelOptions(
            res.data.items.map((ch) => ({ id: ch.id, name: ch.name }))
          )
        }
      })
      .catch(() => {})
  }, [])

  // Fetch data when the shared time range or the channel selection changes.
  const startTimestamp = props.filters.start_timestamp
  const endTimestamp = props.filters.end_timestamp
  useEffect(() => {
    const fetchData = async () => {
      if (!startTimestamp || !endTimestamp) return
      setLoading(true)
      try {
        const startTs = Math.floor(startTimestamp.getTime() / 1000)
        const endTs = Math.floor(endTimestamp.getTime() / 1000)
        const params: {
          start_timestamp: number
          end_timestamp: number
          channel_id?: number
        } = { start_timestamp: startTs, end_timestamp: endTs }
        if (channelId) params.channel_id = channelId
        const res = await getChannelCostDates(params)
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
  }, [startTimestamp, endTimestamp, channelId])

  const chartData = useMemo(
    () => processChannelCostChartData(loading ? [] : data, channelNames, t),
    [data, loading, channelNames, t]
  )

  const spec = chartData.spec_channel_cost
  const chartKey = [
    loading ? 'loading' : 'ready',
    data.length,
    resolvedTheme,
    customization.preset,
  ].join('-')

  const summary = [
    { label: t('Purchase Cost'), value: chartData.totalCostDisplay },
    { label: t('Revenue'), value: chartData.totalSaleDisplay },
    { label: t('Profit'), value: chartData.totalProfitDisplay },
  ]

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex w-full flex-col gap-1.5 border-b px-3 py-2 sm:gap-3 sm:px-5 sm:py-3 lg:flex-row lg:items-center lg:justify-between'>
        <div className='flex items-center gap-2'>
          <IconBadge tone='chart-4' size='sm'>
            <BarChart3 />
          </IconBadge>
          <div className='text-sm font-semibold'>{t('Channel Cost')}</div>
        </div>

        <div className='flex flex-wrap items-center gap-2'>
          <div className='flex items-center gap-3 text-xs sm:text-sm'>
            {summary.map((item) => (
              <div key={item.label} className='flex items-center gap-1'>
                <span className='text-muted-foreground'>{item.label}</span>
                <span className='font-semibold'>
                  {loading ? '…' : item.value}
                </span>
              </div>
            ))}
          </div>
          <Select
            items={[
              { value: '0', label: t('All Channels') },
              ...channelOptions.map((ch) => ({
                value: String(ch.id),
                label: `${ch.name} (#${ch.id})`,
              })),
            ]}
            value={String(channelId || 0)}
            onValueChange={(value) =>
              setChannelId(value === '0' ? undefined : Number(value))
            }
          >
            <SelectTrigger className='h-8 w-40 text-xs'>
              <SelectValue placeholder={t('All Channels')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='0'>{t('All Channels')}</SelectItem>
                {channelOptions.map((ch) => (
                  <SelectItem key={ch.id} value={String(ch.id)}>
                    {ch.name} (#{ch.id})
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
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
  )
}
