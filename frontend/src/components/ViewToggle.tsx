// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { LayoutGrid, List } from 'lucide-react';
import { cn } from './ui/Button';

interface ViewToggleProps {
  view: 'table' | 'card';
  onViewChange: (view: 'table' | 'card') => void;
}

export const ViewToggle = ({ view, onViewChange }: ViewToggleProps) => {
  return (
    <div className="flex gap-2 bg-surface/50 rounded-lg p-1 border border-border">
      <button
        onClick={() => onViewChange('table')}
        className={cn(
          'flex items-center gap-2 px-3 py-1.5 rounded-md text-sm font-medium transition-all',
          view === 'table'
            ? 'bg-primary/20 text-primary border border-primary/30'
            : 'text-zinc-400 hover:text-zinc-200'
        )}
      >
        <List size={16} />
        <span className="hidden sm:inline">Table</span>
      </button>
      <button
        onClick={() => onViewChange('card')}
        className={cn(
          'flex items-center gap-2 px-3 py-1.5 rounded-md text-sm font-medium transition-all',
          view === 'card'
            ? 'bg-primary/20 text-primary border border-primary/30'
            : 'text-zinc-400 hover:text-zinc-200'
        )}
      >
        <LayoutGrid size={16} />
        <span className="hidden sm:inline">Cards</span>
      </button>
    </div>
  );
};
