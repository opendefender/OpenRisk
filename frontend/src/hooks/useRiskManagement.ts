import { useState, useCallback, useEffect } from 'react';
import {
  getRiskRegister,
  getRiskTreatments,
  getRiskDecisions,
  getComplianceReports,
  identifyRisk,
  analyzeRisk,
  treatRisk,
  monitorRisk,
  reviewRisk,
  communicateRisk,
  type IdentifyRiskInput,
  type AnalyzeRiskInput,
  type TreatRiskInput,
  type MonitorRiskInput,
  type ReviewRiskInput,
  type CommunicateRiskInput,
} from '../api/riskManagementService';
import { useAuthStore } from './useAuthStore';

interface UsePhaseState<T> {
  data: T[];
  isLoading: boolean;
  error: string | null;
  isSubmitting: boolean;
}

// Hook for Risk Identification
export const useRiskIdentification = () => {
  const tenantId = useAuthStore((s) => s.user?.id || 'default');
  const [state, setState] = useState<UsePhaseState<any>>({
    data: [],
    isLoading: false,
    error: null,
    isSubmitting: false,
  });

  const fetchRisks = useCallback(async () => {
    if (!tenantId) return;

    setState((prev) => ({ ...prev, isLoading: true, error: null }));
    const result = await getRiskRegister(tenantId);

    if (result.error) {
      setState((prev) => ({ ...prev, isLoading: false, error: result.error || null }));
    } else {
      setState((prev) => ({
        ...prev,
        isLoading: false,
        error: null,
        data: result.data || [],
      }));
    }
  }, [tenantId]);

  const addRisk = useCallback(async (input: IdentifyRiskInput) => {
    setState((prev) => ({ ...prev, isSubmitting: true, error: null }));
    const result = await identifyRisk(input);

    if (result.error) {
      setState((prev) => ({ ...prev, isSubmitting: false, error: result.error || null }));
      return false;
    } else {
      setState((prev) => ({
        ...prev,
        isSubmitting: false,
        error: null,
        data: [...prev.data, result.data],
      }));
      return true;
    }
  }, []);

  useEffect(() => {
    fetchRisks();
  }, [fetchRisks]);

  return { ...state, fetchRisks, addRisk };
};

// Hook for Risk Analysis
export const useRiskAnalysis = () => {
  const tenantId = useAuthStore((s) => s.user?.id || 'default');
  const [state, setState] = useState<UsePhaseState<any>>({
    data: [],
    isLoading: false,
    error: null,
    isSubmitting: false,
  });

  const fetchAnalyses = useCallback(async () => {
    if (!tenantId) return;

    setState((prev) => ({ ...prev, isLoading: true, error: null }));
    const result = await getRiskRegister(tenantId);

    if (result.error) {
      setState((prev) => ({ ...prev, isLoading: false, error: result.error || null }));
    } else {
      setState((prev) => ({
        ...prev,
        isLoading: false,
        error: null,
        data: result.data || [],
      }));
    }
  }, [tenantId]);

  const analyzeNewRisk = useCallback(async (input: AnalyzeRiskInput) => {
    setState((prev) => ({ ...prev, isSubmitting: true, error: null }));
    const result = await analyzeRisk(input);

    if (result.error) {
      setState((prev) => ({ ...prev, isSubmitting: false, error: result.error || null }));
      return false;
    } else {
      setState((prev) => ({
        ...prev,
        isSubmitting: false,
        error: null,
        data: [...prev.data, result.data],
      }));
      return true;
    }
  }, []);

  useEffect(() => {
    fetchAnalyses();
  }, [fetchAnalyses]);

  return { ...state, fetchAnalyses, analyzeNewRisk };
};

// Hook for Risk Treatment
export const useRiskTreatment = () => {
  const tenantId = useAuthStore((s) => s.user?.id || 'default');
  const [state, setState] = useState<UsePhaseState<any>>({
    data: [],
    isLoading: false,
    error: null,
    isSubmitting: false,
  });

  const fetchTreatments = useCallback(async () => {
    if (!tenantId) return;

    setState((prev) => ({ ...prev, isLoading: true, error: null }));
    const result = await getRiskTreatments(tenantId);

    if (result.error) {
      setState((prev) => ({ ...prev, isLoading: false, error: result.error || null }));
    } else {
      setState((prev) => ({
        ...prev,
        isLoading: false,
        error: null,
        data: result.data || [],
      }));
    }
  }, [tenantId]);

  const addTreatment = useCallback(async (input: TreatRiskInput) => {
    setState((prev) => ({ ...prev, isSubmitting: true, error: null }));
    const result = await treatRisk(input);

    if (result.error) {
      setState((prev) => ({ ...prev, isSubmitting: false, error: result.error || null }));
      return false;
    } else {
      setState((prev) => ({
        ...prev,
        isSubmitting: false,
        error: null,
        data: [...prev.data, result.data],
      }));
      return true;
    }
  }, []);

  useEffect(() => {
    fetchTreatments();
  }, [fetchTreatments]);

  return { ...state, fetchTreatments, addTreatment };
};

