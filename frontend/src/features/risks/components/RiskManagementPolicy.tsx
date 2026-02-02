import React from 'react';
import { Settings, Plus, Edit2, Trash2 } from 'lucide-react';
import { motion } from 'framer-motion';
import { Card } from '../../../components/Card';
import { Button } from '../../../components/ui/Button';

interface RiskPolicy {
  id: string;
  name: string;
  framework: string;
  version: string;
  status: string;
  createdDate: string;
  owner: string;
}

export const RiskManagementPolicy = () => {
  const [policies] = React.useState<RiskPolicy[]>([
    {
      id: '1',
      name: 'Enterprise Risk Management Policy',
      framework: 'ISO 31000',
      version: '2.0',
      status: 'Active',
      createdDate: '2024-01-15',
      owner: 'Chief Risk Officer',
    },
    {
      id: '2',
      name: 'NIST RMF Compliance Framework',
      framework: 'NIST RMF',
      version: '1.5',
      status: 'Active',
      createdDate: '2024-01-20',
      owner: 'Compliance Officer',
    },
  ]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <Card>
        <div className="p-6">
          <div className="flex items-start gap-4">
            <Settings size={32} className="text-blue-500 flex-shrink-0 mt-1" />
            <div>
              <h3 className="text-xl font-bold mb-2">Risk Management Governance & Policy</h3>
              <p className="text-zinc-400 mb-4">
                Establish and maintain governance frameworks, risk management policies, and organizational structure aligned with ISO 31000 and NIST RMF.
              </p>
            </div>
          </div>
        </div>
      </Card>

      {/* Policy Framework */}
      <Card>
        <div className="p-6">
          <h3 className="text-lg font-bold mb-4">Risk Management Frameworks</h3>
          <div className="grid grid-cols-2 gap-4">
            <div className="border border-blue-500/20 rounded p-4">
              <h4 className="font-bold mb-2">ISO 31000:2018</h4>
              <p className="text-sm text-zinc-400 mb-4">
                International standard for risk management. Provides principles, framework, and processes for managing risks.
              </p>
              <div className="text-xs text-zinc-500">
                <p>Status: <span className="text-green-400">Active</span></p>
                <p>Coverage: Enterprise-wide</p>
              </div>
            </div>
            <div className="border border-purple-500/20 rounded p-4">
              <h4 className="font-bold mb-2">NIST RMF</h4>
              <p className="text-sm text-zinc-400 mb-4">
                NIST Risk Management Framework. Structured process for managing information security risks.
              </p>
              <div className="text-xs text-zinc-500">
                <p>Status: <span className="text-green-400">Active</span></p>
                <p>Coverage: IT Security</p>
              </div>
            </div>
          </div>
        </div>
      </Card>

      {/* Policies List */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-bold">Active Policies</h3>
          <Button className="bg-blue-600 hover:bg-blue-700 text-white flex items-center gap-2">
            <Plus size={18} />
            Add Policy
          </Button>
        </div>

        {policies.map((policy, idx) => (
          <motion.div
            key={policy.id}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: idx * 0.1 }}
          >
            <Card>
              <div className="p-6">
                <div className="flex items-start justify-between mb-4">
                  <div className="flex-1">
                    <div className="flex items-center gap-3 mb-2">
                      <h3 className="text-lg font-bold">{policy.name}</h3>
                      <span className="text-xs px-2 py-1 rounded bg-green-500/20 text-green-400 font-semibold">
                        {policy.status}
                      </span>
                    </div>
                    <div className="grid grid-cols-4 gap-4 text-sm">
                      <div>
                        <p className="text-zinc-500">Framework</p>
                        <p className="font-medium">{policy.framework}</p>
                      </div>
                      <div>
                        <p className="text-zinc-500">Version</p>
                        <p className="font-medium">{policy.version}</p>
                      </div>
                      <div>
                        <p className="text-zinc-500">Owner</p>
                        <p className="font-medium">{policy.owner}</p>
                      </div>
                      <div>
                        <p className="text-zinc-500">Created</p>
                        <p className="font-medium">{policy.createdDate}</p>
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

      {/* Roles and Responsibilities */}
      <Card>
        <div className="p-6">
          <h3 className="text-lg font-bold mb-4">Risk Management Roles & Responsibilities</h3>
          <div className="space-y-3">
            {[
              {
                role: 'Chief Risk Officer',
                responsibility: 'Overall risk management oversight and governance',
              },
              {
                role: 'Risk Committee',
                responsibility: 'Risk oversight, approval, and escalation management',
              },
              {
                role: 'Risk Management Office',
                responsibility: 'Implementation and day-to-day risk management',
              },
              {
                role: 'Business Unit Managers',
                responsibility: 'Risk identification and treatment in their domains',
              },
            ].map((item, idx) => (
              <div key={idx} className="flex items-start gap-4 p-3 bg-zinc-700/50 rounded">
                <div className="w-32">
                  <p className="font-semibold text-sm">{item.role}</p>
                </div>
                <p className="text-sm text-zinc-400">{item.responsibility}</p>
              </div>
            ))}
          </div>
        </div>
      </Card>
    </div>
  );
};
