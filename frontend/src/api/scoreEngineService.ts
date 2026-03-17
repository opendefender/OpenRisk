import { useAuthStore } from '../hooks/useAuthStore';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

interface ApiResponse<T> {
  data?: T;
  error?: string;
  status: number;
}

// Helper function to get auth header - lazy loads token from store
const getAuthHeader = (): Record<string, string> => {
  try {
    const token = useAuthStore.getState?.().token;
    return {
      'Authorization': `Bearer ${token || ''}`,
      'Content-Type': 'application/json',
    };
  } catch (error) {
    // Fallback if store is not yet initialized
    return {
      'Content-Type': 'application/json',
    };
  }
};

// Score Engine Types
export interface ScoringConfig {
  id: string;
  tenant_id?: string;
  name: string;
  description?: string;
  base_formula: string;
  weighting_factors: Record<string, number>;
  risk_matrix_thresholds: Record<string, number>;
  asset_criticality_mult: Record<string, number>;
  is_default: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface ComputeScoreInput {
  impact: number;
  probability: number;
  asset_ids?: string[];
  config_id?: string;
  apply_trend?: boolean;
  trend_factor?: number;
}

export interface ComputeScoreResponse {
  base_score: number;
  final_score: number;
  risk_level: string;
  impact: number;
  probability: number;
  asset_count: number;
}

export interface ClassifyRiskInput {
  score: number;
  config_id?: string;
}

export interface ClassifyRiskResponse {
  score: number;
  risk_level: string;
  config_id: string;
  matrix: Record<string, number>;
}

export interface RiskMatrixResponse {
  matrix: Record<string, number>;
  config_id: string;
  formula: string;
  weighting: Record<string, number>;
  criticality: Record<string, number>;
}

export interface ScoringMetricsResponse {
  avg_score: number;
  max_score: number;
  risk_stats: Array<{
    level: string;
    count: number;
  }>;
}

// ENDPOINTS

/**
 * Récupère toutes les configurations de scoring disponibles
 */
export const getScoringConfigs = async (): Promise<ApiResponse<{ message: string; default: ScoringConfig }>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/score-engine/configs`, {
      method: 'GET',
      headers: getAuthHeader(),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to fetch scoring configs' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

/**
 * Récupère une configuration de scoring spécifique
 */
export const getScoringConfig = async (configId: string): Promise<ApiResponse<ScoringConfig>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/score-engine/configs/${configId}`, {
      method: 'GET',
      headers: getAuthHeader(),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to fetch config' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

/**
 * Crée une nouvelle configuration de scoring (Admin only)
 */
export const createScoringConfig = async (config: Partial<ScoringConfig>): Promise<ApiResponse<ScoringConfig>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/score-engine/configs`, {
      method: 'POST',
      headers: getAuthHeader(),
      body: JSON.stringify(config),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to create config' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

/**
 * Met à jour une configuration de scoring (Admin only)
 */
export const updateScoringConfig = async (
  configId: string,
  updates: Partial<ScoringConfig>
): Promise<ApiResponse<ScoringConfig>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/score-engine/configs/${configId}`, {
      method: 'PUT',
      headers: getAuthHeader(),
      body: JSON.stringify(updates),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to update config' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

/**
 * Calcule le score d'un risque en utilisant une configuration spécifiée
 */
export const computeRiskScore = async (
  input: ComputeScoreInput
): Promise<ApiResponse<ComputeScoreResponse>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/score-engine/compute`, {
      method: 'POST',
      headers: getAuthHeader(),
      body: JSON.stringify(input),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to compute score' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

/**
 * Classe un risque en niveau basé sur un score
 */
export const classifyRisk = async (
  input: ClassifyRiskInput
): Promise<ApiResponse<ClassifyRiskResponse>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/score-engine/classify`, {
      method: 'POST',
      headers: getAuthHeader(),
      body: JSON.stringify(input),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to classify risk' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

/**
 * Récupère la matrice de risque (seuils de classification)
 */
export const getRiskMatrix = async (configId?: string): Promise<ApiResponse<RiskMatrixResponse>> => {
  try {
    const url = new URL(`${API_BASE_URL}/score-engine/matrix`);
    if (configId) {
      url.searchParams.append('config_id', configId);
    }

    const response = await fetch(url.toString(), {
      method: 'GET',
      headers: getAuthHeader(),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to fetch risk matrix' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

/**
 * Récupère les métriques de scoring (statistiques globales)
 */
export const getScoringMetrics = async (): Promise<ApiResponse<ScoringMetricsResponse>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/score-engine/metrics`, {
      method: 'GET',
      headers: getAuthHeader(),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to fetch metrics' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};
