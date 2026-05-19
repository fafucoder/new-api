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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Banner,
  Button,
  Card,
  Collapse,
  Empty,
  Form,
  Input,
  Popconfirm,
  Select,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  CheckCircle,
  ChevronRight,
  CircleSlash,
  History,
  PlayCircle,
  RefreshCw,
  ShieldAlert,
  ShieldCheck,
  ShieldOff,
  ShieldQuestion,
  Trash2,
  XCircle,
} from 'lucide-react';
import { API, isAdmin, showError } from '../../helpers';

const { Title, Text, Paragraph } = Typography;

// Anthropic-direct channels are the only ones the backend supports. AWS
// Bedrock (type 33) uses a different request format and would need its
// own probe.
const SUPPORTED_CHANNEL_TYPE = 14;

const VERDICT_META = {
  real: {
    icon: ShieldCheck,
    color: 'var(--semi-color-success)',
    bg: 'var(--semi-color-success-light-default)',
    border: 'var(--semi-color-success-light-active)',
    tagColor: 'green',
  },
  suspicious: {
    icon: ShieldAlert,
    color: 'var(--semi-color-warning)',
    bg: 'var(--semi-color-warning-light-default)',
    border: 'var(--semi-color-warning-light-active)',
    tagColor: 'orange',
  },
  fake: {
    icon: ShieldOff,
    color: 'var(--semi-color-danger)',
    bg: 'var(--semi-color-danger-light-default)',
    border: 'var(--semi-color-danger-light-active)',
    tagColor: 'red',
  },
  unknown: {
    icon: ShieldQuestion,
    color: 'var(--semi-color-text-2)',
    bg: 'var(--semi-color-fill-0)',
    border: 'var(--semi-color-border)',
    tagColor: 'grey',
  },
};

const stepLabel = (kind, t) => {
  switch (kind) {
    case 'baseline':
      return t('Step 1 · 基线探测');
    case 'multi_turn':
      return t('Step 2 · 多轮签名重放');
    case 'tamper':
      return t('Step 3 · 签名篡改负测');
    case 'cross':
      return t('可选 · 交叉渠道检测');
    default:
      return kind;
  }
};

// verdictLabel maps a verdict key to a translated string. Defined as a
// function (not a static map of t()-results) so the labels are looked up
// via static t('...') calls — i18next-cli's extractor needs that to find
// the keys.
const verdictLabel = (verdict, t) => {
  switch (verdict) {
    case 'real':
      return t('真实模型');
    case 'suspicious':
      return t('可疑代理');
    case 'fake':
      return t('伪造代理');
    default:
      return t('无法判定');
  }
};

