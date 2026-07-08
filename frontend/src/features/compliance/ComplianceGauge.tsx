// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { motion } from 'framer-motion';
import { ProgressBar } from '../../components/shared/ProgressBar';
import { useI18n } from '../../hooks/useI18n';
import type { ComplianceProgress } from '../../types/compliance';

interface ComplianceGaugeProps {
  progress: ComplianceProgress;
}

export const ComplianceGauge = ({ progress }: ComplianceGaugeProps) => {
  const { t } = useI18n();
  const percent = Math.round(progress.percent_complete);
  const variant = percent >= 67 ? 'success' : percent >= 34 ? 'warning' : 'danger';

  return (
    <motion.div
      initial={{ opacity: 0, y: -8 }}
      animate={{ opacity: 1, y: 0 }}
      className="rounded-2xl border border-zinc-800 bg-zinc-950/60 p-5"
    >
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm font-medium text-zinc-300">
          {t('compliance.progress').replace('{percent}', String(percent))}
        </span>
        <span className="text-xs text-zinc-500">
          {progress.total} {t('compliance.controls')}
        </span>
      </div>
      <ProgressBar value={percent} max={100} variant={variant} showPercentage={false} />
    </motion.div>
  );
};
