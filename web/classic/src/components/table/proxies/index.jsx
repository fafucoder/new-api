import React from 'react';
import CardPro from '../../common/ui/CardPro';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';
import { useProxiesData } from '../../../hooks/proxies/useProxiesData';
import ProxiesTable from './ProxiesTable';
import ProxiesActions from './ProxiesActions';
import ProxiesFilters from './ProxiesFilters';
import EditProxyModal from './modals/EditProxyModal';
import ProxyReferencesModal from './modals/ProxyReferencesModal';

const ProxiesPage = () => {
  const proxiesData = useProxiesData();
  const isMobile = useIsMobile();

  const {
    t,
    total,
    activePage,
    pageSize,
    keyword,
    setKeyword,
    statusFilter,
    setStatusFilter,
    searching,
    handlePageChange,
    handlePageSizeChange,
    searchProxies,
    refresh,
    showEdit,
    editingProxy,
    openEdit,
    closeEdit,
    showReferences,
    referencesProxy,
    closeReferences,
  } = proxiesData;

  return (
    <>
      <EditProxyModal
        visible={showEdit}
        initial={editingProxy}
        onCancel={closeEdit}
        onSuccess={() => {
          closeEdit();
          refresh();
        }}
      />
      <ProxyReferencesModal
        visible={showReferences}
        proxy={referencesProxy}
        onCancel={closeReferences}
      />

      <CardPro
        type='type1'
        actionsArea={
          <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
            <ProxiesActions t={t} openEdit={openEdit} refresh={refresh} />
            <div className='w-full md:w-full lg:w-auto order-1 md:order-2'>
              <ProxiesFilters
                t={t}
                keyword={keyword}
                setKeyword={setKeyword}
                statusFilter={statusFilter}
                setStatusFilter={setStatusFilter}
                searchProxies={searchProxies}
                searching={searching}
              />
            </div>
          </div>
        }
        paginationArea={createCardProPagination({
          currentPage: activePage,
          pageSize,
          total,
          onPageChange: handlePageChange,
          onPageSizeChange: handlePageSizeChange,
          isMobile,
          t,
        })}
        t={t}
      >
        <ProxiesTable {...proxiesData} />
      </CardPro>
    </>
  );
};

export default ProxiesPage;
