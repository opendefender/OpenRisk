import { Plus, Bell, Search } from 'lucide-react';
import { Button } from '../ui/Button';

interface PageHeaderProps {
  onNewRisk: () => void;
}

// Le header flottant, désormais générique et réutilisable
export const PageHeader = ({ onNewRisk }: PageHeaderProps) => {
  return (
    <header className="h-16 shrink-0 border-b border-border bg-background/80 backdrop-blur-md flex items-center justify-between px-6 z-10 sticky top-0">
      
      {/* Search Bar (Linear style) */}
      <div className="flex items-center gap-2 text-zinc-500 bg-surface border border-white/5 px-3 py-1.5 rounded-md w-64 focus-within:border-primary/50 focus-within:text-white transition-colors group">
        <Search size={14} className="group-focus-within:text-primary transition-colors" />
        <input 
            type="text" 
            placeholder="Search risks, assets..." 
            className="bg-transparent border-none outline-none text-sm w-full placeholder:text-zinc-600"
        />
        <kbd className="hidden sm:inline-block px-1.5 py-0.5 text-[10px] font-bold text-zinc-500 bg-zinc-800 rounded border border-zinc-700">⌘K</kbd>
      </div>

      {/* Actions Droite */}
      <div className="flex items-center gap-4">
          <button className="relative text-zinc-400 hover:text-white transition-colors p-2 hover:bg-white/5 rounded-full">
            <Bell size={20} />
            <span className="absolute top-1.5 right-1.5 w-2 h-2 bg-red-500 rounded-full animate-pulse border border-background"></span>
          </button>
          
          <Button onClick={onNewRisk} className="shadow-lg shadow-blue-500/20">
            <Plus size={16} className="mr-2" /> New Risk
          </Button>
      </div>
    </header>
  );
};