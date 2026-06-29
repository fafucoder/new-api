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

import React, { useState } from 'react';
import { Button, Input, Radio, RadioGroup, DatePicker } from '@douyinfe/semi-ui';
import { RefreshCw, Search, X, UserSearch } from 'lucide-react';

const RANGE_OPTIONS = [
  { value: '1d', labelKey: '最近一天' },
  { value: '7d', labelKey: '最近一周' },
  { value: '30d', labelKey: '最近一月' },
  { value: 'last_month', labelKey: '上个月' },
  { value: 'custom', labelKey: '自定义' },
];

const DashboardHeader = ({
  getGreeting,
  greetingVisible,
  showSearchModal,
  refresh,
  loading,
  t,
  isAdminUser,
  onAdminQuery,
  onAdminReset,
  adminQueryLoading,
  adminQueryActive,
}) => {
  const ICON_BUTTON_CLASS = 'text-white hover:bg-opacity-80 !rounded-full';

  const [username, setUsername] = useState('');
  const [range, setRange] = useState('7d');
  const [customDates, setCustomDates] = useState(null);

  const handleQuery = () => {
    const trimmed = username.trim();
    if (!trimmed) return;
    if (range === 'custom') {
      if (!customDates || !customDates[0] || !customDates[1]) return;
      const startTs = Math.floor(customDates[0].getTime() / 1000);
      const endTs = Math.floor(customDates[1].getTime() / 1000);
      const fmt = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
      const label = `${fmt(customDates[0])} ~ ${fmt(customDates[1])}`;
      onAdminQuery(trimmed, { range: 'custom', startTimestamp: startTs, endTimestamp: endTs, label });
    } else if (range === 'last_month') {
      const now = new Date();
      const start = new Date(now.getFullYear(), now.getMonth() - 1, 1);
      const end = new Date(now.getFullYear(), now.getMonth(), 0, 23, 59, 59);
      const startTs = Math.floor(start.getTime() / 1000);
      const endTs = Math.floor(end.getTime() / 1000);
      onAdminQuery(trimmed, { range: 'last_month', startTimestamp: startTs, endTimestamp: endTs, label: t('上个月') });
    } else {
      const labelKey = RANGE_OPTIONS.find(o => o.value === range)?.labelKey || '最近一周';
      onAdminQuery(trimmed, { range, label: t(labelKey) });
    }
  };

  const handleReset = () => {
    setUsername('');
    setRange('7d');
    setCustomDates(null);
    onAdminReset();
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') handleQuery();
  };

  const canQuery = username.trim() && (range !== 'custom' || (customDates && customDates[0] && customDates[1]));

  return (
    <div className='flex items-center justify-between mb-4 flex-wrap gap-3'>
      <h2
        className='text-2xl font-semibold text-gray-800 transition-opacity duration-1000 ease-in-out whitespace-nowrap'
        style={{ opacity: greetingVisible ? 1 : 0 }}
      >
        {getGreeting}
      </h2>
      <div className='flex items-center gap-3 flex-wrap'>
        {isAdminUser && (
          <>
            <div className='flex items-center gap-2'>
              <UserSearch size={16} className='text-gray-500 flex-shrink-0' />
              <Input
                placeholder={t('输入用户名查询')}
                value={username}
                onChange={setUsername}
                onKeyDown={handleKeyDown}
                size='small'
                style={{ width: 160 }}
              />
            </div>
            <RadioGroup
              type='button'
              buttonSize='small'
              value={range}
              onChange={(e) => setRange(e.target.value)}
            >
              {RANGE_OPTIONS.map((opt) => (
                <Radio key={opt.value} value={opt.value}>
                  {t(opt.labelKey)}
                </Radio>
              ))}
            </RadioGroup>
            {range === 'custom' && (
              <DatePicker
                type='dateRange'
                density='compact'
                size='small'
                value={customDates}
                onChange={setCustomDates}
                style={{ width: 240 }}
              />
            )}
            <Button
              theme='solid'
              type='primary'
              size='small'
              icon={<Search size={14} />}
              onClick={handleQuery}
              loading={adminQueryLoading}
              disabled={!canQuery}
            >
              {t('查询')}
            </Button>
            {adminQueryActive && (
              <Button
                theme='borderless'
                type='tertiary'
                size='small'
                icon={<X size={14} />}
                onClick={handleReset}
              >
                {t('重置')}
              </Button>
            )}
          </>
        )}
        <Button
          type='tertiary'
          icon={<Search size={16} />}
          onClick={showSearchModal}
          className={`bg-green-500 hover:bg-green-600 ${ICON_BUTTON_CLASS}`}
        />
        <Button
          type='tertiary'
          icon={<RefreshCw size={16} />}
          onClick={refresh}
          loading={loading}
          className={`bg-blue-500 hover:bg-blue-600 ${ICON_BUTTON_CLASS}`}
        />
      </div>
    </div>
  );
};

export default DashboardHeader;
