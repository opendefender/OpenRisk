// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { motion } from 'framer-motion';
import { useI18n } from '../../hooks/useI18n';
import { CONTROL_STATUSES, type ComplianceControl, type ControlStatus } from '../../types/compliance';

const STATUS_DOT: Record<ControlStatus, string> = {
  not_implemented: 'bg-surface-3',
  in_progress: 'bg-warning',
  implemented: 'bg-success',
  not_applicable: 'bg-surface-3',
};

interface ControlTableProps {
  controls: ComplianceControl[];
  onOpenControl: (controlId: string) => void;
  onStatusChange: (controlId: string, status: ControlStatus) => void;
}

export const ControlTable = ({ controls, onOpenControl, onStatusChange }: ControlTableProps) => {
  const { t } = useI18n();

  return (
    <div className="overflow-x-auto scrollbar-thin rounded-2xl border border-border-subtle">
      <table className="w-full min-w-[560px] text-sm">
        <thead className="bg-surface-1/50 text-left text-xs uppercase tracking-wider text-text-muted">
          <tr>
            <th className="px-4 py-3 font-medium">{t('compliance.referenceCode')}</th>
            <th className="px-4 py-3 font-medium">{t('common.name')}</th>
            <th className="px-4 py-3 font-medium">{t('common.status')}</th>
            <th className="px-4 py-3 font-medium" />
          </tr>
        </thead>
        <tbody className="divide-y divide-border-subtle/70">
          {controls.map((control, index) => (
            <motion.tr
              key={control.id}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: Math.min(index * 0.02, 0.3) }}
              className="cursor-pointer transition-colors hover:bg-surface-1/5"
              onClick={() => control.id && onOpenControl(control.id)}
            >
              <td className="px-4 py-3 font-mono text-xs text-text-secondary">{control.reference_code || '—'}</td>
              <td className="px-4 py-3 text-text-primary">{control.name}</td>
              <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                <div className="flex items-center gap-2">
                  <span className={`h-2 w-2 rounded-full ${STATUS_DOT[control.status ?? 'not_implemented']}`} />
                  <select
                    value={control.status ?? 'not_implemented'}
                    onChange={(e) => control.id && onStatusChange(control.id, e.target.value as ControlStatus)}
                    className="rounded-lg border border-border-subtle bg-surface-0 px-2 py-1 text-xs text-text-primary outline-none focus:ring-2 focus:ring-primary/40"
                  >
                    {CONTROL_STATUSES.map((status) => (
                      <option key={status} value={status}>
                        {t(`compliance.status.${status}`)}
                      </option>
                    ))}
                  </select>
                </div>
              </td>
              <td className="px-4 py-3 text-right text-xs text-text-muted">→</td>
            </motion.tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};
