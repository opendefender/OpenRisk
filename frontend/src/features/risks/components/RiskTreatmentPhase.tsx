import { useState } from 'react';
import { Zap, Plus, Edit2, Trash2 } from 'lucide-react';
import { motion } from 'framer-motion';
import { Card } from '../../../components/Card';
import { Button } from '../../../components/ui/Button';

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
  const [treatments, setTreatments] = useState<RiskTreatment[]>([
    {
      id: '1',
      riskTitle: 'Data Breach Risk',
      strategy: 'Mitigate',
      description: 'Implement enhanced access controls and encryption',
      actionPlan: 'Deploy multi-factor authentication, implement data encryption',
      owner: 'Security Team',
      budget: '$150,000',
      timeline: '3 months',
      status: 'in-progress',
      effectiveness: 75,
    },
  ]);

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

  const handleAddTreatment = () => {
    if (formData.riskTitle && formData.actionPlan && formData.owner) {
      const newTreatment: RiskTreatment = {
        id: editingId || Date.now().toString(),
        ...formData,
      };

      if (editingId) {
        setTreatments(treatments.map((t) => (t.id === editingId ? newTreatment : t)));
        setEditingId(null);
      } else {
        setTreatments([...treatments, newTreatment]);
      }

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
    }
  };

  const handleEdit = (treatment: RiskTreatment) => {
    setFormData(treatment);
    setEditingId(treatment.id);
    setShowForm(true);
  };

  const handleDelete = (id: string) => {
    setTreatments(treatments.filter((item) => item.id !== id));
  };

  const getStrategyColor = (strategy: string) => {
    switch (strategy) {
      case 'Mitigate':
        return 'bg-blue-500/20 text-blue-400';
      case 'Avoid':
        return 'bg-red-500/20 text-red-400';
      case 'Transfer':
        return 'bg-purple-500/20 text-purple-400';
      case 'Accept':
        return 'bg-yellow-500/20 text-yellow-400';
      case 'Enhance':
        return 'bg-green-500/20 text-green-400';
      default:
        return 'bg-zinc-700/20 text-zinc-400';
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'bg-green-500/20 text-green-400';
      case 'in-progress':
        return 'bg-blue-500/20 text-blue-400';
      case 'on-hold':
        return 'bg-yellow-500/20 text-yellow-400';
      case 'planned':
        return 'bg-zinc-500/20 text-zinc-400';
      default:
        return 'bg-zinc-700/20 text-zinc-400';
    }
  };

  return (
    <div className="space-y-6">
      {/* Phase Info */}
      <Card>
        <div className="p-6">
          <div className="flex items-start gap-4">
            <Zap size={32} className="text-blue-500 flex-shrink-0 mt-1" />
            <div>
              <h3 className="text-xl font-bold mb-2">Phase 3: Risk Treatment</h3>
              <p className="text-zinc-400 mb-4">
                Develop and execute treatment plans to address identified and analyzed risks using five treatment strategies: Mitigate, Avoid, Transfer, Accept, or Enhance.
              </p>
              <div className="grid grid-cols-4 gap-4 text-sm">
                <div>
                  <p className="text-zinc-400">Total Treatments</p>
                  <p className="text-2xl font-bold">{treatments.length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">In Progress</p>
                  <p className="text-2xl font-bold text-blue-400">{treatments.filter((t) => t.status === 'in-progress').length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Completed</p>
                  <p className="text-2xl font-bold text-green-400">{treatments.filter((t) => t.status === 'completed').length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Avg. Effectiveness</p>
                  <p className="text-2xl font-bold">
                    {treatments.length > 0
                      ? Math.round(treatments.reduce((sum, t) => sum + t.effectiveness, 0) / treatments.length)
                      : 0}%
                  </p>
                </div>
              </div>
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
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium mb-2">Treatment Strategy *</label>
                  <select
                    value={formData.strategy}
                    onChange={(e) => setFormData({ ...formData, strategy: e.target.value as any })}
                    className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                  >
                    {strategies.map((s) => (
                      <option key={s} value={s}>
                        {s}
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-sm font-medium mb-2">Status *</label>
                  <select
                    value={formData.status}
                    onChange={(e) => setFormData({ ...formData, status: e.target.value as any })}
                    className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                  >
                    {statuses.map((s) => (
                      <option key={s} value={s}>
                        {s.charAt(0).toUpperCase() + s.slice(1)}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Description *</label>
                <textarea
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500 h-20"
                  placeholder="Describe the treatment approach..."
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Action Plan *</label>
                <textarea
                  value={formData.actionPlan}
                  onChange={(e) => setFormData({ ...formData, actionPlan: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500 h-20"
                  placeholder="Detail specific actions required..."
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
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium mb-2">
                    Effectiveness: {formData.effectiveness}%
                  </label>
                  <input
                    type="range"
                    min="0"
                    max="100"
                    step="5"
                    value={formData.effectiveness}
                    onChange={(e) => setFormData({ ...formData, effectiveness: parseInt(e.target.value) })}
                    className="w-full"
                  />
                </div>
              </div>

              <div className="flex gap-2 pt-4">
                <Button
                  onClick={handleAddTreatment}
                  className="bg-blue-600 hover:bg-blue-700 text-white"
                >
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
                  className="bg-zinc-700 hover:bg-zinc-600 text-white"
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

        {treatments.length === 0 ? (
          <Card>
            <div className="p-12 text-center">
              <Zap size={48} className="mx-auto mb-4 text-zinc-500" />
              <p className="text-zinc-400">No treatments defined yet</p>
            </div>
          </Card>
        ) : (
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
                          {treatment.status.charAt(0).toUpperCase() + treatment.status.slice(1)}
                        </span>
                      </div>

                      <p className="text-sm text-zinc-400 mb-3">{treatment.description}</p>

                      <div className="grid grid-cols-4 gap-4 mb-3 text-sm">
                        <div>
                          <p className="text-zinc-500">Owner</p>
                          <p className="font-medium">{treatment.owner}</p>
                        </div>
                        <div>
                          <p className="text-zinc-500">Budget</p>
                          <p className="font-medium">{treatment.budget || 'N/A'}</p>
                        </div>
                        <div>
                          <p className="text-zinc-500">Timeline</p>
                          <p className="font-medium">{treatment.timeline || 'N/A'}</p>
                        </div>
                        <div>
                          <p className="text-zinc-500">Effectiveness</p>
                          <p className="font-medium">{treatment.effectiveness}%</p>
                        </div>
                      </div>

                      <div className="mb-3">
                        <p className="text-xs text-zinc-500 mb-1">Action Plan</p>
                        <p className="text-sm">{treatment.actionPlan}</p>
                      </div>

                      <div className="w-full bg-zinc-700 rounded-full h-2">
                        <div
                          className="h-full rounded-full bg-blue-500 transition-all"
                          style={{ width: `${treatment.effectiveness}%` }}
                        />
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
          ))
        )}
      </div>
    </div>
  );
};
