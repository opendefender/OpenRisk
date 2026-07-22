// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { PrioritizedMitigationsList } from '../features/mitigations/PrioritizedMitigationsList';

export const Recommendations = () => {
  return (
    <div className="p-8 h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto">
        <div className="mb-8">
            <h1 className="text-3xl font-bold text-white mb-2">Intelligence & Recommendations</h1>
            <p className="text-zinc-400">
                Optimisez vos efforts de sécurité en traitant d'abord ce qui compte vraiment.
            </p>
        </div>
        
        
        <div className="bg-surface/50 border border-border rounded-xl p-1">
            <PrioritizedMitigationsList />
        </div>
      </div>
    </div>
  );
};