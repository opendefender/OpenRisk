import React, { useEffect, useMemo, useState } from 'react';
import { useRiskStore, type Risk } from '../hooks/useRiskStore';
import { Button } from '../components/ui/Button';
import { ChevronLeft, ChevronRight } from 'lucide-react';

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
    fetchRisks({ page: localPage, limit: localPageSize, /* sort_by: sortBy, sort_dir: sortDir */ });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    // whenever paging or sorting change, fetch
    fetchRisks({ page: localPage, limit: localPageSize /*, sort_by: sortBy, sort_dir: sortDir */ });
  }, [localPage, localPageSize, sortBy, sortDir, fetchRisks]);

  const totalPages = useMemo(() => Math.max(1, Math.ceil(total / localPageSize)), [total, localPageSize]);

  return (
    <div className="max-w-7xl mx-auto p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-bold">Risk Register</h2>
        <div className="flex items-center gap-3">
          <label className="text-sm text-zinc-400">Sort</label>
          <select value={sortBy} onChange={(e) => setSortBy(e.target.value)} className="bg-surface p-2 rounded">
            <option value="score">Score</option>
            <option value="title">Title</option>
            <option value="created_at">Created</option>
          </select>
          <select value={sortDir} onChange={(e) => setSortDir(e.target.value as 'asc' | 'desc')} className="bg-surface p-2 rounded">
            <option value="desc">Desc</option>
            <option value="asc">Asc</option>
          </select>
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
          <div className="col-span-6">Title</div>
          <div className="col-span-1">Score</div>
          <div className="col-span-2">Status</div>
          <div className="col-span-2">Tags</div>
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
                  <Button onClick={() => { /* TODO: open edit modal */ }} variant="ghost">Edit</Button>
                </div>
              </div>
            </div>
          ))
        )}
      </div>

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
