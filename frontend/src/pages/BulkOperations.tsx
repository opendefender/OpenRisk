import { useEffect, useState } from 'react';
import { CheckCircle, Clock, AlertCircle, Trash2, Play, Pause, RefreshCw, Loader } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'react-hot-toast';

interface BulkOperation {
  id: string;
  operation_type: 'UPDATE_STATUS' | 'ASSIGN_MITIGATION' | 'ADD_TAGS' | 'EXPORT' | 'DELETE';
  status: 'pending' | 'in_progress' | 'completed' | 'failed' | 'cancelled';
  total_items: number;
  completed_items: number;
  failed_items: number;
  progress: number;
  error_message?: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at?: string;
  user_id: string;
}

interface BulkOperationLog {
  id: string;
  bulk_operation_id: string;
  item_id: string;
  item_type: string;
  result: 'success' | 'failure';
  error_message?: string;
  created_at: string;
}

const operationTypeLabels = {
  UPDATE_STATUS: 'Update Status',
  ASSIGN_MITIGATION: 'Assign Mitigation',
  ADD_TAGS: 'Add Tags',
  EXPORT: 'Export',
  DELETE: 'Delete',
};

const statusColors: Record<string, string> = {
  pending: 'bg-yellow-900 text-yellow-200',
  in_progress: 'bg-blue-900 text-blue-200',
  completed: 'bg-green-900 text-green-200',
  failed: 'bg-red-900 text-red-200',
  cancelled: 'bg-zinc-700 text-zinc-200',
};

const statusIcons: Record<string, any> = {
  pending: Clock,
  in_progress: RefreshCw,
  completed: CheckCircle,
  failed: AlertCircle,
  cancelled: AlertCircle,
};

