import { useEffect, useState } from 'react';
import { ArrowUp, ArrowDown, Clock, User, AlertCircle, Loader, Filter } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { useParams } from 'react-router-dom';
import { toast } from 'react-hot-toast';

interface TimelineEvent {
  id: string;
  risk_id: string;
  change_type: string;
  old_value?: string;
  new_value?: string;
  change_description: string;
  changed_by: string;
  changed_at: string;
  risk_snapshot?: {
    title: string;
    status: string;
    score: number;
    level: string;
  };
}

interface RiskSnapshot {
  id: string;
  risk_id: string;
  snapshot_data: Record<string, any>;
  created_at: string;
  event_type: string;
}

const changeTypeColors: Record<string, { bg: string; text: string; icon: any }> = {
  CREATED: { bg: 'bg-green-900', text: 'text-green-200', icon: '➕' },
  STATUS_CHANGE: { bg: 'bg-blue-900', text: 'text-blue-200', icon: '🔄' },
  SCORE_CHANGE: { bg: 'bg-yellow-900', text: 'text-yellow-200', icon: '📊' },
  MITIGATION_ADDED: { bg: 'bg-purple-900', text: 'text-purple-200', icon: '🛡️' },
  MITIGATION_UPDATED: { bg: 'bg-purple-800', text: 'text-purple-200', icon: '🔧' },
  TAG_ADDED: { bg: 'bg-indigo-900', text: 'text-indigo-200', icon: '🏷️' },
  TAG_REMOVED: { bg: 'bg-red-900', text: 'text-red-200', icon: '❌' },
  DESCRIPTION_UPDATED: { bg: 'bg-cyan-900', text: 'text-cyan-200', icon: '📝' },
  LEVEL_CHANGED: { bg: 'bg-orange-900', text: 'text-orange-200', icon: '⚠️' },
  DELETED: { bg: 'bg-gray-900', text: 'text-gray-200', icon: '🗑️' },
};

