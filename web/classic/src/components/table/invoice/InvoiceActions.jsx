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
import { Button, Tooltip } from '@douyinfe/semi-ui';

const InvoiceActions = ({
  canApply,
  summary,
  openApply,
  refresh,
  loading,
  t,
}) => {
  const tooltipContent = (() => {
    if (canApply) return '';
    if (!summary) return '';
    if (!summary.enabled) return t('发票功能已关闭');
    if (summary.has_in_flight) return t('有待处理申请');
    if ((summary.billable || 0) < (summary.minimum_amount || 0))
      return t('余额低于最低额度');
    return '';
  })();

  return (
    <div className='flex flex-wrap gap-2 w-full md:w-auto order-2 md:order-1'>
      {tooltipContent ? (
        <Tooltip content={tooltipContent} position='top'>
          <span className='flex-1 md:flex-initial'>
            <Button
              type='primary'
              className='w-full md:w-auto'
              onClick={openApply}
              disabled={!canApply}
              size='small'
            >
              {t('申请开票')}
            </Button>
          </span>
        </Tooltip>
      ) : (
        <Button
          type='primary'
          className='flex-1 md:flex-initial md:w-auto'
          onClick={openApply}
          size='small'
        >
          {t('申请开票')}
        </Button>
      )}

      <Button
        type='tertiary'
        className='flex-1 md:flex-initial'
        onClick={refresh}
        loading={loading}
        size='small'
      >
        {t('刷新')}
      </Button>
    </div>
  );
};

export default InvoiceActions;
