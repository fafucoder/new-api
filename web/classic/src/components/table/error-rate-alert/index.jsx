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

import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import CardPro from '../../common/ui/CardPro';
import ErrorRateAlertTable from './ErrorRateAlertTable';
import ErrorRateAlertDescription from './ErrorRateAlertDescription';
import ErrorRateAlertActions from './ErrorRateAlertActions';
import EditRuleModal from './modals/EditRuleModal';
import { API, showError, showSuccess } from '../../../helpers';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

function ErrorRateAlertPage() {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [rules, setRules] = useState([]);
  const [loading, setLoading] = useState(false);
  const [showEdit, setShowEdit] = useState(false);
  const [editingRule, setEditingRule] = useState(null);
  const [channels, setChannels] = useState([]);
  const [tags, setTags] = useState([]);
  const [compactMode, setCompactMode] = useState(false);

  useEffect(() => {
    loadRules();
    loadChannels();
    loadTags();
  }, []);

  const loadRules = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/error-rate-alert');
      const { success, message, data } = res.data;
      if (success) {
        setRules(data || []);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  };

  const loadChannels = async () => {
    try {
      const res = await API.get('/api/channel/');
      const { success, data } = res.data;
      if (success) {
        setChannels(data || []);
      }
    } catch (error) {
      console.error('Failed to load channels:', error);
    }
  };

  const loadTags = async () => {
    try {
      const res = await API.get('/api/balance-alert/tags');
      const { success, data } = res.data;
      if (success) {
        setTags(data || []);
      }
    } catch (error) {
      console.error('Failed to load tags:', error);
    }
  };

  const handleCreate = () => {
    setEditingRule({
      name: '',
      scope: 'global',
      scope_value: '',
      time_window: '5m',
      threshold: 10,
      webhook_url: '',
      webhook_secret: '',
      enabled: true,
      remark: '',
    });
    setShowEdit(true);
  };

  const handleEdit = (rule) => {
    setEditingRule({ ...rule });
    setShowEdit(true);
  };

  const handleDelete = async (id) => {
    try {
      const res = await API.delete(`/api/error-rate-alert/${id}`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('删除成功'));
        loadRules();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
  };

  const handleTest = async (id) => {
    try {
      const res = await API.post(`/api/error-rate-alert/${id}/test`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('测试告警已发送'));
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
  };

  const handleToggleEnabled = async (rule, value) => {
    try {
      const res = await API.put(`/api/error-rate-alert/${rule.id}`, {
        ...rule,
        enabled: value,
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('更新成功'));
        loadRules();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
  };

  const handleSubmit = async (values) => {
    try {
      const isEdit = !!editingRule?.id;
      const url = isEdit
        ? `/api/error-rate-alert/${editingRule.id}`
        : '/api/error-rate-alert';
      const method = isEdit ? 'put' : 'post';

      const res = await API[method](url, values);
      const { success, message } = res.data;
      if (success) {
        showSuccess(isEdit ? t('更新成功') : t('创建成功'));
        setShowEdit(false);
        setEditingRule(null);
        loadRules();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
  };

  const closeEdit = () => {
    setShowEdit(false);
    setEditingRule(null);
  };

  return (
    <>
      <EditRuleModal
        visible={showEdit}
        form={editingRule || {}}
        setForm={setEditingRule}
        channels={channels}
        tags={tags}
        onCancel={closeEdit}
        onSubmit={handleSubmit}
        t={t}
      />

      <CardPro
        type='type1'
        descriptionArea={
          <ErrorRateAlertDescription
            compactMode={compactMode}
            setCompactMode={setCompactMode}
            t={t}
          />
        }
        actionsArea={
          <ErrorRateAlertActions
            openCreate={handleCreate}
            refresh={loadRules}
            loading={loading}
            t={t}
          />
        }
        t={t}
      >
        <ErrorRateAlertTable
          rules={rules}
          channels={channels}
          loading={loading}
          compactMode={compactMode}
          openEdit={handleEdit}
          handleTest={handleTest}
          handleDelete={handleDelete}
          handleToggleEnabled={handleToggleEnabled}
          t={t}
        />
      </CardPro>
    </>
  );
}

export default ErrorRateAlertPage;
