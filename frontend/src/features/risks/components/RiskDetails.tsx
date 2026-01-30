import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { CheckCircle, Circle, Plus, User, Server, Database, Zap } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '../../../lib/api';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { useRiskStore, type Risk } from '../../../hooks/useRiskStore';
import { EditRiskModal } from './EditRiskModal';
import { MitigationEditModal } from '../../mitigations/MitigationEditModal';

// --- Interfaces et Types (à mettre idalement dans les stores respectifs) ---

interface RiskDetailsProps {
    risk: Risk;
    onClose?: () => void;
}

// Helper pour l'icne Asset
const getAssetIcon = (type: string) => {
    switch (type.toLowerCase()) {
        case 'server': return <Server size={} className="text-zinc-" />;
        case 'database': return <Database size={} className="text-zinc-" />;
        default: return <Zap size={} className="text-zinc-" />;
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
    const [openMitEdit, setOpenMitEdit] = useState<string | null>(null);

  // Ajout d'une mitigation
  const handleAddMitigation = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newMitigationTitle.trim()) return;

    setIsAdding(true);
    try {
      // Endpoint /risks/:id/mitigations dfini
      await api.post(/risks/${risk.id}/mitigations, {
        title: newMitigationTitle,
        assignee: 'Current User',
        // Le backend gre la cration et l'association
      });
      toast.success("Plan d'action ajout");
      setNewMitigationTitle('');
      await fetchRisks(); // Refresh pour voir la nouvelle mitigation et le score rsiduel potentiel
    } catch (err) {
      toast.error("Erreur serveur lors de l'ajout de l'attnuation");
    } finally {
      setIsAdding(false);
    }
  };

  // Toggle statut (Done/Undo)
  const handleToggleMitigation = async (mitigationId: string) => {
    try {
        // Endpoint /mitigations/:mitigationId/toggle dfini au Commit 
        await api.patch(/mitigations/${mitigationId}/toggle);
        toast.success("Statut mis à jour");
        fetchRisks();
    } catch (e) {
        toast.error("Impossible de mettre à jour le statut");
    }
  };

  // Badge du score (Pour un feedback visuel immdiat)
  const getScoreStyle = (score: number) => {
    if (score >= ) return 'bg-red-/ text-red- border-red-/';
    if (score >= ) return 'bg-orange-/ text-orange- border-orange-/';
    return 'bg-blue-/ text-blue- border-blue-/';
  };

  return (
    <div className="space-y-">
            {/ . En-tête et Badges /}
      <div className="flex flex-wrap gap- items-center">
        <span className={px- py- rounded-full text-xs font-bold border ${getScoreStyle(risk.score)}}>
            SCORE: {risk.score}
        </span>
        <span className={px- py- rounded-full text-xs font-bold border bg-zinc-/ text-zinc- border-zinc-/}>
            STATUS: {risk.status}
        </span>
        {risk.tags?.map(tag => (
            <span key={tag} className="px- py- rounded-full text-xs font-medium bg-zinc- text-zinc- border border-zinc-">
                {tag}
            </span>
        ))}
      </div>

            {/ Actions: Edit / Delete /}
            <div className="flex items-center gap-">
                <Button onClick={() => setOpenEdit(true)} variant="ghost">Modifier</Button>
                <Button onClick={async () => {
                    if (!confirm('Supprimer ce risque ? Cette action est irrversible.')) return;
                    try {
                        await deleteRisk(risk.id);
                        toast.success('Risque supprim');
                        if (onClose) onClose();
                    } catch (e) {
                        toast.error('Erreur lors de la suppression');
                    }
                }} variant="danger">Supprimer</Button>
            </div>

            <p className="text-zinc- leading-relaxed text-sm">
        {risk.description}
      </p>

            {/ Edit Modal /}
            <EditRiskModal isOpen={openEdit} onClose={() => setOpenEdit(false)} risk={risk} onSuccess={() => { setOpenEdit(false); if (onClose) onClose(); }} />

      {/ . Assets Impacts /}
      {risk.assets && risk.assets.length >  && (
          <div className="mb- pt- border-t border-white/">
              <h className="text-xs text-zinc- uppercase font-bold mb-">Assets Impacts</h>
              <div className="flex flex-wrap gap-">
                  {risk.assets.map(asset => (
                      <div key={asset.id} className="flex items-center gap- px- py-. rounded-lg bg-surface/ border border-white/ text-xs text-zinc-">
                          {getAssetIcon(asset.type)}
                          {asset.name}
                      </div>
                  ))}
              </div>
          </div>
      )}

      {/ . Tabs Navigation (Linear Style) /}
      <div className="flex border-b border-white/ mt- mb-">
        <button 
            onClick={() => setActiveTab('overview')}
            className={pb- px- text-sm font-medium transition-colors relative ${activeTab === 'overview' ? 'text-white' : 'text-zinc- hover:text-zinc-'}}
        >
            Vue d'ensemble
            {activeTab === 'overview' && <motion.div layoutId="activeTab" className="absolute bottom- left- right- h-. bg-primary" />}
        </button>
        <button 
            onClick={() => setActiveTab('mitigations')}
            className={pb- px- text-sm font-medium transition-colors relative ${activeTab === 'mitigations' ? 'text-white' : 'text-zinc- hover:text-zinc-'}}
        >
            Plan d'Attnuation
            <span className="ml- bg-zinc- text-zinc- text-[px] px-. py-. rounded-full">{risk.mitigations?.length || }</span>
            {activeTab === 'mitigations' && <motion.div layoutId="activeTab" className="absolute bottom- left- right- h-. bg-primary" />}
        </button>
      </div>

      {/ . Tab Content /}
      <div className="min-h-[px]">
        <AnimatePresence mode="wait">
          {activeTab === 'overview' ? (
              <motion.div
                  key="overview"
                  initial={{ opacity: , x: - }}
                  animate={{ opacity: , x:  }}
                  exit={{ opacity: , x:  }}
                  transition={{ duration: . }}
                  className="grid grid-cols- gap-"
              >
                  <div className="p- rounded-xl bg-zinc-/ border border-white/">
                      <h className="text-xs text-zinc- uppercase font-bold mb-">Impact (C-I-D)</h>
                      <div className="text-xl font-mono text-white">{risk.impact}/</div>
                  </div>
                  <div className="p- rounded-xl bg-zinc-/ border border-white/">
                      <h className="text-xs text-zinc- uppercase font-bold mb-">Probabilit</h>
                      <div className="text-xl font-mono text-white">{risk.probability}/</div>
                  </div>
                  <div className="col-span- p- rounded-xl bg-zinc-/ border border-white/">
                      <h className="text-xs text-zinc- uppercase font-bold mb-">Propritaire</h>
                      <div className="flex items-center gap-">
                          <div className="w- h- rounded-full bg-indigo- flex items-center justify-center text-[px] font-bold">JD</div>
                          <span className="text-sm">John Doe (Security Team)</span>
                      </div>
                  </div>
              </motion.div>
          ) : (
              <motion.div
                  key="mitigations"
                  initial={{ opacity: , x:  }}
                  animate={{ opacity: , x:  }}
                  exit={{ opacity: , x: - }}
                  transition={{ duration: . }}
                  className="space-y-"
              >
                  {/ Liste des Mitigations /}
                  <div className="space-y-">
                      {risk.mitigations?.map((mitigation) => (
                          <div key={mitigation.id} className="group flex items-center gap- p- rounded-lg bg-zinc-/ border border-white/ hover:border-white/ transition-all">
                              <button 
                                  onClick={() => handleToggleMitigation(mitigation.id)}
                                  className={shrink- transition-colors ${mitigation.status === 'DONE' ? 'text-emerald-' : 'text-zinc- hover:text-zinc-'}}
                              >
                                  {mitigation.status === 'DONE' ? <CheckCircle size={} /> : <Circle size={} />}
                              </button>
                              <div className="flex-">
                                  <p className={text-sm ${mitigation.status === 'DONE' ? 'text-zinc- line-through' : 'text-zinc-'}}>
                                      {mitigation.title}
                                  </p>
                              </div>
                              <div className="flex items-center gap- text-zinc- text-xs">
                                  <User size={} /> {mitigation.assignee || 'Non assign'}
                              </div>
                              <div className="flex items-center gap-">
                                  <Button variant="ghost" onClick={() => setOpenMitEdit(mitigation.id)}>Éditer</Button>
                              </div>
                          </div>
                      ))}
                      
                      {(!risk.mitigations || risk.mitigations.length === ) && (
                          <div className="text-center py- text-zinc- text-sm">
                              Aucune action dfinie. Ajoutez-en une pour rduire ce risque.
                          </div>
                      )}
                  </div>

                  {/ Formulaire d'ajout rapide /}
                  <form onSubmit={handleAddMitigation} className="mt- flex gap-">
                      <Input 
                          placeholder="Nouvelle action (ex: Mettre à jour Apache)..." 
                          value={newMitigationTitle}
                          onChange={(e) => setNewMitigationTitle(e.target.value)}
                          className="flex-"
                      />
                      <Button type="submit" variant="secondary" isLoading={isAdding} disabled={!newMitigationTitle}>
                          <Plus size={} />
                      </Button>
                  </form>
                            {/ Edit mitigation modal (lazy: import) /}
                            {openMitEdit && (
                                <MitigationEditModal
                                    isOpen={!!openMitEdit}
                                    onClose={() => setOpenMitEdit(null)}
                                    mitigation={risk.mitigations?.find(m => m.id === openMitEdit) || null}
                                    onSaved={() => fetchRisks()}
                                />
                            )}
              </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
};