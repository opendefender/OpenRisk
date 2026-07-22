// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { memo } from 'react';
import { motion } from 'framer-motion';
import { LayoutGrid, Table2, Calendar } from 'lucide-react';
import { useMitigationStore } from './store';
import { cn } from '../../utils/cn';

type ViewMode = 'kanban' | 'table' | 'gantt';

const VIEWS: Array<{ id: ViewMode; label: string; icon: React.ReactNode }> = [
  { id: 'kanban', label: 'Kanban', icon: <LayoutGrid size={18} /> },
  { id: 'table', label: 'Tableau', icon: <Table2 size={18} /> },
  { id: 'gantt', label: 'Gantt', icon: <Calendar size={18} /> },
];

export const ViewSwitcher = memo(function ViewSwitcher() {
  const { viewMode, setViewMode } = useMitigationStore();

  return (
    <div className="flex items-center gap-2 bg-zinc-800/30 p-1 rounded-lg border border-zinc-700">
      {VIEWS.map((view) => (
        <motion.button
          key={view.id}
          onClick={() => setViewMode(view.id)}
          className={cn(
            'px-3 py-2 rounded flex items-center gap-2 transition-colors text-sm font-medium',
            viewMode === view.id
              ? 'bg-blue-600 text-white'
              : 'text-zinc-400 hover:text-white'
          )}
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.98 }}
        >
          {view.icon}
          <span className="hidden sm:inline">{view.label}</span>
        </motion.button>
      ))}
    </div>
  );
});
