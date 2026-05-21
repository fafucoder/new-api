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
import BalanceAlertTable from './BalanceAlertTable';
import BalanceAlertActions from './BalanceAlertActions';
import BalanceAlertFilters from './BalanceAlertFilters';
import BalanceAlertDescription from './BalanceAlertDescription';
import EditRuleModal from './modals/EditRuleModal';
import TopupModal from './modals/TopupModal';
import { useBalanceAlertData } from '../../../hooks/balanceAlert/useBalanceAlertData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

function BalanceAlertPage() {
  const data = useBalanceAlertData();
  const isMobile = useIsMobile();

  const {
    t,
    rules,
    totalCount,
    tags,
    loading,
    searching,
    activePage,
    pageSize,
    handlePageChange,
    handlePageSizeChange,
    formInitValues,
    setFormApi,
    searchRules,
    compactMode,
    setCompactMode,
    refresh,
    openCreate,
    openEdit,
    openTopup,
    handleDelete,
    handleTest,
    handleToggleEnabled,
    showEdit,
    form,
    setForm,
    closeEdit,
    submitForm,
    topupRule,
    topupForm,
    setTopupForm,
    topupSubmitting,
    closeTopup,
    submitTopup,
  } = data;

  return (
    <>
      <EditRuleModal
        visible={showEdit}
        form={form}
        setForm={setForm}
        tags={tags}
        onCancel={closeEdit}
        onSubmit={submitForm}
        t={t}
      />

      <TopupModal
        visible={!!topupRule}
        rule={topupRule}
        topupForm={topupForm}
        setTopupForm={setTopupForm}
        submitting={topupSubmitting}
        onCancel={closeTopup}
        onSubmit={submitTopup}
        t={t}
      />

      <CardPro
        type='type1'
        descriptionArea={
          <BalanceAlertDescription
            compactMode={compactMode}
            setCompactMode={setCompactMode}
            t={t}
          />
        }
        actionsArea={
          <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
            <BalanceAlertActions
              openCreate={openCreate}
              refresh={refresh}
              loading={loading}
              t={t}
            />

            <div className='w-full md:w-full lg:w-auto order-1 md:order-2'>
              <BalanceAlertFilters
                formInitValues={formInitValues}
                setFormApi={setFormApi}
                searchRules={searchRules}
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
        <BalanceAlertTable {...data} />
      </CardPro>
    </>
  );
}

export default BalanceAlertPage;
