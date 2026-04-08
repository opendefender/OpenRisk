import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { BarChart3, TrendingUp, Calculator, Shield, AlertTriangle, CheckCircle } from 'lucide-react';
import { toast } from 'sonner';
import {
  computeRiskScore,
  getRiskMatrix,
  classifyRisk,
  getScoringMetrics,
  type ComputeScoreInput,
  type ComputeScoreResponse,
  type RiskMatrixResponse,
  type ScoringMetricsResponse,
} from '../../../api/scoreEngineService';

interface ScoreEngineVisualizerProps {
  impact: number;
  probability: number;
  assetIds?: string[];
  configId?: string;
  onScoreComputed?: (score: ComputeScoreResponse) => void;
}

const getRiskLevelColor = (level: string): string => {
  switch (level.toLowerCase()) {
    case 'critical':
      return 'bg-red-500/10 border-red-500 text-red-700';
    case 'high':
      return 'bg-orange-500/10 border-orange-500 text-orange-700';
    case 'medium':
      return 'bg-yellow-500/10 border-yellow-500 text-yellow-700';
    case 'low':
      return 'bg-green-500/10 border-green-500 text-green-700';
    default:
      return 'bg-gray-500/10 border-gray-500 text-gray-700';
  }
};

const getRiskLevelIcon = (level: string) => {
  switch (level.toLowerCase()) {
    case 'critical':
      return <AlertTriangle className="w-5 h-5" />;
    case 'high':
      return <AlertTriangle className="w-5 h-5" />;
    case 'medium':
      return <TrendingUp className="w-5 h-5" />;
    case 'low':
      return <CheckCircle className="w-5 h-5" />;
    default:
      return <Shield className="w-5 h-5" />;
  }
};

/**
 * ScoreEngineVisualizer
 * Composant pour afficher et calculer les scores de risque en temps réel
 */
