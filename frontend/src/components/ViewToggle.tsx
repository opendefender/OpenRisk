import { LayoutGrid, List } from 'lucide-react';
import { cn } from './ui/Button';

interface ViewToggleProps {
  view: 'table' | 'card';
  onViewChange: (view: 'table' | 'card') => void;
}

export const ViewToggle = ({ view, onViewChange }: ViewToggleProps) => {
  return (
    <div className="flex gap- bg-surface/ rounded-lg p- border border-border">
      <button
        onClick={() => onViewChange('table')}
        className={cn(
          'flex items-center gap- px- py-. rounded-md text-sm font-medium transition-all',
          view === 'table'
            ? 'bg-primary/ text-primary border border-primary/'
            : 'text-zinc- hover:text-zinc-'
        )}
      >
        <List size={} />
        <span className="hidden sm:inline">Table</span>
      </button>
      <button
        onClick={() => onViewChange('card')}
        className={cn(
          'flex items-center gap- px- py-. rounded-md text-sm font-medium transition-all',
          view === 'card'
            ? 'bg-primary/ text-primary border border-primary/'
            : 'text-zinc- hover:text-zinc-'
        )}
      >
        <LayoutGrid size={} />
        <span className="hidden sm:inline">Cards</span>
      </button>
    </div>
  );
};
