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
        case 1: return { label: 'Faible', color: 'bg-green-500/20 text-green-300' };
        case 2: return { label: 'Moyen', color: 'bg-yellow-500/20 text-yellow-300' };
        case 3: return { label: 'Élevé', color: 'bg-red-500/20 text-red-300' };
        default: return { label: 'N/A', color: 'bg-zinc-500/20 text-zinc-300' };
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
        <Loader2 className="animate-spin text-blue-500" size={32} />
      </div>
    );
  }
  
  if (mitigations.length === 0) {
      return <div className="p-6 text-center text-zinc-500">Aucune mitigation en cours. Créez des risques pour générer des actions !</div>;
  }

  return (
    <div className="space-y-4 p-4">
      <h2 className="text-xl font-bold text-white flex items-center">
        <Zap size={20} className="text-yellow-400 mr-2" /> Priorité Intelligente
      </h2>
      <p className="text-zinc-400 text-sm">
        Liste des actions classées par leur impact maximal par rapport à l'effort minimal (Coût/Temps).
      </p>

      {mitigations.map((m) => {
        const costInfo = formatCost(m.cost);
        const priorityColor = m.weighted_priority > 10 ? 'bg-red-700/50' : m.weighted_priority > 5 ? 'bg-orange-700/50' : 'bg-zinc-700/50';

        return (
          <div 
            key={m.id} 
            className={`p-4 border border-zinc-700 rounded-lg shadow-xl transition-all duration-200 hover:border-blue-500 ${priorityColor}`}
          >
            <div className="flex justify-between items-start">
              <h3 className="text-lg font-semibold text-white">{m.title}</h3>
              <Badge variant="default" className="flex items-center space-x-1 bg-blue-600 hover:bg-blue-600">
                <Zap size={12} />
                <span>SPP: {m.weighted_priority.toFixed(2)}</span>
              </Badge>
            </div>

            <div className="mt-2 text-sm text-zinc-300 flex flex-wrap gap-x-4 gap-y-2">
                <div className="flex items-center">
                    <ShieldAlert size={14} className="text-red-400 mr-1" />
                    <span>Risque: {m.risk?.title || 'N/A'} (Score: {m.risk?.score || '?'})</span>
                </div>
                <div className="flex items-center">
                    <DollarSign size={14} className="text-zinc-400 mr-1" />
                    <Badge className={costInfo.color}>{costInfo.label} Coût</Badge>
                </div>
                <div className="flex items-center">
                    <Clock size={14} className="text-zinc-400 mr-1" />
                    <span>{m.mitigation_time} Jours Est.</span>
                </div>
            </div>

            <div className="mt-3 w-full bg-zinc-700 rounded-full h-2.5">
                <div 
                    className="h-2.5 rounded-full bg-emerald-500 transition-all duration-500" 
                    style={{ width: `${m.progress}%` }}
                ></div>
            </div>
            <p className="text-xs text-zinc-400 mt-1">{m.progress}% Complété</p>
          </div>
        );
      })}
    </div>
  );
};