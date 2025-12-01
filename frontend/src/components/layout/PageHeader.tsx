import { Plus, Bell, Search, ChevronLeft, ChevronRight, Filter } from 'lucide-react';
import { Button } from '../ui/Button';
import { useEffect, useRef, useState } from 'react';
import { useRiskStore } from '../../hooks/useRiskStore';

interface PageHeaderProps {
  onNewRisk: () => void;
}

// Le header flottant, désormais générique et réutilisable
export const PageHeader = ({ onNewRisk }: PageHeaderProps) => {
  const [query, setQuery] = useState('');
  const [showFilters, setShowFilters] = useState(false);
  const [statusFilter, setStatusFilter] = useState('');
  const [tagFilter, setTagFilter] = useState('');
  const [minScore, setMinScore] = useState<number | ''>('');
  const [maxScore, setMaxScore] = useState<number | ''>('');

  const fetchRisks = useRiskStore((s) => s.fetchRisks);
  const risks = useRiskStore((s) => s.risks);
  const isLoading = useRiskStore((s) => s.isLoading);
  const page = useRiskStore((s) => s.page);
  const total = useRiskStore((s) => s.total);
  const pageSize = useRiskStore((s) => s.pageSize);
  const setPage = useRiskStore((s) => s.setPage);

  const debounceRef = useRef<number | null>(null);

  useEffect(() => {
    // Debounce typing for 300ms
    if (debounceRef.current) {
      window.clearTimeout(debounceRef.current);
    }
    debounceRef.current = window.setTimeout(() => {
      // When typing, fetch few suggestions only
      fetchRisks(query ? { q: query, limit: 5 } : { page, limit: pageSize });
    }, 300);

    return () => {
      if (debounceRef.current) window.clearTimeout(debounceRef.current);
    };
  }, [query, fetchRisks, page, pageSize]);

  const applyFilters = async () => {
    setShowFilters(false);
    const params: any = { page: 1, limit: pageSize };
    if (statusFilter) params.status = statusFilter;
    if (tagFilter) params.tag = tagFilter;
    if (minScore !== '') params.min_score = minScore;
    if (maxScore !== '') params.max_score = maxScore;
    await fetchRisks(params);
  };

  const clearFilters = async () => {
    setStatusFilter(''); setTagFilter(''); setMinScore(''); setMaxScore('');
    await fetchRisks({ page: 1, limit: pageSize });
  };

  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <header className="h-16 shrink-0 border-b border-border bg-background/80 backdrop-blur-md flex items-center justify-between px-6 z-10 sticky top-0">
      
      {/* Search Bar (Linear style) */}
      <div className="relative">
        <div className="flex items-center gap-2 text-zinc-500 bg-surface border border-white/5 px-3 py-1.5 rounded-md w-64 focus-within:border-primary/50 focus-within:text-white transition-colors group">
          <Search size={14} className="group-focus-within:text-primary transition-colors" />
          <input 
              type="text" 
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search risks, assets..." 
              className="bg-transparent border-none outline-none text-sm w-56 placeholder:text-zinc-600"
          />
          <button onClick={() => setShowFilters((s) => !s)} className="p-1 rounded hover:bg-white/5">
            <Filter size={16} />
          </button>
          <kbd className="hidden sm:inline-block px-1.5 py-0.5 text-[10px] font-bold text-zinc-500 bg-zinc-800 rounded border border-zinc-700">⌘K</kbd>
        </div>

        {/* Suggestions / Typeahead Panel */}
        {query && !isLoading && risks.length > 0 && (
          <div className="absolute mt-1 w-64 bg-surface border border-white/5 rounded-md shadow-lg z-20">
            {risks.slice(0, 5).map((r) => (
              <div key={r.id} className="px-3 py-2 hover:bg-white/5 cursor-pointer">
                <div className="text-sm font-medium">{r.title}</div>
                <div className="text-xs text-zinc-400">Score: {r.score} · {r.tags?.slice(0,2).join(', ')}</div>
              </div>
            ))}
          </div>
        )}

        {/* Filters popover */}
        {showFilters && (
          <div className="absolute left-0 mt-14 w-80 bg-surface border border-white/5 rounded-md shadow-lg p-4 z-20">
            <div className="flex flex-col gap-2">
              <label className="text-xs text-zinc-400">Status</label>
              <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="bg-background/50 p-2 rounded">
                <option value="">Any</option>
                <option value="DRAFT">DRAFT</option>
                <option value="PLANNED">PLANNED</option>
                <option value="IN_PROGRESS">IN_PROGRESS</option>
                <option value="DONE">DONE</option>
              </select>

              <label className="text-xs text-zinc-400">Tag</label>
              <input value={tagFilter} onChange={(e) => setTagFilter(e.target.value)} placeholder="e.g. database" className="bg-background/50 p-2 rounded" />

              <div className="flex gap-2">
                <div className="flex-1">
                  <label className="text-xs text-zinc-400">Min Score</label>
                  <input type="number" value={minScore as any} onChange={(e) => setMinScore(e.target.value === '' ? '' : Number(e.target.value))} className="bg-background/50 p-2 rounded w-full" />
                </div>
                <div className="flex-1">
                  <label className="text-xs text-zinc-400">Max Score</label>
                  <input type="number" value={maxScore as any} onChange={(e) => setMaxScore(e.target.value === '' ? '' : Number(e.target.value))} className="bg-background/50 p-2 rounded w-full" />
                </div>
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button onClick={clearFilters} className="text-sm text-zinc-400">Clear</button>
                <button onClick={applyFilters} className="text-sm bg-primary px-3 py-1 rounded text-white">Apply</button>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Actions Droite */}
      <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <button className="relative text-zinc-400 hover:text-white transition-colors p-2 hover:bg-white/5 rounded-full">
              <Bell size={20} />
              <span className="absolute top-1.5 right-1.5 w-2 h-2 bg-red-500 rounded-full animate-pulse border border-background"></span>
            </button>

            {/* Pagination controls */}
            <div className="flex items-center gap-1 bg-surface p-1 rounded">
              <button onClick={() => setPage(Math.max(1, page - 1))} className="p-1 hover:bg-white/5 rounded">
                <ChevronLeft size={16} />
              </button>
              <div className="px-2 text-sm">{page} / {totalPages}</div>
              <button onClick={() => setPage(Math.min(totalPages, page + 1))} className="p-1 hover:bg-white/5 rounded">
                <ChevronRight size={16} />
              </button>
            </div>
          </div>
          
          <Button onClick={onNewRisk} className="shadow-lg shadow-blue-500/20">
            <Plus size={16} className="mr-2" /> New Risk
          </Button>
      </div>
    </header>
  );
};