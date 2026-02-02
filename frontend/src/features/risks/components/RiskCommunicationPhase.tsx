import { useState } from 'react';
import { FileText, Plus, Edit2, Trash2, Loader2 } from 'lucide-react';
import { motion } from 'framer-motion';
import { Card } from '../../../components/Card';
import { Button } from '../../../components/ui/Button';
import { useRiskCommunication } from '../../../hooks/useRiskManagement';
import { toast } from 'sonner';

interface RiskCommunication {
  id: string;
  riskTitle: string;
  communicationType: string;
  targetAudience: string;
  content: string;
  scheduledDate: string;
  status: string;
}

export const RiskCommunicationPhase = () => {
  const { data: communications, isLoading, error, isSubmitting, communicateRisk } = useRiskCommunication();
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formData, setFormData] = useState({
    riskTitle: '',
    communicationType: 'Email',
    targetAudience: 'Risk Team',
    content: '',
  });

  const communicationTypes = ['Email', 'Report', 'Presentation', 'Newsletter', 'Alert'];
  const audiences = ['Risk Team', 'Executive Leadership', 'Board of Directors', 'All Staff', 'Specific Department'];

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'Published':
        return 'bg-green-500/20 text-green-400';
      case 'Sent':
        return 'bg-blue-500/20 text-blue-400';
      case 'Scheduled':
        return 'bg-yellow-500/20 text-yellow-400';
      case 'Draft':
        return 'bg-zinc-500/20 text-zinc-400';
      default:
        return 'bg-zinc-500/20 text-zinc-400';
    }
  };

  const handleAddCommunication = async () => {
    if (formData.riskTitle && formData.content && formData.targetAudience) {
      const success = await communicateRisk({
        risk_id: editingId || Date.now().toString(),
        communication_type: formData.communicationType,
        target_audience: formData.targetAudience,
        communication_content: formData.content,
        scheduled_date: new Date().toISOString().split('T')[0],
      });

      if (success) {
        toast.success('Risk communication saved successfully');
        setFormData({
          riskTitle: '',
          communicationType: 'Email',
          targetAudience: 'Risk Team',
          content: '',
        });
        setShowForm(false);
        setEditingId(null);
      } else {
        toast.error('Failed to save risk communication');
      }
    } else {
      toast.error('Please fill in all required fields');
    }
  };

  return (
    <div className="space-y-6">
      {/* Phase Info */}
      <Card>
        <div className="p-6">
          <div className="flex items-start gap-4">
            <FileText size={32} className="text-blue-500 flex-shrink-0 mt-1" />
            <div>
              <h3 className="text-xl font-bold mb-2">Phase 6: Risk Communication</h3>
              <p className="text-zinc-400 mb-4">
                Communicate risk information, management strategies, and outcomes to relevant stakeholders. Ensure transparency, accountability, and informed decision-making across the organization.
              </p>
              <div className="grid grid-cols-4 gap-4 text-sm">
                <div>
                  <p className="text-zinc-400">Total Communications</p>
                  <p className="text-2xl font-bold">{isLoading ? '...' : communications.length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Published</p>
                  <p className="text-2xl font-bold text-green-400">
                    {isLoading ? '...' : communications.filter((c) => c.status === 'Published').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Scheduled</p>
                  <p className="text-2xl font-bold text-yellow-400">
                    {isLoading ? '...' : communications.filter((c) => c.status === 'Scheduled').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Recent</p>
                  <p className="text-2xl font-bold text-blue-400">{isLoading ? '...' : communications.length}</p>
                </div>
              </div>
              {error && <p className="text-red-400 text-sm mt-3">{error}</p>}
            </div>
          </div>
        </div>
      </Card>

      {/* Communication Form */}
      {showForm && (
        <Card>
          <div className="p-6">
            <h3 className="text-lg font-bold mb-4">{editingId ? 'Edit Communication' : 'Add Communication'}</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-2">Risk Title *</label>
                <input
                  type="text"
                  value={formData.riskTitle}
                  onChange={(e) => setFormData({ ...formData, riskTitle: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500"
                  placeholder="e.g., Risk Register Report"
                  disabled={isSubmitting}
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium mb-2">Communication Type *</label>
                  <select
                    value={formData.communicationType}
                    onChange={(e) => setFormData({ ...formData, communicationType: e.target.value })}
                    className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                    disabled={isSubmitting}
                  >
                    {communicationTypes.map((t) => (
                      <option key={t} value={t}>
                        {t}
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-sm font-medium mb-2">Target Audience *</label>
                  <select
                    value={formData.targetAudience}
                    onChange={(e) => setFormData({ ...formData, targetAudience: e.target.value })}
                    className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                    disabled={isSubmitting}
                  >
                    {audiences.map((a) => (
                      <option key={a} value={a}>
                        {a}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Communication Content *</label>
                <textarea
                  value={formData.content}
                  onChange={(e) => setFormData({ ...formData, content: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500 h-24"
                  placeholder="Enter communication content..."
                  disabled={isSubmitting}
                />
              </div>

              <div className="flex gap-2 pt-4">
                <Button
                  onClick={handleAddCommunication}
                  disabled={isSubmitting}
                  className="bg-blue-600 hover:bg-blue-700 text-white flex items-center gap-2 disabled:opacity-50"
                >
                  {isSubmitting && <Loader2 size={16} className="animate-spin" />}
                  {editingId ? 'Update Communication' : 'Add Communication'}
                </Button>
                <Button
                  onClick={() => {
                    setShowForm(false);
                    setEditingId(null);
                    setFormData({
                      riskTitle: '',
                      communicationType: 'Email',
                      targetAudience: 'Risk Team',
                      content: '',
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

      {/* Communications List */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-bold">Risk Communications</h3>
          {!showForm && (
            <Button
              onClick={() => setShowForm(true)}
              className="bg-blue-600 hover:bg-blue-700 text-white flex items-center gap-2"
            >
              <Plus size={18} />
              Add Communication
            </Button>
          )}
        </div>

        {isLoading && (
          <Card>
            <div className="p-12 text-center">
              <Loader2 size={48} className="mx-auto mb-4 text-zinc-500 animate-spin" />
              <p className="text-zinc-400">Loading communications...</p>
            </div>
          </Card>
        )}

        {!isLoading && communications.length === 0 && (
          <Card>
            <div className="p-12 text-center">
              <FileText size={48} className="mx-auto mb-4 text-zinc-500" />
              <p className="text-zinc-400">No communications scheduled</p>
            </div>
          </Card>
        )}

        {!isLoading &&
          communications.map((comm, idx) => (
            <motion.div
              key={comm.id}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.1 }}
            >
              <Card>
                <div className="p-6">
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-3">
                        <h3 className="text-lg font-bold">{comm.riskTitle}</h3>
                        <span className={`text-xs px-2 py-1 rounded font-semibold ${getStatusColor(comm.status)}`}>
                          {comm.status}
                        </span>
                      </div>

                      <div className="grid grid-cols-4 gap-4 mb-4">
                        <div>
                          <p className="text-xs text-zinc-500">Type</p>
                          <p className="text-sm font-medium">{comm.communicationType}</p>
                        </div>
                        <div>
                          <p className="text-xs text-zinc-500">Audience</p>
                          <p className="text-sm font-medium">{comm.targetAudience}</p>
                        </div>
                        <div className="col-span-2">
                          <p className="text-xs text-zinc-500">Scheduled Date</p>
                          <p className="text-sm font-medium">{comm.scheduledDate}</p>
                        </div>
                      </div>

                      <div>
                        <p className="text-xs text-zinc-500 mb-1">Content</p>
                        <p className="text-sm">{comm.content}</p>
                      </div>
                    </div>

                    <div className="flex gap-2 ml-4">
                      <button
                        onClick={() => {
                          setFormData({
                            riskTitle: comm.riskTitle,
                            communicationType: comm.communicationType,
                            targetAudience: comm.targetAudience,
                            content: comm.content,
                          });
                          setEditingId(comm.id);
                          setShowForm(true);
                        }}
                        className="text-zinc-400 hover:text-blue-500 transition-colors p-2"
                      >
                        <Edit2 size={20} />
                      </button>
                      <button
                        onClick={() => toast.info('Delete coming soon')}
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
