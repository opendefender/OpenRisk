import { useState } from 'react';
import { AlertCircle, Plus, Trash2 } from 'lucide-react';
import { motion } from 'framer-motion';
import { Card } from '../../../components/Card';
import { Button } from '../../../components/ui/Button';

interface RiskIdentification {
  id: string;
  title: string;
  category: string;
  context: string;
  method: string;
  identifiedBy: string;
  identificationDate: string;
  status: 'draft' | 'identified' | 'pending-analysis';
}

export const RiskIdentificationPhase = () => {
  const [identifications, setIdentifications] = useState<RiskIdentification[]>([
    {
      id: '1',
      title: 'Data Breach Risk',
      category: 'Security',
      context: 'External threat actors targeting customer data',
      method: 'Threat Assessment',
      identifiedBy: 'Security Team',
      identificationDate: '2024-02-01',
      status: 'identified',
    },
    {
      id: '2',
      title: 'Compliance Gap',
      category: 'Regulatory',
      context: 'Potential non-compliance with GDPR requirements',
      method: 'Audit Finding',
      identifiedBy: 'Compliance Officer',
      identificationDate: '2024-02-02',
      status: 'pending-analysis',
    },
  ]);

  const [showForm, setShowForm] = useState(false);
  const [formData, setFormData] = useState({
    title: '',
    category: 'Security',
    context: '',
    method: 'Manual',
    identifiedBy: '',
  });

  const categories = ['Security', 'Operational', 'Financial', 'Compliance', 'Reputational', 'Strategic'];
  const methods = ['Workshop', 'Interview', 'Assessment', 'Scanning', 'Manual'];

  const handleAddIdentification = () => {
    if (formData.title && formData.context && formData.identifiedBy) {
      const newIdentification: RiskIdentification = {
        id: Date.now().toString(),
        ...formData,
        identificationDate: new Date().toISOString().split('T')[0],
        status: 'identified',
      };
      setIdentifications([...identifications, newIdentification]);
      setFormData({
        title: '',
        category: 'Security',
        context: '',
        method: 'Manual',
        identifiedBy: '',
      });
      setShowForm(false);
    }
  };

  const handleDelete = (id: string) => {
    setIdentifications(identifications.filter((item) => item.id !== id));
  };

  return (
    <div className="space-y-6">
      {/* Phase Info */}
      <Card>
        <div className="p-6">
          <div className="flex items-start gap-4">
            <AlertCircle size={32} className="text-blue-500 flex-shrink-0 mt-1" />
            <div>
              <h3 className="text-xl font-bold mb-2">Phase 1: Risk Identification</h3>
              <p className="text-zinc-400 mb-4">
                Identify and document risks within the organization's context. This includes understanding the business environment, risk sources, and potential risk events.
              </p>
              <div className="grid grid-cols-3 gap-4 text-sm">
                <div>
                  <p className="text-zinc-400">Total Identified</p>
                  <p className="text-2xl font-bold">{identifications.length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Pending Analysis</p>
                  <p className="text-2xl font-bold">{identifications.filter((i) => i.status === 'pending-analysis').length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Completion Rate</p>
                  <p className="text-2xl font-bold">100%</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Card>

      {/* Add New Identification Form */}
      {showForm && (
        <Card>
          <div className="p-6">
            <h3 className="text-lg font-bold mb-4">Add New Risk Identification</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-2">Risk Title *</label>
                <input
                  type="text"
                  value={formData.title}
                  onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500"
                  placeholder="e.g., System Outage Risk"
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium mb-2">Category *</label>
                  <select
                    value={formData.category}
                    onChange={(e) => setFormData({ ...formData, category: e.target.value })}
                    className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                  >
                    {categories.map((cat) => (
                      <option key={cat} value={cat}>
                        {cat}
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-sm font-medium mb-2">Identification Method *</label>
                  <select
                    value={formData.method}
                    onChange={(e) => setFormData({ ...formData, method: e.target.value })}
                    className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white focus:outline-none focus:border-blue-500"
                  >
                    {methods.map((method) => (
                      <option key={method} value={method}>
                        {method}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Risk Context *</label>
                <textarea
                  value={formData.context}
                  onChange={(e) => setFormData({ ...formData, context: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500 h-24"
                  placeholder="Describe the risk context and environment..."
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">Identified By *</label>
                <input
                  type="text"
                  value={formData.identifiedBy}
                  onChange={(e) => setFormData({ ...formData, identifiedBy: e.target.value })}
                  className="w-full bg-zinc-700 border border-zinc-600 rounded px-3 py-2 text-white placeholder-zinc-400 focus:outline-none focus:border-blue-500"
                  placeholder="e.g., Security Team"
                />
              </div>

              <div className="flex gap-2 pt-4">
                <Button
                  onClick={handleAddIdentification}
                  className="bg-blue-600 hover:bg-blue-700 text-white"
                >
                  Add Identification
                </Button>
                <Button
                  onClick={() => setShowForm(false)}
                  className="bg-zinc-700 hover:bg-zinc-600 text-white"
                >
                  Cancel
                </Button>
              </div>
            </div>
          </div>
        </Card>
      )}

      {/* Identified Risks */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-bold">Identified Risks</h3>
          {!showForm && (
            <Button
              onClick={() => setShowForm(true)}
              className="bg-blue-600 hover:bg-blue-700 text-white flex items-center gap-2"
            >
              <Plus size={18} />
              Add Risk
            </Button>
          )}
        </div>

        {identifications.length === 0 ? (
          <Card>
            <div className="p-12 text-center">
              <AlertCircle size={48} className="mx-auto mb-4 text-zinc-500" />
              <p className="text-zinc-400">No risks identified yet</p>
            </div>
          </Card>
        ) : (
          identifications.map((identification, idx) => (
            <motion.div
              key={identification.id}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.1 }}
            >
              <Card>
                <div className="p-6">
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-2">
                        <h3 className="text-lg font-bold">{identification.title}</h3>
                        <span
                          className={`text-xs px-2 py-1 rounded ${
                            identification.status === 'identified'
                              ? 'bg-green-500/20 text-green-400'
                              : 'bg-yellow-500/20 text-yellow-400'
                          }`}
                        >
                          {identification.status === 'identified' ? 'Identified' : 'Pending Analysis'}
                        </span>
                      </div>
                      <p className="text-sm text-zinc-400 mb-3">{identification.context}</p>
                      <div className="grid grid-cols-4 gap-4 text-sm">
                        <div>
                          <p className="text-zinc-500">Category</p>
                          <p className="font-medium">{identification.category}</p>
                        </div>
                        <div>
                          <p className="text-zinc-500">Method</p>
                          <p className="font-medium">{identification.method}</p>
                        </div>
                        <div>
                          <p className="text-zinc-500">Identified By</p>
                          <p className="font-medium">{identification.identifiedBy}</p>
                        </div>
                        <div>
                          <p className="text-zinc-500">Date</p>
                          <p className="font-medium">{identification.identificationDate}</p>
                        </div>
                      </div>
                    </div>
                    <button
                      onClick={() => handleDelete(identification.id)}
                      className="text-zinc-400 hover:text-red-500 transition-colors p-2"
                    >
                      <Trash2 size={20} />
                    </button>
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
