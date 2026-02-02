import { FileText } from 'lucide-react';
import { motion } from 'framer-motion';
import { Card } from '../../../components/Card';

interface RiskCommunication {
  title: string;
  audience: string;
  type: string;
  status: string;
  date: string;
}

export const RiskCommunicationPhase = () => {
  const communications: RiskCommunication[] = [
    {
      title: 'Risk Register Report',
      audience: 'Executive Leadership',
      type: 'Report',
      status: 'Published',
      date: '2024-02-01',
    },
    {
      title: 'Risk Treatment Update',
      audience: 'Risk Team',
      type: 'Email',
      status: 'Sent',
      date: '2024-02-02',
    },
    {
      title: 'Compliance Status Report',
      audience: 'Board of Directors',
      type: 'Report',
      status: 'Scheduled',
      date: '2024-02-15',
    },
  ];

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
                  <p className="text-2xl font-bold">{communications.length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Published</p>
                  <p className="text-2xl font-bold text-green-400">
                    {communications.filter((c) => c.status === 'Published').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Sent</p>
                  <p className="text-2xl font-bold text-blue-400">
                    {communications.filter((c) => c.status === 'Sent').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Scheduled</p>
                  <p className="text-2xl font-bold text-yellow-400">
                    {communications.filter((c) => c.status === 'Scheduled').length}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Card>

      {/* Communications List */}
      <div className="space-y-3">
        <h3 className="text-lg font-bold">Risk Communications</h3>

        {communications.length === 0 ? (
          <Card>
            <div className="p-12 text-center">
              <FileText size={48} className="mx-auto mb-4 text-zinc-500" />
              <p className="text-zinc-400">No communications scheduled</p>
            </div>
          </Card>
        ) : (
          communications.map((comm, idx) => (
            <motion.div
              key={`${comm.title}-${idx}`}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.1 }}
            >
              <Card>
                <div className="p-6">
                  <div className="flex items-center justify-between mb-4">
                    <h3 className="text-lg font-bold">{comm.title}</h3>
                    <span className={`text-xs px-2 py-1 rounded font-semibold ${
                      comm.status === 'Published' ? 'bg-green-500/20 text-green-400' :
                      comm.status === 'Sent' ? 'bg-blue-500/20 text-blue-400' :
                      'bg-yellow-500/20 text-yellow-400'
                    }`}>
                      {comm.status}
                    </span>
                  </div>

                  <div className="grid grid-cols-4 gap-4">
                    <div>
                      <p className="text-xs text-zinc-500">Type</p>
                      <p className="text-sm font-medium">{comm.type}</p>
                    </div>
                    <div>
                      <p className="text-xs text-zinc-500">Audience</p>
                      <p className="text-sm font-medium">{comm.audience}</p>
                    </div>
                    <div>
                      <p className="text-xs text-zinc-500">Date</p>
                      <p className="text-sm font-medium">{comm.date}</p>
                    </div>
                    <div>
                      <p className="text-xs text-zinc-500">Status</p>
                      <p className="text-sm font-medium">{comm.status}</p>
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
