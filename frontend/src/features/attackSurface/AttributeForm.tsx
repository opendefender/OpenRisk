// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useMemo } from 'react';
import { Fingerprint } from 'lucide-react';
import type { AttributeBag, AttributeDef, AttributeValue } from './schemaTypes';
import { isListType } from './schemaTypes';

/**
 * AttributeForm GENERATES the asset form from a schema. It renders exactly the
 * attributes the server declared, in the order and grouping the schema gives —
 * there is no hand-written field list anywhere, which is what keeps a
 * tenant's schema edit visible in the form without a deploy.
 *
 * It validates nothing beyond what a browser input does for free. Validation is
 * the server's job (domain.ValidateAttributes) and stays there: re-implementing
 * the rules here would create a second, drifting copy of the contract. What this
 * component does is make the valid shape easy to produce — right widget, right
 * options, required marks — and then surface the server's error verbatim.
 */
interface AttributeFormProps {
  defs: AttributeDef[];
  values: AttributeBag;
  onChange: (next: AttributeBag) => void;
  disabled?: boolean;
  /** Field-level errors keyed by attribute key, if the caller can map them. */
  errors?: Record<string, string>;
}

export function AttributeForm({ defs, values, onChange, disabled, errors }: AttributeFormProps) {
  // Group in declaration order: a schema author orders attributes deliberately,
  // and alphabetising the groups would throw that away.
  const groups = useMemo(() => {
    const out: { name: string; defs: AttributeDef[] }[] = [];
    for (const d of defs) {
      const name = d.group?.trim() || 'Général';
      const found = out.find((g) => g.name === name);
      if (found) found.defs.push(d);
      else out.push({ name, defs: [d] });
    }
    return out;
  }, [defs]);

  const set = (key: string, value: AttributeValue | undefined) => {
    const next = { ...values };
    if (value === undefined || value === '' || (Array.isArray(value) && value.length === 0)) {
      delete next[key];
    } else {
      next[key] = value;
    }
    onChange(next);
  };

  if (defs.length === 0) {
    return (
      <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
        Cette catégorie ne déclare aucun attribut.
      </p>
    );
  }

  return (
    <div className="space-y-5">
      {groups.map((group) => (
        <fieldset key={group.name} className="space-y-3">
          <legend
            className="text-xs font-semibold uppercase tracking-wide"
            style={{ color: 'var(--text-muted)' }}
          >
            {group.name}
          </legend>
          <div className="grid gap-3 sm:grid-cols-2">
            {group.defs.map((def) => (
              <AttributeField
                key={def.key}
                def={def}
                value={values[def.key]}
                onChange={(v) => set(def.key, v)}
                disabled={disabled}
                error={errors?.[def.key]}
              />
            ))}
          </div>
        </fieldset>
      ))}
    </div>
  );
}

interface FieldProps {
  def: AttributeDef;
  value: AttributeValue | undefined;
  onChange: (v: AttributeValue | undefined) => void;
  disabled?: boolean;
  error?: string;
}

const inputClass =
  'w-full rounded-lg border px-3 py-2 text-sm outline-none transition focus:ring-2';
const inputStyle = {
  background: 'var(--surface-2)',
  borderColor: 'var(--border)',
  color: 'var(--text-primary)',
} as const;

function AttributeField({ def, value, onChange, disabled, error }: FieldProps) {
  const wide = def.type === 'text' || isListType(def.type);
  const id = `attr-${def.key}`;

  return (
    <div className={wide ? 'sm:col-span-2' : undefined}>
      <label
        htmlFor={id}
        className="mb-1 flex items-center gap-1.5 text-xs font-medium"
        style={{ color: 'var(--text-secondary)' }}
      >
        {def.label}
        {def.required ? <span style={{ color: 'var(--critical)' }}>*</span> : null}
        {def.fingerprint ? (
          <span
            title="Sert d'empreinte pour corréler les vulnérabilités à cet actif"
            className="inline-flex items-center"
            style={{ color: 'var(--accent)' }}
          >
            <Fingerprint size={12} />
          </span>
        ) : null}
      </label>

      <FieldWidget id={id} def={def} value={value} onChange={onChange} disabled={disabled} />

      {def.help ? (
        <p className="mt-1 text-[11px]" style={{ color: 'var(--text-muted)' }}>
          {def.help}
        </p>
      ) : null}
      {error ? (
        <p className="mt-1 text-[11px]" style={{ color: 'var(--critical)' }}>
          {error}
        </p>
      ) : null}
    </div>
  );
}

