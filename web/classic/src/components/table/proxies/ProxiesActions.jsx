import React from 'react';
import { Button } from '@douyinfe/semi-ui';
import { IconPlus, IconRefresh } from '@douyinfe/semi-icons';

const ProxiesActions = ({ t, openEdit, refresh }) => (
  <div className='flex flex-wrap items-center gap-2'>
    <Button
      theme='solid'
      type='primary'
      icon={<IconPlus />}
      onClick={() => openEdit(null)}
    >
      {t('新建代理')}
    </Button>
    <Button theme='light' icon={<IconRefresh />} onClick={refresh}>
      {t('刷新')}
    </Button>
  </div>
);

export default ProxiesActions;
