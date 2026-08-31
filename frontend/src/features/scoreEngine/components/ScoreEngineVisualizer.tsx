// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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
      return 'bg-danger/10 border-danger text-danger-text';
    case 'high':
      return 'bg-warning/10 border-warning text-warning-text';
    case 'medium':
      return 'bg-warning/10 border-warning text-warning-text';
    case 'low':
      return 'bg-success/10 border-success text-success-text';
    default:
      return 'bg-surface-3/10 border-border-strong text-fg-primary';
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
          <div className="inline-flex items-center gap-2 text-fg-muted">
            <div className="w-4 h-4 bg-accent rounded-full animate-bounce" />
            <span>Calcul du score...</span>
          </div>
        </div>
      )}

      {/* Risk Matrix */}
      {matrix && matrix.matrix && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="bg-surface-sunken dark:bg-surface-1 border border-border-subtle dark:border-border-subtle rounded-lg p-4"
        >
          <div className="flex items-center gap-2 mb-3">
            <BarChart3 className="w-5 h-5 text-info-text" />
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

          <p className="text-xs text-fg-muted dark:text-fg-secondary mt-3">
            Formule: <span className="font-mono">{matrix.formula}</span>
          </p>
        </motion.div>
      )}

      {/* Statistics */}
      {metrics && metrics.risk_stats && metrics.risk_stats.length > 0 && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
          className="bg-surface-sunken dark:bg-surface-1 border border-border-subtle dark:border-border-subtle rounded-lg p-4"
        >
          <div className="flex items-center gap-2 mb-3">
            <TrendingUp className="w-5 h-5 text-success-text" />
            <h4 className="font-semibold">Statistiques Globales</h4>
          </div>

          <div className="grid grid-cols-2 gap-4 mb-4">
            <div className="bg-surface-1 dark:bg-surface-2 border border-border-subtle dark:border-border-default rounded-lg p-3">
              <p className="text-xs text-fg-muted dark:text-fg-secondary mb-1">Score Moyen</p>
              <p className="text-2xl font-bold">{metrics.avg_score.toFixed(2)}</p>
            </div>
            <div className="bg-surface-1 dark:bg-surface-2 border border-border-subtle dark:border-border-default rounded-lg p-3">
              <p className="text-xs text-fg-muted dark:text-fg-secondary mb-1">Score Max</p>
              <p className="text-2xl font-bold">{metrics.max_score.toFixed(2)}</p>
            </div>
          </div>

          <div className="space-y-2">
            <p className="text-xs font-semibold text-fg-primary dark:text-fg-secondary">Distribution:</p>
            {metrics.risk_stats.map((stat) => (
              <div key={stat.level} className="flex items-center justify-between text-sm">
                <span className="capitalize">{stat.level}</span>
                <div className="flex-1 mx-2 bg-surface-sunken dark:bg-surface-3 rounded-full h-2 overflow-hidden">
                  <div
                    className={`h-full transition-all ${
                      stat.level === 'critical'
                        ? 'bg-danger'
                        : stat.level === 'high'
                          ? 'bg-warning'
                          : stat.level === 'medium'
                            ? 'bg-warning'
                            : 'bg-success'
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
