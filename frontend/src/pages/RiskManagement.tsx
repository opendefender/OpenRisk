import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import {
  CheckCircle2,
  AlertCircle,
  Zap,
  Gauge,
  Shield,
  FileText,
  TrendingUp,
} from 'lucide-react';
import { Card } from '../components/Card';
import { RiskIdentificationPhase } from '../features/risks/components/RiskIdentificationPhase';
import { RiskAnalysisPhase } from '../features/risks/components/RiskAnalysisPhase';
import { RiskTreatmentPhase } from '../features/risks/components/RiskTreatmentPhase';
import { RiskMonitoringPhase } from '../features/risks/components/RiskMonitoringPhase';
import { RiskReviewPhase } from '../features/risks/components/RiskReviewPhase';
import { RiskCommunicationPhase } from '../features/risks/components/RiskCommunicationPhase';
import { RiskManagementPolicy } from '../features/risks/components/RiskManagementPolicy';
import { RiskDecisionManagement } from '../features/risks/components/RiskDecisionManagement';
import { RiskAuditCompliance } from '../features/risks/components/RiskAuditCompliance';
import { toast } from 'sonner';

export interface RiskPhase {
  id: string;
  name: string;
  description: string;
  icon: React.ReactNode;
  status: 'not-started' | 'in-progress' | 'completed';
  completionPercentage: number;
}

