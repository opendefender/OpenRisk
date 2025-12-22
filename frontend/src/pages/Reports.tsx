import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { FileText, Download, Calendar, Eye, Share2, Trash2 } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { useReportStore, type Report } from '../hooks/useReportStore';

export const Reports = () => {
  const { reports, stats, total, page, pageSize, isLoading, error, fetchReports, fetchReportStats } = useReportStore();
  const [filterType, setFilterType] = useState<string>('all');
  const [filterStatus, setFilterStatus] = useState<string>('all');

  useEffect(() => {
    const type = filterType === 'all' ? undefined : filterType;
    const status = filterStatus === 'all' ? undefined : filterStatus;
    fetchReports({ page, limit: pageSize, type, status });
    fetchReportStats();
  }, [page, pageSize, filterType, filterStatus, fetchReports, fetchReportStats]);
  const filteredReports = reports.filter((report) => {
    const matchesType = filterType === 'all' || report.type === filterType;
    const matchesStatus = filterStatus === 'all' || report.status === filterStatus;
    return matchesType && matchesStatus;
  });

  const getTypeColor = (type: string) => {
    switch (type) {
      case 'executive':
        return 'bg-blue-500/10 text-blue-400 border-blue-500/20';
      case 'technical':
        return 'bg-purple-500/10 text-purple-400 border-purple-500/20';
      case 'compliance':
        return 'bg-green-500/10 text-green-400 border-green-500/20';
      case 'incident':
        return 'bg-red-500/10 text-red-400 border-red-500/20';
      default:
        return 'bg-zinc-500/10 text-zinc-400 border-zinc-500/20';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'completed':
        return '✓';
      case 'generating':
        return '⟳';
      case 'scheduled':
        return '○';
      default:
        return '';
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'text-green-400';
      case 'generating':
        return 'text-yellow-400 animate-spin';
      case 'scheduled':
        return 'text-zinc-400';
      default:
        return 'text-zinc-400';
    }
  };

  return (
    <div className="max-w-7xl mx-auto p-6">
      {/* Header */}
      <div className="mb-6">
        <h2 className="text-2xl font-bold mb-2">Reports</h2>
        <p className="text-zinc-400">Generate and manage security reports for compliance and auditing</p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-zinc-400 text-sm mb-2">Total Reports</div>
          <div className="text-3xl font-bold">{stats?.total_reports || total}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-zinc-400 text-sm mb-2">Completed</div>
          <div className="text-3xl font-bold text-green-400">
            {stats?.completed || 0}
          </div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-zinc-400 text-sm mb-2">Generating</div>
          <div className="text-3xl font-bold text-yellow-400">
            {stats?.generating || 0}
          </div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-zinc-400 text-sm mb-2">Scheduled</div>
          <div className="text-3xl font-bold text-blue-400">
            {stats?.scheduled || 0}
          </div>
        </div>
      </div>

      {/* Action & Filters */}
      <div className="flex flex-col gap-4 mb-6">
        <div className="flex justify-between items-center">
          <Button className="shadow-lg shadow-blue-500/20">
            <FileText size={16} className="mr-2" /> Generate New Report
          </Button>
          <div className="flex gap-4">
            <select
              value={filterType}
              onChange={(e) => setFilterType(e.target.value)}
              className="bg-surface border border-border px-3 py-2 rounded-lg text-sm"
            >
              <option value="all">All Types</option>
              <option value="executive">Executive</option>
              <option value="technical">Technical</option>
              <option value="compliance">Compliance</option>
              <option value="incident">Incident</option>
            </select>
            <select
              value={filterStatus}
              onChange={(e) => setFilterStatus(e.target.value)}
              className="bg-surface border border-border px-3 py-2 rounded-lg text-sm"
            >
              <option value="all">All Status</option>
              <option value="completed">Completed</option>
              <option value="generating">Generating</option>
              <option value="scheduled">Scheduled</option>
            </select>
          </div>
        </div>
      </div>

      {/* Reports Table */}
      <div className="bg-surface border border-border rounded-lg overflow-hidden">
        <div className="grid grid-cols-12 gap-4 px-6 py-3 text-xs text-zinc-400 border-b border-border font-semibold">
          <div className="col-span-3">Title</div>
          <div className="col-span-2">Type</div>
          <div className="col-span-2">Generated By</div>
          <div className="col-span-2">Date</div>
          <div className="col-span-1">Status</div>
          <div className="col-span-2">Actions</div>
        </div>

        <div className="divide-y divide-border">
          {isLoading && (
            <div className="text-center py-12 text-zinc-400">
              <p>Loading reports...</p>
            </div>
          )}
          {error && (
            <div className="text-center py-12 text-red-400">
              <p>Error: {error}</p>
            </div>
          )}
          {!isLoading && filteredReports.length === 0 ? (
            <div className="text-center py-12 text-zinc-400">
              <FileText size={48} className="mx-auto mb-4 opacity-50" />
              <p>No reports found</p>
            </div>
          ) : (
            filteredReports.map((report, index) => (
              <motion.div
                key={report.id}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ delay: index * 0.05 }}
                className="grid grid-cols-12 gap-4 px-6 py-4 items-center hover:bg-white/5 transition-colors"
              >
                <div className="col-span-3">
                  <div className="flex items-center gap-2">
                    <FileText size={16} className="text-zinc-500" />
                    <div>
                      <p className="font-medium text-white truncate">{report.title}</p>
                      <p className="text-xs text-zinc-500">{report.format}</p>
                    </div>
                  </div>
                </div>
                <div className="col-span-2">
                  <span className={`text-xs font-bold px-2 py-1 rounded border ${getTypeColor(report.type)}`}>
                    {report.type.toUpperCase()}
                  </span>
                </div>
                <div className="col-span-2">
                  <p className="text-sm text-zinc-300">{report.generated_by}</p>
                </div>
                <div className="col-span-2">
                  <div className="flex items-center gap-2 text-sm text-zinc-300">
                    <Calendar size={14} />
                    {new Date(report.created_at).toLocaleDateString()}
                  </div>
                </div>
                <div className="col-span-1">
                  <span className={`text-lg font-bold ${getStatusColor(report.status)}`}>
                    {getStatusIcon(report.status)}
                  </span>
                </div>
                <div className="col-span-2 flex justify-end gap-2">
                  {report.status === 'completed' && (
                    <>
                      <Button
                        variant="ghost"
                        size="sm"
                        title="Download"
                      >
                        <Download size={16} />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        title="Preview"
                      >
                        <Eye size={16} />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        title="Share"
                      >
                        <Share2 size={16} />
                      </Button>
                    </>
                  )}
                  <Button
                    variant="ghost"
                    size="sm"
                    title="Delete"
                  >
                    <Trash2 size={16} className="text-red-400" />
                  </Button>
                </div>
              </motion.div>
            ))
          )}
        </div>
      </div>

      {/* Report Scheduling */}
      <div className="mt-6 bg-surface border border-border rounded-lg p-6">
        <h3 className="text-lg font-bold mb-4">Scheduled Reports</h3>
        <p className="text-zinc-400 mb-4">Configure automatic report generation</p>
        <Button variant="outline">Add Scheduled Report</Button>
      </div>
    </div>
  );
};
