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
import InvoiceTable from './InvoiceTable';
import InvoiceActions from './InvoiceActions';
import InvoiceFilters from './InvoiceFilters';
import InvoiceDescription from './InvoiceDescription';
import ApplyInvoiceModal from './modals/ApplyInvoiceModal';
import InvoiceSummaryCard from './InvoiceSummaryCard';
import { useInvoiceData } from '../../../hooks/invoice/useInvoiceData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

function InvoicePage() {
  const data = useInvoiceData();
  const isMobile = useIsMobile();

  const {
    t,
    summary,
    invoices,
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
    canApply,
    openApply,
    applyVisible,
    applyForm,
    setApplyForm,
    applySubmitting,
    closeApply,
    submitApply,
  } = data;

  return (
    <>
      <ApplyInvoiceModal
        visible={applyVisible}
        applyForm={applyForm}
        setApplyForm={setApplyForm}
        summary={summary}
        submitting={applySubmitting}
        onCancel={closeApply}
        onSubmit={submitApply}
      />

      <InvoiceSummaryCard summary={summary} t={t} style={{ marginBottom: 10 }} />

      <CardPro
        type='type1'
        descriptionArea={
          <InvoiceDescription
            compactMode={compactMode}
            setCompactMode={setCompactMode}
            t={t}
          />
        }
        actionsArea={
          <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
            <InvoiceActions
              canApply={canApply}
              summary={summary}
              openApply={openApply}
              refresh={refresh}
              loading={loading}
              t={t}
            />

            <div className='w-full md:w-full lg:w-auto order-1 md:order-2'>
              <InvoiceFilters
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
        <InvoiceTable {...data} />
      </CardPro>
    </>
  );
}

export default InvoicePage;