export const RiskManagement = () => {
  const [activeTab, setActiveTab] = useState<'overview' | 'phases' | 'policy' | 'decisions' | 'audit'>('overview');
  const [phases] = useState<RiskPhase[]>([
    {
      id: 'identify',
      name: 'Identify',
      description: 'Risk identification and context assessment',
      icon: <AlertCircle size={24} />,
      status: 'in-progress',
      completionPercentage: 45,
    },
    {
      id: 'analyze',
      name: 'Analyze',
      description: 'Risk analysis and scoring',
      icon: <Gauge size={24} />,
      status: 'in-progress',
      completionPercentage: 60,
    },
    {
      id: 'treat',
      name: 'Treat',
      description: 'Risk treatment planning and execution',
      icon: <Zap size={24} />,
      status: 'not-started',
      completionPercentage: 0,
    },
    {
      id: 'monitor',
      name: 'Monitor',
      description: 'Risk monitoring and control',
      icon: <TrendingUp size={24} />,
      status: 'not-started',
      completionPercentage: 0,
    },
    {
      id: 'review',
      name: 'Review',
      description: 'Risk review and assessment',
      icon: <Shield size={24} />,
      status: 'not-started',
      completionPercentage: 0,
    },
    {
      id: 'communicate',
      name: 'Communicate',
      description: 'Risk communication and reporting',
      icon: <FileText size={24} />,
      status: 'not-started',
      completionPercentage: 0,
    },
  ]);

  const [selectedPhase, setSelectedPhase] = useState<string | null>(null);

  useEffect(() => {
    if (activeTab === 'phases' && !selectedPhase) {
      setSelectedPhase('identify');
    }
  }, [activeTab, selectedPhase]);

  const renderPhaseComponent = () => {
    switch (selectedPhase) {
      case 'identify':
        return <RiskIdentificationPhase />;
      case 'analyze':
        return <RiskAnalysisPhase />;
      case 'treat':
        return <RiskTreatmentPhase />;
      case 'monitor':
        return <RiskMonitoringPhase />;
      case 'review':
        return <RiskReviewPhase />;
      case 'communicate':
        return <RiskCommunicationPhase />;
      default:
        return null;
    }
  };

  return (
    <div className="max-w-7xl mx-auto p-6">
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-3xl font-bold mb-2">Risk Management Operating System</h1>
        <p className="text-zinc-400">ISO 31000 & NIST RMF Compliant Lifecycle Management</p>
      </div>

      {/* Tab Navigation */}
      <div className="flex gap-2 mb-6 border-b border-border">
        <button
          onClick={() => setActiveTab('overview')}
          className={`px-4 py-2 font-medium transition-colors ${
            activeTab === 'overview'
              ? 'text-blue-500 border-b-2 border-blue-500'
              : 'text-zinc-400 hover:text-zinc-300'
          }`}
        >
          Overview
        </button>
        <button
          onClick={() => setActiveTab('phases')}
          className={`px-4 py-2 font-medium transition-colors ${
            activeTab === 'phases'
              ? 'text-blue-500 border-b-2 border-blue-500'
              : 'text-zinc-400 hover:text-zinc-300'
          }`}
        >
          Lifecycle Phases
        </button>
        <button
          onClick={() => setActiveTab('policy')}
          className={`px-4 py-2 font-medium transition-colors ${
            activeTab === 'policy'
              ? 'text-blue-500 border-b-2 border-blue-500'
              : 'text-zinc-400 hover:text-zinc-300'
          }`}
        >
          Governance & Policy
        </button>
        <button
          onClick={() => setActiveTab('decisions')}
          className={`px-4 py-2 font-medium transition-colors ${
            activeTab === 'decisions'
              ? 'text-blue-500 border-b-2 border-blue-500'
              : 'text-zinc-400 hover:text-zinc-300'
          }`}
        >
          Decisions
        </button>
        <button
          onClick={() => setActiveTab('audit')}
          className={`px-4 py-2 font-medium transition-colors ${
            activeTab === 'audit'
              ? 'text-blue-500 border-b-2 border-blue-500'
              : 'text-zinc-400 hover:text-zinc-300'
          }`}
        >
          Audit & Compliance
        </button>
      </div>

      {/* Overview Tab */}
      {activeTab === 'overview' && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
          className="space-y-6"
        >
          {/* Lifecycle Overview */}
          <Card>
            <div className="p-6">
              <h2 className="text-xl font-bold mb-6">ISO 31000 / NIST RMF Lifecycle</h2>
              <div className="grid grid-cols-6 gap-4">
                {phases.map((phase, index) => (
                  <div key={phase.id} className="relative">
                    <motion.div
                      whileHover={{ scale: 1.05 }}
                      className="text-center cursor-pointer group"
                      onClick={() => {
                        setActiveTab('phases');
                        setSelectedPhase(phase.id);
                      }}
                    >
                      <div
                        className={`w-16 h-16 mx-auto mb-3 rounded-full flex items-center justify-center transition-colors ${
                          phase.status === 'completed'
                            ? 'bg-green-500/20 text-green-500'
                            : phase.status === 'in-progress'
                              ? 'bg-blue-500/20 text-blue-500'
                              : 'bg-zinc-700/50 text-zinc-400'
                        } group-hover:scale-110 transition-transform`}
                      >
                        {phase.icon}
                      </div>
                      <h3 className="font-semibold text-sm mb-1">{phase.name}</h3>
                      <p className="text-xs text-zinc-400">{phase.description}</p>

                      {/* Progress Bar */}
                      <div className="w-full bg-zinc-700 rounded-full h-1 mt-3">
                        <div
                          className={`h-full rounded-full transition-all ${
                            phase.status === 'completed'
                              ? 'bg-green-500'
                              : phase.status === 'in-progress'
                                ? 'bg-blue-500'
                                : 'bg-zinc-600'
                          }`}
                          style={{ width: `${phase.completionPercentage}%` }}
                        />
                      </div>
                      <p className="text-xs text-zinc-400 mt-1">{phase.completionPercentage}%</p>
                    </motion.div>
                    {/* Arrow connector */}
                    {index < phases.length - 1 && (
                      <div className="hidden lg:block absolute left-[85%] top-8 w-[15%]">
                        <svg className="w-full h-4" viewBox="0 0 100 40" preserveAspectRatio="none">
                          <path
                            d="M 0 20 Q 50 0 100 20"
                            stroke="currentColor"
                            strokeWidth="2"
                            fill="none"
                            className="text-zinc-600"
                          />
                          <polygon
                            points="100,20 95,17 95,23"
                            fill="currentColor"
                            className="text-zinc-600"
                          />
                        </svg>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          </Card>

          {/* Key Metrics */}
          <div className="grid grid-cols-4 gap-4">
            <Card>
              <div className="p-6">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-zinc-400 text-sm mb-2">Total Risks</p>
                    <p className="text-3xl font-bold">142</p>
                  </div>
                  <AlertCircle size={32} className="text-blue-500/20" />
                </div>
              </div>
            </Card>
            <Card>
              <div className="p-6">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-zinc-400 text-sm mb-2">Critical Risks</p>
                    <p className="text-3xl font-bold">8</p>
                  </div>
                  <Zap size={32} className="text-red-500/20" />
                </div>
              </div>
            </Card>
            <Card>
              <div className="p-6">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-zinc-400 text-sm mb-2">Treatments Active</p>
                    <p className="text-3xl font-bold">67</p>
                  </div>
                  <Shield size={32} className="text-green-500/20" />
                </div>
              </div>
            </Card>
            <Card>
              <div className="p-6">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-zinc-400 text-sm mb-2">Compliance Score</p>
                    <p className="text-3xl font-bold">94%</p>
                  </div>
                  <CheckCircle2 size={32} className="text-green-500/20" />
                </div>
              </div>
            </Card>
          </div>

          {/* Recent Activity */}
          <Card>
            <div className="p-6">
              <h3 className="text-lg font-bold mb-4">Recent Activity</h3>
              <div className="space-y-4">
                {[
                  { event: 'Risk Treatment Approved', time: '2 hours ago', type: 'success' },
                  { event: 'Critical Risk Identified', time: '4 hours ago', type: 'alert' },
                  { event: 'Compliance Review Completed', time: '1 day ago', type: 'success' },
                  { event: 'Risk Assessment Updated', time: '2 days ago', type: 'info' },
                ].map((activity, idx) => (
                  <div key={idx} className="flex items-center justify-between py-2 border-b border-border last:border-0">
                    <span className="text-sm">{activity.event}</span>
                    <span className="text-xs text-zinc-400">{activity.time}</span>
                  </div>
                ))}
              </div>
            </div>
          </Card>
        </motion.div>
      )}

      {/* Phases Tab */}
      {activeTab === 'phases' && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
          className="space-y-6"
        >
          {/* Phase Selector */}
          <div className="flex gap-2 flex-wrap">
            {phases.map((phase) => (
              <button
                key={phase.id}
                onClick={() => setSelectedPhase(phase.id)}
                className={`flex items-center gap-2 px-4 py-2 rounded transition-colors ${
                  selectedPhase === phase.id
                    ? 'bg-blue-500 text-white'
                    : 'bg-zinc-700 text-zinc-300 hover:bg-zinc-600'
                }`}
              >
                {phase.icon}
                <span>{phase.name}</span>
              </button>
            ))}
          </div>

          {/* Phase Content */}
          {renderPhaseComponent()}
        </motion.div>
      )}

      {/* Policy Tab */}
      {activeTab === 'policy' && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
        >
          <RiskManagementPolicy />
        </motion.div>
      )}

      {/* Decisions Tab */}
      {activeTab === 'decisions' && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
        >
          <RiskDecisionManagement />
        </motion.div>
      )}

      {/* Audit Tab */}
      {activeTab === 'audit' && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
        >
          <RiskAuditCompliance />
        </motion.div>
      )}
    </div>
  );
};
