import { useState } from 'react';
import { Gauge, Plus, Edit2, Trash2 } from 'lucide-react';
import { motion } from 'framer-motion';
import { Card } from '../../../components/Card';
import { Button } from '../../../components/ui/Button';

interface RiskAnalysis {
  id: string;
  riskTitle: string;
  probability: number;
  impact: number;
  riskScore: number;
  riskLevel: 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';
  rootCause: string;
  affectedAreas: string[];
  methodology: string;
  analysisDate: string;
}

export const RiskAnalysisPhase = () => {
  const [analyses, setAnalyses] = useState<RiskAnalysis[]>([
    {
      id: '1',
      riskTitle: 'Data Breach Risk',
      probability: 3,
      impact: 5,
      riskScore: 15,
      riskLevel: 'HIGH',
      rootCause: 'Weak access controls',
      affectedAreas: ['Customer Data', 'System Security'],
      methodology: 'Qualitative Assessment',
      analysisDate: '2024-02-01',
    },
    {
      id: '2',
      riskTitle: 'Compliance Gap',
      probability: 2,
      impact: 4,
      riskScore: 8,
      riskLevel: 'MEDIUM',
      rootCause: 'Process gaps',
      affectedAreas: ['Compliance', 'Operations'],
      methodology: 'Process Review',
      analysisDate: '2024-02-02',
    },
  ]);

  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formData, setFormData] = useState({
    riskTitle: '',
    probability: 3,
    impact: 3,
    rootCause: '',
    affectedAreas: '',
    methodology: 'Qualitative Assessment',
  });

  const getRiskLevel = (score: number): 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL' => {
    if (score <= 5) return 'LOW';
    if (score <= 12) return 'MEDIUM';
    if (score <= 19) return 'HIGH';
    return 'CRITICAL';
  };

  const getRiskLevelColor = (level: string) => {
    switch (level) {
      case 'CRITICAL':
        return 'bg-red-500/20 text-red-400';
      case 'HIGH':
        return 'bg-orange-500/20 text-orange-400';
      case 'MEDIUM':
        return 'bg-yellow-500/20 text-yellow-400';
      case 'LOW':
        return 'bg-green-500/20 text-green-400';
      default:
        return 'bg-zinc-700/20 text-zinc-400';
    }
  };

  const handleAddAnalysis = () => {
    if (formData.riskTitle && formData.rootCause && formData.affectedAreas) {
      const riskScore = formData.probability * formData.impact;
      const riskLevel = getRiskLevel(riskScore);

      const newAnalysis: RiskAnalysis = {
        id: editingId || Date.now().toString(),
        riskTitle: formData.riskTitle,
        probability: formData.probability,
        impact: formData.impact,
        riskScore,
        riskLevel,
        rootCause: formData.rootCause,
        affectedAreas: formData.affectedAreas.split(',').map((a) => a.trim()),
        methodology: formData.methodology,
        analysisDate: new Date().toISOString().split('T')[0],
      };

      if (editingId) {
        setAnalyses(analyses.map((a) => (a.id === editingId ? newAnalysis : a)));
        setEditingId(null);
      } else {
        setAnalyses([...analyses, newAnalysis]);
      }

      setFormData({
        riskTitle: '',
        probability: 3,
        impact: 3,
        rootCause: '',
        affectedAreas: '',
        methodology: 'Qualitative Assessment',
      });
      setShowForm(false);
    }
  };

  const handleEdit = (analysis: RiskAnalysis) => {
    setFormData({
      riskTitle: analysis.riskTitle,
      probability: analysis.probability,
      impact: analysis.impact,
      rootCause: analysis.rootCause,
      affectedAreas: analysis.affectedAreas.join(', '),
      methodology: analysis.methodology,
    });
    setEditingId(analysis.id);
    setShowForm(true);
  };

  const handleDelete = (id: string) => {
    setAnalyses(analyses.filter((item) => item.id !== id));
  };

  return (
    <div className="space-y-6">
      {/* Phase Info */}
      <Card>
        <div className="p-6">
          <div className="flex items-start gap-4">
            <Gauge size={32} className="text-blue-500 flex-shrink-0 mt-1" />
            <div>
              <h3 className="text-xl font-bold mb-2">Phase 2: Risk Analysis</h3>
              <p className="text-zinc-400 mb-4">
                Analyze identified risks through probability and impact assessment. Calculate risk scores and determine risk levels to prioritize management efforts.
              </p>
              <div className="grid grid-cols-4 gap-4 text-sm">
                <div>
                  <p className="text-zinc-400">Total Analyzed</p>
                  <p className="text-2xl font-bold">{analyses.length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Critical</p>
                  <p className="text-2xl font-bold text-red-400">{analyses.filter((a) => a.riskLevel === 'CRITICAL').length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">High</p>
                  <p className="text-2xl font-bold text-orange-400">{analyses.filter((a) => a.riskLevel === 'HIGH').length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Avg. Score</p>
                  <p className="text-2xl font-bold">
                    {analyses.length > 0
                      ? (analyses.reduce((sum, a) => sum + a.riskScore, 0) / analyses.length).toFixed(1)
                      : 0}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Card>

      {/* Analysis Form */}
      {showForm && (
        <Card>
          <div className="p-6">
            <h3 className="text-lg font-bold mb-4">{editingId ? 'Edit Risk Analysis' : 'Add Risk Analysis'}</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-2">Risk Title *</label>
                <input
                  type="text"
                  value={formData.riskTitle}
                  onChange={(e) => setFormData({ ...formData, riskTitle: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500"
                  placeholder="e.g., Data Breach Risk"
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium mb-2">
                    Probability (1-5) *
                  </label>
                  <div className="flex items-center gap-4">
                    <input
                      type="range"
                      min="1"
                      max="5"
                      value={formData.probability}
                      onChange={(e) => setFormData({ ...formData, probability: parseInt(e.target.value) })}
                      className="flex-1"
                    />
                    <span className="text-lg font-bold w-8">{formData.probability}</span>
                  </div>
                </div>

                <div>
                  <label className="block text-sm font-medium mb-2">
                    Impact (1-5) *
                  </label>
                  <div className="flex items-center gap-4">
                    <input
                      type="range"
                      min="1"
                      max="5"
                      value={formData.impact}
                      onChange={(e) => setFormData({ ...formData, impact: parseInt(e.target.value) })}
                      className="flex-1"
                    />
                    <span className="text-lg font-bold w-8">{formData.impact}</span>
                  </div>
                </div>
              </div>

              <div className="bg-zinc-700/50 p-4 rounded text-sm">
                <p className="text-zinc-400 mb-2">Calculated Risk Score</p>
                <p className="text-3xl font-bold">
                  {formData.probability * formData.impact}
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Root Cause *</label>
                <textarea
                  value={formData.rootCause}
                  onChange={(e) => setFormData({ ...formData, rootCause: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500 h-20"
                  placeholder="Describe the root cause..."
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Affected Areas (comma-separated) *</label>
                <input
                  type="text"
                  value={formData.affectedAreas}
                  onChange={(e) => setFormData({ ...formData, affectedAreas: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500"
                  placeholder="e.g., Customer Data, System Security"
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Methodology *</label>
                <select
                  value={formData.methodology}
                  onChange={(e) => setFormData({ ...formData, methodology: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                >
                  <option>Qualitative Assessment</option>
                  <option>Quantitative Analysis</option>
                  <option>Process Review</option>
                  <option>Historical Data</option>
                </select>
              </div>

              <div className="flex gap-2 pt-4">
                <Button
                  onClick={handleAddAnalysis}
                  className="bg-blue-600 hover:bg-blue-700 text-white"
                >
                  {editingId ? 'Update Analysis' : 'Add Analysis'}
                </Button>
                <Button
                  onClick={() => {
                    setShowForm(false);
                    setEditingId(null);
                    setFormData({
                      riskTitle: '',
                      probability: 3,
                      impact: 3,
                      rootCause: '',
                      affectedAreas: '',
                      methodology: 'Qualitative Assessment',
                    });
                  }}
                  className="bg-zinc-700 hover:bg-zinc-600 text-white"
                >
                  Cancel
                </Button>
              </div>
            </div>
          </div>
        </Card>
      )}

      {/* Analysis Results */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-bold">Risk Analyses</h3>
          {!showForm && (
            <Button
              onClick={() => setShowForm(true)}
              className="bg-blue-600 hover:bg-blue-700 text-white flex items-center gap-2"
            >
              <Plus size={18} />
              Add Analysis
            </Button>
          )}
        </div>

        {analyses.length === 0 ? (
          <Card>
            <div className="p-12 text-center">
              <Gauge size={48} className="mx-auto mb-4 text-zinc-500" />
              <p className="text-zinc-400">No risk analyses yet</p>
            </div>
          </Card>
        ) : (
          analyses.map((analysis, idx) => (
            <motion.div
              key={analysis.id}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.1 }}
            >
              <Card>
                <div className="p-6">
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-3">
                        <h3 className="text-lg font-bold">{analysis.riskTitle}</h3>
                        <span className={`text-xs px-2 py-1 rounded font-semibold ${getRiskLevelColor(analysis.riskLevel)}`}>
                          {analysis.riskLevel}
                        </span>
                      </div>

                      <div className="grid grid-cols-6 gap-4 mb-4">
                        <div>
                          <p className="text-xs text-zinc-500">Probability</p>
                          <p className="text-2xl font-bold">{analysis.probability}/5</p>
                        </div>
                        <div>
                          <p className="text-xs text-zinc-500">Impact</p>
                          <p className="text-2xl font-bold">{analysis.impact}/5</p>
                        </div>
                        <div className="col-span-2">
                          <p className="text-xs text-zinc-500">Risk Score</p>
                          <p className="text-2xl font-bold">{analysis.riskScore}</p>
                        </div>
                        <div className="col-span-2">
                          <p className="text-xs text-zinc-500">Analysis Date</p>
                          <p className="text-sm font-medium">{analysis.analysisDate}</p>
                        </div>
                      </div>

                      <div className="mb-3">
                        <p className="text-xs text-zinc-500 mb-1">Root Cause</p>
                        <p className="text-sm">{analysis.rootCause}</p>
                      </div>

                      <div className="mb-3">
                        <p className="text-xs text-zinc-500 mb-1">Affected Areas</p>
                        <div className="flex flex-wrap gap-2">
                          {analysis.affectedAreas.map((area, i) => (
                            <span key={i} className="text-xs bg-zinc-700 px-2 py-1 rounded">
                              {area}
                            </span>
                          ))}
                        </div>
                      </div>

                      <p className="text-xs text-zinc-500">
                        Methodology: <span className="text-zinc-300">{analysis.methodology}</span>
                      </p>
                    </div>

                    <div className="flex gap-2 ml-4">
                      <button
                        onClick={() => handleEdit(analysis)}
                        className="text-zinc-400 hover:text-blue-500 transition-colors p-2"
                      >
                        <Edit2 size={20} />
                      </button>
                      <button
                        onClick={() => handleDelete(analysis.id)}
                        className="text-zinc-400 hover:text-red-500 transition-colors p-2"
                      >
                        <Trash2 size={20} />
                      </button>
                    </div>
                  </div>
                </div>
              </Card>
            </motion.div>
          ))
        )}
      </div>
    </div>
  );
};