// Hook for Risk Monitoring
export const useRiskMonitoring = () => {
  const tenantId = useAuthStore((s) => s.user?.id || 'default');
  const [state, setState] = useState<UsePhaseState<any>>({
    data: [],
    isLoading: false,
    error: null,
    isSubmitting: false,
  });

  const fetchMonitoringData = useCallback(async () => {
    if (!tenantId) return;

    setState((prev) => ({ ...prev, isLoading: true, error: null }));
    const result = await getRiskRegister(tenantId);

    if (result.error) {
      setState((prev) => ({ ...prev, isLoading: false, error: result.error || null }));
    } else {
      setState((prev) => ({
        ...prev,
        isLoading: false,
        error: null,
        data: result.data || [],
      }));
    }
  }, [tenantId]);

  const updateMonitoring = useCallback(async (input: MonitorRiskInput) => {
    setState((prev) => ({ ...prev, isSubmitting: true, error: null }));
    const result = await monitorRisk(input);

    if (result.error) {
      setState((prev) => ({ ...prev, isSubmitting: false, error: result.error || null }));
      return false;
    } else {
      setState((prev) => ({
        ...prev,
        isSubmitting: false,
        error: null,
        data: [...prev.data, result.data],
      }));
      return true;
    }
  }, []);

  useEffect(() => {
    fetchMonitoringData();
  }, [fetchMonitoringData]);

  return { ...state, fetchMonitoringData, updateMonitoring };
};

// Hook for Risk Review
export const useRiskReview = () => {
  const tenantId = useAuthStore((s) => s.user?.id || 'default');
  const [state, setState] = useState<UsePhaseState<any>>({
    data: [],
    isLoading: false,
    error: null,
    isSubmitting: false,
  });

  const fetchReviews = useCallback(async () => {
    if (!tenantId) return;

    setState((prev) => ({ ...prev, isLoading: true, error: null }));
    const result = await getRiskRegister(tenantId);

    if (result.error) {
      setState((prev) => ({ ...prev, isLoading: false, error: result.error || null }));
    } else {
      setState((prev) => ({
        ...prev,
        isLoading: false,
        error: null,
        data: result.data || [],
      }));
    }
  }, [tenantId]);

  const addReview = useCallback(async (input: ReviewRiskInput) => {
    setState((prev) => ({ ...prev, isSubmitting: true, error: null }));
    const result = await reviewRisk(input);

    if (result.error) {
      setState((prev) => ({ ...prev, isSubmitting: false, error: result.error || null }));
      return false;
    } else {
      setState((prev) => ({
        ...prev,
        isSubmitting: false,
        error: null,
        data: [...prev.data, result.data],
      }));
      return true;
    }
  }, []);

  useEffect(() => {
    fetchReviews();
  }, [fetchReviews]);

  return { ...state, fetchReviews, addReview };
};

// Hook for Risk Communication
export const useRiskCommunication = () => {
  const tenantId = useAuthStore((s) => s.user?.id || 'default');
  const [state, setState] = useState<UsePhaseState<any>>({
    data: [],
    isLoading: false,
    error: null,
    isSubmitting: false,
  });

  const fetchCommunications = useCallback(async () => {
    if (!tenantId) return;

    setState((prev) => ({ ...prev, isLoading: true, error: null }));
    const result = await getRiskRegister(tenantId);

    if (result.error) {
      setState((prev) => ({ ...prev, isLoading: false, error: result.error || null }));
    } else {
      setState((prev) => ({
        ...prev,
        isLoading: false,
        error: null,
        data: result.data || [],
      }));
    }
  }, [tenantId]);

  const addCommunication = useCallback(async (input: CommunicateRiskInput) => {
    setState((prev) => ({ ...prev, isSubmitting: true, error: null }));
    const result = await communicateRisk(input);

    if (result.error) {
      setState((prev) => ({ ...prev, isSubmitting: false, error: result.error || null }));
      return false;
    } else {
      setState((prev) => ({
        ...prev,
        isSubmitting: false,
        error: null,
        data: [...prev.data, result.data],
      }));
      return true;
    }
  }, []);

  useEffect(() => {
    fetchCommunications();
  }, [fetchCommunications]);

  return { ...state, fetchCommunications, addCommunication };
};

// Hook for Compliance
export const useRiskCompliance = () => {
  const tenantId = useAuthStore((s) => s.user?.id || 'default');
  const [state, setState] = useState<UsePhaseState<any>>({
    data: [],
    isLoading: false,
    error: null,
    isSubmitting: false,
  });

  const fetchCompliance = useCallback(async () => {
    if (!tenantId) return;

    setState((prev) => ({ ...prev, isLoading: true, error: null }));
    const result = await getComplianceReports(tenantId);

    if (result.error) {
      setState((prev) => ({ ...prev, isLoading: false, error: result.error || null }));
    } else {
      setState((prev) => ({
        ...prev,
        isLoading: false,
        error: null,
        data: result.data || [],
      }));
    }
  }, [tenantId]);

  useEffect(() => {
    fetchCompliance();
  }, [fetchCompliance]);

  return { ...state, fetchCompliance };
};

// Hook for Risk Decisions
export const useRiskDecisions = () => {
  const tenantId = useAuthStore((s) => s.user?.id || 'default');
  const [state, setState] = useState<UsePhaseState<any>>({
    data: [],
    isLoading: false,
    error: null,
    isSubmitting: false,
  });

  const fetchDecisions = useCallback(async () => {
    if (!tenantId) return;

    setState((prev) => ({ ...prev, isLoading: true, error: null }));
    const result = await getRiskDecisions(tenantId);

    if (result.error) {
      setState((prev) => ({ ...prev, isLoading: false, error: result.error || null }));
    } else {
      setState((prev) => ({
        ...prev,
        isLoading: false,
        error: null,
        data: result.data || [],
      }));
    }
  }, [tenantId]);

  useEffect(() => {
    fetchDecisions();
  }, [fetchDecisions]);

  return { ...state, fetchDecisions };
};
