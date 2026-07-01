// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { ResponsiveContainer, ScatterChart, Scatter, XAxis, YAxis, ZAxis, Tooltip, Cell } from 'recharts';

const data = [
  { x: 1, y: 1, z: 5, name: 'Server Patching', score: 1 }, // Low/Low
  { x: 5, y: 5, z: 20, name: 'Database Injection', score: 25 }, // High/High
  { x: 3, y: 4, z: 10, name: 'Phishing', score: 12 },
  { x: 5, y: 2, z: 8, name: 'DDOS', score: 10 },
  { x: 2, y: 5, z: 15, name: 'Insider Threat', score: 10 },
];

const COLORS = {
  Low: '#3b82f6',
  Medium: '#f59e0b',
  High: '#f97316',
  Critical: '#ef4444'
};

export const RiskHeatmap = () => {
  return (
    <div className="h-full w-full p-2">
      <h3 className="text-sm font-medium text-zinc-400 mb-2 uppercase tracking-wider">Risk Heatmap</h3>
      <ResponsiveContainer width="100%" height="90%">
        <ScatterChart margin={{ top: 20, right: 20, bottom: 20, left: 0 }}>
          <XAxis type="number" dataKey="x" name="Impact" unit="" domain={[0, 6]} tickCount={6} stroke="#52525b" />
          <YAxis type="number" dataKey="y" name="Probability" unit="" domain={[0, 6]} tickCount={6} stroke="#52525b" />
          <ZAxis type="number" dataKey="z" range={[100, 400]} />
          <Tooltip 
            cursor={{ strokeDasharray: '3 3' }} 
            contentStyle={{ backgroundColor: '#18181b', borderColor: '#27272a', color: '#fff' }}
          />
          <Scatter name="Risks" data={data}>
            {data.map((entry, index) => {
               const color = entry.score >= 20 ? COLORS.Critical : entry.score >= 10 ? COLORS.Medium : COLORS.Low;
               return <Cell key={`cell-${index}`} fill={color} stroke="none" />;
            })}
          </Scatter>
        </ScatterChart>
      </ResponsiveContainer>
    </div>
  );
};