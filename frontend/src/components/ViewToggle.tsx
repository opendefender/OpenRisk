// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
