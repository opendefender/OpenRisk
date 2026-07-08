// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useEffect, useState, useMemo } from 'react';
import { motion } from 'framer-motion';
import { api } from '../../../lib/api';

interface MatrixCellData {
    impact: number;
    probability: number;
    count: number;
}

// The API returns raw impact (0.0–10.0) and probability (0.0–1.0) — CLAUDE.md's Score
// Engine domain — not pre-bucketed 1-5 integers. Bucket them onto the 5x5 grid here
// instead of comparing raw values directly (which would almost never match, since
// probability is continuous and impact/probability=0 previously bucketed to nothing).
const bucketImpact = (impact: number) => Math.min(5, Math.max(1, Math.ceil(impact / 2) || 1));
const bucketProbability = (probability: number) => Math.min(5, Math.max(1, Math.ceil(probability / 0.2) || 1));

// Fonction pour déterminer la couleur du risque (buckets 1-5 sur chaque axe)
const getCellColor = (impactBucket: number, probabilityBucket: number, count: number) => {
    const score = impactBucket * probabilityBucket;
    if (count === 0) return 'bg-zinc-900 border-zinc-700 hover:bg-zinc-800';

    if (score >= 15) return 'bg-red-700/50 border-red-600 ring-red-500/30';
    if (score >= 10) return 'bg-orange-700/50 border-orange-600 ring-orange-500/30';
    if (score >= 5) return 'bg-yellow-700/50 border-yellow-600 ring-yellow-500/30';
    return 'bg-blue-700/50 border-blue-600 ring-blue-500/30';
};

export const RiskMatrix = () => {
    const [data, setData] = useState<MatrixCellData[]>([]);
    const [isLoading, setIsLoading] = useState(true);

    useEffect(() => {
        api.get('/stats/risk-matrix')
            .then(res => setData(res.data))
            .catch(err => console.error("Failed to fetch matrix data:", err))
            .finally(() => setIsLoading(false));
    }, []);

    // Transformation des données en un format de carte 5x5 — agrège les cellules brutes
    // (impact/probability continus) dans les buckets 1-5 correspondants, en sommant les
    // counts des cellules qui tombent dans le même bucket.
    const matrixMap = useMemo(() => {
        const map = new Map<string, number>();
        data.forEach(cell => {
            const key = `${bucketImpact(cell.impact)}-${bucketProbability(cell.probability)}`;
            map.set(key, (map.get(key) ?? 0) + cell.count);
        });
        return map;
    }, [data]);

    if (isLoading) {
        return (
            <div className="grid grid-cols-5 gap-1 p-4">
                {Array.from({ length: 25 }).map((_, i) => (
                    <div key={i} className="h-10 animate-pulse rounded-lg bg-white/5 border border-white/10" />
                ))}
            </div>
        );
    }

    // Le Risk Matrix est généralement représenté avec la Probabilité sur l'axe Y et l'Impact sur l'axe X.
    const dimensions = [5, 4, 3, 2, 1]; // Pour l'axe Y (Probabilité, de haut en bas)
    const columns = [1, 2, 3, 4, 5]; // Pour l'axe X (Impact)

    return (
        <div className="p-4">
            <h3 className="text-lg font-bold text-white mb-4">Risk Exposure Matrix (5x5)</h3>
            
            <div className="flex">
                {/* Axe Y (Probabilité) */}
                <div className="flex flex-col justify-end text-right pr-2 text-xs text-zinc-500 font-mono tracking-wider">
                    {dimensions.map(p => (
                        <div key={p} className="h-10 flex items-center justify-end">{p}</div>
                    ))}
                    <div className="h-10 text-white font-bold flex items-center justify-end">PROBA</div>
                </div>

                {/* La Grille (Cells) */}
                <div className="flex-1">
                    <div className="grid grid-cols-5 gap-1">
                        {dimensions.map(p => ( // Lignes (Probabilité)
                            columns.map(i => { // Colonnes (Impact)
                                const key = `${i}-${p}`;
                                const count = matrixMap.get(key) || 0;
                                const colorClass = getCellColor(i, p, count);
                                
                                return (
                                    <motion.div
                                        key={key}
                                        initial={{ opacity: 0, scale: 0.8 }}
                                        animate={{ opacity: 1, scale: 1 }}
                                        transition={{ delay: Math.min((i + (5 - p) * 5) * 0.012, 0.3) }}
                                        whileHover={{ scale: 1.05 }}
                                        className={`h-10 flex items-center justify-center rounded-lg text-sm font-bold transition-colors duration-150 cursor-pointer
                                                    ${colorClass} ${count > 0 ? 'ring-2' : ''}`}
                                        title={`Risks: ${count} | Score: ${i * p}`}
                                    >
                                        {count > 0 ? (
                                            <span className="text-white text-md font-extrabold drop-shadow-md">{count}</span>
                                        ) : (
                                            <span className="text-zinc-700/50">-</span>
                                        )}
                                    </motion.div>
                                );
                            })
                        ))}
                    </div>

                    {/* Axe X (Impact) */}
                    <div className="grid grid-cols-5 gap-1 mt-2">
                        {columns.map(i => (
                            <div key={i} className="h-10 flex items-center justify-center text-xs text-zinc-500 font-mono tracking-wider">
                                {i}
                            </div>
                        ))}
                    </div>
                    <div className="text-center mt-2 text-white font-bold">IMPACT</div>
                </div>
            </div>
        </div>
    );
};