function FieldWidget({
  id,
  def,
  value,
  onChange,
  disabled,
}: FieldProps & { id: string }) {
  switch (def.type) {
    case 'boolean':
      return (
        <label className="flex items-center gap-2 text-sm" style={{ color: 'var(--text-primary)' }}>
          <input
            id={id}
            type="checkbox"
            checked={value === true}
            disabled={disabled}
            onChange={(e) => onChange(e.target.checked ? true : undefined)}
          />
          <span style={{ color: 'var(--text-muted)' }}>{value === true ? 'Oui' : 'Non'}</span>
        </label>
      );

    case 'enum':
      return (
        <select
          id={id}
          className={inputClass}
          style={inputStyle}
          value={typeof value === 'string' ? value : ''}
          disabled={disabled}
          onChange={(e) => onChange(e.target.value || undefined)}
        >
          <option value="">—</option>
          {(def.enum ?? []).map((opt) => (
            <option key={opt} value={opt}>
              {opt}
            </option>
          ))}
        </select>
      );

    case 'multi_enum': {
      const selected = Array.isArray(value) ? value : [];
      return (
        <div className="flex flex-wrap gap-1.5">
          {(def.enum ?? []).map((opt) => {
            const on = selected.includes(opt);
            return (
              <button
                key={opt}
                type="button"
                disabled={disabled}
                onClick={() =>
                  onChange(on ? selected.filter((v) => v !== opt) : [...selected, opt])
                }
                className="rounded-full border px-2.5 py-1 text-xs transition"
                style={{
                  borderColor: on ? 'var(--accent)' : 'var(--border)',
                  background: on ? 'var(--accent-soft)' : 'var(--surface-2)',
                  color: on ? 'var(--accent)' : 'var(--text-secondary)',
                }}
              >
                {opt}
              </button>
            );
          })}
        </div>
      );
    }

    case 'text':
      return (
        <textarea
          id={id}
          rows={3}
          className={inputClass}
          style={inputStyle}
          value={typeof value === 'string' ? value : ''}
          disabled={disabled}
          onChange={(e) => onChange(e.target.value || undefined)}
        />
      );

    case 'ip_list':
    case 'string_list': {
      // A comma-separated input, which the server also accepts in that form.
      const text = Array.isArray(value) ? value.join(', ') : '';
      return (
        <input
          id={id}
          className={inputClass}
          style={inputStyle}
          value={text}
          disabled={disabled}
          placeholder={def.type === 'ip_list' ? '10.0.0.1, 10.0.0.2' : 'valeur1, valeur2'}
          onChange={(e) => {
            const parts = e.target.value
              .split(',')
              .map((p) => p.trim())
              .filter(Boolean);
            onChange(parts.length ? parts : undefined);
          }}
        />
      );
    }

    default: {
      const htmlType =
        def.type === 'number' || def.type === 'integer'
          ? 'number'
          : def.type === 'date'
            ? 'date'
            : def.type === 'email'
              ? 'email'
              : def.type === 'url'
                ? 'url'
                : 'text';
      return (
        <input
          id={id}
          type={htmlType}
          step={def.type === 'integer' ? 1 : undefined}
          min={def.min ?? undefined}
          max={def.max ?? undefined}
          className={inputClass}
          style={inputStyle}
          value={value === undefined || value === null ? '' : String(value)}
          disabled={disabled}
          onChange={(e) => {
            const raw = e.target.value;
            if (raw === '') return onChange(undefined);
            if (htmlType === 'number') {
              const n = Number(raw);
              return onChange(Number.isNaN(n) ? raw : n);
            }
            onChange(raw);
          }}
        />
      );
    }
  }
}
