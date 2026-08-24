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
import { Download, Table2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { Button } from '@/components/ui/button'
import { IconBadge } from '@/components/ui/icon-badge'
import { aggregateModelSummary } from '@/features/dashboard/lib'
import { exportModelConsumptionExcel } from '@/features/dashboard/lib/export-model-consumption'
import type { ModelQuotaSummary } from '@/features/dashboard/lib/model-summary'
import type { QuotaDataItem } from '@/features/dashboard/types'
import { getCurrencyDisplay } from '@/lib/currency'
import { formatNumber, formatQuota, formatTokens } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

interface ModelSummaryTableProps {
  data: QuotaDataItem[]
  loading?: boolean
}

/** 依据一批数据的最大时间戳（秒）推导「YYYY.MM」期间 */
function derivePeriod(rows: QuotaDataItem[]): string {
  const maxTs = rows.reduce(
    (max, item) => Math.max(max, Number(item.created_at) || 0),
    0
  )
  const d = maxTs > 0 ? new Date(maxTs * 1000) : new Date()
  return `${d.getFullYear()}.${String(d.getMonth() + 1).padStart(2, '0')}`
}

export function ModelSummaryTable(props: ModelSummaryTableProps) {
  const { t } = useTranslation()
  const username = useAuthStore((state) => state.auth.user?.username)
  const [exporting, setExporting] = useState(false)

  const summary: ModelQuotaSummary[] = useMemo(
    () => aggregateModelSummary(props.loading ? [] : props.data),
    [props.data, props.loading]
  )

  const hasData = summary.length > 0

  const handleExport = async () => {
    if (!hasData) {
      toast.error(t('No data'))
      return
    }
    setExporting(true)
    try {
      await exportModelConsumptionExcel({
        modelSummary: summary,
        username: username || 'user',
        period: derivePeriod(props.data),
        quotaPerUnit: getCurrencyDisplay().config.quotaPerUnit,
      })
      toast.success(t('Report downloaded'))
    } catch (e) {
      toast.error(
        `${t('Failed to download report')}: ${
          e instanceof Error ? e.message : String(e)
        }`
      )
    } finally {
      setExporting(false)
    }
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex w-full flex-col gap-1.5 border-b px-3 py-2 sm:gap-3 sm:px-5 sm:py-3 lg:flex-row lg:items-center lg:justify-between'>
        <div className='flex items-center gap-2'>
          <IconBadge tone='chart-2' size='sm'>
            <Table2 />
          </IconBadge>
          <div className='text-sm font-semibold'>
            {t('Model Consumption Stats')}
          </div>
        </div>

        <Button
          variant='outline'
          size='sm'
          disabled={!hasData || exporting}
          onClick={handleExport}
        >
          <Download className='size-3.5' />
          {t('Download Report')}
        </Button>
      </div>

      <div className='max-h-96 overflow-auto p-1.5 sm:p-2'>
        <StaticDataTable
          data={props.loading ? [] : summary}
          getRowKey={(row) => row.model_name}
          emptyClassName='text-muted-foreground py-8'
          emptyContent={props.loading ? t('Loading...') : t('No data')}
          columns={[
            {
              id: 'model_name',
              header: t('Model Name'),
              cell: (row) => (
                <span className='font-medium'>{row.model_name}</span>
              ),
            },
            {
              id: 'quota',
              header: t('Consumed Quota'),
              className: 'text-right',
              cellClassName: 'text-right',
              cell: (row) => formatQuota(row.quota),
            },
            {
              id: 'count',
              header: t('Requests'),
              className: 'text-right',
              cellClassName: 'text-right',
              cell: (row) => formatNumber(row.count),
            },
            {
              id: 'token_used',
              header: t('Token Usage'),
              className: 'text-right',
              cellClassName: 'text-right',
              cell: (row) => formatTokens(row.token_used),
            },
            {
              id: 'input_tokens',
              header: t('Input Tokens'),
              className: 'text-right',
              cellClassName: 'text-right',
              cell: (row) => formatTokens(row.input_tokens),
            },
            {
              id: 'cached_tokens',
              header: t('Cached Tokens'),
              className: 'text-right',
              cellClassName: 'text-right',
              cell: (row) => formatTokens(row.cached_tokens),
            },
            {
              id: 'output_tokens',
              header: t('Output Tokens'),
              className: 'text-right',
              cellClassName: 'text-right',
              cell: (row) => formatTokens(row.output_tokens),
            },
          ]}
        />
      </div>
    </div>
  )
}
