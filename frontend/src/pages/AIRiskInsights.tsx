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
        predictedScore: ,
        confidence: .,
        factors: [
          {
            name: 'Outdated Security Libraries',
            impact: .,
            description: 'Critical security dependencies are  months outdated',
          },
          {
            name: 'Missing Multi-Factor Authentication',
            impact: .,
            description: 'Admin accounts lack FA protection',
          },
          {
            name: 'Weak Password Policy',
            impact: .,
            description: 'Password requirements below industry standards',
          },
        ],
        recommendation: ' HIGH: Schedule risk mitigation activities within  week. Focus on Outdated Security Libraries',
        timestamp: new Date().toISOString(),
      },
      {
        riskID: 'risk:data-exposure',
        predictedScore: ,
        confidence: .,
        factors: [
          {
            name: 'Unencrypted Data Transit',
            impact: .,
            description: 'Some API endpoints transmit data over unencrypted connections',
          },
          {
            name: 'Insufficient Access Controls',
            impact: .,
            description: 'Database access not properly restricted',
          },
        ],
        recommendation: ' MEDIUM: Monitor closely and plan preventive measures. Review quarterly.',
        timestamp: new Date(Date.now() -   ).toISOString(),
      },
      {
        riskID: 'risk:sql-injection',
        predictedScore: ,
        confidence: .,
        factors: [
          {
            name: 'Parameterized Queries Used',
            impact: -.,
            description: 'Strong protection in place',
          },
          {
            name: 'Input Validation Present',
            impact: -.,
            description: 'Comprehensive input validation configured',
          },
        ],
        recommendation: ' LOW: Standard monitoring sufficient. Review annually.',
        timestamp: new Date(Date.now() -   ).toISOString(),
      },
    ];

    const mockAnomalies: AnomalyScore[] = [
      {
        resourceID: 'db:query-latency',
        anomalyScore: .,
        severity: 'HIGH',
        details: 'Query latency . standard deviations from baseline (expected: ms, actual: ms)',
        pattern: 'SPIKE_DETECTED',
        timestamp: new Date(Date.now() - ).toISOString(),
      },
      {
        resourceID: 'api:error-rate',
        anomalyScore: .,
        severity: 'MEDIUM',
        details: 'Error rate showing gradual increase over last  minutes',
        pattern: 'INCREASING_TREND',
        timestamp: new Date(Date.now() - ).toISOString(),
      },
    ];

    setPredictions(mockPredictions);
    setAnomalies(mockAnomalies);
    setLoading(false);
  }, []);

  const getRiskColor = (score: number) => {
    if (score >= ) return 'from-red- to-red-';
    if (score >= ) return 'from-orange- to-orange-';
    if (score >= ) return 'from-yellow- to-yellow-';
    return 'from-green- to-green-';
  };

  const getAnomalySeverityColor = (severity: string) => {
    switch (severity) {
      case 'CRITICAL':
        return 'bg-red- border-red- text-red-';
      case 'HIGH':
        return 'bg-orange- border-orange- text-orange-';
      case 'MEDIUM':
        return 'bg-yellow- border-yellow- text-yellow-';
      case 'LOW':
        return 'bg-green- border-green- text-green-';
      default:
        return 'bg-gray- border-gray- text-gray-';
    }
  };

  const getSeverityIcon = (severity: string) => {
    switch (severity) {
      case 'CRITICAL':
        return '';
      case 'HIGH':
        return '';
      case 'MEDIUM':
        return '';
      case 'LOW':
        return '';
      default:
        return '';
    }
  };

  if (loading) {
    return <div className="text-center py-">Loading AI insights...</div>;
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate- via-slate- to-slate- p-">
      <div className="max-w-xl mx-auto">
        {/ Header /}
        <div className="mb-">
          <h className="text-xl font-bold text-white mb-"> AI Risk Intelligence</h>
          <p className="text-slate-">ML-powered predictions and anomaly detection</p>
        </div>

        {/ Risk Predictions /}
        <div className="mb-">
          <h className="text-xl font-bold text-white mb-">Risk Predictions</h>
          <div className="space-y-">
            {predictions.map((prediction) => (
              <div
                key={prediction.riskID}
                className="bg-slate- rounded-lg border border-slate- p- hover:border-slate- transition-all"
              >
                <div className="flex items-start gap-">
                  {/ Risk Score Visualization /}
                  <div className="flex-shrink-">
                    <div className="relative w- h- flex items-center justify-center">
                      <svg className="w- h- transform -rotate-" viewBox="   ">
                        <circle
                          cx=""
                          cy=""
                          r=""
                          fill="none"
                          stroke="currentColor"
                          strokeWidth=""
                          className="text-slate-"
                        />
                        <circle
                          cx=""
                          cy=""
                          r=""
                          fill="none"
                          stroke="currentColor"
                          strokeWidth=""
                          strokeDasharray={${prediction.predictedScore  .} }
                          className={bg-gradient-to-r ${getRiskColor(prediction.predictedScore)} text-red-}
                        />
                      </svg>
                      <div className="absolute text-center">
                        <div className="text-xl font-bold text-white">{prediction.predictedScore}</div>
                        <div className="text-xs text-slate-">Risk Score</div>
                      </div>
                    </div>
                  </div>

                  {/ Risk Details /}
                  <div className="flex-">
                    <div className="flex items-center justify-between mb-">
                      <h className="text-xl font-semibold text-white capitalize">
                        {prediction.riskID.replace('risk:', '').replace(/-/g, ' ')}
                      </h>
                      <div className="flex items-center gap- bg-slate- px- py- rounded-full">
                        <span className="text-sm text-slate-">Confidence:</span>
                        <span className="text-sm font-semibold text-blue-">
                          {(prediction.confidence  ).toFixed()}%
                        </span>
                      </div>
                    </div>

                    {/ Risk Factors /}
                    <div className="mb-">
                      <p className="text-sm font-semibold text-slate- mb-">Contributing Factors:</p>
                      <div className="space-y-">
                        {prediction.factors.map((factor, idx) => (
                          <div key={idx} className="flex items-center gap- text-sm">
                            <div className="w- h- bg-blue- rounded-full"></div>
                            <div>
                              <span className="font-medium text-slate-">{factor.name}</span>
                              <span className="text-slate- ml-">({(factor.impact  ).toFixed()}% impact)</span>
                              <p className="text-slate- text-xs mt-">{factor.description}</p>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>

                    {/ Recommendation /}
                    <div className="bg-slate- rounded px- py- text-slate-">
                      {prediction.recommendation}
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/ Anomalies Detected /}
        <div>
          <h className="text-xl font-bold text-white mb-">Anomalies Detected</h>
          <div className="space-y-">
            {anomalies.map((anomaly) => (
              <div
                key={anomaly.resourceID}
                className={rounded-lg border-l- p- ${getAnomalySeverityColor(anomaly.severity)}}
              >
                <div className="flex items-start justify-between mb-">
                  <div className="flex items-center gap-">
                    <span className="text-xl">{getSeverityIcon(anomaly.severity)}</span>
                    <div>
                      <h className="font-semibold capitalize">
                        {anomaly.resourceID.replace(':', ': ')}
                      </h>
                      <p className="text-sm opacity-">{anomaly.pattern.replace(/_/g, ' ')}</p>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-xl font-bold">
                      {(anomaly.anomalyScore  ).toFixed()}%
                    </div>
                    <div className="text-xs opacity-">Anomaly Score</div>
                  </div>
                </div>
                <p className="text-sm">{anomaly.details}</p>
                <div className="text-xs opacity- mt-">
                  Detected: {new Date(anomaly.timestamp).toLocaleString()}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/ AI Insights Footer /}
        <div className="mt- bg-slate- rounded-lg p- border border-slate-">
          <p className="text-slate- text-sm">
             <strong>AI Insight:</strong> The system has identified {predictions.length} critical risk areas and {anomalies.length} anomalies.
            Focus on the high-risk predictions to improve overall security posture.
          </p>
        </div>
      </div>
    </div>
  );
};

export default AIRiskInsights;
