import { useState } from 'react';
import { CheckCircle2, Plus, Edit2, Trash2 } from 'lucide-react';
import { motion } from 'framer-motion';
import { Card } from '../../../components/Card';
import { Button } from '../../../components/ui/Button';

interface RiskDecision {
  id: string;
  decision: string;
  relatedRisk: string;
  rationale: string;
  owner: string;
  validityPeriod: string;
  status: string;
  approvedDate: string;
}

export const RiskDecisionManagement = () => {
  const [decisions] = useState<RiskDecision[]>([
    {
      id: '1',
      decision: 'Accept risk of system outage up to 4 hours',
      relatedRisk: 'System Downtime Risk',
      rationale: 'Cost of mitigation exceeds business benefit',
      owner: 'Chief Technology Officer',
      validityPeriod: '12 months',
      status: 'Approved',
      approvedDate: '2024-01-15',
    },
    {
      id: '2',
      decision: 'Mitigate data breach risk with enhanced encryption',
      relatedRisk: 'Data Breach Risk',
      rationale: 'High impact justifies investment',
      owner: 'Chief Security Officer',
      validityPeriod: 'Ongoing',
      status: 'Approved',
      approvedDate: '2024-01-20',
    },
    {
      id: '3',
      decision: 'Transfer compliance risk through cyber insurance',
      relatedRisk: 'Regulatory Compliance Risk',
      rationale: 'External expertise reduces exposure',
      owner: 'Compliance Officer',
      validityPeriod: 'Annual renewal',
      status: 'Pending Approval',
      approvedDate: '2024-02-01',
    },
  ]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <Card>
        <div className="p-6">
          <div className="flex items-start gap-4">
            <CheckCircle2 size={32} className="text-blue-500 flex-shrink-0 mt-1" />
            <div>
              <h3 className="text-xl font-bold mb-2">Risk Decision Management</h3>
              <p className="text-zinc-400 mb-4">
                Track all risk-related decisions with full rationale documentation, approval workflows, and validity tracking for audit compliance.
              </p>
              <div className="grid grid-cols-4 gap-4 text-sm">
                <div>
                  <p className="text-zinc-400">Total Decisions</p>
                  <p className="text-2xl font-bold">{decisions.length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Approved</p>
                  <p className="text-2xl font-bold text-green-400">
                    {decisions.filter((d) => d.status === 'Approved').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Pending Review</p>
                  <p className="text-2xl font-bold text-yellow-400">
                    {decisions.filter((d) => d.status === 'Pending Approval').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Active</p>
                  <p className="text-2xl font-bold">{decisions.length}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Card>

      {/* Decisions List */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-bold">Risk Decisions</h3>
          <Button className="bg-blue-600 hover:bg-blue-700 text-white flex items-center gap-2">
            <Plus size={18} />
            Record Decision
          </Button>
        </div>

        {decisions.map((decision, idx) => (
          <motion.div
            key={decision.id}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: idx * 0.1 }}
          >
            <Card>
              <div className="p-6">
                <div className="flex items-start justify-between mb-4">
                  <div className="flex-1">
                    <div className="flex items-center gap-3 mb-2">
                      <h3 className="text-lg font-bold">{decision.decision}</h3>
                      <span
                        className={`text-xs px-2 py-1 rounded font-semibold ${
                          decision.status === 'Approved'
                            ? 'bg-green-500/20 text-green-400'
                            : 'bg-yellow-500/20 text-yellow-400'
                        }`}
                      >
                        {decision.status}
                      </span>
                    </div>

                    <p className="text-sm text-zinc-400 mb-3">{decision.rationale}</p>

                    <div className="grid grid-cols-4 gap-4 text-sm">
                      <div>
                        <p className="text-zinc-500">Related Risk</p>
                        <p className="font-medium">{decision.relatedRisk}</p>
                      </div>
                      <div>
                        <p className="text-zinc-500">Owner</p>
                        <p className="font-medium">{decision.owner}</p>
                      </div>
                      <div>
                        <p className="text-zinc-500">Validity Period</p>
                        <p className="font-medium">{decision.validityPeriod}</p>
                      </div>
                      <div>
                        <p className="text-zinc-500">Approved Date</p>
                        <p className="font-medium">{decision.approvedDate}</p>
                      </div>
                    </div>
                  </div>

                  <div className="flex gap-2 ml-4">
                    <button className="text-zinc-400 hover:text-blue-500 transition-colors p-2">
                      <Edit2 size={20} />
                    </button>
                    <button className="text-zinc-400 hover:text-red-500 transition-colors p-2">
                      <Trash2 size={20} />
                    </button>
                  </div>
                </div>
              </div>
            </Card>
          </motion.div>
        ))}
      </div>

      {/* Risk Acceptance Terms */}
      <Card>
        <div className="p-6">
          <h3 className="text-lg font-bold mb-4">Risk Acceptance Standards</h3>
          <div className="space-y-2 text-sm">
            <p className="text-zinc-400">
              <span className="font-medium text-white">Validity Period:</span> Risk acceptance decisions are reviewed annually or upon significant organizational changes.
            </p>
            <p className="text-zinc-400">
              <span className="font-medium text-white">Approval Authority:</span> Decisions must be approved by appropriate governance level based on risk materiality.
            </p>
            <p className="text-zinc-400">
              <span className="font-medium text-white">Documentation:</span> All decisions require documented rationale and linkage to risk register entries.
            </p>
          </div>
        </div>
      </Card>
    </div>
  );
};
