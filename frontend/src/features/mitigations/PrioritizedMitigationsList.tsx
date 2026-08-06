// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useEffect, useState } from 'react';
import { api } from '../../lib/api';
import { Loader2, Zap, Clock, DollarSign, ShieldAlert } from 'lucide-react';
import { Badge } from '../../components/ui/Badge'; 

interface RiskData {
  title: string;
  score: number;
}

interface MitigationData {
  id: string;
  title: string;
  progress: number;
  cost: number;
  mitigation_time: number;
  weighted_priority: number;
  risk: RiskData; 
}

const formatCost = (cost: number) => {
    switch(cost) {
        case 1: return { label: 'Faible', color: 'bg-success/20 text-success-text' };
        case 2: return { label: 'Moyen', color: 'bg-warning/20 text-warning-text' };
        case 3: return { label: 'Élevé', color: 'bg-danger/20 text-danger-text' };
        default: return { label: 'N/A', color: 'bg-surface-3/20 text-text-secondary' };
    }
};

export const PrioritizedMitigationsList = () => {
  const [mitigations, setMitigations] = useState<MitigationData[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    api.get('/mitigations/recommended')
       .then(res => {
           setMitigations(res.data);
       })
       .catch(console.error)
       .finally(() => setIsLoading(false));
  }, []);

  if (isLoading) {
    return (
      <div className="flex justify-center items-center h-48">
        <Loader2 className="animate-spin text-info-text" size={32} />
      </div>
    );
  }
  
  if (mitigations.length === 0) {
      return <div className="p-6 text-center text-text-muted">Aucune mitigation en cours. Créez des risques pour générer des actions !</div>;
  }

  return (
    <div className="space-y-4 p-4">
      <h2 className="text-xl font-bold text-text-primary flex items-center">
        <Zap size={20} className="text-warning-text mr-2" /> Priorité Intelligente
      </h2>
      <p className="text-text-secondary text-sm">
        Liste des actions classées par leur impact maximal par rapport à l'effort minimal (Coût/Temps).
      </p>

      {mitigations.map((m) => {
        const costInfo = formatCost(m.cost);
        const priorityColor = m.weighted_priority > 10 ? 'bg-danger/50' : m.weighted_priority > 5 ? 'bg-warning/50' : 'bg-surface-3/50';

        return (
          <div 
            key={m.id} 
            className={`p-4 border border-border-default rounded-lg shadow-xl transition-all duration-200 hover:border-accent-400 ${priorityColor}`}
          >
            <div className="flex justify-between items-start">
              <h3 className="text-lg font-semibold text-text-primary">{m.title}</h3>
              <Badge variant="default" className="flex items-center space-x-1 bg-accent-500 hover:bg-accent-500">
                <Zap size={12} />
                <span>SPP: {m.weighted_priority.toFixed(2)}</span>
              </Badge>
            </div>

            <div className="mt-2 text-sm text-text-secondary flex flex-wrap gap-x-4 gap-y-2">
                <div className="flex items-center">
                    <ShieldAlert size={14} className="text-danger-text mr-1" />
                    <span>Risque: {m.risk?.title || 'N/A'} (Score: {m.risk?.score || '?'})</span>
                </div>
                <div className="flex items-center">
                    <DollarSign size={14} className="text-text-secondary mr-1" />
                    <Badge className={costInfo.color}>{costInfo.label} Coût</Badge>
                </div>
                <div className="flex items-center">
                    <Clock size={14} className="text-text-secondary mr-1" />
                    <span>{m.mitigation_time} Jours Est.</span>
                </div>
            </div>

            <div className="mt-3 w-full bg-surface-3 rounded-full h-2.5">
                <div 
                    className="h-2.5 rounded-full bg-success transition-all duration-500" 
                    style={{ width: `${m.progress}%` }}
                ></div>
            </div>
            <p className="text-xs text-text-secondary mt-1">{m.progress}% Complété</p>
          </div>
        );
      })}
    </div>
  );
};