export default function RiskTimeline() {
  const { riskId } = useParams<{ riskId: string }>();
  const [timeline, setTimeline] = useState<TimelineEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<string>('all');
  const [sortBy, setSortBy] = useState<'newest' | 'oldest'>('newest');
  const [expandedEvent, setExpandedEvent] = useState<string | null>(null);
  const [riskTitle, setRiskTitle] = useState('Risk Timeline');

  const changeTypes = [
    { value: 'all', label: 'All Changes' },
    { value: 'CREATED', label: 'Created' },
    { value: 'STATUS_CHANGE', label: 'Status Changed' },
    { value: 'SCORE_CHANGE', label: 'Score Changed' },
    { value: 'MITIGATION_ADDED', label: 'Mitigation Added' },
    { value: 'MITIGATION_UPDATED', label: 'Mitigation Updated' },
    { value: 'TAG_ADDED', label: 'Tag Added' },
    { value: 'TAG_REMOVED', label: 'Tag Removed' },
    { value: 'LEVEL_CHANGED', label: 'Level Changed' },
  ];

  useEffect(() => {
    if (riskId) {
      fetchTimeline();
    }
  }, [riskId]);

  const fetchTimeline = async () => {
    if (!riskId) return;
    try {
      const response = await fetch(`/api/v1/risks/${riskId}/timeline`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
      });

      if (response.ok) {
        const data = await response.json();
        setTimeline(data || []);

        // Try to get risk title from first event
        if (data && data.length > 0 && data[0].risk_snapshot?.title) {
          setRiskTitle(`${data[0].risk_snapshot.title} - Timeline`);
        }
      } else if (response.status === 404) {
        toast.error('Risk not found');
      }
    } catch (error) {
      toast.error('Failed to fetch timeline');
    } finally {
      setLoading(false);
    }
  };

  const filteredTimeline = timeline.filter((event) => {
    if (filter === 'all') return true;
    return event.change_type === filter;
  });

  const sortedTimeline = [...filteredTimeline].sort((a, b) => {
    const dateA = new Date(a.changed_at).getTime();
    const dateB = new Date(b.changed_at).getTime();
    return sortBy === 'newest' ? dateB - dateA : dateA - dateB;
  });

  const getTrendIndicator = (changeType: string, oldValue?: string, newValue?: string) => {
    if (changeType === 'SCORE_CHANGE') {
      const oldScore = parseFloat(oldValue || '0');
      const newScore = parseFloat(newValue || '0');
      if (newScore > oldScore) return <ArrowUp className="w-4 h-4 text-red-400" />;
      if (newScore < oldScore) return <ArrowDown className="w-4 h-4 text-green-400" />;
    }
    return null;
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen bg-zinc-950">
        <Loader className="w-8 h-8 text-blue-500 animate-spin" />
      </div>
    );
  }

  if (!riskId) {
    return (
      <div className="min-h-screen bg-zinc-950 text-white p-6 flex items-center justify-center">
        <div className="text-center">
          <AlertCircle className="w-12 h-12 text-zinc-600 mx-auto mb-4" />
          <p className="text-zinc-400">No risk selected. Please select a risk to view its timeline.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-zinc-950 text-white p-6">
      <div className="max-w-4xl mx-auto">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold">{riskTitle}</h1>
          <p className="text-zinc-400 mt-2">
            View all changes and events for this risk
          </p>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-4 gap-4 mb-8">
          {[
            { label: 'Total Changes', value: timeline.length },
            {
              label: 'Status Changes',
              value: timeline.filter((e) => e.change_type === 'STATUS_CHANGE').length,
            },
            {
              label: 'Score Changes',
              value: timeline.filter((e) => e.change_type === 'SCORE_CHANGE').length,
            },
            {
              label: 'Latest Event',
              value: timeline.length > 0
                ? new Date(timeline[0].changed_at).toLocaleDateString()
                : 'N/A',
            },
          ].map((stat, idx) => (
            <motion.div
              key={idx}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.1 }}
              className="bg-zinc-900 border border-zinc-800 rounded-lg p-4"
            >
              <div className="text-sm text-zinc-400 mb-1">{stat.label}</div>
              <div className="text-2xl font-bold text-white">{stat.value}</div>
            </motion.div>
          ))}
        </div>

        {/* Filters */}
        <div className="flex gap-4 mb-6 flex-wrap">
          <div className="flex gap-2 flex-wrap">
            {changeTypes.map((ct) => (
              <button
                key={ct.value}
                onClick={() => setFilter(ct.value)}
                className={`px-3 py-1 rounded text-sm transition ${
                  filter === ct.value
                    ? 'bg-blue-600 text-white'
                    : 'bg-zinc-800 text-zinc-300 hover:bg-zinc-700'
                }`}
              >
                {ct.label}
              </button>
            ))}
          </div>
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as any)}
            className="ml-auto px-3 py-1 bg-zinc-800 border border-zinc-700 rounded text-sm text-white focus:outline-none focus:border-blue-500"
          >
            <option value="newest">Newest First</option>
            <option value="oldest">Oldest First</option>
          </select>
        </div>

        {/* Timeline */}
        <div className="space-y-4">
          {sortedTimeline.length === 0 ? (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              className="text-center py-12 bg-zinc-900 rounded-lg border border-zinc-800"
            >
              <Clock className="w-12 h-12 text-zinc-600 mx-auto mb-3" />
              <p className="text-zinc-400">No changes recorded for this risk</p>
            </motion.div>
          ) : (
            <AnimatePresence>
              {sortedTimeline.map((event, idx) => {
                const colors =
                  changeTypeColors[event.change_type] ||
                  changeTypeColors['DESCRIPTION_UPDATED'];

                return (
                  <motion.div
                    key={event.id}
                    layout
                    initial={{ opacity: 0, x: -20 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: 20 }}
                    transition={{ delay: idx * 0.05 }}
                    className="relative"
                  >
                    {/* Timeline Line */}
                    {idx < sortedTimeline.length - 1 && (
                      <div className="absolute left-6 top-12 w-0.5 h-12 bg-zinc-800" />
                    )}

                    {/* Event Card */}
                    <div className="relative bg-zinc-900 border border-zinc-800 rounded-lg p-4 hover:border-zinc-700 transition cursor-pointer"
                      onClick={() =>
                        setExpandedEvent(
                          expandedEvent === event.id ? null : event.id
                        )
                      }
                    >
                      <div className="flex gap-4">
                        {/* Timeline Dot */}
                        <div className="flex flex-col items-center">
                          <div
                            className={`w-6 h-6 rounded-full border-4 border-zinc-900 flex items-center justify-center text-lg ${colors.bg}`}
                          >
                            {colors.icon}
                          </div>
                        </div>

                        {/* Event Content */}
                        <div className="flex-1 min-w-0">
                          <div className="flex items-start justify-between gap-4 mb-2">
                            <div className="flex-1">
                              <h3 className="font-semibold flex items-center gap-2">
                                {event.change_type.replace(/_/g, ' ')}
                                {getTrendIndicator(
                                  event.change_type,
                                  event.old_value,
                                  event.new_value
                                )}
                              </h3>
                              <p className="text-sm text-zinc-400">
                                {event.change_description}
                              </p>
                            </div>
                            <span className={`px-2 py-1 rounded text-xs whitespace-nowrap font-medium ${colors.bg} ${colors.text}`}>
                              {event.change_type.replace(/_/g, ' ')}
                            </span>
                          </div>

                          {/* Old vs New Values */}
                          {(event.old_value || event.new_value) && (
                            <div className="grid grid-cols-2 gap-4 my-3 text-sm">
                              {event.old_value && (
                                <div>
                                  <span className="text-zinc-400">Old Value</span>
                                  <p className="font-mono text-red-400 text-xs break-all">
                                    {event.old_value}
                                  </p>
                                </div>
                              )}
                              {event.new_value && (
                                <div>
                                  <span className="text-zinc-400">New Value</span>
                                  <p className="font-mono text-green-400 text-xs break-all">
                                    {event.new_value}
                                  </p>
                                </div>
                              )}
                            </div>
                          )}

                          {/* Meta Info */}
                          <div className="flex gap-4 text-xs text-zinc-500">
                            <div className="flex items-center gap-1">
                              <User className="w-3 h-3" />
                              {event.changed_by || 'System'}
                            </div>
                            <div className="flex items-center gap-1">
                              <Clock className="w-3 h-3" />
                              {new Date(event.changed_at).toLocaleString()}
                            </div>
                          </div>

                          {/* Expanded Snapshot */}
                          <AnimatePresence>
                            {expandedEvent === event.id &&
                              event.risk_snapshot && (
                                <motion.div
                                  initial={{ opacity: 0, height: 0 }}
                                  animate={{ opacity: 1, height: 'auto' }}
                                  exit={{ opacity: 0, height: 0 }}
                                  className="mt-4 pt-4 border-t border-zinc-800"
                                >
                                  <div className="text-xs text-zinc-400 mb-2 font-semibold">
                                    Snapshot at Time of Change
                                  </div>
                                  <div className="bg-zinc-800/50 rounded p-3 space-y-2 text-xs">
                                    <div>
                                      <span className="text-zinc-500">Title:</span>{' '}
                                      <span className="text-white">
                                        {event.risk_snapshot.title}
                                      </span>
                                    </div>
                                    <div>
                                      <span className="text-zinc-500">Status:</span>{' '}
                                      <span className="text-white">
                                        {event.risk_snapshot.status}
                                      </span>
                                    </div>
                                    <div>
                                      <span className="text-zinc-500">Score:</span>{' '}
                                      <span className="text-white">
                                        {event.risk_snapshot.score}/10
                                      </span>
                                    </div>
                                    <div>
                                      <span className="text-zinc-500">Level:</span>{' '}
                                      <span className="text-white">
                                        {event.risk_snapshot.level}
                                      </span>
                                    </div>
                                  </div>
                                </motion.div>
                              )}
                          </AnimatePresence>
                        </div>
                      </div>
                    </div>
                  </motion.div>
                );
              })}
            </AnimatePresence>
          )}
        </div>
      </div>
    </div>
  );
}
