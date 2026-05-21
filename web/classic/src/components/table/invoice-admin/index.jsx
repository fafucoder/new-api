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
import CardPro from '../../common/ui/CardPro';
import InvoiceAdminTable from './InvoiceAdminTable';
import InvoiceAdminActions from './InvoiceAdminActions';
import InvoiceAdminFilters from './InvoiceAdminFilters';
import InvoiceAdminDescription from './InvoiceAdminDescription';
import RejectInvoiceModal from './modals/RejectInvoiceModal';
import IssueUploadModal from './modals/IssueUploadModal';
import { useInvoiceAdminData } from '../../../hooks/invoiceAdmin/useInvoiceAdminData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

function InvoiceAdminPage() {
  const data = useInvoiceAdminData();
  const isMobile = useIsMobile();

  const {
    t,
    totalCount,
    loading,
    searching,
    activePage,
    pageSize,
    handlePageChange,
    handlePageSizeChange,
    formInitValues,
    setFormApi,
    searchInvoices,
    compactMode,
    setCompactMode,
    refresh,
    rejectingRule,
    rejectReason,
    setRejectReason,
    rejectSubmitting,
    closeReject,
    submitReject,
    uploadingRule,
    uploadSubmitting,
    closeUpload,
    submitUpload,
  } = data;

  return (
    <>
      <RejectInvoiceModal
        visible={!!rejectingRule}
        rule={rejectingRule}
        reason={rejectReason}
        setReason={setRejectReason}
        submitting={rejectSubmitting}
        onCancel={closeReject}
        onSubmit={submitReject}
      />
      <IssueUploadModal
        visible={!!uploadingRule}
        rule={uploadingRule}
        submitting={uploadSubmitting}
        onCancel={closeUpload}
        onSubmit={submitUpload}
      />

      <CardPro
        type='type1'
        descriptionArea={
          <InvoiceAdminDescription
            compactMode={compactMode}
            setCompactMode={setCompactMode}
            t={t}
          />
        }
        actionsArea={
          <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
            <InvoiceAdminActions
              refresh={refresh}
              loading={loading}
              t={t}
            />

            <div className='w-full md:w-full lg:w-auto order-1 md:order-2'>
              <InvoiceAdminFilters
                formInitValues={formInitValues}
                setFormApi={setFormApi}
                searchInvoices={searchInvoices}
                loading={loading}
                searching={searching}
                t={t}
              />
            </div>
          </div>
        }
        paginationArea={createCardProPagination({
          currentPage: activePage,
          pageSize: pageSize,
          total: totalCount,
          onPageChange: handlePageChange,
          onPageSizeChange: handlePageSizeChange,
          isMobile: isMobile,
          t: t,
        })}
        t={t}
      >
        <InvoiceAdminTable {...data} />
      </CardPro>
    </>
  );
}

export default InvoiceAdminPage;