// formatTimestamp converts a unix seconds value to "YYYY-MM-DD HH:mm".
const formatTimestamp = (ts) => {
  if (!ts) return '—';
  const d = new Date(ts * 1000);
  if (isNaN(d.getTime())) return '—';
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

const ChannelValidationPage = () => {
  const { t } = useTranslation();
  const admin = isAdmin();
  const [channels, setChannels] = useState([]);
  const [channelsLoading, setChannelsLoading] = useState(admin);
  const [channelModels, setChannelModels] = useState([]);
  const [channelModelsLoading, setChannelModelsLoading] = useState(false);
  const [userModels, setUserModels] = useState([]);
  const [userModelsLoading, setUserModelsLoading] = useState(!admin);
  const [form, setForm] = useState({
    channel_id: null,
    model: '',
    max_tokens: 512,
    cross_channel_id: 0,
  });
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState(null);
  const [error, setError] = useState('');

  // History panel state
  const [records, setRecords] = useState([]);
  const [recordsLoading, setRecordsLoading] = useState(false);
  const [recordsTotal, setRecordsTotal] = useState(0);
  const [selectedRecordId, setSelectedRecordId] = useState(null);
  const [recordView, setRecordView] = useState('live'); // 'live' or 'history'

  // Load channel list (admin) or model list (user) on mount.
  useEffect(() => {
    if (admin) {
      const loadChannels = async () => {
        try {
          const res = await API.get(
            `/api/channel/?type=${SUPPORTED_CHANNEL_TYPE}&p=0&p_size=200`,
          );
          const { success, data } = res.data || {};
          if (success) {
            const list = Array.isArray(data?.items)
              ? data.items
              : Array.isArray(data)
                ? data
                : [];
            setChannels(list.filter((ch) => ch && ch.status !== 2));
          }
        } catch (e) {
          showError(e);
        } finally {
          setChannelsLoading(false);
        }
      };
      loadChannels();
      return;
    }
    const loadModels = async () => {
      try {
        const res = await API.get('/api/channel-validation/models');
        const { success, data } = res.data || {};
        if (success) {
          const list = Array.isArray(data?.models) ? data.models : [];
          setUserModels(list);
          // Auto-select the first model if none chosen yet.
          if (list.length > 0) {
            setForm((f) => (f.model ? f : { ...f, model: list[0] }));
          }
        }
      } catch (e) {
        showError(e);
      } finally {
        setUserModelsLoading(false);
      }
    };
    loadModels();
  }, [admin]);

  // When admin picks a channel, fetch that channel's configured models
  // and reset the model selection if it's not in the new list.
  useEffect(() => {
    if (!admin) return;
    if (!form.channel_id) {
      setChannelModels([]);
      setForm((f) => (f.model ? { ...f, model: '' } : f));
      return;
    }
    let cancelled = false;
    const loadChannelModels = async () => {
      setChannelModelsLoading(true);
      try {
        const res = await API.get(
          `/api/channel-validation/channels/${form.channel_id}/models`,
        );
        const { success, data } = res.data || {};
        if (cancelled) return;
        if (success) {
          const list = Array.isArray(data?.models) ? data.models : [];
          setChannelModels(list);
          setForm((f) => {
            if (list.length === 0) return { ...f, model: '' };
            if (f.model && list.includes(f.model)) return f;
            return { ...f, model: list[0] };
          });
        }
      } catch (e) {
        if (!cancelled) showError(e);
      } finally {
        if (!cancelled) setChannelModelsLoading(false);
      }
    };
    loadChannelModels();
    return () => {
      cancelled = true;
    };
  }, [admin, form.channel_id]);

  const loadRecords = useCallback(async () => {
    setRecordsLoading(true);
    try {
      const res = await API.get(
        '/api/channel-validation/records?page=0&page_size=30',
      );
      const { success, data } = res.data || {};
      if (success) {
        setRecords(Array.isArray(data?.items) ? data.items : []);
        setRecordsTotal(data?.total || 0);
      }
    } catch (e) {
      showError(e);
    } finally {
      setRecordsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadRecords();
  }, [loadRecords]);

  const channelOptions = useMemo(
    () =>
      channels.map((ch) => ({
        value: ch.id,
        label: ch.name ? `${ch.name} (#${ch.id})` : `#${ch.id}`,
      })),
    [channels],
  );

  const channelModelOptions = useMemo(
    () => channelModels.map((m) => ({ value: m, label: m })),
    [channelModels],
  );

  const userModelOptions = useMemo(
    () => userModels.map((m) => ({ value: m, label: m })),
    [userModels],
  );

  const handleRun = async () => {
    if (admin && !form.channel_id) {
      setError(t('请先选择一个 Anthropic 渠道'));
      return;
    }
    if (!form.model || !form.model.trim()) {
      setError(t('请选择模型'));
      return;
    }
    setError('');
    setResult(null);
    setRunning(true);
    try {
      const payload = admin
        ? {
            ...form,
            max_tokens: Number(form.max_tokens) || 512,
            cross_channel_id: Number(form.cross_channel_id) || 0,
          }
        : {
            model: form.model,
            max_tokens: Number(form.max_tokens) || 512,
          };
      const res = await API.post('/api/channel-validation/run', payload);
      const { success, data, message } = res.data || {};
      if (success) {
        setResult(data);
        setRecordView('live');
        setSelectedRecordId(data?.record_id || null);
        await loadRecords();
      } else {
        setError(message || t('检测失败'));
      }
    } catch (e) {
      setError(e?.response?.data?.message || e?.message || t('网络错误'));
    } finally {
      setRunning(false);
    }
  };

  const handleSelectRecord = async (id) => {
    if (!id) return;
    setSelectedRecordId(id);
    try {
      const res = await API.get(`/api/channel-validation/records/${id}`);
      const { success, data } = res.data || {};
      if (success && data?.result) {
        setResult(data.result);
        setRecordView('history');
        setError('');
      }
    } catch (e) {
      showError(e);
    }
  };

  const handleDeleteRecord = async (id) => {
    try {
      const res = await API.delete(`/api/channel-validation/records/${id}`);
      if (res.data?.success) {
        if (id === selectedRecordId) {
          setSelectedRecordId(null);
          setResult(null);
        }
        await loadRecords();
      }
    } catch (e) {
      showError(e);
    }
  };

  return (
    <div className='mt-[60px] px-2'>
      <div className='channel-validation-page'>
        <div className='cv-header'>
          <div className='cv-header-left'>
            <Title heading={2} className='cv-title'>
              <ShieldCheck
                size={20}
                style={{ verticalAlign: '-3px', marginRight: 6 }}
              />
              {t('模型检测')}
            </Title>
            <Text type='secondary' className='cv-subtitle'>
              {t(
                '通过 3 步探测(基线 / 多轮签名重放 / 签名篡改负测)判断上游是否真的是 Claude — 检测"挂羊头卖狗肉"代理。',
              )}
            </Text>
          </div>
        </div>

      <Card className='cv-form-card' bordered>
        <Form labelPosition='top'>
          <div className='cv-form-grid'>
            {admin ? (
              <>
                <Form.Slot label={t('Anthropic 渠道')} required>
                  <Select
                    style={{ width: '100%' }}
                    value={form.channel_id}
                    onChange={(v) =>
                      setForm((f) => ({ ...f, channel_id: v || null }))
                    }
                    placeholder={t(
                      '选择已配置的 Anthropic 渠道(channel_type=14)',
                    )}
                    loading={channelsLoading}
                    optionList={channelOptions}
                    emptyContent={<Empty description={t('暂无可用渠道')} />}
                    showClear
                    filter
                  />
                </Form.Slot>
                <Form.Slot label={t('Claude 模型')} required>
                  <Select
                    style={{ width: '100%' }}
                    value={form.model || undefined}
                    onChange={(v) =>
                      setForm((f) => ({ ...f, model: v || '' }))
                    }
                    placeholder={
                      form.channel_id
                        ? t('选择该渠道支持的模型')
                        : t('请先选择渠道')
                    }
                    loading={channelModelsLoading}
                    optionList={channelModelOptions}
                    emptyContent={
                      <Empty
                        description={
                          form.channel_id
                            ? t('该渠道未配置模型')
                            : t('请先选择渠道')
                        }
                      />
                    }
                    disabled={!form.channel_id}
                    showClear
                    filter
                  />
                </Form.Slot>
                <Form.Slot label={t('Max Tokens')}>
                  <Input
                    value={form.max_tokens}
                    onChange={(v) =>
                      setForm((f) => ({ ...f, max_tokens: v }))
                    }
                    placeholder='512'
                  />
                </Form.Slot>
                <Form.Slot label={t('交叉检测渠道(可选)')}>
                  <Select
                    style={{ width: '100%' }}
                    value={form.cross_channel_id || undefined}
                    onChange={(v) =>
                      setForm((f) => ({
                        ...f,
                        cross_channel_id: v || 0,
                      }))
                    }
                    placeholder={t(
                      '选另一个已知真 Claude 渠道用于交叉检测签名',
                    )}
                    optionList={channelOptions}
                    showClear
                    filter
                  />
                </Form.Slot>
              </>
            ) : (
              <>
                <Form.Slot label={t('Claude 模型')} required>
                  <Select
                    style={{ width: '100%' }}
                    value={form.model || undefined}
                    onChange={(v) =>
                      setForm((f) => ({ ...f, model: v || '' }))
                    }
                    placeholder={t('选择你的组可用的 Claude 模型')}
                    loading={userModelsLoading}
                    optionList={userModelOptions}
                    emptyContent={
                      <Empty
                        description={t('当前用户组未配置 Claude 模型')}
                      />
                    }
                    showClear
                    filter
                  />
                </Form.Slot>
                <Form.Slot label={t('Max Tokens')}>
                  <Input
                    value={form.max_tokens}
                    onChange={(v) =>
                      setForm((f) => ({ ...f, max_tokens: v }))
                    }
                    placeholder='512'
                  />
                </Form.Slot>
              </>
            )}
          </div>

          <div className='cv-run-row'>
            <Button
              theme='solid'
              type='primary'
              size='large'
              icon={<PlayCircle size={16} />}
              onClick={handleRun}
              loading={running}
              disabled={running}
            >
              {running ? t('检测中…') : t('开始检测')}
            </Button>
            {error ? (
              <Text type='danger' style={{ marginLeft: 12 }}>
                {error}
              </Text>
            ) : null}
          </div>
        </Form>
      </Card>

      <div className='cv-main-grid'>
        <HistoryPanel
          records={records}
          loading={recordsLoading}
          selectedId={selectedRecordId}
          admin={admin}
          total={recordsTotal}
          onSelect={handleSelectRecord}
          onDelete={handleDeleteRecord}
          onRefresh={loadRecords}
        />
        <div className='cv-result-pane'>
          {running ? (
            <Card className='cv-loading-card' bordered>
              <div className='cv-loading'>
                <Spin size='large' />
                <Text type='secondary' style={{ marginLeft: 12 }}>
                  {t('正在按顺序执行 Step 1/2/3,通常需要 30-90 秒…')}
                </Text>
              </div>
            </Card>
          ) : null}
          {result ? (
            <ResultPanel
              result={result}
              admin={admin}
              view={recordView}
            />
          ) : !running ? (
            <Card bordered className='cv-empty-card'>
              <Empty
                image={<History size={48} color='var(--semi-color-text-3)' />}
                title={t('暂无检测结果')}
                description={t(
                  '运行一次检测或从左侧历史记录中选取一条查看。',
                )}
              />
            </Card>
          ) : null}
        </div>
      </div>

      <style>{`
        .channel-validation-page { padding: 24px; }

        .cv-header {
          display: flex;
          align-items: flex-start;
          justify-content: space-between;
          gap: 24px;
          margin-bottom: 16px;
        }
        .cv-header-left { flex: 1; min-width: 0; }
        .cv-title { margin-bottom: 8px !important; }
        .cv-subtitle { font-size: 13px; }
        .cv-form-card { margin-bottom: 16px; }
        .cv-form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
        .cv-run-row { display: flex; align-items: center; margin-top: 12px; }
        .cv-loading-card { margin-bottom: 16px; }
        .cv-loading { display: flex; align-items: center; padding: 8px; }
        .cv-empty-card { padding: 40px 12px; }
        .cv-main-grid { display: grid; grid-template-columns: 360px minmax(0, 1fr); gap: 16px; align-items: flex-start; }
        @media (max-width: 960px) {
          .cv-main-grid { grid-template-columns: 1fr; }
          .cv-form-grid { grid-template-columns: 1fr; }
        }
        .cv-history-card { position: sticky; top: 72px; }
        .cv-history-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; gap: 8px; }
        .cv-history-title { display: flex; align-items: center; gap: 6px; font-weight: 600; }
        .cv-history-actions { display: flex; gap: 4px; }
        .cv-history-list { display: flex; flex-direction: column; gap: 6px; max-height: 60vh; overflow-y: auto; padding-right: 2px; }
        .cv-history-item { display: flex; align-items: flex-start; gap: 8px; padding: 10px; border: 1px solid var(--semi-color-border); border-radius: 8px; cursor: pointer; transition: background 0.15s, border 0.15s; background: var(--semi-color-bg-1); }
        .cv-history-item:hover { background: var(--semi-color-fill-0); }
        .cv-history-item.active { border-color: var(--semi-color-primary); background: var(--semi-color-primary-light-default); }
        .cv-history-icon { flex-shrink: 0; margin-top: 2px; }
        .cv-history-body { flex: 1; min-width: 0; }
        .cv-history-line1 { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
        .cv-history-model { font-weight: 600; font-size: 13px; truncate: true; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 180px; }
        .cv-history-channel { font-size: 11px; color: var(--semi-color-text-2); margin-top: 2px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
        .cv-history-line3 { display: flex; gap: 8px; font-size: 11px; color: var(--semi-color-text-2); margin-top: 4px; }
        .cv-history-delete { opacity: 0; transition: opacity 0.15s; }
        .cv-history-item:hover .cv-history-delete { opacity: 1; }
        .cv-history-empty { padding: 16px 8px; text-align: center; color: var(--semi-color-text-2); font-size: 13px; }
        .cv-result-pane { display: flex; flex-direction: column; gap: 16px; min-width: 0; }
        .cv-verdict-banner { padding: 16px; border-radius: 8px; border: 1px solid; display: flex; align-items: center; gap: 12px; }
        .cv-verdict-icon { flex-shrink: 0; }
        .cv-verdict-body { flex: 1; min-width: 0; }
        .cv-view-tag { margin-left: 8px; vertical-align: middle; }
        .cv-section-card { margin-top: 0; }
        .cv-section-grid { display: grid; grid-template-columns: 1fr; gap: 8px; }
        .cv-check-row { display: flex; align-items: flex-start; gap: 10px; padding: 10px 12px; border: 1px solid var(--semi-color-border); border-radius: 6px; }
        .cv-check-row.pass { background: var(--semi-color-success-light-default); border-color: var(--semi-color-success-light-active); }
        .cv-check-row.fail.required { background: var(--semi-color-danger-light-default); border-color: var(--semi-color-danger-light-active); }
        .cv-check-row.fail:not(.required) { background: var(--semi-color-warning-light-default); border-color: var(--semi-color-warning-light-active); }
        .cv-check-row.not_checked { background: var(--semi-color-fill-0); }
        .cv-check-text { flex: 1; min-width: 0; }
        .cv-meta-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 8px; margin: 12px 0; }
        .cv-meta-row { display: flex; gap: 8px; font-size: 12px; color: var(--semi-color-text-2); }
        .cv-meta-row .label { min-width: 90px; flex-shrink: 0; color: var(--semi-color-text-1); }
        .cv-step-card { border: 1px solid var(--semi-color-border); border-radius: 8px; margin-bottom: 8px; }
        .cv-raw-pre { white-space: pre-wrap; word-break: break-word; max-height: 260px; overflow-y: auto; background: var(--semi-color-fill-0); padding: 8px; border-radius: 6px; font-size: 11px; font-family: ui-monospace, SFMono-Regular, monospace; }
      `}</style>
      </div>
    </div>
  );
};

// --- HistoryPanel ---

const HistoryPanel = ({
  records,
  loading,
  selectedId,
  admin,
  total,
  onSelect,
  onDelete,
  onRefresh,
}) => {
  const { t } = useTranslation();
  return (
    <Card className='cv-history-card' bordered bodyStyle={{ padding: 12 }}>
      <div className='cv-history-header'>
        <span className='cv-history-title'>
          <History size={16} />
          <span>
            {t('检测历史')}
            {total > 0 ? (
              <Text type='secondary' size='small' style={{ marginLeft: 6 }}>
                ({total})
              </Text>
            ) : null}
          </span>
        </span>
        <div className='cv-history-actions'>
          <Button
            theme='borderless'
            type='tertiary'
            size='small'
            icon={<RefreshCw size={14} />}
            onClick={onRefresh}
            loading={loading}
            aria-label={t('刷新')}
          />
        </div>
      </div>
      {loading && records.length === 0 ? (
        <div className='cv-history-empty'>
          <Spin size='middle' />
        </div>
      ) : records.length === 0 ? (
        <div className='cv-history-empty'>{t('暂无历史记录')}</div>
      ) : (
        <div className='cv-history-list'>
          {records.map((r) => {
            const meta = VERDICT_META[r.verdict] || VERDICT_META.unknown;
            const Icon = meta.icon;
            const active = selectedId === r.id;
            return (
              <div
                key={r.id}
                className={`cv-history-item${active ? ' active' : ''}`}
                onClick={() => onSelect(r.id)}
              >
                <Icon
                  size={18}
                  color={meta.color}
                  className='cv-history-icon'
                />
                <div className='cv-history-body'>
                  <div className='cv-history-line1'>
                    <span className='cv-history-model' title={r.model}>
                      {r.model || '—'}
                    </span>
                    <Tag size='small' color={meta.tagColor}>
                      {verdictLabel(r.verdict, t)}
                    </Tag>
                  </div>
                  {admin && r.channel_name ? (
                    <div
                      className='cv-history-channel'
                      title={r.channel_name}
                    >
                      {r.channel_name} (#{r.channel_id})
                    </div>
                  ) : null}
                  <div className='cv-history-line3'>
                    <span>{formatTimestamp(r.created_time)}</span>
                    <span>·</span>
                    <span>{r.duration_ms} ms</span>
                  </div>
                </div>
                <Popconfirm
                  title={t('删除该记录?')}
                  onConfirm={(e) => {
                    if (e?.stopPropagation) e.stopPropagation();
                    onDelete(r.id);
                  }}
                  onCancel={(e) => {
                    if (e?.stopPropagation) e.stopPropagation();
                  }}
                >
                  <Button
                    theme='borderless'
                    type='tertiary'
                    size='small'
                    className='cv-history-delete'
                    icon={<Trash2 size={14} />}
                    onClick={(e) => e.stopPropagation()}
                  />
                </Popconfirm>
              </div>
            );
          })}
        </div>
      )}
    </Card>
  );
};

// --- ResultPanel ---

// CHECK_GROUPS partitions the flat check array into the four
// dimensions used by the aio-coding-hub UI. Anything not matched by a
// prefix lands in the "其他" group.
const buildCheckGroups = (t) => [
  {
    key: 'protocol',
    title: t('协议与模型'),
    prefixes: ['baseline.'],
  },
  {
    key: 'thinking',
    title: t('Thinking 验证'),
    prefixes: ['thinking.', 'multi_turn.', 'tamper.', 'cross.'],
  },
  {
    key: 'features',
    title: t('功能支持'),
    prefixes: ['usage.', 'tool.', 'feature.'],
  },
];

const groupChecks = (checks, t) => {
  const groups = buildCheckGroups(t).map((g) => ({ ...g, items: [] }));
  const others = { key: 'other', title: t('输出表现'), items: [] };
  (checks || []).forEach((c) => {
    const target = groups.find((g) =>
      g.prefixes.some((p) => (c.key || '').startsWith(p)),
    );
    if (target) target.items.push(c);
    else others.items.push(c);
  });
  return [...groups, others].filter((g) => g.items.length > 0);
};

const ResultPanel = ({ result, admin, view }) => {
  const { t } = useTranslation();
  const verdict = VERDICT_META[result.verdict] || VERDICT_META.unknown;
  const VIcon = verdict.icon;
  const groups = useMemo(() => groupChecks(result.checks, t), [result.checks, t]);

  return (
    <>
      <div
        className='cv-verdict-banner'
        style={{ background: verdict.bg, borderColor: verdict.border }}
      >
        <VIcon size={32} className='cv-verdict-icon' color={verdict.color} />
        <div className='cv-verdict-body'>
          <Title heading={4} style={{ margin: 0, color: verdict.color }}>
            {verdictLabel(result.verdict, t)}
            {view === 'history' ? (
              <Tag size='small' color='blue' className='cv-view-tag'>
                {t('历史记录')}
              </Tag>
            ) : null}
          </Title>
          <Paragraph style={{ margin: '4px 0 0 0' }}>
            {result.summary || t('—')}
          </Paragraph>
        </div>
      </div>

      <Card className='cv-meta-card' bordered>
        <div className='cv-meta-grid'>
          {admin ? (
            <>
              <MetaRow
                label={t('渠道')}
                value={
                  result.channel_name
                    ? `${result.channel_name} (#${result.channel_id})`
                    : '—'
                }
              />
              <MetaRow label={t('Base URL')} value={result.base_url || '—'} mono />
            </>
          ) : null}
          <MetaRow label={t('请求模型')} value={result.requested_model} mono />
          <MetaRow
            label={t('响应模型')}
            value={result.responded_model || '—'}
            mono
          />
          <MetaRow label={t('总耗时')} value={`${result.duration_ms} ms`} />
          {result.started_at ? (
            <MetaRow
              label={t('开始时间')}
              value={formatTimestamp(result.started_at)}
            />
          ) : null}
        </div>
      </Card>

      {groups.map((group) => (
        <Card
          key={group.key}
          title={group.title}
          bordered
          className='cv-section-card'
        >
          <div className='cv-section-grid'>
            {group.items.map((c) => (
              <CheckRow key={c.key} check={c} t={t} />
            ))}
          </div>
        </Card>
      ))}

      <Card title={t('每步详情')} bordered>
        <Collapse accordion>
          {(result.steps || []).map((step, idx) => (
            <Collapse.Panel
              header={
                <div
                  style={{ display: 'flex', alignItems: 'center', gap: 8 }}
                >
                  <Tag color={step.ok ? 'green' : 'red'} size='small'>
                    {step.ok ? t('通过') : t('失败')}
                  </Tag>
                  <Text strong>{stepLabel(step.kind, t)}</Text>
                  <Text type='secondary' size='small'>
                    HTTP {step.status || '—'} · {step.duration_ms} ms
                  </Text>
                </div>
              }
              itemKey={`step-${idx}`}
              key={step.kind + idx}
            >
              <StepDetail step={step} t={t} />
            </Collapse.Panel>
          ))}
        </Collapse>
      </Card>
    </>
  );
};

const MetaRow = ({ label, value, mono }) => (
  <div className='cv-meta-row'>
    <span className='label'>{label}</span>
    <span
      style={{
        fontFamily: mono ? 'ui-monospace, monospace' : undefined,
        wordBreak: 'break-all',
      }}
    >
      {value}
    </span>
  </div>
);

const CheckRow = ({ check, t }) => {
  const passed = check.pass;
  const notChecked = check.not_checked;
  let cls = 'pass';
  let Icon = CheckCircle;
  let color = 'var(--semi-color-success)';
  if (notChecked) {
    cls = 'not_checked';
    Icon = CircleSlash;
    color = 'var(--semi-color-text-2)';
  } else if (!passed) {
    cls = check.required ? 'fail required' : 'fail';
    Icon = XCircle;
    color = check.required
      ? 'var(--semi-color-danger)'
      : 'var(--semi-color-warning)';
  }
  return (
    <div className={`cv-check-row ${cls}`}>
      <Icon size={16} style={{ marginTop: 2, color }} />
      <div className='cv-check-text'>
        <div>
          <Text strong>{check.label}</Text>
          {check.required ? (
            <Tag size='small' color='blue' style={{ marginLeft: 8 }}>
              {t('必须')}
            </Tag>
          ) : null}
          {notChecked ? (
            <Tag size='small' color='grey' style={{ marginLeft: 8 }}>
              {t('未检查')}
            </Tag>
          ) : null}
        </div>
        {check.detail ? (
          <Text type='tertiary' size='small'>
            {check.detail}
          </Text>
        ) : null}
      </div>
    </div>
  );
};

const StepDetail = ({ step, t }) => {
  return (
    <div>
      <div className='cv-meta-grid'>
        <MetaRow
          label={t('Content-Type')}
          value={step.content_type || '—'}
          mono
        />
        <MetaRow label={t('SSE')} value={step.is_sse ? t('是') : t('否')} />
        <MetaRow
          label={t('thinking 字符')}
          value={String(step.thinking_chars || 0)}
        />
        <MetaRow
          label={t('signature 字符')}
          value={String(step.signature_chars || 0)}
        />
        <MetaRow label={t('stop_reason')} value={step.stop_reason || '—'} mono />
        <MetaRow label={t('Response ID')} value={step.response_id || '—'} mono />
        <MetaRow
          label={t('Service Tier')}
          value={step.service_tier || '—'}
          mono
        />
        <MetaRow
          label={t('cache_creation')}
          value={step.has_cache_creation_detail ? t('已暴露') : t('未暴露')}
        />
      </div>

      {step.error ? (
        <Banner
          type='danger'
          description={step.error}
          fullMode={false}
          style={{ marginBottom: 12 }}
        />
      ) : null}

      {step.sse_error_event_seen ? (
        <Banner
          type='warning'
          description={`SSE error: ${step.sse_error_event_message || '(no message)'}`}
          fullMode={false}
          style={{ marginBottom: 12 }}
        />
      ) : null}

      {step.output_text_preview ? (
        <>
          <Text strong size='small'>
            {t('输出文本(预览)')}:
          </Text>
          <pre className='cv-raw-pre'>{step.output_text_preview}</pre>
        </>
      ) : null}

      {step.thinking_full ? (
        <>
          <Text strong size='small'>
            {t('thinking 内容')}:
          </Text>
          <pre className='cv-raw-pre'>{step.thinking_full}</pre>
        </>
      ) : null}

      {step.signature_full ? (
        <>
          <Text strong size='small'>
            {t('signature')}:
          </Text>
          <pre className='cv-raw-pre'>{step.signature_full}</pre>
        </>
      ) : null}

      {step.raw_excerpt ? (
        <>
          <Text strong size='small'>
            {t('原始响应(前 16KB)')}:
          </Text>
          <pre className='cv-raw-pre'>{step.raw_excerpt}</pre>
        </>
      ) : null}
    </div>
  );
};

export default ChannelValidationPage;
