import React from 'react';
import { Button, Input, Select } from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';

const ProxiesFilters = ({
  t,
  keyword,
  setKeyword,
  statusFilter,
  setStatusFilter,
  searchProxies,
  searching,
}) => (
  <div className='flex flex-wrap items-center gap-2'>
    <Input
      placeholder={t('搜索名称/地址/描述')}
      value={keyword}
      onChange={(v) => setKeyword(v)}
      style={{ width: 240 }}
      showClear
      onEnterPress={searchProxies}
    />
    <Select
      value={statusFilter}
      onChange={(v) => setStatusFilter(v)}
      style={{ width: 140 }}
    >
      <Select.Option value={0}>{t('全部状态')}</Select.Option>
      <Select.Option value={1}>{t('启用')}</Select.Option>
      <Select.Option value={2}>{t('禁用')}</Select.Option>
    </Select>
    <Button
      icon={<IconSearch />}
      theme='light'
      loading={searching}
      onClick={searchProxies}
    >
      {t('搜索')}
    </Button>
  </div>
);

export default ProxiesFilters;
