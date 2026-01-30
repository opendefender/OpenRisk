import { Plus, Bell, Search, ChevronLeft, ChevronRight, Filter } from 'lucide-react';
import { Button } from '../ui/Button';
import { useEffect, useRef, useState } from 'react';
import { useRiskStore } from '../../hooks/useRiskStore';

interface PageHeaderProps {
  onNewRisk: () => void;
}

// Le header flottant, dÃsormais gÃnÃrique et rÃutilisable
export const PageHeader = ({ onNewRisk }: PageHeaderProps) => {
  const [query, setQuery] = useState('');
  const [showFilters, setShowFilters] = useState(false);
  const [statusFilter, setStatusFilter] = useState('');
  const [tagFilter, setTagFilter] = useState('');
  const [minScore, setMinScore] = useState<number | ''>('');
  const [maxScore, setMaxScore] = useState<number | ''>('');

  const fetchRisks = useRiskStore((s) => s.fetchRisks);
  const risks = useRiskStore((s) => s.risks);
  const setSelectedRisk = useRiskStore((s) => s.setSelectedRisk);
  const isLoading = useRiskStore((s) => s.isLoading);
  const page = useRiskStore((s) => s.page);
  const total = useRiskStore((s) => s.total);
  const pageSize = useRiskStore((s) => s.pageSize);
  const setPage = useRiskStore((s) => s.setPage);

  const debounceRef = useRef<number | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [highlighted, setHighlighted] = useState<number>(-);
  const [suggestionsOpen, setSuggestionsOpen] = useState<boolean>(false);

  useEffect(() => {
    // Debounce typing for ms
    if (debounceRef.current) {
      window.clearTimeout(debounceRef.current);
    }
    debounceRef.current = window.setTimeout(() => {
      // When typing, fetch few suggestions only
      fetchRisks(query ? { q: query, limit:  } : { page, limit: pageSize });
      setSuggestionsOpen(Boolean(query));
      setHighlighted(-);
    }, );

    return () => {
      if (debounceRef.current) window.clearTimeout(debounceRef.current);
    };
  }, [query, fetchRisks, page, pageSize]);

  // Close suggestions on outside click
  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      if (!containerRef.current) return;
      if (!containerRef.current.contains(e.target as Node)) {
        setSuggestionsOpen(false);
        setHighlighted(-);
      }
    };
    window.addEventListener('click', onClick);
    return () => window.removeEventListener('click', onClick);
  }, []);

  // Reset highlight when risks list changes
  useEffect(() => {
    setHighlighted(-);
  }, [risks]);

  const applyFilters = async () => {
    setShowFilters(false);
    const params: any = { page: , limit: pageSize };
    if (statusFilter) params.status = statusFilter;
    if (tagFilter) params.tag = tagFilter;
    if (minScore !== '') params.min_score = minScore;
    if (maxScore !== '') params.max_score = maxScore;
    await fetchRisks(params);
  };

  const clearFilters = async () => {
    setStatusFilter(''); setTagFilter(''); setMinScore(''); setMaxScore('');
    await fetchRisks({ page: , limit: pageSize });
  };

  const totalPages = Math.max(, Math.ceil(total / pageSize));

  return (
    <header className="h- shrink- border-b border-border bg-background/ backdrop-blur-md flex items-center justify-between px- z- sticky top-">
      
      {/ Search Bar (Linear style) /}
      <div className="relative">
        <div ref={containerRef} className="flex items-center gap- text-zinc- bg-surface border border-white/ px- py-. rounded-md w- focus-within:border-primary/ focus-within:text-white transition-colors group">
          <Search size={} className="group-focus-within:text-primary transition-colors" />
          <input 
              type="text" 
              value={query}
              ref={inputRef}
              onChange={(e) => { setQuery(e.target.value); setSuggestionsOpen(true); }}
              onKeyDown={(e) => {
                if (!suggestionsOpen) return;
                if (e.key === 'ArrowDown') {
                  e.preventDefault();
                  setHighlighted((h) => Math.min(h + , Math.max(, risks.length - )));
                } else if (e.key === 'ArrowUp') {
                  e.preventDefault();
                  setHighlighted((h) => Math.max(h - , ));
                } else if (e.key === 'Enter') {
                  e.preventDefault();
                  if (highlighted >=  && highlighted < risks.length) {
                    const r = risks[highlighted];
                    setQuery(r.title);
                    setSuggestionsOpen(false);
                    // open risk details via global store
                    setSelectedRisk(r);
                  }
                } else if (e.key === 'Escape') {
                  e.preventDefault();
                  setSuggestionsOpen(false);
                  setHighlighted(-);
                }
              }}
              placeholder="Search risks, assets..." 
              className="bg-transparent border-none outline-none text-sm w- placeholder:text-zinc-"
          />
          <button onClick={() => setShowFilters((s) => !s)} className="p- rounded hover:bg-white/">
            <Filter size={} />
          </button>
          <kbd className="hidden sm:inline-block px-. py-. text-[px] font-bold text-zinc- bg-zinc- rounded border border-zinc-">âŒ˜K</kbd>
        </div>

        {/ Suggestions / Typeahead Panel /}
        {suggestionsOpen && query && !isLoading && risks.length >  && (
          <div className="absolute mt- w- bg-surface border border-white/ rounded-md shadow-lg z-">
            {risks.slice(, ).map((r, idx) => (
              <div
                key={r.id}
                role="option"
                aria-selected={highlighted === idx}
                onMouseEnter={() => setHighlighted(idx)}
                onMouseLeave={() => setHighlighted(-)}
                onClick={() => {
                  setQuery(r.title);
                  setSuggestionsOpen(false);
                  // open risk details via global store
                  setSelectedRisk(r);
                }}
                className={px- py- cursor-pointer ${highlighted === idx ? 'bg-primary/' : 'hover:bg-white/'}}
              >
                <div className="text-sm font-medium">{r.title}</div>
                <div className="text-xs text-zinc-">Score: {r.score} Â· {r.tags?.slice(,).join(', ')}</div>
              </div>
            ))}
          </div>
        )}

        {/ Filters popover /}
        {showFilters && (
          <div className="absolute left- mt- w- bg-surface border border-white/ rounded-md shadow-lg p- z-">
            <div className="flex flex-col gap-">
              <label className="text-xs text-zinc-">Status</label>
              <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="bg-background/ p- rounded">
                <option value="">Any</option>
                <option value="DRAFT">DRAFT</option>
                <option value="PLANNED">PLANNED</option>
                <option value="IN_PROGRESS">IN_PROGRESS</option>
                <option value="DONE">DONE</option>
              </select>

              <label className="text-xs text-zinc-">Tag</label>
              <input value={tagFilter} onChange={(e) => setTagFilter(e.target.value)} placeholder="e.g. database" className="bg-background/ p- rounded" />

              <div className="flex gap-">
                <div className="flex-">
                  <label className="text-xs text-zinc-">Min Score</label>
                  <input type="number" value={minScore as any} onChange={(e) => setMinScore(e.target.value === '' ? '' : Number(e.target.value))} className="bg-background/ p- rounded w-full" />
                </div>
                <div className="flex-">
                  <label className="text-xs text-zinc-">Max Score</label>
                  <input type="number" value={maxScore as any} onChange={(e) => setMaxScore(e.target.value === '' ? '' : Number(e.target.value))} className="bg-background/ p- rounded w-full" />
                </div>
              </div>

              <div className="flex justify-end gap- pt-">
                <button onClick={clearFilters} className="text-sm text-zinc-">Clear</button>
                <button onClick={applyFilters} className="text-sm bg-primary px- py- rounded text-white">Apply</button>
              </div>
            </div>
          </div>
        )}
      </div>

      {/ Actions Droite /}
      <div className="flex items-center gap-">
          <div className="flex items-center gap-">
            <button className="relative text-zinc- hover:text-white transition-colors p- hover:bg-white/ rounded-full">
              <Bell size={} />
              <span className="absolute top-. right-. w- h- bg-red- rounded-full animate-pulse border border-background"></span>
            </button>

            {/ Pagination controls /}
            <div className="flex items-center gap- bg-surface p- rounded">
              <button onClick={() => setPage(Math.max(, page - ))} className="p- hover:bg-white/ rounded">
                <ChevronLeft size={} />
              </button>
              <div className="px- text-sm">{page} / {totalPages}</div>
              <button onClick={() => setPage(Math.min(totalPages, page + ))} className="p- hover:bg-white/ rounded">
                <ChevronRight size={} />
              </button>
            </div>
          </div>
          
          <Button onClick={onNewRisk} className="shadow-lg shadow-blue-/">
            <Plus size={} className="mr-" /> New Risk
          </Button>
      </div>
    </header>
  );
};