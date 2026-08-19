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
import { MAX_CHART_TREND_POINTS } from '@/features/dashboard/constants'
import type {
  QuotaDataItem,
  ProcessedCacheChartData,
} from '@/features/dashboard/types'
import { formatChartTime, type TimeGranularity } from '@/lib/time'

import { getDashboardChartColors } from './charts'

type TFunction = (key: string) => string

export function processCacheChartData(
  data: QuotaDataItem[],
  timeGranularity: TimeGranularity = 'day',
  t?: TFunction
): ProcessedCacheChartData {
  const tt: TFunction = t ?? ((x) => x)
  const otherLabel = tt('Other')

  const formatPercent = (value: number) => `${value.toFixed(1)}%`

  const emptySpec = (title: string) => ({
    type: 'area' as const,
    data: [{ id: 'cacheData', values: [] }],
    xField: 'Time',
    yField: 'HitRate',
    seriesField: 'Model',
    legends: { visible: true, selectMode: 'single' as const },
    title: { visible: true, text: title },
    background: { fill: 'transparent' },
  })

  if (!data || data.length === 0) {
    return {
      spec_request_hit_rate: emptySpec(tt('Request Hit Rate')),
      spec_token_hit_rate: emptySpec(tt('Token Hit Rate')),
    }
  }

  // Aggregate by time and model
  const timeModelMap = new Map<
    string,
    Map<
      string,
      {
        count: number
        cacheHitCount: number
        cachedTokens: number
        inputTokens: number
      }
    >
  >()
  const modelTotalsMap = new Map<
    string,
    {
      count: number
      cacheHitCount: number
      cachedTokens: number
      inputTokens: number
    }
  >()

  data.forEach((item) => {
    const timestamp = Number(item.created_at)
    const timeKey = formatChartTime(timestamp, timeGranularity)
    const model = item.model_name || 'Unknown'
    const count = Number(item.count) || 0
    const cacheHitCount = Number(item.cache_hit_count) || 0
    const cachedTokens = Number(item.cached_tokens) || 0
    const inputTokens = Number(item.input_tokens) || 0

    if (!timeModelMap.has(timeKey)) {
      timeModelMap.set(timeKey, new Map())
    }
    const modelMap = timeModelMap.get(timeKey)!
    const existing = modelMap.get(model) || {
      count: 0,
      cacheHitCount: 0,
      cachedTokens: 0,
      inputTokens: 0,
    }
    modelMap.set(model, {
      count: existing.count + count,
      cacheHitCount: existing.cacheHitCount + cacheHitCount,
      cachedTokens: existing.cachedTokens + cachedTokens,
      inputTokens: existing.inputTokens + inputTokens,
    })

    const totalExisting = modelTotalsMap.get(model) || {
      count: 0,
      cacheHitCount: 0,
      cachedTokens: 0,
      inputTokens: 0,
    }
    modelTotalsMap.set(model, {
      count: totalExisting.count + count,
      cacheHitCount: totalExisting.cacheHitCount + cacheHitCount,
      cachedTokens: totalExisting.cachedTokens + cachedTokens,
      inputTokens: totalExisting.inputTokens + inputTokens,
    })
  })

  const sortedTimes = Array.from(timeModelMap.keys()).sort()
  const sortedModels = [...Array.from(modelTotalsMap.keys())].sort()
  const modelColorDomain = Array.from(new Set([...sortedModels, otherLabel]))
  const modelColorRange = getDashboardChartColors(modelColorDomain.length)
  const modelColor = {
    type: 'ordinal',
    domain: modelColorDomain,
    range: modelColorRange,
  }

  // Pad time points if too few
  const fillTimePoints = (times: string[]) => {
    if (times.length >= MAX_CHART_TREND_POINTS) return times
    const lastTime = Math.max(
      ...data.map((item) => Number(item.created_at) || 0)
    )
    const intervalSec =
      timeGranularity === 'week'
        ? 604800
        : timeGranularity === 'day'
          ? 86400
          : 3600
    return Array.from({ length: MAX_CHART_TREND_POINTS }, (_, i) =>
      formatChartTime(
        lastTime - (MAX_CHART_TREND_POINTS - 1 - i) * intervalSec,
        timeGranularity
      )
    )
  }
  const chartTimes = fillTimePoints(sortedTimes)

  // Top models by total count (limit to 20, bucket rest into "Other")
  const MAX_MODELS = 20
  const rankedModels = Array.from(modelTotalsMap.entries())
    .map(([model, stats]) => ({ model, count: stats.count }))
    .sort((a, b) => b.count - a.count)
  const topModels = rankedModels.slice(0, MAX_MODELS).map((m) => m.model)
  const otherModels = rankedModels.slice(MAX_MODELS).map((m) => m.model)

  // Build request hit rate data
  const requestHitRateValues: Array<{
    Time: string
    Model: string
    HitRate: number
    Count: number
    HitCount: number
  }> = []

  // Build token hit rate data
  const tokenHitRateValues: Array<{
    Time: string
    Model: string
    HitRate: number
    CachedTokens: number
    InputTokens: number
  }> = []

  chartTimes.forEach((time) => {
    topModels.forEach((model) => {
      const stats = timeModelMap.get(time)?.get(model)
      const count = stats?.count || 0
      const hitCount = stats?.cacheHitCount || 0
      const requestHitRate = count > 0 ? (hitCount / count) * 100 : 0
      requestHitRateValues.push({
        Time: time,
        Model: model,
        HitRate: Number(requestHitRate.toFixed(1)),
        Count: count,
        HitCount: hitCount,
      })

      const cachedTokens = stats?.cachedTokens || 0
      const inputTokens = stats?.inputTokens || 0
      const tokenHitRate = inputTokens > 0 ? (cachedTokens / inputTokens) * 100 : 0
      tokenHitRateValues.push({
        Time: time,
        Model: model,
        HitRate: Number(tokenHitRate.toFixed(1)),
        CachedTokens: cachedTokens,
        InputTokens: inputTokens,
      })
    })

    // "Other" bucket
    if (otherModels.length > 0) {
      let otherCount = 0
      let otherHitCount = 0
      let otherCachedTokens = 0
      let otherInputTokens = 0
      otherModels.forEach((model) => {
        const stats = timeModelMap.get(time)?.get(model)
        otherCount += stats?.count || 0
        otherHitCount += stats?.cacheHitCount || 0
        otherCachedTokens += stats?.cachedTokens || 0
        otherInputTokens += stats?.inputTokens || 0
      })
      requestHitRateValues.push({
        Time: time,
        Model: otherLabel,
        HitRate:
          otherCount > 0
            ? Number(((otherHitCount / otherCount) * 100).toFixed(1))
            : 0,
        Count: otherCount,
        HitCount: otherHitCount,
      })
      tokenHitRateValues.push({
        Time: time,
        Model: otherLabel,
        HitRate:
          otherInputTokens > 0
            ? Number(
                ((otherCachedTokens / otherInputTokens) * 100).toFixed(1)
              )
            : 0,
        CachedTokens: otherCachedTokens,
        InputTokens: otherInputTokens,
      })
    }
  })

  const commonChartSpec = {
    area: {
      style: {
        fillOpacity: 0.08,
        curveType: 'monotone',
      },
    },
    line: {
      style: {
        lineWidth: 2,
        curveType: 'monotone',
      },
    },
    point: { visible: false },
    background: { fill: 'transparent' },
    animation: true,
    color: modelColor,
    axes: [
      { orient: 'bottom', type: 'band' },
      {
        orient: 'left',
        type: 'linear',
        label: {
          formatMethod: (value: number) => formatPercent(value),
        },
      },
    ],
  }

  return {
    spec_request_hit_rate: {
      type: 'area',
      data: [{ id: 'requestHitData', values: requestHitRateValues }],
      xField: 'Time',
      yField: 'HitRate',
      seriesField: 'Model',
      stack: false,
      legends: { visible: true, selectMode: 'single' },
      title: {
        visible: true,
        text: tt('Request Hit Rate'),
      },
      tooltip: {
        mark: {
          content: [
            {
              key: (datum: Record<string, unknown>) => datum?.Model,
              value: (datum: Record<string, unknown>) =>
                `${datum?.HitRate}% (${datum?.HitCount}/${datum?.Count})`,
            },
          ],
        },
        dimension: {
          content: [
            {
              key: (datum: Record<string, unknown>) => datum?.Model,
              value: (datum: Record<string, unknown>) =>
                Number(datum?.HitRate) || 0,
            },
          ],
          updateContent: (
            array: Array<{ key: string; value: string | number }>
          ) => {
            array.sort(
              (a, b) => (Number(b.value) || 0) - (Number(a.value) || 0)
            )
            for (let i = 0; i < array.length; i++) {
              array[i].value = formatPercent(Number(array[i].value) || 0)
            }
            return array
          },
        },
      },
      ...commonChartSpec,
    },
    spec_token_hit_rate: {
      type: 'area',
      data: [{ id: 'tokenHitData', values: tokenHitRateValues }],
      xField: 'Time',
      yField: 'HitRate',
      seriesField: 'Model',
      stack: false,
      legends: { visible: true, selectMode: 'single' },
      title: {
        visible: true,
        text: tt('Token Hit Rate'),
      },
      tooltip: {
        mark: {
          content: [
            {
              key: (datum: Record<string, unknown>) => datum?.Model,
              value: (datum: Record<string, unknown>) => {
                const cached = Number(datum?.CachedTokens) || 0
                const input = Number(datum?.InputTokens) || 0
                return `${datum?.HitRate}% (${cached.toLocaleString()}/${input.toLocaleString()})`
              },
            },
          ],
        },
        dimension: {
          content: [
            {
              key: (datum: Record<string, unknown>) => datum?.Model,
              value: (datum: Record<string, unknown>) =>
                Number(datum?.HitRate) || 0,
            },
          ],
          updateContent: (
            array: Array<{ key: string; value: string | number }>
          ) => {
            array.sort(
              (a, b) => (Number(b.value) || 0) - (Number(a.value) || 0)
            )
            for (let i = 0; i < array.length; i++) {
              array[i].value = formatPercent(Number(array[i].value) || 0)
            }
            return array
          },
        },
      },
      ...commonChartSpec,
    },
  }
}
