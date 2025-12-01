import React, { useEffect, useMemo, useState } from 'react';
import { useRiskStore, type Risk } from '../hooks/useRiskStore';
import { Button } from '../components/ui/Button';
import { EditRiskModal } from '../features/risks/components/EditRiskModal';
import { ChevronLeft, ChevronRight, ChevronUp, ChevronDown } from 'lucide-react';

export const Risks = () => {
  const { risks, total, page, pageSize, isLoading } = useRiskStore();
  const fetchRisks = useRiskStore((s) => s.fetchRisks);
  const setPage = useRiskStore((s) => s.setPage);
  const setSelectedRisk = useRiskStore((s) => s.setSelectedRisk);

  const [localPage, setLocalPage] = useState<number>(page);
  const [localPageSize, setLocalPageSize] = useState<number>(pageSize);
  const [sortBy, setSortBy] = useState<string>('score');
  const [sortDir, setSortDir] = useState<'desc' | 'asc'>('desc');

  useEffect(() => {
    // initial fetch
    fetchRisks({ page: localPage, limit: localPageSize, sort_by: sortBy, sort_dir: sortDir });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    // whenever paging or sorting change, fetch
    fetchRisks({ page: localPage, limit: localPageSize, sort_by: sortBy, sort_dir: sortDir });
  }, [localPage, localPageSize, sortBy, sortDir, fetchRisks]);

  const [editRisk, setEditRisk] = useState<Risk | null>(null);

  const totalPages = useMemo(() => Math.max(1, Math.ceil(total / localPageSize)), [total, localPageSize]);

  return (
    <div className="max-w-7xl mx-auto p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-bold">Risk Register</h2>
        <div className="flex items-center gap-3">
          <div className="text-sm text-zinc-400">Sort by clicking table headers</div>
          <label className="text-sm text-zinc-400">Per page</label>
          <select value={localPageSize} onChange={(e) => { setLocalPageSize(Number(e.target.value)); setLocalPage(1); }} className="bg-surface p-2 rounded">
            <option value={5}>5</option>
            <option value={10}>10</option>
            <option value={20}>20</option>
          </select>
        </div>
      </div>

      <div className="bg-surface border border-border rounded-md overflow-hidden">
        <div className="grid grid-cols-12 gap-2 px-4 py-2 text-xs text-zinc-400 border-b border-border">
          <div className="col-span-4">
            <button
              type="button"
              className="flex items-center gap-2 focus:outline-none"
              onClick={() => {
                if (sortBy === 'title') setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
                setSortBy('title');
              }}
              aria-sort={sortBy === 'title' ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
            >
              <span>Title</span>
              {sortBy === 'title' && (sortDir === 'asc' ? <ChevronUp size={14} /> : <ChevronDown size={14} />)}
            </button>
          </div>
          <div className="col-span-1">
            <button
              type="button"
              className="flex items-center gap-2 focus:outline-none"
              onClick={() => {
                if (sortBy === 'score') setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
                setSortBy('score');
              }}
              aria-sort={sortBy === 'score' ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
            >
              <span>Score</span>
              {sortBy === 'score' && (sortDir === 'asc' ? <ChevronUp size={14} /> : <ChevronDown size={14} />)}
            </button>
          </div>
          <div className="col-span-1">
            <button
              type="button"
              className="flex items-center gap-2 focus:outline-none"
              onClick={() => {
                if (sortBy === 'impact') setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
                setSortBy('impact');
              }}
              aria-sort={sortBy === 'impact' ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
            >
              <span>Impact</span>
              {sortBy === 'impact' && (sortDir === 'asc' ? <ChevronUp size={14} /> : <ChevronDown size={14} />)}
            </button>
          </div>
          <div className="col-span-1">
            <button
              type="button"
              className="flex items-center gap-2 focus:outline-none"
              onClick={() => {
                if (sortBy === 'probability') setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
                setSortBy('probability');
              }}
              aria-sort={sortBy === 'probability' ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
            >
              <span>Probability</span>
              {sortBy === 'probability' && (sortDir === 'asc' ? <ChevronUp size={14} /> : <ChevronDown size={14} />)}
            </button>
          </div>
          <div className="col-span-2">
            <button
              type="button"
              className="flex items-center gap-2 focus:outline-none"
              onClick={() => {
                if (sortBy === 'status') setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
                setSortBy('status');
              }}
              aria-sort={sortBy === 'status' ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
            >
              <span>Status</span>
              {sortBy === 'status' && (sortDir === 'asc' ? <ChevronUp size={14} /> : <ChevronDown size={14} />)}
            </button>
          </div>
          <div className="col-span-2">
            <button
              type="button"
              className="flex items-center gap-2 focus:outline-none"
              onClick={() => {
                if (sortBy === 'created_at') setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
                setSortBy('created_at');
              }}
              aria-sort={sortBy === 'created_at' ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
            >
              <span>Created</span>
              {sortBy === 'created_at' && (sortDir === 'asc' ? <ChevronUp size={14} /> : <ChevronDown size={14} />)}
            </button>
          </div>
          <div className="col-span-1">Actions</div>
        </div>

        {isLoading ? (
          <div className="p-6 text-center">Loading…</div>
        ) : risks.length === 0 ? (
          <div className="p-6 text-center">No risks found.</div>
        ) : (
          risks.map((r: Risk) => (
            <div key={r.id} className="grid grid-cols-12 gap-2 px-4 py-3 items-center hover:bg-white/2">
              <div className="col-span-6">
                <div className="font-medium text-sm">{r.title}</div>
                <div className="text-xs text-zinc-500">{r.description?.slice(0, 120)}</div>
              </div>
              <div className="col-span-1 font-mono font-bold">{r.score}</div>
              <div className="col-span-2 text-sm">{r.status}</div>
              <div className="col-span-2 text-sm">{r.tags?.slice(0,3).join(', ')}</div>
              <div className="col-span-1">
                <div className="flex gap-2">
                  <Button onClick={() => setSelectedRisk(r)} variant="ghost">View</Button>
                  <Button onClick={() => setEditRisk(r)} variant="ghost">Edit</Button>
                </div>
              </div>
            </div>
          ))
        )}
      </div>

      <EditRiskModal isOpen={!!editRisk} onClose={() => setEditRisk(null)} risk={editRisk} />

      <div className="mt-4 flex items-center justify-between">
        <div className="text-sm text-zinc-400">Total: {total}</div>
        <div className="flex items-center gap-2">
          <Button onClick={() => setLocalPage((p) => Math.max(1, p - 1))} className="p-2"><ChevronLeft /></Button>
          <div className="px-3">{localPage} / {totalPages}</div>
          <Button onClick={() => setLocalPage((p) => Math.min(totalPages, p + 1))} className="p-2"><ChevronRight /></Button>
        </div>
      </div>
    </div>
  );
};

export default Risks;
