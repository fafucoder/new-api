import React, { useMemo } from 'react';
import { Empty } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import CardTable from '../../common/ui/CardTable';
import { getProxiesColumns } from './ProxiesColumnDefs';

const ProxiesTable = (props) => {
  const {
    t,
    proxies,
    loading,
    searching,
    compactMode,
    openEdit,
    openReferences,
    handleTest,
    handleDelete,
    handleToggleStatus,
    testingIds,
  } = props;

  const allColumns = useMemo(
    () =>
      getProxiesColumns({
        t,
        openEdit,
        openReferences,
        handleTest,
        handleDelete,
        handleToggleStatus,
        testingIds,
      }),
    [
      t,
      openEdit,
      openReferences,
      handleTest,
      handleDelete,
      handleToggleStatus,
      testingIds,
    ],
  );

  const tableColumns = useMemo(() => {
    return compactMode
      ? allColumns.map(({ fixed, ...rest }) => rest)
      : allColumns;
  }, [compactMode, allColumns]);

  return (
    <CardTable
      columns={tableColumns}
      dataSource={proxies}
      scroll={compactMode ? undefined : { x: 'max-content' }}
      hidePagination
      empty={
        <Empty
          image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
          darkModeImage={
            <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
          }
          description={t('暂无代理')}
          style={{ padding: 30 }}
        />
      }
      className='rounded-xl overflow-hidden'
      size='middle'
      loading={loading || searching}
    />
  );
};

export default ProxiesTable;
