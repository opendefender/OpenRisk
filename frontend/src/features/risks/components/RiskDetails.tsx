import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { CheckCircle2, Circle, Plus, User, Server, Database, Zap } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '../../../lib/api';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { useRiskStore, type Risk } from '../../../hooks/useRiskStore';
import { EditRiskModal } from './EditRiskModal';

// --- Interfaces et Types (à mettre idéalement dans les stores respectifs) ---

interface RiskDetailsProps {
    risk: Risk;
    onClose?: () => void;
}

// Helper pour l'icône Asset
const getAssetIcon = (type: string) => {
    switch (type.toLowerCase()) {
        case 'server': return <Server size={14} className="text-zinc-400" />;
        case 'database': return <Database size={14} className="text-zinc-400" />;
        default: return <Zap size={14} className="text-zinc-400" />;
    }
};

// --- Composant Principal ---

export const RiskDetails = ({ risk, onClose }: RiskDetailsProps) => {
    const { fetchRisks } = useRiskStore();
    const deleteRisk = useRiskStore((s) => s.deleteRisk);
  const [activeTab, setActiveTab] = useState<'overview' | 'mitigations'>('overview');
  const [newMitigationTitle, setNewMitigationTitle] = useState('');
  const [isAdding, setIsAdding] = useState(false);
    const [openEdit, setOpenEdit] = useState(false);

  // Ajout d'une mitigation
  const handleAddMitigation = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newMitigationTitle.trim()) return;

    setIsAdding(true);
    try {
      // Endpoint /risks/:id/mitigations défini
      await api.post(`/risks/${risk.id}/mitigations`, {
        title: newMitigationTitle,
        assignee: 'Current User',
        // Le backend gère la création et l'association
      });
      toast.success("Plan d'action ajouté");
      setNewMitigationTitle('');
      await fetchRisks(); // Refresh pour voir la nouvelle mitigation et le score résiduel potentiel
    } catch (err) {
      toast.error("Erreur serveur lors de l'ajout de l'atténuation");
    } finally {
      setIsAdding(false);
    }
  };

  // Toggle statut (Done/Undo)
  const handleToggleMitigation = async (mitigationId: string) => {
    try {
        // Endpoint /mitigations/:mitigationId/toggle défini au Commit #8
        await api.patch(`/mitigations/${mitigationId}/toggle`);
        toast.success("Statut mis à jour");
        fetchRisks();
    } catch (e) {
        toast.error("Impossible de mettre à jour le statut");
    }
  };

  // Badge du score (Pour un feedback visuel immédiat)
  const getScoreStyle = (score: number) => {
    if (score >= 15) return 'bg-red-600/10 text-red-400 border-red-600/20';
    if (score >= 8) return 'bg-orange-600/10 text-orange-400 border-orange-600/20';
    return 'bg-blue-600/10 text-blue-400 border-blue-600/20';
  };

  return (
    <div className="space-y-6">
            {/* 1. En-tête et Badges */}
      <div className="flex flex-wrap gap-2 items-center">
        <span className={`px-3 py-1 rounded-full text-xs font-bold border ${getScoreStyle(risk.score)}`}>
            SCORE: {risk.score}
        </span>
        <span className={`px-3 py-1 rounded-full text-xs font-bold border bg-zinc-800/50 text-zinc-400 border-zinc-700/50`}>
            STATUS: {risk.status}
        </span>
        {risk.tags?.map(tag => (
            <span key={tag} className="px-3 py-1 rounded-full text-xs font-medium bg-zinc-800 text-zinc-400 border border-zinc-700">
                {tag}
            </span>
        ))}
      </div>

            {/* Actions: Edit / Delete */}
            <div className="flex items-center gap-2">
                <Button onClick={() => setOpenEdit(true)} variant="ghost">Modifier</Button>
                <Button onClick={async () => {
                    if (!confirm('Supprimer ce risque ? Cette action est irréversible.')) return;
                    try {
                        await deleteRisk(risk.id);
                        toast.success('Risque supprimé');
                        if (onClose) onClose();
                    } catch (e) {
                        toast.error('Erreur lors de la suppression');
                    }
                }} variant="destructive">Supprimer</Button>
            </div>

            <p className="text-zinc-300 leading-relaxed text-sm">
        {risk.description}
      </p>

            {/* Edit Modal */}
            <EditRiskModal isOpen={openEdit} onClose={() => setOpenEdit(false)} risk={risk} onSuccess={() => { setOpenEdit(false); if (onClose) onClose(); }} />

      {/* 2. Assets Impactés (Ajout Commit #13) */}
      {risk.assets && risk.assets.length > 0 && (
          <div className="mb-6 pt-4 border-t border-white/5">
              <h4 className="text-xs text-zinc-500 uppercase font-bold mb-2">Assets Impactés</h4>
              <div className="flex flex-wrap gap-2">
                  {risk.assets.map(asset => (
                      <div key={asset.id} className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-surface/50 border border-white/5 text-xs text-zinc-300">
                          {getAssetIcon(asset.type)}
                          {asset.name}
                      </div>
                  ))}
              </div>
          </div>
      )}

      {/* 3. Tabs Navigation (Linear Style) */}
      <div className="flex border-b border-white/10 mt-8 mb-4">
        <button 
            onClick={() => setActiveTab('overview')}
            className={`pb-3 px-4 text-sm font-medium transition-colors relative ${activeTab === 'overview' ? 'text-white' : 'text-zinc-500 hover:text-zinc-300'}`}
        >
            Vue d'ensemble
            {activeTab === 'overview' && <motion.div layoutId="activeTab" className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary" />}
        </button>
        <button 
            onClick={() => setActiveTab('mitigations')}
            className={`pb-3 px-4 text-sm font-medium transition-colors relative ${activeTab === 'mitigations' ? 'text-white' : 'text-zinc-500 hover:text-zinc-300'}`}
        >
            Plan d'Atténuation
            <span className="ml-2 bg-zinc-800 text-zinc-400 text-[10px] px-1.5 py-0.5 rounded-full">{risk.mitigations?.length || 0}</span>
            {activeTab === 'mitigations' && <motion.div layoutId="activeTab" className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary" />}
        </button>
      </div>

      {/* 4. Tab Content */}
      <div className="min-h-[300px]">
        <AnimatePresence mode="wait">
          {activeTab === 'overview' ? (
              <motion.div
                  key="overview"
                  initial={{ opacity: 0, x: -10 }}
                  animate={{ opacity: 1, x: 0 }}
                  exit={{ opacity: 0, x: 10 }}
                  transition={{ duration: 0.15 }}
                  className="grid grid-cols-2 gap-4"
              >
                  <div className="p-4 rounded-xl bg-zinc-900/50 border border-white/5">
                      <h4 className="text-xs text-zinc-500 uppercase font-bold mb-2">Impact (C-I-D)</h4>
                      <div className="text-2xl font-mono text-white">{risk.impact}/5</div>
                  </div>
                  <div className="p-4 rounded-xl bg-zinc-900/50 border border-white/5">
                      <h4 className="text-xs text-zinc-500 uppercase font-bold mb-2">Probabilité</h4>
                      <div className="text-2xl font-mono text-white">{risk.probability}/5</div>
                  </div>
                  <div className="col-span-2 p-4 rounded-xl bg-zinc-900/50 border border-white/5">
                      <h4 className="text-xs text-zinc-500 uppercase font-bold mb-2">Propriétaire</h4>
                      <div className="flex items-center gap-2">
                          <div className="w-6 h-6 rounded-full bg-indigo-500 flex items-center justify-center text-[10px] font-bold">JD</div>
                          <span className="text-sm">John Doe (Security Team)</span>
                      </div>
                  </div>
              </motion.div>
          ) : (
              <motion.div
                  key="mitigations"
                  initial={{ opacity: 0, x: 10 }}
                  animate={{ opacity: 1, x: 0 }}
                  exit={{ opacity: 0, x: -10 }}
                  transition={{ duration: 0.15 }}
                  className="space-y-4"
              >
                  {/* Liste des Mitigations */}
                  <div className="space-y-2">
                      {risk.mitigations?.map((mitigation) => (
                          <div key={mitigation.id} className="group flex items-center gap-3 p-3 rounded-lg bg-zinc-900/30 border border-white/5 hover:border-white/10 transition-all">
                              <button 
                                  onClick={() => handleToggleMitigation(mitigation.id)}
                                  className={`shrink-0 transition-colors ${mitigation.status === 'DONE' ? 'text-emerald-500' : 'text-zinc-600 hover:text-zinc-400'}`}
                              >
                                  {mitigation.status === 'DONE' ? <CheckCircle2 size={20} /> : <Circle size={20} />}
                              </button>
                              <div className="flex-1">
                                  <p className={`text-sm ${mitigation.status === 'DONE' ? 'text-zinc-500 line-through' : 'text-zinc-200'}`}>
                                      {mitigation.title}
                                  </p>
                              </div>
                              <div className="flex items-center gap-2 text-zinc-600 text-xs">
                                  <User size={12} /> Assigné
                              </div>
                          </div>
                      ))}
                      
                      {(!risk.mitigations || risk.mitigations.length === 0) && (
                          <div className="text-center py-8 text-zinc-500 text-sm">
                              Aucune action définie. Ajoutez-en une pour réduire ce risque.
                          </div>
                      )}
                  </div>

                  {/* Formulaire d'ajout rapide */}
                  <form onSubmit={handleAddMitigation} className="mt-4 flex gap-2">
                      <Input 
                          placeholder="Nouvelle action (ex: Mettre à jour Apache)..." 
                          value={newMitigationTitle}
                          onChange={(e) => setNewMitigationTitle(e.target.value)}
                          className="flex-1"
                      />
                      <Button type="submit" variant="secondary" isLoading={isAdding} disabled={!newMitigationTitle}>
                          <Plus size={16} />
                      </Button>
                  </form>
              </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
};