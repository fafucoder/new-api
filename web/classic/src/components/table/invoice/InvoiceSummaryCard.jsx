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

import React from 'react';
import { Card, Typography } from '@douyinfe/semi-ui';

const { Text } = Typography;

const formatMoney = (n) => {
  if (n === undefined || n === null || isNaN(n)) return '—';
  return `$${Number(n).toFixed(4)}`;
};

const StatItem = ({ label, value, color }) => (
  <div className='min-w-[140px]'>
    <Text type='tertiary' size='small'>
      {label}
    </Text>
    <div>
      <Text strong style={color ? { color } : undefined}>
        {value}
      </Text>
    </div>
  </div>
);

const InvoiceSummaryCard = ({ summary, t, style }) => {
  if (!summary) return null;
  return (
    <Card
      className='!rounded-2xl shadow-sm border-0'
      bodyStyle={{ padding: 16 }}
      style={style}
    >
      <div className='flex flex-wrap gap-8'>
        <StatItem label={t('充值总额')} value={formatMoney(summary.topup_total)} />
        <StatItem
          label={t('已锁定金额')}
          value={formatMoney(summary.invoiced_total)}
        />
        <StatItem
          label={t('可开票余额')}
          value={formatMoney(summary.billable)}
          color='var(--semi-color-success)'
        />
        <StatItem
          label={t('最低开票额度')}
          value={formatMoney(summary.minimum_amount)}
        />
      </div>
    </Card>
  );
};

export default InvoiceSummaryCard;
