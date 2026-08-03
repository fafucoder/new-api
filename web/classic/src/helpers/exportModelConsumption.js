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

import ExcelJS from 'exceljs';

// 样式常量（与参考模板 2026.06-大模型调用消耗统计.xlsx 完全一致）
const COLOR = {
  titleFill: 'FF003366',
  headerFill: 'FF0070C0',
  stripeFill: 'FFEBF1F8',
  totalFill: 'FFD9D9D9',
  headerFont: 'FFFFFFFF',
  bodyFont: 'FF333333',
  totalFont: 'FF000000',
};

const THIN_BORDER = {
  top: { style: 'thin' },
  bottom: { style: 'thin' },
  left: { style: 'thin' },
  right: { style: 'thin' },
};

// 列宽（Excel 字符宽度），与参考模板一致
const COLUMN_WIDTHS = [41, 20.67, 38.78, 22.11, 19.87];

const NUM_FMT_INT = '#,##0';
const NUM_FMT_USD = '"$"#,##0.00';

/**
 * 将「模型消耗统计」列表导出为 Excel 报表。
 * 样式、列宽、行高与参考模板保持一致。
 *
 * @param {Object} params
 * @param {Array}  params.modelSummary - [{ model_name, count, token_used, quota }]
 * @param {string} params.username     - 用户名，用于文件名
 * @param {string} params.period       - 期间，形如 "2026.06"
 * @param {number} params.quotaPerUnit - 额度换算单位（quota / quotaPerUnit = USD）
 */
export async function exportModelConsumptionExcel({
  modelSummary,
  username,
  period,
  quotaPerUnit,
}) {
  const rows = Array.isArray(modelSummary) ? modelSummary : [];
  const perUnit =
    Number.isFinite(quotaPerUnit) && quotaPerUnit > 0 ? quotaPerUnit : 500000;

  const workbook = new ExcelJS.Workbook();
  const ws = workbook.addWorksheet('模型调用消耗统计');

  // 列宽
  COLUMN_WIDTHS.forEach((w, i) => {
    ws.getColumn(i + 1).width = w;
  });

  // 第 1 行：标题（合并 A1:E1）
  ws.mergeCells('A1:E1');
  const titleCell = ws.getCell('A1');
  titleCell.value = `${period}大模型调用消耗统计报表`;
  titleCell.font = { name: '微软雅黑', size: 14, bold: true, color: { argb: COLOR.headerFont } };
  titleCell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLOR.titleFill } };
  titleCell.alignment = { horizontal: 'center', vertical: 'center' };
  ws.getRow(1).height = 30;

  // 第 2 行：表头
  const headers = ['模型名称', '请求次数', 'Token用量', '消耗额度(USD)', '折扣后额度(USD)'];
  const headerRow = ws.getRow(2);
  headers.forEach((h, i) => {
    const cell = headerRow.getCell(i + 1);
    cell.value = h;
    cell.font = { name: '微软雅黑', size: 11, bold: true, color: { argb: COLOR.headerFont } };
    cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLOR.headerFill } };
    cell.alignment = { horizontal: 'center', vertical: 'center' };
    cell.border = THIN_BORDER;
  });
  headerRow.height = 29;

  // 数据行（从第 3 行开始）
  const firstDataRow = 3;
  rows.forEach((item, idx) => {
    const rowNum = firstDataRow + idx;
    const row = ws.getRow(rowNum);
    const usd = (Number(item.quota) || 0) / perUnit;
    const values = [
      item.model_name ?? '',
      Number(item.count) || 0,
      Number(item.token_used) || 0,
      usd,
      usd, // 折扣后额度：无独立折扣数据，与消耗额度一致
    ];
    const striped = idx % 2 === 0; // 首行开始条纹填充
    values.forEach((v, i) => {
      const cell = row.getCell(i + 1);
      cell.value = v;
      cell.font = { name: '微软雅黑', size: 11, color: { argb: COLOR.bodyFont } };
      cell.alignment = {
        horizontal: i === 0 ? 'left' : 'right',
        vertical: 'center',
      };
      cell.border = THIN_BORDER;
      if (striped) {
        cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLOR.stripeFill } };
      }
      if (i === 1 || i === 2) cell.numFmt = NUM_FMT_INT;
      if (i === 3 || i === 4) cell.numFmt = NUM_FMT_USD;
    });
    row.height = 22;
  });

  // 合计行
  const totalRowNum = firstDataRow + rows.length;
  const lastDataRow = totalRowNum - 1;
  const totalRow = ws.getRow(totalRowNum);
  const totalDefs = [
    { v: '合计', align: 'center' },
    { v: rows.length ? { formula: `SUM(B${firstDataRow}:B${lastDataRow})` } : 0, align: 'right', fmt: NUM_FMT_INT },
    { v: rows.length ? { formula: `SUM(C${firstDataRow}:C${lastDataRow})` } : 0, align: 'right', fmt: NUM_FMT_INT },
    { v: rows.length ? { formula: `SUM(D${firstDataRow}:D${lastDataRow})` } : 0, align: 'right', fmt: NUM_FMT_USD },
    { v: rows.length ? { formula: `SUM(E${firstDataRow}:E${lastDataRow})` } : 0, align: 'right', fmt: NUM_FMT_USD },
  ];
  totalDefs.forEach((def, i) => {
    const cell = totalRow.getCell(i + 1);
    cell.value = def.v;
    cell.font = { name: '微软雅黑', size: 11, bold: true, color: { argb: COLOR.totalFont } };
    cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: COLOR.totalFill } };
    cell.alignment = { horizontal: def.align, vertical: 'center' };
    cell.border = THIN_BORDER;
    if (def.fmt) cell.numFmt = def.fmt;
  });
  totalRow.height = 22;

  // 冻结表头
  ws.views = [{ state: 'frozen', ySplit: 1 }];

  // 生成并触发下载
  const buffer = await workbook.xlsx.writeBuffer();
  const blob = new Blob([buffer], {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `${period}-大模型调用消耗统计-${username || 'user'}.xlsx`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}
