// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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