import React, { useState, useCallback, useMemo } from 'react';
import {
  Button,
  Collapsible,
  Input,
  InputNumber,
  Select,
  Tag,
  Typography,
  Popconfirm,
} from '@douyinfe/semi-ui';
import {
  IconPlus,
  IconDelete,
  IconChevronDown,
  IconChevronUp,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

let _idCounter = 0;
const uid = () => `gmp_${++_idCounter}`;

function parseJSON(str) {
  if (!str || !str.trim()) return {};
  try {
    return JSON.parse(str);
  } catch {
    return {};
  }
}

function flattenRules(nested) {
  const rules = [];
  for (const [group, inner] of Object.entries(nested)) {
    if (typeof inner !== 'object' || inner === null) continue;
    for (const [modelName, price] of Object.entries(inner)) {
      rules.push({
        _id: uid(),
        group,
        modelName,
        price: typeof price === 'number' ? price : 0,
      });
    }
  }
  return rules;
}

function nestRules(rules) {
  const result = {};
  rules.forEach(({ group, modelName, price }) => {
    if (!group || !modelName) return;
    if (!result[group]) result[group] = {};
    result[group][modelName] = price;
  });
  return result;
}

export function serializeGroupModelPrice(rules) {
  const nested = nestRules(rules);
  return Object.keys(nested).length === 0
    ? '{}'
    : JSON.stringify(nested, null, 2);
}

function GroupSection({ groupName, items, onUpdate, onRemove, onAdd, t }) {
  const [open, setOpen] = useState(false);

  return (
    <div
      style={{
        border: '1px solid var(--semi-color-border)',
        borderRadius: 8,
        overflow: 'hidden',
      }}
    >
      <div
        className='flex items-center justify-between cursor-pointer'
        style={{
          padding: '8px 12px',
          background: 'var(--semi-color-fill-0)',
        }}
        onClick={() => setOpen(!open)}
      >
        <div className='flex items-center gap-2'>
          {open ? <IconChevronUp size='small' /> : <IconChevronDown size='small' />}
          <Text strong>{groupName}</Text>
          <Tag size='small' color='blue'>{items.length} {t('条规则')}</Tag>
        </div>
        <div className='flex items-center gap-1' onClick={(e) => e.stopPropagation()}>
          <Button
            icon={<IconPlus />}
            size='small'
            theme='borderless'
            onClick={() => onAdd(groupName)}
          />
          <Popconfirm
            title={t('确认删除该分组的所有规则？')}
            onConfirm={() => items.forEach((item) => onRemove(item._id))}
            position='left'
          >
            <Button
              icon={<IconDelete />}
              size='small'
              type='danger'
              theme='borderless'
            />
          </Popconfirm>
        </div>
      </div>
      <Collapsible isOpen={open} keepDOM>
        <div style={{ padding: '8px 12px' }}>
          {items.map((rule) => (
            <div
              key={rule._id}
              className='flex items-center gap-2'
              style={{ marginBottom: 6 }}
            >
              <Input
                size='small'
                value={rule.modelName}
                placeholder={t('模型名称')}
                onChange={(v) => onUpdate(rule._id, 'modelName', v)}
                style={{ flex: 1 }}
              />
              <InputNumber
                size='small'
                min={0}
                step={0.01}
                value={rule.price}
                style={{ width: 120 }}
                onChange={(v) => onUpdate(rule._id, 'price', v ?? 0)}
              />
              <Popconfirm
                title={t('确认删除该规则？')}
                onConfirm={() => onRemove(rule._id)}
                position='left'
              >
                <Button
                  icon={<IconDelete />}
                  type='danger'
                  theme='borderless'
                  size='small'
                />
              </Popconfirm>
            </div>
          ))}
        </div>
      </Collapsible>
    </div>
  );
}

export default function GroupModelPriceRules({
  value,
  groupNames = [],
  onChange,
}) {
  const { t } = useTranslation();
  const [rules, setRules] = useState(() => flattenRules(parseJSON(value)));
  const [newGroupName, setNewGroupName] = useState('');

  const emitChange = useCallback(
    (newRules) => {
      setRules(newRules);
      onChange?.(serializeGroupModelPrice(newRules));
    },
    [onChange],
  );

  const updateRule = useCallback(
    (id, field, val) => {
      emitChange(rules.map((r) => (r._id === id ? { ...r, [field]: val } : r)));
    },
    [rules, emitChange],
  );

  const removeRule = useCallback(
    (id) => {
      emitChange(rules.filter((r) => r._id !== id));
    },
    [rules, emitChange],
  );

  const addRuleToGroup = useCallback(
    (groupName) => {
      emitChange([
        ...rules,
        { _id: uid(), group: groupName, modelName: '', price: 0 },
      ]);
    },
    [rules, emitChange],
  );

  const addNewGroup = useCallback(() => {
    const name = newGroupName.trim();
    if (!name) return;
    emitChange([
      ...rules,
      { _id: uid(), group: name, modelName: '', price: 0 },
    ]);
    setNewGroupName('');
  }, [rules, emitChange, newGroupName]);

  const groupOptions = useMemo(
    () => groupNames.map((n) => ({ value: n, label: n })),
    [groupNames],
  );

  const grouped = useMemo(() => {
    const map = {};
    const order = [];
    rules.forEach((r) => {
      if (!r.group) return;
      if (!map[r.group]) {
        map[r.group] = [];
        order.push(r.group);
      }
      map[r.group].push(r);
    });
    return order.map((name) => ({ name, items: map[name] }));
  }, [rules]);

  if (grouped.length === 0 && rules.length === 0) {
    return (
      <div>
        <Text type='tertiary' className='block text-center py-4'>
          {t('暂无规则，点击下方按钮添加')}
        </Text>
        <div className='mt-2 flex justify-center gap-2'>
          <Select
            size='small'
            filter
            allowCreate
            placeholder={t('选择分组')}
            optionList={groupOptions}
            value={newGroupName || undefined}
            onChange={setNewGroupName}
            style={{ width: 200 }}
            position='bottomLeft'
          />
          <Button icon={<IconPlus />} theme='outline' onClick={addNewGroup}>
            {t('添加分组规则')}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className='space-y-2'>
      {grouped.map((group) => (
        <GroupSection
          key={group.name}
          groupName={group.name}
          items={group.items}
          onUpdate={updateRule}
          onRemove={removeRule}
          onAdd={addRuleToGroup}
          t={t}
        />
      ))}
      <div className='mt-3 flex justify-center gap-2'>
        <Select
          size='small'
          filter
          allowCreate
          placeholder={t('选择分组')}
          optionList={groupOptions}
          value={newGroupName || undefined}
          onChange={setNewGroupName}
          style={{ width: 200 }}
          position='bottomLeft'
        />
        <Button icon={<IconPlus />} theme='outline' onClick={addNewGroup}>
          {t('添加分组规则')}
        </Button>
      </div>
    </div>
  );
}
