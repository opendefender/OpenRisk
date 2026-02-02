import { useState } from 'react';
import { FileText, Download } from 'lucide-react';
import { motion } from 'framer-motion';
import { Card } from '../../../components/Card';
import { Button } from '../../../components/ui/Button';

interface ComplianceReport {
  id: string;
  framework: string;
  status: string;
  complianceScore: number;
  lastUpdated: string;
  nextAudit: string;
}

export const RiskAuditCompliance = () => {
  const [reports] = useState<ComplianceReport[]>([
    {
      id: '1',
      framework: 'ISO 31000:2018',
      status: 'Compliant',
      complianceScore: 92,
      lastUpdated: '2024-02-01',
      nextAudit: '2024-08-01',
    },
    {
      id: '2',
      framework: 'NIST RMF',
      status: 'Compliant',
      complianceScore: 88,
      lastUpdated: '2024-02-01',
      nextAudit: '2024-08-15',
    },
    {
      id: '3',
      framework: 'ISO 27001',
      status: 'Compliant',
      complianceScore: 85,
      lastUpdated: '2024-01-15',
      nextAudit: '2024-07-01',
    },
    {
      id: '4',
      framework: 'NIST 800-53',
      status: 'Compliant',
      complianceScore: 90,
      lastUpdated: '2024-02-01',
      nextAudit: '2024-08-30',
    },
  ]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <Card>
        <div className="p-6">
          <div className="flex items-start gap-4">
            <FileText size={32} className="text-blue-500 flex-shrink-0 mt-1" />
            <div>
              <h3 className="text-xl font-bold mb-2">Audit & Compliance Reporting</h3>
              <p className="text-zinc-400 mb-4">
                Generate audit-ready reports and maintain compliance evidence for multiple frameworks including ISO 31000, NIST RMF, ISO 27001, HIPAA, GDPR, and PCI-DSS.
              </p>
              <div className="grid grid-cols-4 gap-4 text-sm">
                <div>
                  <p className="text-zinc-400">Frameworks Tracked</p>
                  <p className="text-2xl font-bold">{reports.length}</p>
                </div>
                <div>
                  <p className="text-zinc-400">Compliant</p>
                  <p className="text-2xl font-bold text-green-400">
                    {reports.filter((r) => r.status === 'Compliant').length}
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Avg. Compliance</p>
                  <p className="text-2xl font-bold">
                    {Math.round(reports.reduce((sum, r) => sum + r.complianceScore, 0) / reports.length)}%
                  </p>
                </div>
                <div>
                  <p className="text-zinc-400">Audit Ready</p>
                  <p className="text-2xl font-bold text-green-400">Yes</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Card>

      {/* Framework Compliance */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-bold">Compliance Status by Framework</h3>
          <Button className="bg-blue-600 hover:bg-blue-700 text-white flex items-center gap-2">
            <Download size={18} />
            Export Report
          </Button>
        </div>

        {reports.map((report, idx) => (
          <motion.div
            key={report.id}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: idx * 0.1 }}
          >
            <Card>
              <div className="p-6">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-lg font-bold">{report.framework}</h3>
                  <span className="text-xs px-2 py-1 rounded bg-green-500/20 text-green-400 font-semibold">
                    {report.status}
                  </span>
                </div>

                <div className="grid grid-cols-4 gap-4 mb-4">
                  <div>
                    <p className="text-xs text-zinc-500 mb-1">Compliance Score</p>
                    <p className="text-2xl font-bold">{report.complianceScore}%</p>
                  </div>
                  <div>
                    <p className="text-xs text-zinc-500 mb-1">Last Updated</p>
                    <p className="text-sm font-medium">{report.lastUpdated}</p>
                  </div>
                  <div>
                    <p className="text-xs text-zinc-500 mb-1">Next Audit</p>
                    <p className="text-sm font-medium">{report.nextAudit}</p>
                  </div>
                  <div>
                    <p className="text-xs text-zinc-500 mb-1">Status</p>
                    <p className="text-sm font-medium text-green-400">{report.status}</p>
                  </div>
                </div>

                <div className="w-full bg-zinc-700 rounded-full h-2">
                  <div
                    className="h-full rounded-full bg-green-500 transition-all"
                    style={{ width: `${report.complianceScore}%` }}
                  />
                </div>
              </div>
            </Card>
          </motion.div>
        ))}
      </div>

      {/* Audit Trail */}
      <Card>
        <div className="p-6">
          <h3 className="text-lg font-bold mb-4">Audit Trail & Evidence Management</h3>
          <div className="space-y-3">
            {[
              {
                event: 'Risk Register Updated',
                details: '5 new risks identified',
                timestamp: '2024-02-02 14:30',
                user: 'Alice Johnson',
              },
              {
                event: 'Treatment Plan Approved',
                details: 'Data Security Treatment',
                timestamp: '2024-02-01 10:15',
                user: 'Bob Smith',
              },
              {
                event: 'Compliance Review',
                details: 'ISO 31000 Assessment',
                timestamp: '2024-01-31 09:00',
                user: 'Carol Davis',
              },
              {
                event: 'Risk Decision Recorded',
                details: 'Accept system downtime risk',
                timestamp: '2024-01-30 16:45',
                user: 'David Wilson',
              },
            ].map((item, idx) => (
              <div key={idx} className="border-l-2 border-blue-500 pl-4 py-2">
                <div className="flex items-center justify-between mb-1">
                  <p className="font-medium">{item.event}</p>
                  <span className="text-xs text-zinc-500">{item.timestamp}</span>
                </div>
                <p className="text-sm text-zinc-400">{item.details}</p>
                <p className="text-xs text-zinc-500">by {item.user}</p>
              </div>
            ))}
          </div>
        </div>
      </Card>

      {/* Compliance Evidence */}
      <Card>
        <div className="p-6">
          <h3 className="text-lg font-bold mb-4">Compliance Evidence Storage</h3>
          <div className="grid grid-cols-2 gap-4">
            {[
              { title: 'Risk Assessment Reports', count: '12', status: 'Stored' },
              { title: 'Treatment Plans', count: '8', status: 'Stored' },
              { title: 'Decision Documentation', count: '15', status: 'Stored' },
              { title: 'Audit Records', count: '6', status: 'Stored' },
            ].map((item, idx) => (
              <div key={idx} className="border border-zinc-600 rounded p-4">
                <p className="text-sm text-zinc-400">{item.title}</p>
                <div className="flex items-center justify-between mt-2">
                  <p className="text-2xl font-bold">{item.count}</p>
                  <span className="text-xs px-2 py-1 rounded bg-green-500/20 text-green-400">
                    {item.status}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </Card>

      {/* Compliance Mapping */}
      <Card>
        <div className="p-6">
          <h3 className="text-lg font-bold mb-4">Multi-Framework Compliance Mapping</h3>
          <div className="space-y-2 text-sm">
            <p className="text-zinc-400">
              <span className="font-medium text-white">ISO 31000:</span> Comprehensive risk management framework for all risk types
            </p>
            <p className="text-zinc-400">
              <span className="font-medium text-white">NIST RMF:</span> Risk management framework for federal information systems
            </p>
            <p className="text-zinc-400">
              <span className="font-medium text-white">ISO 27001:</span> Information security management system requirements
            </p>
            <p className="text-zinc-400">
              <span className="font-medium text-white">NIST 800-53:</span> Security and privacy controls for federal systems
            </p>
            <p className="text-zinc-400">
              <span className="font-medium text-white">GDPR:</span> Data protection and privacy compliance requirements
            </p>
            <p className="text-zinc-400">
              <span className="font-medium text-white">HIPAA:</span> Healthcare data security and privacy requirements
            </p>
            <p className="text-zinc-400">
              <span className="font-medium text-white">PCI-DSS:</span> Payment card industry data security standards
            </p>
          </div>
        </div>
      </Card>
    </div>
  );
};
