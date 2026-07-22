// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { motion } from 'framer-motion';
import { useI18n } from '../../hooks/useI18n';
import { CONTROL_STATUSES, type ComplianceControl, type ControlStatus } from '../../types/compliance';

const STATUS_DOT: Record<ControlStatus, string> = {
  not_implemented: 'bg-zinc-500',
  in_progress: 'bg-yellow-500',
  implemented: 'bg-emerald-500',
  not_applicable: 'bg-zinc-700',
};

interface ControlTableProps {
  controls: ComplianceControl[];
  onOpenControl: (controlId: string) => void;
  onStatusChange: (controlId: string, status: ControlStatus) => void;
}

export const ControlTable = ({ controls, onOpenControl, onStatusChange }: ControlTableProps) => {
  const { t } = useI18n();

  return (
    <div className="overflow-x-auto scrollbar-thin rounded-2xl border border-zinc-800">
      <table className="w-full min-w-[560px] text-sm">
        <thead className="bg-zinc-900/50 text-left text-xs uppercase tracking-wider text-zinc-500">
          <tr>
            <th className="px-4 py-3 font-medium">{t('compliance.referenceCode')}</th>
            <th className="px-4 py-3 font-medium">{t('common.name')}</th>
            <th className="px-4 py-3 font-medium">{t('common.status')}</th>
            <th className="px-4 py-3 font-medium" />
          </tr>
        </thead>
        <tbody className="divide-y divide-zinc-800/70">
          {controls.map((control, index) => (
            <motion.tr
              key={control.id}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: Math.min(index * 0.02, 0.3) }}
              className="cursor-pointer transition-colors hover:bg-white/5"
              onClick={() => control.id && onOpenControl(control.id)}
            >
              <td className="px-4 py-3 font-mono text-xs text-zinc-400">{control.reference_code || '—'}</td>
              <td className="px-4 py-3 text-zinc-100">{control.name}</td>
              <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                <div className="flex items-center gap-2">
                  <span className={`h-2 w-2 rounded-full ${STATUS_DOT[control.status ?? 'not_implemented']}`} />
                  <select
                    value={control.status ?? 'not_implemented'}
                    onChange={(e) => control.id && onStatusChange(control.id, e.target.value as ControlStatus)}
                    className="rounded-lg border border-zinc-800 bg-zinc-950 px-2 py-1 text-xs text-zinc-100 outline-none focus:ring-2 focus:ring-primary/40"
                  >
                    {CONTROL_STATUSES.map((status) => (
                      <option key={status} value={status}>
                        {t(`compliance.status.${status}`)}
                      </option>
                    ))}
                  </select>
                </div>
              </td>
              <td className="px-4 py-3 text-right text-xs text-zinc-500">→</td>
            </motion.tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};
