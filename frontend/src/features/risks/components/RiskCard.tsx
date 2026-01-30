import { User, Database, ShieldAlert, Box } from 'lucide-react';
import type { Risk } from '../../../hooks/useRiskStore';

const SourceIcon = ({ source }: { source: string }) => {
  switch (source) {
    case 'THEHIVE':
      return <ShieldAlert size={} className="text-yellow-" />;
    case 'OPENRMF':
      return <Database size={} className="text-blue-" />;
    case 'OPENCTI':
      return <Box size={} className="text-purple-" />;
    default:
      return <User size={} className="text-zinc-" />; // Manual
  }
};

interface RiskCardProps {
  risk: Risk;
  onClick?: () => void;
}

export const RiskCard = ({ risk, onClick }: RiskCardProps) => {
  const riskLevelColor = {
    CRITICAL: 'bg-red-/ border-red-/',
    HIGH: 'bg-orange-/ border-orange-/',
    MEDIUM: 'bg-yellow-/ border-yellow-/',
    LOW: 'bg-blue-/ border-blue-/',
  }[risk.level || 'MEDIUM'] || 'bg-blue-/ border-blue-/';

  return (
    <div
      onClick={onClick}
      className={border rounded-lg p- cursor-pointer transition-colors hover:bg-zinc-/ ${riskLevelColor}}
    >
      <div className="flex items-start justify-between mb-">
        <h className="font-semibold text-white flex-">{risk.title}</h>
        <div className="flex items-center gap- ml-">
          <span className="text-lg font-bold text-white">{Math.round(risk.score || )}</span>
          <span className="text-xs text-zinc-">/ </span>
        </div>
      </div>

      <p className="text-sm text-zinc- mb- line-clamp-">{risk.description}</p>

      <div className="flex items-center gap- text-[px] font-bold border border-white/ px- py- rounded bg-zinc-">
        <SourceIcon source={risk.source} />
        <span className="text-zinc-">{risk.source}</span>
      </div>
    </div>
  );
};