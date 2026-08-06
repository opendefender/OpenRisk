// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { CircularProgressbarWithChildren, buildStyles } from 'react-circular-progressbar';
import 'react-circular-progressbar/dist/styles.css';
import { ShieldCheck } from 'lucide-react';

export const GlobalScore = ({ score }: { score: number }) => {
  return (
    <div className="h-full flex flex-col items-center justify-center p-4">
      <div className="w-32 h-32 relative">
        <CircularProgressbarWithChildren
          value={score}
          styles={buildStyles({
            pathColor: score > 80 ? '#10b981' : score > 50 ? '#f59e0b' : '#ef4444',
            trailColor: 'rgba(255,255,255,0.1)',
            pathTransitionDuration: 1.5,
          })}
        >
            <div className="flex flex-col items-center animate-fade-in">
                <ShieldCheck size={28} className="text-text-secondary mb-1" />
                <span className="text-3xl font-bold text-text-primary">{score}</span>
                <span className="text-[10px] uppercase text-text-muted tracking-widest">Sec. Score</span>
            </div>
        </CircularProgressbarWithChildren>
      </div>
      <p className="mt-4 text-center text-sm text-text-secondary">
        Votre posture de sécurité est <span className="text-success-text font-medium">optimale</span>.
      </p>
    </div>
  );
};