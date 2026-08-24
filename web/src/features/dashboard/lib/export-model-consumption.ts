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
import ExcelJS from 'exceljs'

import type { ModelQuotaSummary } from './model-summary'

// 样式常量（与参考模板 大模型调用消耗统计.xlsx 保持一致）
const COLOR = {
  titleFill: 'FF003366',
  headerFill: 'FF0070C0',
  stripeFill: 'FFEBF1F8',
  totalFill: 'FFD9D9D9',
  headerFont: 'FFFFFFFF',
  bodyFont: 'FF333333',
  totalFont: 'FF000000',
} as const

const THIN_BORDER: Partial<ExcelJS.Borders> = {
  top: { style: 'thin' },
  bottom: { style: 'thin' },
  left: { style: 'thin' },
  right: { style: 'thin' },
}

const NUM_FMT_INT = '#,##0'
const NUM_FMT_USD = '"$"#,##0.00'

type ColumnType = 'text' | 'int' | 'usd'

interface ColumnDef {
  header: string
  width: number
  type: ColumnType
  /** 从聚合行取出该列的原始值 */
  get: (row: ModelQuotaSummary, usd: number) => string | number
}

// 列定义（顺序即导出顺序），新增列只需在此增删一行
const COLUMNS: ColumnDef[] = [
  {
    header: '模型名称',
    width: 41,
    type: 'text',
    get: (row) => row.model_name ?? '',
  },
  {
    header: '请求次数',
    width: 15,
    type: 'int',
    get: (row) => Number(row.count) || 0,
  },
  {
    header: 'Token用量',
    width: 18,
    type: 'int',
    get: (row) => Number(row.token_used) || 0,
  },
  {
    header: '输入Token',
    width: 18,
    type: 'int',
    get: (row) => Number(row.input_tokens) || 0,
  },
  {
    header: '缓存Token',
    width: 18,
    type: 'int',
    get: (row) => Number(row.cached_tokens) || 0,
  },
  {
    header: '输出Token',
    width: 18,
    type: 'int',
    get: (row) => Number(row.output_tokens) || 0,
  },
  {
    header: '消耗额度(USD)',
    width: 20,
    type: 'usd',
    get: (_row, usd) => usd,
  },
  {
    header: '折扣后额度(USD)',
    width: 20,
    type: 'usd',
    // 折扣后额度：无独立折扣数据，与消耗额度一致
    get: (_row, usd) => usd,
  },
]

/** 数字格式：整数列 / USD 列 / 文本列无格式 */
function columnNumFmt(type: ColumnType): string | undefined {
  if (type === 'int') return NUM_FMT_INT
  if (type === 'usd') return NUM_FMT_USD
  return undefined
}

export interface ExportModelConsumptionParams {
  /** 按模型汇总的消耗排行 */
  modelSummary: ModelQuotaSummary[]
  /** 用户名，用于文件名 */
  username: string
  /** 期间，形如 "2026.06" */
  period: string
  /** 额度换算单位（quota / quotaPerUnit = USD） */
  quotaPerUnit: number
}

/**
 * 将「模型消耗统计」列表导出为 Excel 报表并触发浏览器下载。
 * 列由 COLUMNS 定义驱动，标题合并范围、SUM 合计、数字格式随列数自动计算。
 */
export async function exportModelConsumptionExcel({
  modelSummary,
  username,
  period,
  quotaPerUnit,
}: ExportModelConsumptionParams): Promise<void> {
  const rows = Array.isArray(modelSummary) ? modelSummary : []
  const perUnit =
    Number.isFinite(quotaPerUnit) && quotaPerUnit > 0 ? quotaPerUnit : 500000

  const workbook = new ExcelJS.Workbook()
  const ws = workbook.addWorksheet('模型调用消耗统计')

  const lastColLetter = ws.getColumn(COLUMNS.length).letter

  // 列宽
  COLUMNS.forEach((col, i) => {
    ws.getColumn(i + 1).width = col.width
  })

  // 第 1 行：标题（合并整行）
  ws.mergeCells(`A1:${lastColLetter}1`)
  const titleCell = ws.getCell('A1')
  titleCell.value = `${period}大模型调用消耗统计报表`
  titleCell.font = {
    name: '微软雅黑',
    size: 14,
    bold: true,
    color: { argb: COLOR.headerFont },
  }
  titleCell.fill = {
    type: 'pattern',
    pattern: 'solid',
    fgColor: { argb: COLOR.titleFill },
  }
  titleCell.alignment = { horizontal: 'center', vertical: 'middle' }
  ws.getRow(1).height = 30

  // 第 2 行：表头
  const headerRow = ws.getRow(2)
  COLUMNS.forEach((col, i) => {
    const cell = headerRow.getCell(i + 1)
    cell.value = col.header
    cell.font = {
      name: '微软雅黑',
      size: 11,
      bold: true,
      color: { argb: COLOR.headerFont },
    }
    cell.fill = {
      type: 'pattern',
      pattern: 'solid',
      fgColor: { argb: COLOR.headerFill },
    }
    cell.alignment = { horizontal: 'center', vertical: 'middle' }
    cell.border = THIN_BORDER
  })
  headerRow.height = 29

  // 数据行（从第 3 行开始）
  const firstDataRow = 3
  rows.forEach((item, idx) => {
    const rowNum = firstDataRow + idx
    const row = ws.getRow(rowNum)
    const usd = (Number(item.quota) || 0) / perUnit
    const striped = idx % 2 === 0 // 首行开始条纹填充
    COLUMNS.forEach((col, i) => {
      const cell = row.getCell(i + 1)
      cell.value = col.get(item, usd)
      cell.font = { name: '微软雅黑', size: 11, color: { argb: COLOR.bodyFont } }
      cell.alignment = {
        horizontal: col.type === 'text' ? 'left' : 'right',
        vertical: 'middle',
      }
      cell.border = THIN_BORDER
      if (striped) {
        cell.fill = {
          type: 'pattern',
          pattern: 'solid',
          fgColor: { argb: COLOR.stripeFill },
        }
      }
      const fmt = columnNumFmt(col.type)
      if (fmt) cell.numFmt = fmt
    })
    row.height = 22
  })

  // 合计行
  const totalRowNum = firstDataRow + rows.length
  const lastDataRow = totalRowNum - 1
  const totalRow = ws.getRow(totalRowNum)
  COLUMNS.forEach((col, i) => {
    const cell = totalRow.getCell(i + 1)
    if (i === 0) {
      cell.value = '合计'
      cell.alignment = { horizontal: 'center', vertical: 'middle' }
    } else {
      const letter = ws.getColumn(i + 1).letter
      cell.value = rows.length
        ? { formula: `SUM(${letter}${firstDataRow}:${letter}${lastDataRow})` }
        : 0
      cell.alignment = { horizontal: 'right', vertical: 'middle' }
      const fmt = columnNumFmt(col.type)
      if (fmt) cell.numFmt = fmt
    }
    cell.font = {
      name: '微软雅黑',
      size: 11,
      bold: true,
      color: { argb: COLOR.totalFont },
    }
    cell.fill = {
      type: 'pattern',
      pattern: 'solid',
      fgColor: { argb: COLOR.totalFill },
    }
    cell.border = THIN_BORDER
  })
  totalRow.height = 22

  // 冻结表头
  ws.views = [{ state: 'frozen', ySplit: 1 }]

  // 生成并触发下载
  const buffer = await workbook.xlsx.writeBuffer()
  const blob = new Blob([buffer], {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `${period}-大模型调用消耗统计-${username || 'user'}.xlsx`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}
