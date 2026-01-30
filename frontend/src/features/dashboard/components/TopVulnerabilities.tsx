import { useEffect, useState } from 'react';
import { api } from '../../../lib/api';
import { Loader, AlertCircle, AlertTriangle, AlertOctagon } from 'lucide-react';

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
      return { bg: 'bg-red-/', text: 'text-red-', border: 'border-red-/', badge: 'bg-red-/' };
    case 'high':
      return { bg: 'bg-orange-/', text: 'text-orange-', border: 'border-orange-/', badge: 'bg-orange-/' };
    case 'medium':
      return { bg: 'bg-yellow-/', text: 'text-yellow-', border: 'border-yellow-/', badge: 'bg-yellow-/' };
    default:
      return { bg: 'bg-blue-/', text: 'text-blue-', border: 'border-blue-/', badge: 'bg-blue-/' };
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
        const res = await api.get('/stats/top-vulnerabilities?limit=');
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
      <div className="flex justify-center items-center h-full text-zinc-">
        <Loader className="animate-spin mr-" size={} />
        Loading Vulnerabilities...
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-zinc-">
        <AlertTriangle size={} className="mb- text-orange-/" />
        <p className="text-sm">{error}</p>
      </div>
    );
  }

  return (
    <div className="h-full w-full flex flex-col space-y- overflow-y-auto pr-">
      {vulnerabilities.length >  ? (
        vulnerabilities.map((vuln, index) => {
          const colors = getSeverityColor(vuln.severity);
          const Icon = getSeverityIcon(vuln.severity);

          return (
            <div
              key={vuln.id}
              className={group p- rounded-lg border ${colors.border} ${colors.bg} hover:bg-white/ transition-all duration- cursor-pointer transform hover:scale-}
            >
              <div className="flex items-start gap-">
                <Icon className={${colors.text} flex-shrink- mt-.} size={} />
                
                <div className="flex- min-w-">
                  <div className="flex items-center gap- mb-">
                    <p className="text-sm font-semibold text-white group-hover:text-primary transition-colors truncate">
                      {index + }. {vuln.title}
                    </p>
                  </div>
                  
                  <div className="flex items-center gap- flex-wrap">
                    <span className={text-xs font-bold px- py- rounded ${colors.badge} ${colors.text} border ${colors.border}}>
                      {vuln.severity}
                    </span>
                    
                    {vuln.cvssScore && (
                      <span className="text-xs text-zinc-">
                        CVSS: <span className="text-white font-bold">{vuln.cvssScore}</span>
                      </span>
                    )}
                    
                    {vuln.affectedAssets && (
                      <span className="text-xs text-zinc-">
                        <span className="text-white font-bold">{vuln.affectedAssets}</span> assets
                      </span>
                    )}
                  </div>
                </div>

                <div className="text-xs text-zinc- group-hover:text-primary transition-colors flex-shrink-">
                  →
                </div>
              </div>
            </div>
          );
        })
      ) : (
        <div className="flex flex-col items-center justify-center py- text-center text-zinc-">
          <AlertCircle size={} className="mb- opacity-" />
          <p className="text-sm">No vulnerabilities found</p>
        </div>
      )}
    </div>
  );
};
