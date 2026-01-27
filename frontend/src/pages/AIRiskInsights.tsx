import React, { useEffect, useState } from 'react';

interface RiskPrediction {
  riskID: string;
  predictedScore: number;
  confidence: number;
  factors: Array<{
    name: string;
    impact: number;
    description: string;
  }>;
  recommendation: string;
  timestamp: string;
}

interface AnomalyScore {
  resourceID: string;
  anomalyScore: number;
  severity: 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';
  details: string;
  pattern: string;
  timestamp: string;
}

export const AIRiskInsights: React.FC = () => {
  const [predictions, setPredictions] = useState<RiskPrediction[]>([]);
  const [anomalies, setAnomalies] = useState<AnomalyScore[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Simulate fetching AI predictions
    const mockPredictions: RiskPrediction[] = [
      {
        riskID: 'risk:authentication-bypass',
        predictedScore: 72,
        confidence: 0.89,
        factors: [
          {
            name: 'Outdated Security Libraries',
            impact: 0.8,
            description: 'Critical security dependencies are 6 months outdated',
          },
          {
            name: 'Missing Multi-Factor Authentication',
            impact: 0.7,
            description: 'Admin accounts lack 2FA protection',
          },
          {
            name: 'Weak Password Policy',
            impact: 0.6,
            description: 'Password requirements below industry standards',
          },
        ],
        recommendation: '🟠 HIGH: Schedule risk mitigation activities within 1 week. Focus on Outdated Security Libraries',
        timestamp: new Date().toISOString(),
      },
      {
        riskID: 'risk:data-exposure',
        predictedScore: 54,
        confidence: 0.76,
        factors: [
          {
            name: 'Unencrypted Data Transit',
            impact: 0.75,
            description: 'Some API endpoints transmit data over unencrypted connections',
          },
          {
            name: 'Insufficient Access Controls',
            impact: 0.65,
            description: 'Database access not properly restricted',
          },
        ],
        recommendation: '🟡 MEDIUM: Monitor closely and plan preventive measures. Review quarterly.',
        timestamp: new Date(Date.now() - 2 * 60000).toISOString(),
      },
      {
        riskID: 'risk:sql-injection',
        predictedScore: 35,
        confidence: 0.92,
        factors: [
          {
            name: 'Parameterized Queries Used',
            impact: -0.9,
            description: 'Strong protection in place',
          },
          {
            name: 'Input Validation Present',
            impact: -0.8,
            description: 'Comprehensive input validation configured',
          },
        ],
        recommendation: '🟢 LOW: Standard monitoring sufficient. Review annually.',
        timestamp: new Date(Date.now() - 5 * 60000).toISOString(),
      },
    ];

    const mockAnomalies: AnomalyScore[] = [
      {
        resourceID: 'db:query-latency',
        anomalyScore: 0.82,
        severity: 'HIGH',
        details: 'Query latency 3.2 standard deviations from baseline (expected: 45ms, actual: 285ms)',
        pattern: 'SPIKE_DETECTED',
        timestamp: new Date(Date.now() - 1000).toISOString(),
      },
      {
        resourceID: 'api:error-rate',
        anomalyScore: 0.45,
        severity: 'MEDIUM',
        details: 'Error rate showing gradual increase over last 30 minutes',
        pattern: 'INCREASING_TREND',
        timestamp: new Date(Date.now() - 30000).toISOString(),
      },
    ];

    setPredictions(mockPredictions);
    setAnomalies(mockAnomalies);
    setLoading(false);
  }, []);

  const getRiskColor = (score: number) => {
    if (score >= 75) return 'from-red-500 to-red-600';
    if (score >= 60) return 'from-orange-500 to-orange-600';
    if (score >= 40) return 'from-yellow-500 to-yellow-600';
    return 'from-green-500 to-green-600';
  };

  const getAnomalySeverityColor = (severity: string) => {
    switch (severity) {
      case 'CRITICAL':
        return 'bg-red-100 border-red-300 text-red-900';
      case 'HIGH':
        return 'bg-orange-100 border-orange-300 text-orange-900';
      case 'MEDIUM':
        return 'bg-yellow-100 border-yellow-300 text-yellow-900';
      case 'LOW':
        return 'bg-green-100 border-green-300 text-green-900';
      default:
        return 'bg-gray-100 border-gray-300 text-gray-900';
    }
  };

  const getSeverityIcon = (severity: string) => {
    switch (severity) {
      case 'CRITICAL':
        return '🔴';
      case 'HIGH':
        return '🟠';
      case 'MEDIUM':
        return '🟡';
      case 'LOW':
        return '🟢';
      default:
        return '⚪';
    }
  };

  if (loading) {
    return <div className="text-center py-8">Loading AI insights...</div>;
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900 p-8">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-4xl font-bold text-white mb-2">🤖 AI Risk Intelligence</h1>
          <p className="text-slate-300">ML-powered predictions and anomaly detection</p>
        </div>

        {/* Risk Predictions */}
        <div className="mb-12">
          <h2 className="text-2xl font-bold text-white mb-6">Risk Predictions</h2>
          <div className="space-y-4">
            {predictions.map((prediction) => (
              <div
                key={prediction.riskID}
                className="bg-slate-700 rounded-lg border border-slate-600 p-6 hover:border-slate-500 transition-all"
              >
                <div className="flex items-start gap-6">
                  {/* Risk Score Visualization */}
                  <div className="flex-shrink-0">
                    <div className="relative w-24 h-24 flex items-center justify-center">
                      <svg className="w-24 h-24 transform -rotate-90" viewBox="0 0 100 100">
                        <circle
                          cx="50"
                          cy="50"
                          r="45"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="3"
                          className="text-slate-600"
                        />
                        <circle
                          cx="50"
                          cy="50"
                          r="45"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="3"
                          strokeDasharray={`${prediction.predictedScore * 2.83} 283`}
                          className={`bg-gradient-to-r ${getRiskColor(prediction.predictedScore)} text-red-500`}
                        />
                      </svg>
                      <div className="absolute text-center">
                        <div className="text-3xl font-bold text-white">{prediction.predictedScore}</div>
                        <div className="text-xs text-slate-400">Risk Score</div>
                      </div>
                    </div>
                  </div>

                  {/* Risk Details */}
                  <div className="flex-1">
                    <div className="flex items-center justify-between mb-3">
                      <h3 className="text-xl font-semibold text-white capitalize">
                        {prediction.riskID.replace('risk:', '').replace(/-/g, ' ')}
                      </h3>
                      <div className="flex items-center gap-2 bg-slate-600 px-3 py-1 rounded-full">
                        <span className="text-sm text-slate-300">Confidence:</span>
                        <span className="text-sm font-semibold text-blue-300">
                          {(prediction.confidence * 100).toFixed(0)}%
                        </span>
                      </div>
                    </div>

                    {/* Risk Factors */}
                    <div className="mb-4">
                      <p className="text-sm font-semibold text-slate-300 mb-2">Contributing Factors:</p>
                      <div className="space-y-2">
                        {prediction.factors.map((factor, idx) => (
                          <div key={idx} className="flex items-center gap-3 text-sm">
                            <div className="w-2 h-2 bg-blue-400 rounded-full"></div>
                            <div>
                              <span className="font-medium text-slate-100">{factor.name}</span>
                              <span className="text-slate-400 ml-2">({(factor.impact * 100).toFixed(0)}% impact)</span>
                              <p className="text-slate-500 text-xs mt-1">{factor.description}</p>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>

                    {/* Recommendation */}
                    <div className="bg-slate-600 rounded px-4 py-3 text-slate-100">
                      {prediction.recommendation}
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Anomalies Detected */}
        <div>
          <h2 className="text-2xl font-bold text-white mb-6">Anomalies Detected</h2>
          <div className="space-y-4">
            {anomalies.map((anomaly) => (
              <div
                key={anomaly.resourceID}
                className={`rounded-lg border-l-4 p-6 ${getAnomalySeverityColor(anomaly.severity)}`}
              >
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center gap-3">
                    <span className="text-3xl">{getSeverityIcon(anomaly.severity)}</span>
                    <div>
                      <h3 className="font-semibold capitalize">
                        {anomaly.resourceID.replace(':', ': ')}
                      </h3>
                      <p className="text-sm opacity-75">{anomaly.pattern.replace(/_/g, ' ')}</p>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-2xl font-bold">
                      {(anomaly.anomalyScore * 100).toFixed(0)}%
                    </div>
                    <div className="text-xs opacity-75">Anomaly Score</div>
                  </div>
                </div>
                <p className="text-sm">{anomaly.details}</p>
                <div className="text-xs opacity-50 mt-2">
                  Detected: {new Date(anomaly.timestamp).toLocaleString()}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* AI Insights Footer */}
        <div className="mt-8 bg-slate-700 rounded-lg p-4 border border-slate-600">
          <p className="text-slate-300 text-sm">
            💡 <strong>AI Insight:</strong> The system has identified {predictions.length} critical risk areas and {anomalies.length} anomalies.
            Focus on the high-risk predictions to improve overall security posture.
          </p>
        </div>
      </div>
    </div>
  );
};

export default AIRiskInsights;
