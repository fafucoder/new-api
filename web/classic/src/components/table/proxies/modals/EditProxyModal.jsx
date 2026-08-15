import React, { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Modal, Form, Toast } from '@douyinfe/semi-ui';
import { createProxy, updateProxy } from '../../../../services/proxy';

const EditProxyModal = ({ visible, initial, onCancel, onSuccess }) => {
  const { t } = useTranslation();
  const [formApi, setFormApi] = React.useState(null);
  const [submitting, setSubmitting] = React.useState(false);

  useEffect(() => {
    if (!visible || !formApi) return;
    if (initial) {
      formApi.setValues({
        name: initial.name || '',
        type: initial.type || 'http',
        url: initial.url || '',
        test_url: initial.test_url || '',
        description: initial.description || '',
        status: initial.status || 1,
      });
    } else {
      formApi.setValues({
        name: '',
        type: 'http',
        url: '',
        test_url: '',
        description: '',
        status: 1,
      });
    }
  }, [visible, initial, formApi]);

  const handleOk = async () => {
    if (!formApi) return;
    let values;
    try {
      values = await formApi.validate();
    } catch {
      return;
    }
    const payload = {
      ...values,
      status: Number(values.status) || 1,
    };
    if (initial?.id) {
      payload.id = initial.id;
    }
    setSubmitting(true);
    try {
      const res = initial?.id
        ? await updateProxy(payload)
        : await createProxy(payload);
      if (res.data?.success) {
        Toast.success(t('保存成功'));
        onSuccess?.();
      } else {
        Toast.error(res.data?.message || t('保存失败'));
      }
    } catch (e) {
      Toast.error(e?.message || t('保存失败'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={initial?.id ? t('编辑代理') : t('新建代理')}
      visible={visible}
      onOk={handleOk}
      onCancel={onCancel}
      confirmLoading={submitting}
      width={560}
      maskClosable={false}
    >
      <Form getFormApi={setFormApi} labelPosition='left' labelWidth={110}>
        <Form.Input
          field='name'
          label={t('代理名称')}
          rules={[{ required: true, message: t('请输入代理名称') }]}
          maxLength={64}
        />
        <Form.Select
          field='type'
          label={t('代理类型')}
          rules={[{ required: true }]}
          optionList={[
            { label: 'HTTP', value: 'http' },
            { label: 'HTTPS', value: 'https' },
            { label: 'SOCKS5', value: 'socks5' },
          ]}
        />
        <Form.Input
          field='url'
          label={t('代理地址')}
          rules={[{ required: true, message: t('请输入代理地址') }]}
          placeholder='socks5://user:pass@host:port'
          maxLength={512}
        />
        <Form.Input
          field='test_url'
          label={t('测试目标 URL')}
          placeholder={t('留空使用全局默认')}
          maxLength={512}
        />
        <Form.TextArea
          field='description'
          label={t('备注')}
          maxLength={255}
          rows={2}
        />
        <Form.Select
          field='status'
          label={t('状态')}
          optionList={[
            { label: t('启用'), value: 1 },
            { label: t('禁用'), value: 2 },
          ]}
        />
      </Form>
    </Modal>
  );
};

export default EditProxyModal;
