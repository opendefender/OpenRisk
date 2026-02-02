import { useAuthStore } from '../hooks/useAuthStore';

const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080/api/v1';

interface ApiResponse<T> {
  data?: T;
  error?: string;
  status: number;
}

// Helper function to get auth header
const getAuthHeader = () => {
  const token = useAuthStore.getState().token;
  return {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json',
  };
};

// PHASE 1: RISK IDENTIFICATION
export interface IdentifyRiskInput {
  risk_id: string;
  risk_category: string;
  risk_context: string;
  identification_method: string;
}

export const identifyRisk = async (input: IdentifyRiskInput): Promise<ApiResponse<any>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/risk-management/identify`, {
      method: 'POST',
      headers: getAuthHeader(),
      body: JSON.stringify(input),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to identify risk' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

// PHASE 2: RISK ANALYSIS
export interface AnalyzeRiskInput {
  risk_id: string;
  probability_score: number;
  impact_score: number;
  root_cause: string;
  affected_areas: string[];
  analysis_methodology: string;
}

export const analyzeRisk = async (input: AnalyzeRiskInput): Promise<ApiResponse<any>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/risk-management/analyze`, {
      method: 'POST',
      headers: getAuthHeader(),
      body: JSON.stringify(input),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to analyze risk' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

// PHASE 3: RISK TREATMENT
export interface TreatRiskInput {
  risk_id: string;
  treatment_strategy: 'Mitigate' | 'Avoid' | 'Transfer' | 'Accept' | 'Enhance';
  treatment_description: string;
  treatment_plan: string;
  responsible_owner: string;
  estimated_budget: string;
  estimated_timeline: string;
}

export const treatRisk = async (input: TreatRiskInput): Promise<ApiResponse<any>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/risk-management/treat`, {
      method: 'POST',
      headers: getAuthHeader(),
      body: JSON.stringify(input),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to create treatment plan' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

// PHASE 4: RISK MONITORING
export interface MonitorRiskInput {
  risk_id: string;
  monitoring_type: string;
  current_status: string;
  control_effectiveness: number;
  monitoring_notes: string;
}

export const monitorRisk = async (input: MonitorRiskInput): Promise<ApiResponse<any>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/risk-management/monitor`, {
      method: 'POST',
      headers: getAuthHeader(),
      body: JSON.stringify(input),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to monitor risk' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

// PHASE 5: RISK REVIEW
export interface ReviewRiskInput {
  risk_id: string;
  review_type: string;
  review_date: string;
  findings: string;
  effectiveness_rating: number;
}

export const reviewRisk = async (input: ReviewRiskInput): Promise<ApiResponse<any>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/risk-management/review`, {
      method: 'POST',
      headers: getAuthHeader(),
      body: JSON.stringify(input),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to review risk' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

// PHASE 6: RISK COMMUNICATION
export interface CommunicateRiskInput {
  risk_id: string;
  communication_type: string;
  target_audience: string;
  communication_content: string;
  scheduled_date: string;
}

export const communicateRisk = async (input: CommunicateRiskInput): Promise<ApiResponse<any>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/risk-management/communicate`, {
      method: 'POST',
      headers: getAuthHeader(),
      body: JSON.stringify(input),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to communicate risk' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

// GET endpoints for fetching data
export const getRiskRegister = async (tenantId: string): Promise<ApiResponse<any>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/risk-management/register/${tenantId}`, {
      method: 'GET',
      headers: getAuthHeader(),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to fetch risk register' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

export const getRiskTreatments = async (tenantId: string): Promise<ApiResponse<any>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/risk-management/treatments/${tenantId}`, {
      method: 'GET',
      headers: getAuthHeader(),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to fetch treatments' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

export const getRiskDecisions = async (tenantId: string): Promise<ApiResponse<any>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/risk-management/decisions/${tenantId}`, {
      method: 'GET',
      headers: getAuthHeader(),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to fetch decisions' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};

export const getComplianceReports = async (tenantId: string): Promise<ApiResponse<any>> => {
  try {
    const response = await fetch(`${API_BASE_URL}/risk-management/compliance/${tenantId}`, {
      method: 'GET',
      headers: getAuthHeader(),
    });

    const data = await response.json();
    return {
      data,
      status: response.status,
      error: !response.ok ? data.error || 'Failed to fetch compliance reports' : undefined,
    };
  } catch (error) {
    return {
      status: 500,
      error: error instanceof Error ? error.message : 'Network error',
    };
  }
};
