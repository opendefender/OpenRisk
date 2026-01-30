import { useEffect, useState } from 'react';
import { api } from '../../lib/api';
import { Loader, Zap, Clock, DollarSign, ShieldAlert } from 'lucide-react';
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
        case : return { label: 'Faible', color: 'bg-green-/ text-green-' };
        case : return { label: 'Moyen', color: 'bg-yellow-/ text-yellow-' };
        case : return { label: 'Élev', color: 'bg-red-/ text-red-' };
        default: return { label: 'N/A', color: 'bg-zinc-/ text-zinc-' };
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
      <div className="flex justify-center items-center h-">
        <Loader className="animate-spin text-blue-" size={} />
      </div>
    );
  }
  
  if (mitigations.length === ) {
      return <div className="p- text-center text-zinc-">Aucune mitigation en cours. Crez des risques pour gnrer des actions !</div>;
  }

  return (
    <div className="space-y- p-">
      <h className="text-xl font-bold text-white flex items-center">
        <Zap size={} className="text-yellow- mr-" /> Priorit Intelligente
      </h>
      <p className="text-zinc- text-sm">
        Liste des actions classes par leur impact maximal par rapport à l'effort minimal (Coût/Temps).
      </p>

      {mitigations.map((m) => {
        const costInfo = formatCost(m.cost);
        const priorityColor = m.weighted_priority >  ? 'bg-red-/' : m.weighted_priority >  ? 'bg-orange-/' : 'bg-zinc-/';

        return (
          <div 
            key={m.id} 
            className={p- border border-zinc- rounded-lg shadow-xl transition-all duration- hover:border-blue- ${priorityColor}}
          >
            <div className="flex justify-between items-start">
              <h className="text-lg font-semibold text-white">{m.title}</h>
              <Badge variant="default" className="flex items-center space-x- bg-blue- hover:bg-blue-">
                <Zap size={} />
                <span>SPP: {m.weighted_priority.toFixed()}</span>
              </Badge>
            </div>

            <div className="mt- text-sm text-zinc- flex flex-wrap gap-x- gap-y-">
                <div className="flex items-center">
                    <ShieldAlert size={} className="text-red- mr-" />
                    <span>Risque: {m.risk?.title || 'N/A'} (Score: {m.risk?.score || '?'})</span>
                </div>
                <div className="flex items-center">
                    <DollarSign size={} className="text-zinc- mr-" />
                    <Badge className={costInfo.color}>{costInfo.label} Coût</Badge>
                </div>
                <div className="flex items-center">
                    <Clock size={} className="text-zinc- mr-" />
                    <span>{m.mitigation_time} Jours Est.</span>
                </div>
            </div>

            <div className="mt- w-full bg-zinc- rounded-full h-.">
                <div 
                    className="h-. rounded-full bg-emerald- transition-all duration-" 
                    style={{ width: ${m.progress}% }}
                ></div>
            </div>
            <p className="text-xs text-zinc- mt-">{m.progress}% Complt</p>
          </div>
        );
      })}
    </div>
  );
};