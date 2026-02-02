import { useState } from 'react';
import { TrendingUp, Plus, Edit2, Trash2, Loader2 } from 'lucide-react';
import { motion } from 'framer-motion';
import { Card } from '../../../components/Card';
import { Button } from '../../../components/ui/Button';
import { useRiskMonitoring } from '../../../hooks/useRiskManagement';
import { toast } from 'sonner';

interface RiskMonitor {
  id: string;
  riskTitle: string;
  monitoringType: string;
  currentStatus: 'Green' | 'Yellow' | 'Red' | 'Unknown';
  controlEffectiveness: number;
  monitoringNotes: string;
  lastMonitorDate: string;
}

export const RiskMonitoringPhase = () => {
  const { data: monitors, isLoading, error, isSubmitting, monitorRisk } = useRiskMonitoring();
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formData, setFormData] = useState({
    riskTitle: '',
    monitoringType: 'Continuous',
    currentStatus: 'Green' as 'Green' | 'Yellow' | 'Red' | 'Unknown',
    controlEffectiveness: 75,
    monitoringNotes: '',
  });

  const monitoringTypes = ['Continuous', 'Periodic', 'Ad-hoc', 'Automated'];
  const statuses = ['Green', 'Yellow', 'Red', 'Unknown'];

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'Green':
        return 'bg-green-500/20 text-green-400';
      case 'Yellow':
        return 'bg-yellow-500/20 text-yellow-400';
      case 'Red':
        return 'bg-red-500/20 text-red-400';
      case 'Unknown':
        return 'bg-zinc-500/20 text-zinc-400';
      default:
        return 'bg-zinc-500/20 text-zinc-400';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'Green':
        return '✓';
      case 'Yellow':
        return '⚠';
      case 'Red':
        return '✕';
      case 'Unknown':
        return '?';
      default:
        return '';
    }
  };

  const handleAddMonitor = async () => {
    if (formData.riskTitle && formData.monitoringNotes) {
      const success = await monitorRisk({
        risk_id: editingId || Date.now().toString(),
        monitoring_type: formData.monitoringType,
        current_status: formData.currentStatus,
        control_effectiveness: formData.controlEffectiveness,
        monitoring_notes: formData.monitoringNotes,
      });

      if (success) {
        toast.success('Risk monitoring saved successfully');
        setFormData({
          riskTitle: '',
          monitoringType: 'Continuous',
          currentStatus: 'Green',
          controlEffectiveness: 75,
          monitoringNotes: '',
        });
        setShowForm(false);
        setEditingId(null);
      } else {
        toast.error('Failed to save risk monitoring');
      }
    } else {
      toast.error('Please fill in all required fields');
    }
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
            <TrendingUp size={32} className="text-blue-500 flex-shrink-0 mt-1" />
            <div>
              <h3 className="text-xl font-bold mb-2">Phase 4: Risk Monitoring</h3>
              <p className="text-zinc-400 mb-4">
                Continuously monitor risk levels and treatment effectiveness. Track control performance and adjust strategies as needed.
              </p>
              <div className="grid grid-cols-5 gap-4 text-sm">
                <div>
                  <p className="text-zinc-400">Monitored Risks</p>
                  <p className="text-2xl font-bold">{isLoading ? '...' : monitors.length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Green</p>
                  <p className="text-2xl font-bold text-green-400">
                    {isLoading ? '...' : monitors.filter((m) => m.currentStatus === 'Green').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Yellow</p>
                  <p className="text-2xl font-bold text-yellow-400">
                    {isLoading ? '...' : monitors.filter((m) => m.currentStatus === 'Yellow').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Red</p>
                  <p className="text-2xl font-bold text-red-400">
                    {isLoading ? '...' : monitors.filter((m) => m.currentStatus === 'Red').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Avg. Effectiveness</p>
                  <p className="text-2xl font-bold">
                    {isLoading
                      ? '...'
                      : monitors.length > 0
                        ? (monitors.reduce((sum, m) => sum + m.controlEffectiveness, 0) / monitors.length).toFixed(0)
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

      {/* Monitoring Form */}
      {showForm && (
        <Card>
          <div className="p-6">
            <h3 className="text-lg font-bold mb-4">{editingId ? 'Edit Risk Monitoring' : 'Add Risk Monitoring'}</h3>
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

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium mb-2">Monitoring Type *</label>
                  <select
                    value={formData.monitoringType}
                    onChange={(e) => setFormData({ ...formData, monitoringType: e.target.value })}
                    className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                    disabled={isSubmitting}
                  >
                    {monitoringTypes.map((t) => (
                      <option key={t} value={t}>
                        {t}
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-sm font-medium mb-2">Current Status *</label>
                  <select
                    value={formData.currentStatus}
                    onChange={(e) => setFormData({ ...formData, currentStatus: e.target.value as any })}
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
                <label className="block text-sm font-medium mb-2">Control Effectiveness (%)</label>
                <div className="flex items-center gap-4">
                  <input
                    type="range"
                    min="0"
                    max="100"
                    value={formData.controlEffectiveness}
                    onChange={(e) => setFormData({ ...formData, controlEffectiveness: parseInt(e.target.value) })}
                    className="flex-1"
                    disabled={isSubmitting}
                  />
                  <span className="text-lg font-bold w-12">{formData.controlEffectiveness}%</span>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Monitoring Notes *</label>
                <textarea
                  value={formData.monitoringNotes}
                  onChange={(e) => setFormData({ ...formData, monitoringNotes: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500 h-20"
                  placeholder="Enter monitoring observations and findings..."
                  disabled={isSubmitting}
                />
              </div>

              <div className="flex gap-2 pt-4">
                <Button
                  onClick={handleAddMonitor}
                  disabled={isSubmitting}
                  className="bg-blue-600 hover:bg-blue-700 text-white flex items-center gap-2 disabled:opacity-50"
                >
                  {isSubmitting && <Loader2 size={16} className="animate-spin" />}
                  {editingId ? 'Update Monitoring' : 'Add Monitoring'}
                </Button>
                <Button
                  onClick={() => {
                    setShowForm(false);
                    setEditingId(null);
                    setFormData({
                      riskTitle: '',
                      monitoringType: 'Continuous',
                      currentStatus: 'Green',
                      controlEffectiveness: 75,
                      monitoringNotes: '',
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

      {/* Monitoring List */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-bold">Risk Monitoring</h3>
          {!showForm && (
            <Button
              onClick={() => setShowForm(true)}
              className="bg-blue-600 hover:bg-blue-700 text-white flex items-center gap-2"
            >
              <Plus size={18} />
              Add Monitoring
            </Button>
          )}
        </div>

        {isLoading && (
          <Card>
            <div className="p-12 text-center">
              <Loader2 size={48} className="mx-auto mb-4 text-zinc-500 animate-spin" />
              <p className="text-zinc-400">Loading risk monitoring data...</p>
            </div>
          </Card>
        )}

        {!isLoading && monitors.length === 0 && (
          <Card>
            <div className="p-12 text-center">
              <TrendingUp size={48} className="mx-auto mb-4 text-zinc-500" />
              <p className="text-zinc-400">No risk monitoring records yet</p>
            </div>
          </Card>
        )}

        {!isLoading &&
          monitors.map((monitor, idx) => (
            <motion.div
              key={monitor.id}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.1 }}
            >
              <Card>
                <div className="p-6">
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-3">
                        <h3 className="text-lg font-bold">{monitor.riskTitle}</h3>
                        <span className={`text-2xl font-bold ${getStatusColor(monitor.currentStatus)}`}>
                          {getStatusIcon(monitor.currentStatus)} {monitor.currentStatus}
                        </span>
                      </div>

                      <div className="grid grid-cols-5 gap-4 mb-4">
                        <div>
                          <p className="text-xs text-zinc-500">Monitoring Type</p>
                          <p className="text-sm font-medium">{monitor.monitoringType}</p>
                        </div>
                        <div>
                          <p className="text-xs text-zinc-500">Control Effectiveness</p>
                          <p className="text-sm font-medium">{monitor.controlEffectiveness}%</p>
                        </div>
                        <div className="col-span-3">
                          <p className="text-xs text-zinc-500">Last Monitored</p>
                          <p className="text-sm font-medium">{monitor.lastMonitorDate}</p>
                        </div>
                      </div>

                      <div>
                        <p className="text-xs text-zinc-500 mb-1">Monitoring Notes</p>
                        <p className="text-sm">{monitor.monitoringNotes}</p>
                      </div>
                    </div>

                    <div className="flex gap-2 ml-4">
                      <button
                        onClick={() => {
                          setFormData({
                            riskTitle: monitor.riskTitle,
                            monitoringType: monitor.monitoringType,
                            currentStatus: monitor.currentStatus,
                            controlEffectiveness: monitor.controlEffectiveness,
                            monitoringNotes: monitor.monitoringNotes,
                          });
                          setEditingId(monitor.id);
                          setShowForm(true);
                        }}
                        className="text-zinc-400 hover:text-blue-500 transition-colors p-2"
                      >
                        <Edit2 size={20} />
                      </button>
                      <button
                        onClick={() => handleDelete(monitor.id)}
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
