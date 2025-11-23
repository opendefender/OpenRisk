import { useState } from 'react';
import { Sidebar } from './components/layout/Sidebar';
import { DashboardGrid } from './features/dashboard/DashboardGrid';
import { CreateRiskModal } from './features/risks/components/CreateRiskModal';
import { Button } from './components/ui/Button';
import { Plus, Bell, Search } from 'lucide-react';
import { motion } from 'framer-motion';

function App() {
  const [isModalOpen, setIsModalOpen] = useState(false);

  return (
    <div className="flex h-screen bg-background text-white overflow-hidden font-sans selection:bg-primary/30">
      {/* 1. Sidebar Fixe */}
      <Sidebar />

      {/* 2. Main Content Area */}
      <div className="flex-1 flex flex-col h-screen overflow-hidden relative">
        
        {/* Header Flottant / Glassmorphism */}
        <header className="h-16 shrink-0 border-b border-border bg-background/50 backdrop-blur-md flex items-center justify-between px-6 z-10">
          
          {/* Search Bar (Linear style) */}
          <div className="flex items-center gap-2 text-zinc-500 bg-surface border border-white/5 px-3 py-1.5 rounded-md w-64 focus-within:border-primary/50 focus-within:text-white transition-colors">
            <Search size={14} />
            <input 
                type="text" 
                placeholder="Search risks, assets..." 
                className="bg-transparent border-none outline-none text-sm w-full placeholder:text-zinc-600"
            />
            <kbd className="hidden sm:inline-block px-1.5 py-0.5 text-[10px] font-bold text-zinc-500 bg-zinc-800 rounded border border-zinc-700">⌘K</kbd>
          </div>

          <div className="flex items-center gap-4">
             <button className="relative text-zinc-400 hover:text-white transition-colors">
                <Bell size={20} />
                <span className="absolute top-0 right-0 w-2 h-2 bg-red-500 rounded-full animate-pulse"></span>
             </button>
             
             <Button onClick={() => setIsModalOpen(true)} className="shadow-lg shadow-blue-500/20">
                <Plus size={16} className="mr-2" /> New Risk
             </Button>
          </div>
        </header>

        {/* Dashboard Scrollable Area */}
        <main className="flex-1 overflow-y-auto overflow-x-hidden p-6 relative scrollbar-thin scrollbar-thumb-zinc-800 scrollbar-track-transparent">
           <motion.div
             initial={{ opacity: 0, y: 20 }}
             animate={{ opacity: 1, y: 0 }}
             transition={{ duration: 0.5 }}
           >
              <DashboardGrid />
           </motion.div>
        </main>

      </div>

      <CreateRiskModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} />
    </div>
  );
}

export default App;