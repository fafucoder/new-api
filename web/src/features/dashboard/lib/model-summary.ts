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
import type { QuotaDataItem } from '@/features/dashboard/types'

/**
 * Per-model consumption summary (no time dimension).
 * Aggregated on the client from the detailed quota data already loaded by the
 * models section.
 *
 * Token fields follow the backend semantics recorded into `quota_data`
 * (see `service/text_quota.go` / `model/log.go`):
 * - `token_used`    = prompt + completion（总用量）
 * - `input_tokens`  = 总输入（含命中缓存的部分）
 * - `cached_tokens` = 输入中命中缓存的 token
 * - `output_tokens` = completion，派生自 `token_used - input_tokens + cached_tokens`
 */
export interface ModelQuotaSummary {
  model_name: string
  quota: number
  count: number
  token_used: number
  input_tokens: number
  cached_tokens: number
  output_tokens: number
}

/**
 * Aggregate detailed quota data into a per-model consumption ranking.
 *
 * Groups rows by `model_name`, sums the quota / count / token metrics, derives
 * the output (completion) token count, drops empty model names, and sorts by
 * consumed quota descending.
 */
export function aggregateModelSummary(
  data: QuotaDataItem[]
): ModelQuotaSummary[] {
  const byModel = new Map<string, ModelQuotaSummary>()

  for (const item of data) {
    const modelName = item.model_name
    if (!modelName) continue

    const existing = byModel.get(modelName)
    if (existing) {
      existing.quota += Number(item.quota) || 0
      existing.count += Number(item.count) || 0
      existing.token_used += Number(item.token_used) || 0
      existing.input_tokens += Number(item.input_tokens) || 0
      existing.cached_tokens += Number(item.cached_tokens) || 0
    } else {
      byModel.set(modelName, {
        model_name: modelName,
        quota: Number(item.quota) || 0,
        count: Number(item.count) || 0,
        token_used: Number(item.token_used) || 0,
        input_tokens: Number(item.input_tokens) || 0,
        cached_tokens: Number(item.cached_tokens) || 0,
        output_tokens: 0,
      })
    }
  }

  const result = [...byModel.values()]
  for (const row of result) {
    // 输出 token = 总用量 - 输入(含缓存) + 缓存 = completion（保底非负）
    row.output_tokens = Math.max(
      0,
      row.token_used - row.input_tokens + row.cached_tokens
    )
  }

  return result.sort((a, b) => b.quota - a.quota)
}
