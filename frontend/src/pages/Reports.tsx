import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { FileText, Download, Calendar, Eye, Share, Trash } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { useReportStore } from '../hooks/useReportStore';

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
        return 'bg-blue-/ text-blue- border-blue-/';
      case 'technical':
        return 'bg-purple-/ text-purple- border-purple-/';
      case 'compliance':
        return 'bg-green-/ text-green- border-green-/';
      case 'incident':
        return 'bg-red-/ text-red- border-red-/';
      default:
        return 'bg-zinc-/ text-zinc- border-zinc-/';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'completed':
        return '';
      case 'generating':
        return '';
      case 'scheduled':
        return '';
      default:
        return '';
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'text-green-';
      case 'generating':
        return 'text-yellow- animate-spin';
      case 'scheduled':
        return 'text-zinc-';
      default:
        return 'text-zinc-';
    }
  };

  return (
    <div className="max-w-xl mx-auto p-">
      {/ Header /}
      <div className="mb-">
        <h className="text-xl font-bold mb-">Reports</h>
        <p className="text-zinc-">Generate and manage security reports for compliance and auditing</p>
      </div>

      {/ Stats /}
      <div className="grid grid-cols- md:grid-cols- gap- mb-">
        <div className="bg-surface border border-border rounded-lg p-">
          <div className="text-zinc- text-sm mb-">Total Reports</div>
          <div className="text-xl font-bold">{stats?.total_reports || total}</div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-">
          <div className="text-zinc- text-sm mb-">Completed</div>
          <div className="text-xl font-bold text-green-">
            {stats?.completed || }
          </div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-">
          <div className="text-zinc- text-sm mb-">Generating</div>
          <div className="text-xl font-bold text-yellow-">
            {stats?.generating || }
          </div>
        </div>
        <div className="bg-surface border border-border rounded-lg p-">
          <div className="text-zinc- text-sm mb-">Scheduled</div>
          <div className="text-xl font-bold text-blue-">
            {stats?.scheduled || }
          </div>
        </div>
      </div>

      {/ Action & Filters /}
      <div className="flex flex-col gap- mb-">
        <div className="flex justify-between items-center">
          <Button className="shadow-lg shadow-blue-/">
            <FileText size={} className="mr-" /> Generate New Report
          </Button>
          <div className="flex gap-">
            <select
              value={filterType}
              onChange={(e) => setFilterType(e.target.value)}
              className="bg-surface border border-border px- py- rounded-lg text-sm"
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
              className="bg-surface border border-border px- py- rounded-lg text-sm"
            >
              <option value="all">All Status</option>
              <option value="completed">Completed</option>
              <option value="generating">Generating</option>
              <option value="scheduled">Scheduled</option>
            </select>
          </div>
        </div>
      </div>

      {/ Reports Table /}
      <div className="bg-surface border border-border rounded-lg overflow-hidden">
        <div className="grid grid-cols- gap- px- py- text-xs text-zinc- border-b border-border font-semibold">
          <div className="col-span-">Title</div>
          <div className="col-span-">Type</div>
          <div className="col-span-">Generated By</div>
          <div className="col-span-">Date</div>
          <div className="col-span-">Status</div>
          <div className="col-span-">Actions</div>
        </div>

        <div className="divide-y divide-border">
          {isLoading && (
            <div className="text-center py- text-zinc-">
              <p>Loading reports...</p>
            </div>
          )}
          {error && (
            <div className="text-center py- text-red-">
              <p>Error: {error}</p>
            </div>
          )}
          {!isLoading && filteredReports.length ===  ? (
            <div className="text-center py- text-zinc-">
              <FileText size={} className="mx-auto mb- opacity-" />
              <p>No reports found</p>
            </div>
          ) : (
            filteredReports.map((report, index) => (
              <motion.div
                key={report.id}
                initial={{ opacity:  }}
                animate={{ opacity:  }}
                transition={{ delay: index  . }}
                className="grid grid-cols- gap- px- py- items-center hover:bg-white/ transition-colors"
              >
                <div className="col-span-">
                  <div className="flex items-center gap-">
                    <FileText size={} className="text-zinc-" />
                    <div>
                      <p className="font-medium text-white truncate">{report.title}</p>
                      <p className="text-xs text-zinc-">{report.format}</p>
                    </div>
                  </div>
                </div>
                <div className="col-span-">
                  <span className={text-xs font-bold px- py- rounded border ${getTypeColor(report.type)}}>
                    {report.type.toUpperCase()}
                  </span>
                </div>
                <div className="col-span-">
                  <p className="text-sm text-zinc-">{report.generated_by}</p>
                </div>
                <div className="col-span-">
                  <div className="flex items-center gap- text-sm text-zinc-">
                    <Calendar size={} />
                    {new Date(report.created_at).toLocaleDateString()}
                  </div>
                </div>
                <div className="col-span-">
                  <span className={text-lg font-bold ${getStatusColor(report.status)}}>
                    {getStatusIcon(report.status)}
                  </span>
                </div>
                <div className="col-span- flex justify-end gap-">
                  {report.status === 'completed' && (
                    <>
                      <Button
                        variant="ghost"
                        title="Download"
                      >
                        <Download size={} />
                      </Button>
                      <Button
                        variant="ghost"
                        title="Preview"
                      >
                        <Eye size={} />
                      </Button>
                      <Button
                        variant="ghost"
                        title="Share"
                      >
                        <Share size={} />
                      </Button>
                    </>
                  )}
                  <Button
                    variant="ghost"
                    title="Delete"
                  >
                    <Trash size={} className="text-red-" />
                  </Button>
                </div>
              </motion.div>
            ))
          )}
        </div>
      </div>

      {/ Report Scheduling /}
      <div className="mt- bg-surface border border-border rounded-lg p-">
        <h className="text-lg font-bold mb-">Scheduled Reports</h>
        <p className="text-zinc- mb-">Configure automatic report generation</p>
        <Button variant="secondary">Add Scheduled Report</Button>
      </div>
    </div>
  );
};