export const ScoreEngineVisualizer = ({
  impact,
  probability,
  assetIds = [],
  configId = 'default',
  onScoreComputed,
}: ScoreEngineVisualizerProps) => {
  const [score, setScore] = useState<ComputeScoreResponse | null>(null);
  const [matrix, setMatrix] = useState<RiskMatrixResponse | null>(null);
  const [metrics, setMetrics] = useState<ScoringMetricsResponse | null>(null);
  const [loading, setLoading] = useState(false);

  // Calculer le score automatiquement quand impact ou probability change
  useEffect(() => {
    if (impact >= 1 && impact <= 5 && probability >= 1 && probability <= 5) {
      computeScore();
    }
  }, [impact, probability, assetIds, configId]);

  // Charger les données au montage
  useEffect(() => {
    loadMatrix();
    loadMetrics();
  }, [configId]);

  const computeScore = async () => {
    setLoading(true);
    try {
      const input: ComputeScoreInput = {
        impact,
        probability,
        asset_ids: assetIds.length > 0 ? assetIds : undefined,
        config_id: configId,
        apply_trend: false,
      };

      const response = await computeRiskScore(input);

      if (response.error) {
        toast.error('Erreur', {
          description: response.error,
        });
      } else if (response.data) {
        setScore(response.data);
        onScoreComputed?.(response.data);
      }
    } catch (error) {
      console.error('Error computing score:', error);
      toast.error('Erreur', {
        description: 'Impossible de calculer le score',
      });
    } finally {
      setLoading(false);
    }
  };

  const loadMatrix = async () => {
    try {
      const response = await getRiskMatrix(configId);
      if (response.data) {
        setMatrix(response.data);
      }
    } catch (error) {
      console.error('Error loading matrix:', error);
    }
  };

  const loadMetrics = async () => {
    try {
      const response = await getScoringMetrics();
      if (response.data) {
        setMetrics(response.data);
      }
    } catch (error) {
      console.error('Error loading metrics:', error);
    }
  };

  return (
    <div className="space-y-4">
      {/* Score Display */}
      {score && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className={`border-2 rounded-lg p-6 ${getRiskLevelColor(score.risk_level)}`}
        >
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              {getRiskLevelIcon(score.risk_level)}
              <h3 className="text-lg font-semibold capitalize">{score.risk_level}</h3>
            </div>
            <Calculator className="w-5 h-5 opacity-50" />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <p className="text-sm opacity-75 mb-1">Score de Base</p>
              <p className="text-2xl font-bold">{score.base_score.toFixed(2)}</p>
            </div>
            <div>
              <p className="text-sm opacity-75 mb-1">Score Final</p>
              <p className="text-2xl font-bold">{score.final_score.toFixed(2)}</p>
            </div>
          </div>

          <div className="mt-4 pt-4 border-t border-current opacity-50 space-y-2">
            <div className="flex justify-between text-sm">
              <span>Impact:</span>
              <span className="font-medium">{score.impact}/5</span>
            </div>
            <div className="flex justify-between text-sm">
              <span>Probabilité:</span>
              <span className="font-medium">{score.probability}/5</span>
            </div>
            {score.asset_count > 0 && (
              <div className="flex justify-between text-sm">
                <span>Assets liés:</span>
                <span className="font-medium">{score.asset_count}</span>
              </div>
            )}
          </div>
        </motion.div>
      )}

      {loading && (
        <div className="text-center py-4">
          <div className="inline-flex items-center gap-2 text-gray-600">
            <div className="w-4 h-4 bg-blue-500 rounded-full animate-bounce" />
            <span>Calcul du score...</span>
          </div>
        </div>
      )}

      {/* Risk Matrix */}
      {matrix && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-lg p-4"
        >
          <div className="flex items-center gap-2 mb-3">
            <BarChart3 className="w-5 h-5 text-blue-600" />
            <h4 className="font-semibold">Matrice de Risque</h4>
          </div>

          <div className="grid grid-cols-4 gap-2">
            {Object.entries(matrix.matrix).map(([level, threshold]) => (
              <div
                key={level}
                className={`border rounded-lg p-3 text-center text-sm font-medium capitalize ${getRiskLevelColor(level)}`}
              >
                <div className="text-xs opacity-75 mb-1">{level}</div>
                <div className="text-lg font-bold">{threshold}</div>
              </div>
            ))}
          </div>

          <p className="text-xs text-gray-600 dark:text-gray-400 mt-3">
            Formule: <span className="font-mono">{matrix.formula}</span>
          </p>
        </motion.div>
      )}

      {/* Statistics */}
      {metrics && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
          className="bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-lg p-4"
        >
          <div className="flex items-center gap-2 mb-3">
            <TrendingUp className="w-5 h-5 text-green-600" />
            <h4 className="font-semibold">Statistiques Globales</h4>
          </div>

          <div className="grid grid-cols-2 gap-4 mb-4">
            <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-3">
              <p className="text-xs text-gray-600 dark:text-gray-400 mb-1">Score Moyen</p>
              <p className="text-2xl font-bold">{metrics.avg_score.toFixed(2)}</p>
            </div>
            <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-3">
              <p className="text-xs text-gray-600 dark:text-gray-400 mb-1">Score Max</p>
              <p className="text-2xl font-bold">{metrics.max_score.toFixed(2)}</p>
            </div>
          </div>

          <div className="space-y-2">
            <p className="text-xs font-semibold text-gray-700 dark:text-gray-300">Distribution:</p>
            {metrics.risk_stats.map((stat) => (
              <div key={stat.level} className="flex items-center justify-between text-sm">
                <span className="capitalize">{stat.level}</span>
                <div className="flex-1 mx-2 bg-gray-200 dark:bg-gray-700 rounded-full h-2 overflow-hidden">
                  <div
                    className={`h-full transition-all ${
                      stat.level === 'critical'
                        ? 'bg-red-500'
                        : stat.level === 'high'
                          ? 'bg-orange-500'
                          : stat.level === 'medium'
                            ? 'bg-yellow-500'
                            : 'bg-green-500'
                    }`}
                    style={{
                      width: `${Math.max(
                        (stat.count / Math.max(...metrics.risk_stats.map((s) => s.count))) * 100,
                        5
                      )}%`,
                    }}
                  />
                </div>
                <span className="font-medium text-right w-8">{stat.count}</span>
              </div>
            ))}
          </div>
        </motion.div>
      )}
    </div>
  );
};

export default ScoreEngineVisualizer;
