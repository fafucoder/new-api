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
import { getCurrencyDisplay } from '@/lib/currency'

import type { QuotaDataItem, VChartSpec } from '../types'

type TFunction = (key: string) => string

// Convert a raw quota amount to a display currency string, mirroring the
// dashboard's renderQuotaCompat behavior (token mode shows raw counts).
function renderQuota(rawQuota: number, digits = 4): string {
  const { config, meta } = getCurrencyDisplay()
  if (meta.kind === 'tokens') return rawQuota.toLocaleString()
  const usd = rawQuota / config.quotaPerUnit
  const rate = 'exchangeRate' in meta ? meta.exchangeRate : 1
  const symbol = 'symbol' in meta ? meta.symbol : '$'
  const value = usd * rate
  const fixed = value.toFixed(digits)
  if (Number.parseFloat(fixed) === 0 && rawQuota > 0 && value > 0) {
    return symbol + Math.pow(10, -digits).toFixed(digits)
  }
  return symbol + fixed
}

export interface ProcessedChannelCostChartData {
  spec_channel_cost: VChartSpec
  totalCostRaw: number
  totalSaleRaw: number
  totalProfitRaw: number
  totalCostDisplay: string
  totalSaleDisplay: string
  totalProfitDisplay: string
}

interface ChannelAgg {
  channelId: number
  cost: number
  sale: number
}

// Max channels rendered as individual bar groups; the rest are bucketed into an
// "Other" group so the chart stays legible for large deployments.
const MAX_CHANNELS = 30

/**
 * Aggregate per-channel cost (进货价) and sale price (售价) into a grouped bar
 * chart spec: one x-axis group per channel with two bars (Cost, Sale).
 *
 * channelNames maps channel id -> display name so bars are labeled by name
 * rather than raw id.
 */
