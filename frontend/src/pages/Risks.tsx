import { useEffect, useMemo, useState } from 'react';
import { useRiskStore, type Risk } from '../hooks/useRiskStore';
import { Button } from '../components/ui/Button';
import { EditRiskModal } from '../features/risks/components/EditRiskModal';
import { ChevronLeft, ChevronRight, ChevronUp, ChevronDown, TrendingUp, AlertCircle, Shield } from 'lucide-react';
import { ViewToggle } from '../components/ViewToggle';
import { motion } from 'framer-motion';

export const Risks = () => {
  const { risks, total, page, pageSize, isLoading } = useRiskStore();
  const fetchRisks = useRiskStore((s) => s.fetchRisks);
  const setSelectedRisk = useRiskStore((s) => s.setSelectedRisk);

  const [localPage, setLocalPage] = useState<number>(page);
  const [localPageSize, setLocalPageSize] = useState<number>(pageSize);
  const [sortBy, setSortBy] = useState<string>('score');
  const [sortDir, setSortDir] = useState<'desc' | 'asc'>('desc');
  const [view, setView] = useState<'table' | 'card'>(() => {
    const saved = localStorage.getItem('riskView');
    return (saved as 'table' | 'card') || 'table';
  });

  useEffect(() => {
    localStorage.setItem('riskView', view);
  }, [view]);

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

  const totalPages = useMemo(() => Math.max(, Math.ceil(total / localPageSize)), [total, localPageSize]);

  return (
    <div className="max-w-xl mx-auto p-">
      <div className="flex items-center justify-between mb-">
        <h className="text-lg font-bold">Risk Register</h>
        <div className="flex items-center gap-">
          <ViewToggle view={view} onViewChange={setView} />
          <div className="flex items-center gap-">
            <label className="text-sm text-zinc-">Per page</label>
            <select value={localPageSize} onChange={(e) => { setLocalPageSize(Number(e.target.value)); setLocalPage(); }} className="bg-surface p- rounded text-sm">
              <option value={}></option>
              <option value={}></option>
              <option value={}></option>
            </select>
          </div>
        </div>
      </div>

      {view === 'table' && (
        <div className="bg-surface border border-border rounded-md overflow-hidden">
          <div className="grid grid-cols- gap- px- py- text-xs text-zinc- border-b border-border">
            <div className="col-span-">
              <button
                type="button"
                className="flex items-center gap- focus:outline-none"
                onClick={() => {
                  if (sortBy === 'title') setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
                  setSortBy('title');
                }}
                aria-sort={sortBy === 'title' ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
              >
                <span>Title</span>
                {sortBy === 'title' && (sortDir === 'asc' ? <ChevronUp size={} /> : <ChevronDown size={} />)}
              </button>
            </div>
            <div className="col-span-">
              <button
                type="button"
                className="flex items-center gap- focus:outline-none"
                onClick={() => {
                  if (sortBy === 'score') setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
                  setSortBy('score');
                }}
                aria-sort={sortBy === 'score' ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
              >
                <span>Score</span>
                {sortBy === 'score' && (sortDir === 'asc' ? <ChevronUp size={} /> : <ChevronDown size={} />)}
              </button>
            </div>
            <div className="col-span-">
              <button
                type="button"
                className="flex items-center gap- focus:outline-none"
                onClick={() => {
                  if (sortBy === 'impact') setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
                  setSortBy('impact');
                }}
                aria-sort={sortBy === 'impact' ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
              >
                <span>Impact</span>
                {sortBy === 'impact' && (sortDir === 'asc' ? <ChevronUp size={} /> : <ChevronDown size={} />)}
              </button>
            </div>
            <div className="col-span-">
              <button
                type="button"
                className="flex items-center gap- focus:outline-none"
                onClick={() => {
                  if (sortBy === 'probability') setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
                  setSortBy('probability');
                }}
                aria-sort={sortBy === 'probability' ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
              >
                <span>Probability</span>
                {sortBy === 'probability' && (sortDir === 'asc' ? <ChevronUp size={} /> : <ChevronDown size={} />)}
              </button>
            </div>
            <div className="col-span-">
              <button
                type="button"
                className="flex items-center gap- focus:outline-none"
                onClick={() => {
                  if (sortBy === 'status') setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
                  setSortBy('status');
                }}
                aria-sort={sortBy === 'status' ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
              >
                <span>Status</span>
                {sortBy === 'status' && (sortDir === 'asc' ? <ChevronUp size={} /> : <ChevronDown size={} />)}
              </button>
            </div>
            <div className="col-span-">
              <button
                type="button"
                className="flex items-center gap- focus:outline-none"
                onClick={() => {
                  if (sortBy === 'created_at') setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
                  setSortBy('created_at');
                }}
                aria-sort={sortBy === 'created_at' ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
              >
                <span>Created</span>
                {sortBy === 'created_at' && (sortDir === 'asc' ? <ChevronUp size={} /> : <ChevronDown size={} />)}
              </button>
            </div>
            <div className="col-span-">Actions</div>
          </div>

          {isLoading ? (
            <div className="flex items-center justify-center py-">
              <div className="text-center">
                <div className="inline-block animate-spin mb-">
                  <div className="h- w- border- border-primary border-t-primary/ rounded-full"></div>
                </div>
                <p className="text-zinc-">Loading risks...</p>
              </div>
            </div>
          ) : risks.length ===  ? (
            <div className="p- text-center">No risks found.</div>
          ) : (
            risks.map((r: Risk) => (
              <div key={r.id} className="grid grid-cols- gap- px- py- items-center hover:bg-white/">
                <div className="col-span-">
                  <div className="font-medium text-sm">{r.title}</div>
                  <div className="text-xs text-zinc-">{r.description?.slice(, )}</div>
                </div>
                <div className="col-span- font-mono font-bold">{r.score}</div>
                <div className="col-span- text-sm">{r.status}</div>
                <div className="col-span- text-sm">{r.tags?.slice(,).join(', ')}</div>
                <div className="col-span-">
                  <div className="flex gap-">
                    <Button onClick={() => setSelectedRisk(r)} variant="ghost">View</Button>
                    <Button onClick={() => setEditRisk(r)} variant="ghost">Edit</Button>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      )}

      {view === 'card' && (
        <div className="grid grid-cols- md:grid-cols- lg:grid-cols- gap-">
          {isLoading ? (
            <div className="col-span-full flex items-center justify-center py-">
              <div className="text-center">
                <div className="inline-block animate-spin mb-">
                  <div className="h- w- border- border-primary border-t-primary/ rounded-full"></div>
                </div>
                <p className="text-zinc-">Loading risks...</p>
              </div>
            </div>
          ) : risks.length ===  ? (
            <div className="col-span-full p- text-center">No risks found.</div>
          ) : (
            risks.map((r: Risk) => (
              <motion.div
                key={r.id}
                initial={{ opacity: , y:  }}
                animate={{ opacity: , y:  }}
                whileHover={{ y: - }}
                className="bg-surface border border-border rounded-lg p- hover:border-primary/ transition-all cursor-pointer group"
                onClick={() => setSelectedRisk(r)}
              >
                <div className="flex items-start justify-between mb-">
                  <div className="flex-">
                    <h className="font-semibold text-white group-hover:text-primary transition-colors">{r.title}</h>
                    <p className="text-xs text-zinc- mt-">{r.description?.slice(, )}</p>
                  </div>
                  <div className="ml- text-right">
                    <div className="text-xl font-bold text-primary">{r.score}</div>
                    <div className="text-xs text-zinc-">Score</div>
                  </div>
                </div>

                <div className="space-y- mb- border-t border-border pt-">
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-zinc- flex items-center gap-">
                      <AlertCircle size={} /> Impact
                    </span>
                    <span className="text-sm font-medium">{r.impact}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-zinc- flex items-center gap-">
                      <TrendingUp size={} /> Probability
                    </span>
                    <span className="text-sm font-medium">{r.probability}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-zinc- flex items-center gap-">
                      <Shield size={} /> Status
                    </span>
                    <span className={text-xs px- py- rounded-full font-medium ${
                      r.status === 'MITIGATED' ? 'bg-green-/ text-green-' :
                      r.status === 'OPEN' ? 'bg-red-/ text-red-' :
                      'bg-yellow-/ text-yellow-'
                    }}>
                      {r.status}
                    </span>
                  </div>
                </div>

                {r.tags && r.tags.length >  && (
                  <div className="flex flex-wrap gap- mb-">
                    {r.tags.slice(, ).map((tag, i) => (
                      <span key={i} className="text-xs bg-primary/ text-primary px- py- rounded">
                        {tag}
                      </span>
                    ))}
                  </div>
                )}

                <div className="flex gap- pt- border-t border-border">
                  <Button onClick={() => setSelectedRisk(r)} variant="ghost" className="flex- text-xs">View</Button>
                  <Button onClick={() => setEditRisk(r)} variant="ghost" className="flex- text-xs">Edit</Button>
                </div>
              </motion.div>
            ))
          )}
        </div>
      )}

      <EditRiskModal isOpen={!!editRisk} onClose={() => setEditRisk(null)} risk={editRisk} />

      <div className="mt- flex items-center justify-between">
        <div className="text-sm text-zinc-">Total: {total}</div>
        <div className="flex items-center gap-">
          <Button onClick={() => setLocalPage((p) => Math.max(, p - ))} className="p-"><ChevronLeft /></Button>
          <div className="px-">{localPage} / {totalPages}</div>
          <Button onClick={() => setLocalPage((p) => Math.min(totalPages, p + ))} className="p-"><ChevronRight /></Button>
        </div>
      </div>
    </div>
  );
};

export default Risks;
