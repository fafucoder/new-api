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
import { describe, expect, test } from 'vitest'

import type { QuotaDataItem } from '../types'
import { aggregateModelSummary } from './model-summary'

const rows: QuotaDataItem[] = [
  {
    model_name: 'gpt-4o',
    created_at: 1,
    quota: 100,
    count: 2,
    token_used: 30,
    input_tokens: 20,
    cached_tokens: 5,
  },
  {
    model_name: 'gpt-4o',
    created_at: 2,
    quota: 50,
    count: 1,
    token_used: 20,
    input_tokens: 12,
    cached_tokens: 3,
  },
  {
    model_name: 'claude',
    created_at: 1,
    quota: 200,
    count: 3,
    token_used: 40,
    input_tokens: 30,
    cached_tokens: 10,
  },
  {
    model_name: '',
    created_at: 1,
    quota: 999,
    count: 9,
    token_used: 99,
    input_tokens: 50,
    cached_tokens: 5,
  },
]

describe('aggregateModelSummary', () => {
  test('groups by model_name and sums metrics', () => {
    const result = aggregateModelSummary(rows)
    const gpt = result.find((r) => r.model_name === 'gpt-4o')
    expect(gpt).toEqual({
      model_name: 'gpt-4o',
      quota: 150,
      count: 3,
      token_used: 50,
      input_tokens: 32,
      cached_tokens: 8,
      // output = token_used - input_tokens + cached_tokens = 50 - 32 + 8
      output_tokens: 26,
    })
  })

  test('derives output tokens per model', () => {
    const result = aggregateModelSummary(rows)
    const claude = result.find((r) => r.model_name === 'claude')
    // 40 - 30 + 10
    expect(claude?.output_tokens).toBe(20)
  })

  test('clamps output tokens to non-negative', () => {
    const result = aggregateModelSummary([
      {
        model_name: 'weird',
        created_at: 1,
        quota: 1,
        count: 1,
        token_used: 5,
        input_tokens: 100,
        cached_tokens: 0,
      },
    ])
    expect(result[0].output_tokens).toBe(0)
  })

  test('drops empty model names', () => {
    const result = aggregateModelSummary(rows)
    expect(result.some((r) => r.model_name === '')).toBe(false)
  })

  test('sorts by quota descending', () => {
    const result = aggregateModelSummary(rows)
    expect(result.map((r) => r.model_name)).toEqual(['claude', 'gpt-4o'])
  })

  test('returns empty array for empty input', () => {
    expect(aggregateModelSummary([])).toEqual([])
  })
})
