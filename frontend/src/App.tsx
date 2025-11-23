import { useEffect, useState } from 'react'; // Ajout useState
import { useRiskStore } from './hooks/useRiskStore';
import { motion } from 'framer-motion';
import { ShieldAlert, Plus } from 'lucide-react'; // Ajout Plus icon
import { Button } from './components/ui/Button'; // Utilisation Button pro
import { CreateRiskModal } from './features/risks/components/CreateRiskModal'; // Import Modal

function App() {
  const { risks, fetchRisks, isLoading } = useRiskStore();
  const [isModalOpen, setIsModalOpen] = useState(false); // State Modal

  useEffect(() => {
    fetchRisks();
  }, [fetchRisks]);

  return (
    <div className="min-h-screen bg-background text-white p-8">
      {/* ... Header existant ... */}
      <header className="max-w-5xl mx-auto mb-12 flex justify-between items-center">
        <div>
           {/* ... Titre ... */}
           <h1 className="text-3xl font-bold bg-gradient-to-r from-blue-400 to-purple-500 bg-clip-text text-transparent">
            OpenRisk
          </h1>
        </div>
        <div className="flex gap-3">
          {/* Nouveau Bouton d'action */}
          <Button onClick={() => setIsModalOpen(true)}>
            <Plus size={16} className="mr-2" /> Nouveau Risque
          </Button>
        </div>
      </header>

      {/* ... Reste du code (Stats et Liste) identique ... */}
      <div className="max-w-5xl mx-auto grid grid-cols-1 gap-4">
          {/* ... Liste des risques ... */}
          {risks.map((risk) => (
            // ... Ta carte de risque existante ...
             <motion.div 
            key={risk.id}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            className="bg-surface border border-border p-6 rounded-xl hover:border-primary/50 transition-colors cursor-pointer group"
          >
            <div className="flex justify-between items-start">
              <div>
                <div className="flex gap-2 mb-2">
                    {/* Gestion safe des tags s'ils sont null */}
                  {risk.tags && risk.tags.map(tag => (
                    <span key={tag} className="text-[10px] font-bold uppercase tracking-wider px-2 py-1 bg-zinc-800 text-zinc-400 rounded-full">
                      {tag}
                    </span>
                  ))}
                </div>
                <h3 className="text-xl font-semibold group-hover:text-primary transition-colors">
                  {risk.title}
                </h3>
                <p className="text-zinc-500 mt-1">{risk.description}</p>
              </div>

              {/* Score Badge */}
              <div className="flex flex-col items-end">
                <div className={`text-2xl font-bold ${
                  risk.score >= 20 ? 'text-risk-critical' : 
                  risk.score >= 15 ? 'text-risk-high' : 
                  risk.score >= 10 ? 'text-risk-medium' : 'text-risk-low'
                }`}>
                  {risk.score}
                </div>
                <div className="text-xs text-zinc-500 uppercase font-mono mt-1">Risk Score</div>
              </div>
            </div>
          </motion.div>
          ))}
      </div>

      {/* Le Modal intégré ici */}
      <CreateRiskModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} />
    </div>
  );
}

export default App;