import { useState, useCallback } from 'react';
import {
  getScoringConfigs,
  getScoringConfig,
  createScoringConfig,
  updateScoringConfig,
  computeRiskScore,
  getRiskMatrix,
  classifyRisk,
  getScoringMetrics,
  ScoringConfig,
  ComputeScoreInput,
  ComputeScoreResponse,
  RiskMatrixResponse,
  ScoringMetricsResponse,
  ClassifyRiskInput,
  ClassifyRiskResponse,
} from '../api/scoreEngineService';

interface UseScoreEngineReturn {
  // Configs
  configs: any | null;
  selectedConfig: ScoringConfig | null;
  loadConfigs: () => Promise<void>;
  getConfig: (configId: string) => Promise<void>;
  createConfig: (config: Partial<ScoringConfig>) => Promise<void>;
  updateConfig: (configId: string, updates: Partial<ScoringConfig>) => Promise<void>;

  // Score Computation
  score: ComputeScoreResponse | null;
  computeScore: (input: ComputeScoreInput) => Promise<void>;
  classifyScore: (input: ClassifyRiskInput) => Promise<ClassifyRiskResponse | null>;

  // Matrix & Metrics
  matrix: RiskMatrixResponse | null;
  metrics: ScoringMetricsResponse | null;
  loadMatrix: (configId?: string) => Promise<void>;
  loadMetrics: () => Promise<void>;

  // Loading states
  isLoading: boolean;
  error: string | null;
}

export const useScoreEngine = (): UseScoreEngineReturn => {
  const [configs, setConfigs] = useState<any>(null);
  const [selectedConfig, setSelectedConfig] = useState<ScoringConfig | null>(null);
  const [score, setScore] = useState<ComputeScoreResponse | null>(null);
  const [matrix, setMatrix] = useState<RiskMatrixResponse | null>(null);
  const [metrics, setMetrics] = useState<ScoringMetricsResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadConfigs = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await getScoringConfigs();
      if (response.data) {
        setConfigs(response.data);
        setSelectedConfig(response.data.default);
      } else {
        setError(response.error || 'Failed to load configs');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setIsLoading(false);
    }
  }, []);

  const getConfig = useCallback(async (configId: string) => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await getScoringConfig(configId);
      if (response.data) {
        setSelectedConfig(response.data);
      } else {
        setError(response.error || 'Failed to load config');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setIsLoading(false);
    }
  }, []);

  const createConfig = useCallback(async (config: Partial<ScoringConfig>) => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await createScoringConfig(config);
      if (response.error) {
        setError(response.error);
      } else {
        await loadConfigs();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setIsLoading(false);
    }
  }, [loadConfigs]);

  const updateConfig = useCallback(
    async (configId: string, updates: Partial<ScoringConfig>) => {
      setIsLoading(true);
      setError(null);
      try {
        const response = await updateScoringConfig(configId, updates);
        if (response.error) {
          setError(response.error);
        } else {
          await loadConfigs();
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Unknown error');
      } finally {
        setIsLoading(false);
      }
    },
    [loadConfigs]
  );

  const computeScore = useCallback(async (input: ComputeScoreInput) => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await computeRiskScore(input);
      if (response.data) {
        setScore(response.data);
      } else {
        setError(response.error || 'Failed to compute score');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setIsLoading(false);
    }
  }, []);

  const classifyScore = useCallback(async (input: ClassifyRiskInput) => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await classifyRisk(input);
      if (response.data) {
        return response.data;
      } else {
        setError(response.error || 'Failed to classify risk');
        return null;
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
      return null;
    } finally {
      setIsLoading(false);
    }
  }, []);

  const loadMatrix = useCallback(async (configId?: string) => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await getRiskMatrix(configId);
      if (response.data) {
        setMatrix(response.data);
      } else {
        setError(response.error || 'Failed to load matrix');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setIsLoading(false);
    }
  }, []);

  const loadMetrics = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await getScoringMetrics();
      if (response.data) {
        setMetrics(response.data);
      } else {
        setError(response.error || 'Failed to load metrics');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setIsLoading(false);
    }
  }, []);

  return {
    configs,
    selectedConfig,
    loadConfigs,
    getConfig,
    createConfig,
    updateConfig,
    score,
    computeScore,
    classifyScore,
    matrix,
    metrics,
    loadMatrix,
    loadMetrics,
    isLoading,
    error,
  };
};

export default useScoreEngine;
