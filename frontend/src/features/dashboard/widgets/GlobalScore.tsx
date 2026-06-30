// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
                <ShieldCheck size={28} className="text-zinc-400 mb-1" />
                <span className="text-3xl font-bold text-white">{score}</span>
                <span className="text-[10px] uppercase text-zinc-500 tracking-widest">Sec. Score</span>
            </div>
        </CircularProgressbarWithChildren>
      </div>
      <p className="mt-4 text-center text-sm text-zinc-400">
        Votre posture de sécurité est <span className="text-emerald-400 font-medium">optimale</span>.
      </p>
    </div>
  );
};