export function processChannelCostChartData(
  data: QuotaDataItem[],
  channelNames: Map<number, string>,
  t?: TFunction
): ProcessedChannelCostChartData {
  const tt: TFunction = t ?? ((x) => x)
  const costLabel = tt('Purchase Cost')
  const profitLabel = tt('Profit')
  const otherLabel = tt('Other')

  const emptySpec: VChartSpec = {
    type: 'bar',
    data: [{ id: 'channelCostData', values: [] }],
    xField: 'Channel',
    yField: 'Amount',
    seriesField: 'Metric',
    stack: true,
    legends: { visible: true },
    background: { fill: 'transparent' },
  }

  if (!data || data.length === 0) {
    return {
      spec_channel_cost: emptySpec,
      totalCostRaw: 0,
      totalSaleRaw: 0,
      totalProfitRaw: 0,
      totalCostDisplay: renderQuota(0, 2),
      totalSaleDisplay: renderQuota(0, 2),
      totalProfitDisplay: renderQuota(0, 2),
    }
  }

  // Aggregate across time buckets by channel.
  const channelMap = new Map<number, ChannelAgg>()
  data.forEach((item) => {
    const channelId = Number(item.channel_id) || 0
    const cost = Number(item.cost_quota) || 0
    const sale = Number(item.quota) || 0
    const existing = channelMap.get(channelId) || {
      channelId,
      cost: 0,
      sale: 0,
    }
    existing.cost += cost
    existing.sale += sale
    channelMap.set(channelId, existing)
  })

  // Rank channels by sale price so the busiest channels are shown first.
  const ranked = [...channelMap.values()].sort((a, b) => b.sale - a.sale)
  const topChannels = ranked.slice(0, MAX_CHANNELS)
  const otherChannels = ranked.slice(MAX_CHANNELS)

  const channelLabel = (channelId: number): string => {
    if (channelId <= 0) return tt('Unknown')
    const name = channelNames.get(channelId)
    return name ? `${name} (#${channelId})` : `#${channelId}`
  }

  const values: Array<{
    Channel: string
    Metric: string
    Amount: number
    RawAmount: number
  }> = []

  let totalCostRaw = 0
  let totalSaleRaw = 0

  // Each channel is one stacked bar: bottom segment = cost, top segment =
  // profit (sale - cost), so the full bar height equals the sale price. A
  // negative margin (sale < cost) is clamped to 0 for the segment height but the
  // real profit is kept in RawAmount for the tooltip.
  const pushChannel = (label: string, cost: number, sale: number) => {
    const { config, meta } = getCurrencyDisplay()
    const toDisplay = (raw: number) =>
      meta.kind === 'tokens' ? raw : raw / config.quotaPerUnit
    const profit = sale - cost
    values.push({
      Channel: label,
      Metric: costLabel,
      Amount: toDisplay(cost),
      RawAmount: cost,
    })
    values.push({
      Channel: label,
      Metric: profitLabel,
      Amount: toDisplay(Math.max(profit, 0)),
      RawAmount: profit,
    })
    totalCostRaw += cost
    totalSaleRaw += sale
  }

  topChannels.forEach((ch) => {
    pushChannel(channelLabel(ch.channelId), ch.cost, ch.sale)
  })

  if (otherChannels.length > 0) {
    const otherCost = otherChannels.reduce((sum, ch) => sum + ch.cost, 0)
    const otherSale = otherChannels.reduce((sum, ch) => sum + ch.sale, 0)
    pushChannel(
      `${otherLabel} (${otherChannels.length})`,
      otherCost,
      otherSale
    )
  }

  const totalProfitRaw = totalSaleRaw - totalCostRaw

  const spec: VChartSpec = {
    type: 'bar',
    data: [{ id: 'channelCostData', values }],
    xField: 'Channel',
    yField: 'Amount',
    seriesField: 'Metric',
    stack: true,
    legends: { visible: true },
    color: {
      type: 'ordinal',
      domain: [costLabel, profitLabel],
      range: ['#E8684A', '#5AD8A6'],
    },
    bar: {
      state: {
        hover: { stroke: '#000', lineWidth: 1 },
      },
    },
    axes: [
      { orient: 'bottom', type: 'band', label: { autoRotate: true } },
      {
        orient: 'left',
        type: 'linear',
        label: {
          formatMethod: (value: number) => {
            const { config, meta } = getCurrencyDisplay()
            const raw =
              meta.kind === 'tokens' ? value : value * config.quotaPerUnit
            return renderQuota(raw, 2)
          },
        },
      },
    ],
    tooltip: {
      // Whichever segment (cost or profit) is hovered, show the same three rows
      // for that channel. Only `mark.content` is defined (matching the working
      // rank-bar tooltip); defining a `dimension` tooltip too made VChart
      // auto-generate a row per stacked series and merge it with these, which
      // produced duplicate/extra entries.
      mark: {
        content: [
          {
            key: () => tt('Revenue'),
            value: (datum: Record<string, unknown>) => {
              const sale = values
                .filter((v) => v.Channel === datum?.Channel)
                .reduce((sum, v) => sum + v.RawAmount, 0)
              return renderQuota(sale, 4)
            },
          },
          {
            key: () => costLabel,
            value: (datum: Record<string, unknown>) => {
              const row = values.find(
                (v) => v.Channel === datum?.Channel && v.Metric === costLabel
              )
              return renderQuota(row?.RawAmount || 0, 4)
            },
          },
          {
            key: () => profitLabel,
            value: (datum: Record<string, unknown>) => {
              const row = values.find(
                (v) => v.Channel === datum?.Channel && v.Metric === profitLabel
              )
              return renderQuota(row?.RawAmount || 0, 4)
            },
          },
        ],
      },
    },
    background: { fill: 'transparent' },
    animation: true,
  }

  return {
    spec_channel_cost: spec,
    totalCostRaw,
    totalSaleRaw,
    totalProfitRaw,
    totalCostDisplay: renderQuota(totalCostRaw, 2),
    totalSaleDisplay: renderQuota(totalSaleRaw, 2),
    totalProfitDisplay: renderQuota(totalProfitRaw, 2),
  }
}
