// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useMemo, useState } from 'react';
import { Filter, X } from 'lucide-react';
import { useAssetSchemas } from './useAssetSchemas';
import { ASSET_CATEGORIES, CATEGORY_LABELS, type AssetCategory } from './schemaTypes';
import { useI18n } from '../../hooks/useI18n';

/**
 * Search the inventory by typed attribute.
 *
 * The attribute list offered here comes from the selected category's schema, so
 * it always matches what can actually be stored — and the terms are sent to the
 * server (`?attr.<key>=<value>`), which owns the matching rules. This bar builds
 * a query; it never evaluates one.
 */
interface AttributeSearchBarProps {
  category: AssetCategory | '';
  attributes: Record<string, string>;
  onChange: (next: { category: AssetCategory | ''; attributes: Record<string, string> }) => void;
  resultCount?: number;
}

export function AttributeSearchBar({
  category,
  attributes,
  onChange,
  resultCount,
}: AttributeSearchBarProps) {
  const { t } = useI18n();
  const { defsFor } = useAssetSchemas();
  const defs = defsFor(category);
  const [pendingKey, setPendingKey] = useState('');
  const [pendingValue, setPendingValue] = useState('');

  const activeTerms = useMemo(() => Object.entries(attributes), [attributes]);
  const selectedDef = defs.find((d) => d.key === pendingKey);

  const addTerm = () => {
    if (!pendingKey || !pendingValue.trim()) return;
    onChange({ category, attributes: { ...attributes, [pendingKey]: pendingValue.trim() } });
    setPendingKey('');
    setPendingValue('');
  };

  const removeTerm = (key: string) => {
    const next = { ...attributes };
    delete next[key];
    onChange({ category, attributes: next });
  };

  const inputCls = 'rounded-lg border px-2.5 py-1.5 text-[13px] outline-none';
  const inputSty = {
    background: 'var(--surface-2)',
    borderColor: 'var(--border)',
    color: 'var(--fg-primary)',
  } as const;

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <span
          className="inline-flex items-center gap-1.5 text-[12px] font-medium"
          style={{ color: 'var(--fg-muted)' }}
        >
          <Filter size={13} aria-hidden="true" /> {t('assets.attributeSearch.heading')}
        </span>

        <select
          aria-label={t('assets.attributeSearch.categoryLabel')}
          className={inputCls}
          style={inputSty}
          value={category}
          onChange={(e) =>
            // Attribute keys belong to a category's schema; keeping the old
            // terms after switching would filter on keys the new category does
            // not declare, and silently return nothing.
            onChange({ category: e.target.value as AssetCategory | '', attributes: {} })
          }
        >
          <option value="">{t('assets.attributeSearch.categoryPlaceholder')}</option>
          {ASSET_CATEGORIES.map((c) => (
            <option key={c} value={c}>
              {CATEGORY_LABELS[c]}
            </option>
          ))}
        </select>

        {category ? (
          <>
            <select
              aria-label={t('assets.attributeSearch.attributeLabel')}
              className={inputCls}
              style={inputSty}
              value={pendingKey}
              onChange={(e) => {
                setPendingKey(e.target.value);
                setPendingValue('');
              }}
            >
              <option value="">{t('assets.attributeSearch.attributePlaceholder')}</option>
              {defs.map((d) => (
                <option key={d.key} value={d.key}>
                  {d.label}
                </option>
              ))}
            </select>

            {selectedDef?.type === 'enum' || selectedDef?.type === 'multi_enum' ? (
              <select
                aria-label={t('assets.attributeSearch.valueLabel')}
                className={inputCls}
                style={inputSty}
                value={pendingValue}
                onChange={(e) => setPendingValue(e.target.value)}
              >
                <option value="">{t('assets.attributeSearch.valueOption')}</option>
                {(selectedDef.enum ?? []).map((v) => (
                  <option key={v} value={v}>
                    {v}
                  </option>
                ))}
              </select>
            ) : selectedDef?.type === 'boolean' ? (
              <select
                aria-label={t('assets.attributeSearch.valueLabel')}
                className={inputCls}
                style={inputSty}
                value={pendingValue}
                onChange={(e) => setPendingValue(e.target.value)}
              >
                <option value="">{t('assets.attributeSearch.valueOption')}</option>
                <option value="true">{t('assets.attributeSearch.yes')}</option>
                <option value="false">{t('assets.attributeSearch.no')}</option>
              </select>
            ) : (
              <input
                aria-label={t('assets.attributeSearch.valueLabel')}
                className={inputCls}
                style={inputSty}
                placeholder={t('assets.attributeSearch.valuePlaceholder')}
                value={pendingValue}
                disabled={!pendingKey}
                onChange={(e) => setPendingValue(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    addTerm();
                  }
                }}
              />
            )}

            <button
              type="button"
              onClick={addTerm}
              disabled={!pendingKey || !pendingValue.trim()}
              className="rounded-lg px-3 py-1.5 text-[13px] font-medium disabled:opacity-40"
              style={{ background: 'var(--accent-soft)', color: 'var(--accent-500)' }}
            >
              {t('assets.attributeSearch.apply')}
            </button>
          </>
        ) : (
          <span className="text-[12px]" style={{ color: 'var(--fg-muted)' }}>
            {t('assets.attributeSearch.chooseCategory')}
          </span>
        )}

        {typeof resultCount === 'number' && (category || activeTerms.length) ? (
          <span className="ml-auto text-[12px]" style={{ color: 'var(--fg-muted)' }}>
            {t('common.assets', { count: resultCount })}
          </span>
        ) : null}
      </div>

      {activeTerms.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {activeTerms.map(([key, value]) => {
            const def = defs.find((d) => d.key === key);
            return (
              <span
                key={key}
                className="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[12px]"
                style={{
                  borderColor: 'var(--accent)',
                  background: 'var(--accent-soft)',
                  color: 'var(--accent-500)',
                }}
              >
                {def?.label ?? key} = {value}
                <button
                  type="button"
                  onClick={() => removeTerm(key)}
                  aria-label={t('assets.attributeSearch.removeTerm', {
                    term: `${def?.label ?? key} = ${value}`,
                  })}
                >
                  <X size={12} aria-hidden="true" />
                </button>
              </span>
            );
          })}
        </div>
      )}
    </div>
  );
}
