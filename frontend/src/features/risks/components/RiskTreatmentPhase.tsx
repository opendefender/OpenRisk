import { useState } from 'react';
import { Zap, Plus, Edit2, Trash2, Loader2 } from 'lucide-react';
import { motion } from 'framer-motion';
import { Card } from '../../../components/Card';
import { Button } from '../../../components/ui/Button';
import { useRiskTreatment } from '../../../hooks/useRiskManagement';
import { toast } from 'sonner';

interface RiskTreatment {
  id: string;
  riskTitle: string;
  strategy: 'Mitigate' | 'Avoid' | 'Transfer' | 'Accept' | 'Enhance';
  description: string;
  actionPlan: string;
  owner: string;
  budget: string;
  timeline: string;
  status: 'planned' | 'in-progress' | 'completed' | 'on-hold';
  effectiveness: number;
}

export const RiskTreatmentPhase = () => {
  const { data: treatments, isLoading, error, isSubmitting, addTreatment } = useRiskTreatment();
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formData, setFormData] = useState<{
    riskTitle: string;
    strategy: 'Mitigate' | 'Avoid' | 'Transfer' | 'Accept' | 'Enhance';
    description: string;
    actionPlan: string;
    owner: string;
    budget: string;
    timeline: string;
    status: 'planned' | 'in-progress' | 'completed' | 'on-hold';
    effectiveness: number;
  }>({
    riskTitle: '',
    strategy: 'Mitigate',
    description: '',
    actionPlan: '',
    owner: '',
    budget: '',
    timeline: '',
    status: 'planned',
    effectiveness: 0,
  });

  const strategies = ['Mitigate', 'Avoid', 'Transfer', 'Accept', 'Enhance'] as const;
  const statuses = ['planned', 'in-progress', 'completed', 'on-hold'] as const;

  const getStrategyColor = (strategy: string) => {
    switch (strategy) {
      case 'Mitigate':
        return 'bg-blue-500/20 text-blue-400';
      case 'Avoid':
        return 'bg-red-500/20 text-red-400';
      case 'Transfer':
        return 'bg-yellow-500/20 text-yellow-400';
      case 'Accept':
        return 'bg-zinc-500/20 text-zinc-400';
      case 'Enhance':
        return 'bg-green-500/20 text-green-400';
      default:
        return 'bg-zinc-500/20 text-zinc-400';
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'bg-green-500/20 text-green-400';
      case 'in-progress':
        return 'bg-blue-500/20 text-blue-400';
      case 'planned':
        return 'bg-yellow-500/20 text-yellow-400';
      case 'on-hold':
        return 'bg-red-500/20 text-red-400';
      default:
        return 'bg-zinc-500/20 text-zinc-400';
    }
  };

  const handleAddTreatment = async () => {
    if (formData.riskTitle && formData.actionPlan && formData.owner) {
      const success = await addTreatment({
        risk_id: editingId || Date.now().toString(),
        treatment_strategy: formData.strategy,
        treatment_description: formData.description,
        treatment_plan: formData.actionPlan,
        responsible_owner: formData.owner,
        estimated_budget: formData.budget,
        estimated_timeline: formData.timeline,
      });

      if (success) {
        toast.success('Risk treatment saved successfully');
        setFormData({
          riskTitle: '',
          strategy: 'Mitigate',
          description: '',
          actionPlan: '',
          owner: '',
          budget: '',
          timeline: '',
          status: 'planned',
          effectiveness: 0,
        });
        setShowForm(false);
        setEditingId(null);
      } else {
        toast.error('Failed to save risk treatment');
      }
    } else {
      toast.error('Please fill in all required fields');
    }
  };

  const handleEdit = (treatment: RiskTreatment) => {
    setFormData({
      riskTitle: treatment.riskTitle,
      strategy: treatment.strategy,
      description: treatment.description,
      actionPlan: treatment.actionPlan,
      owner: treatment.owner,
      budget: treatment.budget,
      timeline: treatment.timeline,
      status: treatment.status,
      effectiveness: treatment.effectiveness,
    });
    setEditingId(treatment.id);
    setShowForm(true);
  };

  const handleDelete = async (id: string) => {
    toast.info('Delete functionality coming soon');
  };

  return (
    <div className="space-y-6">
      {/* Phase Info */}
      <Card>
        <div className="p-6">
          <div className="flex items-start gap-4">
            <Zap size={32} className="text-yellow-500 flex-shrink-0 mt-1" />
            <div>
              <h3 className="text-xl font-bold mb-2">Phase 3: Risk Treatment</h3>
              <p className="text-zinc-400 mb-4">
                Develop and implement risk treatment strategies. Plan mitigation actions, resource allocation, and monitor treatment effectiveness.
              </p>
              <div className="grid grid-cols-5 gap-4 text-sm">
                <div>
                  <p className="text-zinc-400">Total Treatments</p>
                  <p className="text-2xl font-bold">{isLoading ? '...' : treatments.length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">In Progress</p>
                  <p className="text-2xl font-bold text-blue-400">
                    {isLoading ? '...' : treatments.filter((t) => t.status === 'in-progress').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Completed</p>
                  <p className="text-2xl font-bold text-green-400">
                    {isLoading ? '...' : treatments.filter((t) => t.status === 'completed').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Planned</p>
                  <p className="text-2xl font-bold text-yellow-400">
                    {isLoading ? '...' : treatments.filter((t) => t.status === 'planned').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Avg. Effectiveness</p>
                  <p className="text-2xl font-bold">
                    {isLoading
                      ? '...'
                      : treatments.length > 0
                        ? (treatments.reduce((sum, t) => sum + t.effectiveness, 0) / treatments.length).toFixed(0)
                        : 0}
                    %
                  </p>
                </div>
              </div>
              {error && <p className="text-red-400 text-sm mt-3">{error}</p>}
            </div>
          </div>
        </div>
      </Card>

      {/* Treatment Form */}
      {showForm && (
        <Card>
          <div className="p-6">
            <h3 className="text-lg font-bold mb-4">{editingId ? 'Edit Risk Treatment' : 'Add Risk Treatment'}</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-2">Risk Title *</label>
                <input
                  type="text"
                  value={formData.riskTitle}
                  onChange={(e) => setFormData({ ...formData, riskTitle: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500"
                  placeholder="e.g., Data Breach Risk"
                  disabled={isSubmitting}
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Strategy *</label>
                <select
                  value={formData.strategy}
                  onChange={(e) => setFormData({ ...formData, strategy: e.target.value as any })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                  disabled={isSubmitting}
                >
                  {strategies.map((s) => (
                    <option key={s} value={s}>
                      {s}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Description</label>
                <textarea
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500 h-20"
                  placeholder="Describe the treatment approach..."
                  disabled={isSubmitting}
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Action Plan *</label>
                <textarea
                  value={formData.actionPlan}
                  onChange={(e) => setFormData({ ...formData, actionPlan: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500 h-20"
                  placeholder="Detail the action plan..."
                  disabled={isSubmitting}
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium mb-2">Owner *</label>
                  <input
                    type="text"
                    value={formData.owner}
                    onChange={(e) => setFormData({ ...formData, owner: e.target.value })}
                    className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500"
                    placeholder="e.g., Security Team"
                    disabled={isSubmitting}
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium mb-2">Budget</label>
                  <input
                    type="text"
                    value={formData.budget}
                    onChange={(e) => setFormData({ ...formData, budget: e.target.value })}
                    className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500"
                    placeholder="e.g., $150,000"
                    disabled={isSubmitting}
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium mb-2">Timeline</label>
                  <input
                    type="text"
                    value={formData.timeline}
                    onChange={(e) => setFormData({ ...formData, timeline: e.target.value })}
                    className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500"
                    placeholder="e.g., 3 months"
                    disabled={isSubmitting}
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium mb-2">Status</label>
                  <select
                    value={formData.status}
                    onChange={(e) => setFormData({ ...formData, status: e.target.value as any })}
                    className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                    disabled={isSubmitting}
                  >
                    {statuses.map((s) => (
                      <option key={s} value={s}>
                        {s}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Effectiveness (%)</label>
                <div className="flex items-center gap-4">
                  <input
                    type="range"
                    min="0"
                    max="100"
                    value={formData.effectiveness}
                    onChange={(e) => setFormData({ ...formData, effectiveness: parseInt(e.target.value) })}
                    className="flex-1"
                    disabled={isSubmitting}
                  />
                  <span className="text-lg font-bold w-12">{formData.effectiveness}%</span>
                </div>
              </div>

              <div className="flex gap-2 pt-4">
                <Button
                  onClick={handleAddTreatment}
                  disabled={isSubmitting}
                  className="bg-blue-600 hover:bg-blue-700 text-white flex items-center gap-2 disabled:opacity-50"
                >
                  {isSubmitting && <Loader2 size={16} className="animate-spin" />}
                  {editingId ? 'Update Treatment' : 'Add Treatment'}
                </Button>
                <Button
                  onClick={() => {
                    setShowForm(false);
                    setEditingId(null);
                    setFormData({
                      riskTitle: '',
                      strategy: 'Mitigate',
                      description: '',
                      actionPlan: '',
                      owner: '',
                      budget: '',
                      timeline: '',
                      status: 'planned',
                      effectiveness: 0,
                    });
                  }}
                  disabled={isSubmitting}
                  className="bg-zinc-700 hover:bg-zinc-600 text-white disabled:opacity-50"
                >
                  Cancel
                </Button>
              </div>
            </div>
          </div>
        </Card>
      )}

      {/* Treatments List */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-bold">Risk Treatments</h3>
          {!showForm && (
            <Button
              onClick={() => setShowForm(true)}
              className="bg-blue-600 hover:bg-blue-700 text-white flex items-center gap-2"
            >
              <Plus size={18} />
              Add Treatment
            </Button>
          )}
        </div>

        {isLoading && (
          <Card>
            <div className="p-12 text-center">
              <Loader2 size={48} className="mx-auto mb-4 text-zinc-500 animate-spin" />
              <p className="text-zinc-400">Loading risk treatments...</p>
            </div>
          </Card>
        )}

        {!isLoading && treatments.length === 0 && (
          <Card>
            <div className="p-12 text-center">
              <Zap size={48} className="mx-auto mb-4 text-zinc-500" />
              <p className="text-zinc-400">No risk treatments yet</p>
            </div>
          </Card>
        )}

        {!isLoading &&
          treatments.map((treatment, idx) => (
            <motion.div
              key={treatment.id}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.1 }}
            >
              <Card>
                <div className="p-6">
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-3">
                        <h3 className="text-lg font-bold">{treatment.riskTitle}</h3>
                        <span className={`text-xs px-2 py-1 rounded font-semibold ${getStrategyColor(treatment.strategy)}`}>
                          {treatment.strategy}
                        </span>
                        <span className={`text-xs px-2 py-1 rounded font-semibold ${getStatusColor(treatment.status)}`}>
                          {treatment.status}
                        </span>
                      </div>

                      <div className="grid grid-cols-4 gap-4 mb-4">
                        <div>
                          <p className="text-xs text-zinc-500">Owner</p>
                          <p className="text-sm font-medium">{treatment.owner}</p>
                        </div>
                        <div>
                          <p className="text-xs text-zinc-500">Budget</p>
                          <p className="text-sm font-medium">{treatment.budget}</p>
                        </div>
                        <div>
                          <p className="text-xs text-zinc-500">Timeline</p>
                          <p className="text-sm font-medium">{treatment.timeline}</p>
                        </div>
                        <div>
                          <p className="text-xs text-zinc-500">Effectiveness</p>
                          <p className="text-sm font-medium">{treatment.effectiveness}%</p>
                        </div>
                      </div>

                      {treatment.description && (
                        <div className="mb-3">
                          <p className="text-xs text-zinc-500 mb-1">Description</p>
                          <p className="text-sm">{treatment.description}</p>
                        </div>
                      )}

                      <div>
                        <p className="text-xs text-zinc-500 mb-1">Action Plan</p>
                        <p className="text-sm">{treatment.actionPlan}</p>
                      </div>
                    </div>

                    <div className="flex gap-2 ml-4">
                      <button
                        onClick={() => handleEdit(treatment)}
                        className="text-zinc-400 hover:text-blue-500 transition-colors p-2"
                      >
                        <Edit2 size={20} />
                      </button>
                      <button
                        onClick={() => handleDelete(treatment.id)}
                        className="text-zinc-400 hover:text-red-500 transition-colors p-2"
                      >
                        <Trash2 size={20} />
                      </button>
                    </div>
                  </div>
                </div>
              </Card>
            </motion.div>
          ))}
      </div>
    </div>
  );
};
