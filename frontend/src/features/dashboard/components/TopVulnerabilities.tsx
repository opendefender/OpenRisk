import { useEffect, useState } from 'react';
import { api } from '../../../lib/api';
import { Loader2, AlertCircle, AlertTriangle, AlertOctagon } from 'lucide-react';

interface Vulnerability {
  id: string;
  title: string;
  severity: 'Critical' | 'High' | 'Medium' | 'Low';
  cvssScore?: number;
  affectedAssets?: number;
}

const getSeverityColor = (severity: string) => {
  switch (severity.toLowerCase()) {
    case 'critical':
      return { bg: 'bg-red-500/10', text: 'text-red-400', border: 'border-red-500/30', badge: 'bg-red-500/20' };
    case 'high':
      return { bg: 'bg-orange-500/10', text: 'text-orange-400', border: 'border-orange-500/30', badge: 'bg-orange-500/20' };
    case 'medium':
      return { bg: 'bg-yellow-500/10', text: 'text-yellow-400', border: 'border-yellow-500/30', badge: 'bg-yellow-500/20' };
    default:
      return { bg: 'bg-blue-500/10', text: 'text-blue-400', border: 'border-blue-500/30', badge: 'bg-blue-500/20' };
  }
};

const getSeverityIcon = (severity: string) => {
  switch (severity.toLowerCase()) {
    case 'critical':
      return AlertOctagon;
    case 'high':
      return AlertTriangle;
    case 'medium':
      return AlertCircle;
    default:
      return AlertCircle;
  }
};

export const TopVulnerabilities = () => {
  const [vulnerabilities, setVulnerabilities] = useState<Vulnerability[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const res = await api.get('/stats/top-vulnerabilities?limit=5');
        setVulnerabilities(res.data || []);
        setError(null);
      } catch (err) {
        console.error('Failed to fetch top vulnerabilities:', err);
        setError('Failed to load vulnerabilities');
        setVulnerabilities([]);
      } finally {
        setIsLoading(false);
      }
    };

    fetchData();
  }, []);

  if (isLoading) {
    return (
      <div className="flex justify-center items-center h-full text-zinc-500">
        <Loader2 className="animate-spin mr-2" size={20} />
        Loading Vulnerabilities...
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-zinc-500">
        <AlertTriangle size={32} className="mb-2 text-orange-500/50" />
        <p className="text-sm">{error}</p>
      </div>
    );
  }

  return (
    <div className="h-full w-full flex flex-col space-y-3 overflow-y-auto pr-2">
      {vulnerabilities.length > 0 ? (
        vulnerabilities.map((vuln, index) => {
          const colors = getSeverityColor(vuln.severity);
          const Icon = getSeverityIcon(vuln.severity);

          return (
            <div
              key={vuln.id}
              className={`group p-3 rounded-lg border ${colors.border} ${colors.bg} hover:bg-white/5 transition-all duration-200 cursor-pointer transform hover:scale-102`}
            >
              <div className="flex items-start gap-3">
                <Icon className={`${colors.text} flex-shrink-0 mt-0.5`} size={18} />
                
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <p className="text-sm font-semibold text-white group-hover:text-primary transition-colors truncate">
                      {index + 1}. {vuln.title}
                    </p>
                  </div>
                  
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className={`text-xs font-bold px-2 py-1 rounded ${colors.badge} ${colors.text} border ${colors.border}`}>
                      {vuln.severity}
                    </span>
                    
                    {vuln.cvssScore && (
                      <span className="text-xs text-zinc-400">
                        CVSS: <span className="text-white font-bold">{vuln.cvssScore}</span>
                      </span>
                    )}
                    
                    {vuln.affectedAssets && (
                      <span className="text-xs text-zinc-400">
                        <span className="text-white font-bold">{vuln.affectedAssets}</span> assets
                      </span>
                    )}
                  </div>
                </div>

                <div className="text-xs text-zinc-500 group-hover:text-primary transition-colors flex-shrink-0">
                  →
                </div>
              </div>
            </div>
          );
        })
      ) : (
        <div className="flex flex-col items-center justify-center py-8 text-center text-zinc-500">
          <AlertCircle size={32} className="mb-2 opacity-50" />
          <p className="text-sm">No vulnerabilities found</p>
        </div>
      )}
    </div>
  );
};