export default function BulkOperations() {
  const [operations, setOperations] = useState<BulkOperation[]>([]);
  const [selectedOperation, setSelectedOperation] = useState<BulkOperation | null>(null);
  const [operationLogs, setOperationLogs] = useState<BulkOperationLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<string>('all');
  const [sortBy, setSortBy] = useState<'newest' | 'oldest'>('newest');

  useEffect(() => {
    fetchOperations();
    const interval = setInterval(fetchOperations, 5000);
    return () => clearInterval(interval);
  }, []);

  const fetchOperations = async () => {
    try {
      const response = await fetch('/api/v1/bulk-operations', {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
      });
      if (response.ok) {
        const data = await response.json();
        setOperations(data || []);
      }
    } catch (error) {
      console.error('Failed to fetch bulk operations');
    } finally {
      setLoading(false);
    }
  };

  const fetchOperationLogs = async (operationId: string) => {
    try {
      const response = await fetch(
        `/api/v1/bulk-operations/${operationId}/logs`,
        {
          headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
        }
      );
      if (response.ok) {
        const data = await response.json();
        setOperationLogs(data || []);
      }
    } catch (error) {
      console.error('Failed to fetch operation logs');
    }
  };

  const handleSelectOperation = (operation: BulkOperation) => {
    setSelectedOperation(operation);
    fetchOperationLogs(operation.id);
  };

  const handleCancelOperation = async (operationId: string) => {
    if (!confirm('Cancel this operation?')) return;

    try {
      const response = await fetch(
        `/api/v1/bulk-operations/${operationId}/cancel`,
        {
          method: 'POST',
          headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
        }
      );

      if (response.ok) {
        toast.success('Operation cancelled');
        fetchOperations();
      } else {
        toast.error('Failed to cancel operation');
      }
    } catch (error) {
      toast.error('Error cancelling operation');
    }
  };

  const handleDeleteOperation = async (operationId: string) => {
    if (!confirm('Delete this operation?')) return;

    try {
      const response = await fetch(`/api/v1/bulk-operations/${operationId}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
      });

      if (response.ok) {
        toast.success('Operation deleted');
        setSelectedOperation(null);
        fetchOperations();
      } else {
        toast.error('Failed to delete operation');
      }
    } catch (error) {
      toast.error('Error deleting operation');
    }
  };

  const filteredOperations = operations.filter((op) => {
    if (filter === 'all') return true;
    return op.status === filter;
  });

  const sortedOperations = [...filteredOperations].sort((a, b) => {
    const dateA = new Date(a.created_at).getTime();
    const dateB = new Date(b.created_at).getTime();
    return sortBy === 'newest' ? dateB - dateA : dateA - dateB;
  });

  const stats = {
    total: operations.length,
    inProgress: operations.filter((op) => op.status === 'in_progress').length,
    completed: operations.filter((op) => op.status === 'completed').length,
    failed: operations.filter((op) => op.status === 'failed').length,
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen bg-zinc-950">
        <Loader className="w-8 h-8 text-blue-500 animate-spin" />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-zinc-950 text-white p-6">
      <div className="max-w-6xl mx-auto">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold">Bulk Operations</h1>
          <p className="text-zinc-400 mt-2">Monitor and manage bulk processing jobs</p>
        </div>

        {/* Stats Cards */}
        <div className="grid grid-cols-4 gap-4 mb-8">
          {[
            { label: 'Total Jobs', value: stats.total, color: 'text-zinc-400' },
            { label: 'In Progress', value: stats.inProgress, color: 'text-blue-400' },
            { label: 'Completed', value: stats.completed, color: 'text-green-400' },
            { label: 'Failed', value: stats.failed, color: 'text-red-400' },
          ].map((stat, idx) => (
            <motion.div
              key={idx}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.1 }}
              className="bg-zinc-900 border border-zinc-800 rounded-lg p-4"
            >
              <div className="text-sm text-zinc-400 mb-1">{stat.label}</div>
              <div className={`text-2xl font-bold ${stat.color}`}>{stat.value}</div>
            </motion.div>
          ))}
        </div>

        {/* Filters */}
        <div className="flex gap-4 mb-6">
          <div className="flex gap-2">
            {[
              { label: 'All', value: 'all' },
              { label: 'Pending', value: 'pending' },
              { label: 'In Progress', value: 'in_progress' },
              { label: 'Completed', value: 'completed' },
              { label: 'Failed', value: 'failed' },
            ].map((f) => (
              <button
                key={f.value}
                onClick={() => setFilter(f.value)}
                className={`px-4 py-2 rounded transition ${
                  filter === f.value
                    ? 'bg-blue-600 text-white'
                    : 'bg-zinc-800 text-zinc-300 hover:bg-zinc-700'
                }`}
              >
                {f.label}
              </button>
            ))}
          </div>
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as any)}
            className="ml-auto px-3 py-2 bg-zinc-800 border border-zinc-700 rounded text-white focus:outline-none focus:border-blue-500"
          >
            <option value="newest">Newest First</option>
            <option value="oldest">Oldest First</option>
          </select>
        </div>

        <div className="grid grid-cols-3 gap-6">
          {/* Operations List */}
          <div className="col-span-2">
            <div className="space-y-3">
              <AnimatePresence>
                {sortedOperations.length === 0 ? (
                  <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    className="text-center py-12 bg-zinc-900 rounded-lg border border-zinc-800"
                  >
                    <AlertCircle className="w-12 h-12 text-zinc-600 mx-auto mb-3" />
                    <p className="text-zinc-400">No operations found</p>
                  </motion.div>
                ) : (
                  sortedOperations.map((operation) => {
                    const StatusIcon = statusIcons[operation.status];
                    return (
                      <motion.div
                        key={operation.id}
                        layout
                        initial={{ opacity: 0, y: 10 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, y: -10 }}
                        onClick={() => handleSelectOperation(operation)}
                        className={`bg-zinc-900 border rounded-lg p-4 cursor-pointer transition hover:border-blue-500 ${
                          selectedOperation?.id === operation.id
                            ? 'border-blue-500'
                            : 'border-zinc-800'
                        }`}
                      >
                        <div className="flex items-start justify-between mb-3">
                          <div className="flex items-center gap-3">
                            <StatusIcon
                              className={`w-5 h-5 ${
                                operation.status === 'in_progress'
                                  ? 'animate-spin'
                                  : ''
                              }`}
                            />
                            <div>
                              <h3 className="font-semibold">
                                {operationTypeLabels[operation.operation_type]}
                              </h3>
                              <p className="text-sm text-zinc-400">
                                {operation.created_at
                                  ? new Date(operation.created_at).toLocaleString()
                                  : 'N/A'}
                              </p>
                            </div>
                          </div>
                          <span
                            className={`px-3 py-1 rounded text-sm font-medium ${
                              statusColors[operation.status]
                            }`}
                          >
                            {operation.status.replace('_', ' ')}
                          </span>
                        </div>

                        {/* Progress Bar */}
                        <div className="mb-3">
                          <div className="flex justify-between text-xs text-zinc-400 mb-1">
                            <span>Progress</span>
                            <span>
                              {operation.completed_items}/{operation.total_items}
                            </span>
                          </div>
                          <div className="w-full bg-zinc-800 rounded-full h-2 overflow-hidden">
                            <motion.div
                              initial={{ width: 0 }}
                              animate={{ width: `${operation.progress}%` }}
                              transition={{ duration: 0.3 }}
                              className={`h-full ${
                                operation.status === 'completed'
                                  ? 'bg-green-500'
                                  : operation.status === 'failed'
                                  ? 'bg-red-500'
                                  : operation.status === 'in_progress'
                                  ? 'bg-blue-500'
                                  : 'bg-yellow-500'
                              }`}
                            />
                          </div>
                        </div>

                        {/* Stats */}
                        <div className="flex gap-4 text-sm">
                          <div className="text-green-400">
                            ✓ {operation.completed_items} succeeded
                          </div>
                          {operation.failed_items > 0 && (
                            <div className="text-red-400">
                              ✗ {operation.failed_items} failed
                            </div>
                          )}
                        </div>
                      </motion.div>
                    );
                  })
                )}
              </AnimatePresence>
            </div>
          </div>

          {/* Details Panel */}
          {selectedOperation && (
            <motion.div
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              className="bg-zinc-900 border border-zinc-800 rounded-lg p-6 h-fit"
            >
              <div className="flex justify-between items-start mb-4">
                <h2 className="text-lg font-bold">Details</h2>
                <button
                  onClick={() => setSelectedOperation(null)}
                  className="text-zinc-400 hover:text-white"
                >
                  ✕
                </button>
              </div>

              <div className="space-y-4 text-sm">
                <div>
                  <label className="text-zinc-400">Operation Type</label>
                  <p className="font-medium">
                    {operationTypeLabels[selectedOperation.operation_type]}
                  </p>
                </div>

                <div>
                  <label className="text-zinc-400">Status</label>
                  <p className="font-medium capitalize">
                    {selectedOperation.status.replace('_', ' ')}
                  </p>
                </div>

                <div>
                  <label className="text-zinc-400">Progress</label>
                  <p className="font-medium">
                    {selectedOperation.completed_items}/{selectedOperation.total_items}{' '}
                    items
                  </p>
                  <div className="w-full bg-zinc-800 rounded-full h-2 mt-2 overflow-hidden">
                    <div
                      className="h-full bg-blue-500"
                      style={{ width: `${selectedOperation.progress}%` }}
                    />
                  </div>
                </div>

                <div>
                  <label className="text-zinc-400">Success Rate</label>
                  <p className="font-medium">
                    {selectedOperation.total_items > 0
                      ? (
                          (selectedOperation.completed_items /
                            selectedOperation.total_items) *
                          100
                        ).toFixed(1)
                      : 0}
                    %
                  </p>
                </div>

                {selectedOperation.error_message && (
                  <div>
                    <label className="text-zinc-400">Error</label>
                    <p className="font-medium text-red-400 text-xs break-words">
                      {selectedOperation.error_message}
                    </p>
                  </div>
                )}

                <div>
                  <label className="text-zinc-400">Created</label>
                  <p className="font-medium text-xs">
                    {new Date(selectedOperation.created_at).toLocaleString()}
                  </p>
                </div>

                {selectedOperation.completed_at && (
                  <div>
                    <label className="text-zinc-400">Completed</label>
                    <p className="font-medium text-xs">
                      {new Date(selectedOperation.completed_at).toLocaleString()}
                    </p>
                  </div>
                )}

                {/* Action Buttons */}
                <div className="flex gap-2 pt-4 border-t border-zinc-700">
                  {selectedOperation.status === 'in_progress' && (
                    <motion.button
                      whileHover={{ scale: 1.05 }}
                      whileTap={{ scale: 0.95 }}
                      onClick={() =>
                        handleCancelOperation(selectedOperation.id)
                      }
                      className="flex-1 flex items-center justify-center gap-2 px-3 py-2 bg-yellow-900 hover:bg-yellow-800 rounded text-sm transition"
                    >
                      <Pause className="w-4 h-4" />
                      Cancel
                    </motion.button>
                  )}
                  <motion.button
                    whileHover={{ scale: 1.05 }}
                    whileTap={{ scale: 0.95 }}
                    onClick={() => handleDeleteOperation(selectedOperation.id)}
                    className="flex-1 flex items-center justify-center gap-2 px-3 py-2 bg-red-900 hover:bg-red-800 rounded text-sm transition"
                  >
                    <Trash2 className="w-4 h-4" />
                    Delete
                  </motion.button>
                </div>
              </div>

              {/* Logs */}
              {operationLogs.length > 0 && (
                <div className="mt-6 pt-6 border-t border-zinc-700">
                  <h3 className="font-semibold mb-3">Recent Logs</h3>
                  <div className="space-y-2 max-h-48 overflow-y-auto">
                    {operationLogs.slice(0, 5).map((log) => (
                      <div
                        key={log.id}
                        className={`p-2 rounded text-xs ${
                          log.result === 'success'
                            ? 'bg-green-900/30 text-green-400'
                            : 'bg-red-900/30 text-red-400'
                        }`}
                      >
                        <div className="font-medium">{log.item_id}</div>
                        {log.error_message && (
                          <div className="text-xs">{log.error_message}</div>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </motion.div>
          )}
        </div>
      </div>
    </div>
  );
}
