import { useState, useEffect } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { X, Server, Database, Laptop, HardDrive, ShieldAlert, Zap } from 'lucide-react';
import { toast } from 'sonner';

import { api } from '../../../lib/api';
import { useRiskStore } from '../../../hooks/useRiskStore';
import { useAssetStore } from '../../../hooks/useAssetStore'; // Import Assets Store
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';

// --- 1. Schéma de Validation Zod ---
const riskSchema = z.object({
  title: z.string().min(5, "Titre requis (min 5 chars)").max(100),
  description: z.string().min(10, "Description requise (min 10 chars)"),
  impact: z.coerce.number().min(1).max(5),
  probability: z.coerce.number().min(1).max(5),
  tags: z.string().transform(val => val.split(',').map(t => t.trim()).filter(t => t !== '')),
  asset_ids: z.array(z.string()).optional(), // Nouveau champ pour les UUIDs des Assets
});

type RiskFormData = z.infer<typeof riskSchema>;

interface CreateRiskModalProps {
  isOpen: boolean;
  onClose: () => void;
}

// Helper pour icône Asset (pour le sélecteur visuel)
const getAssetIcon = (type: string) => {
    switch (type.toLowerCase()) {
        case 'server': return <Server size={14} />;
        case 'database': return <Database size={14} />;
        case 'laptop': return <Laptop size={14} />;
        default: return <HardDrive size={14} />;
    }
};


export const CreateRiskModal = ({ isOpen, onClose }: CreateRiskModalProps) => {
  const { fetchRisks } = useRiskStore();
  const { assets, fetchAssets } = useAssetStore(); // Store pour les Assets
  
  // Charger les assets dès que le modal est ouvert
  useEffect(() => {
      if (isOpen) {
          fetchAssets();
      }
  }, [isOpen, fetchAssets]);

  const { register, handleSubmit, setValue, watch, formState: { errors, isSubmitting }, reset } = useForm<RiskFormData>({
    resolver: zodResolver(riskSchema),
    defaultValues: { 
        impact: 3, 
        probability: 3, 
        asset_ids: [],
        tags: [],
    }
  });

  // Pour gérer la sélection visuelle des assets
  const selectedAssetIds = watch('asset_ids') || [];

  const toggleAsset = (assetId: string) => {
      const current = selectedAssetIds;
      if (current.includes(assetId)) {
          // Désélectionner
          setValue('asset_ids', current.filter(id => id !== assetId), { shouldValidate: true });
      } else {
          // Sélectionner
          setValue('asset_ids', [...current, assetId], { shouldValidate: true });
      }
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onSubmit = async (data: RiskFormData) => {
    try {
      await api.post('/risks', data);
      toast.success('Risque créé avec succès !', {
        description: 'Le risque a été enregistré et le score calculé.',
        icon: <Zap className="w-4 h-4 text-primary" />,
      });
      await fetchRisks();
      handleClose();
    } catch (error) {
      console.error(error);
      toast.error("Erreur de création", {
          description: "Veuillez vérifier les champs et l'état du serveur.",
          icon: <ShieldAlert className="w-4 h-4 text-red-500" />,
      });
    }
  };

  // UI helpers for impact/probability selection (Linear style)
  const renderScoreSelector = (field: keyof RiskFormData, label: string) => (
    <div className="space-y-1.5">
        <label className="text-xs font-medium text-zinc-400 uppercase tracking-wider">{label}</label>
        <div className="flex bg-zinc-900 border border-border rounded-lg p-1">
            {[1, 2, 3, 4, 5].map(score => (
                <motion.button
                    key={score}
                    type="button"
                    onClick={() => setValue(field, score, { shouldValidate: true })}
                    className={`flex-1 text-center py-2 text-sm font-medium rounded-md transition-colors ${
                        watch(field) === score ? 'bg-primary text-white' : 'text-zinc-400 hover:bg-zinc-800'
                    }`}
                >
                    {score}
                </motion.button>
            ))}
        </div>
        {errors[field] && <p className="text-xs text-red-500 mt-1">{errors[field]?.message as string}</p>}
    </div>
  );

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Backdrop */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={handleClose}
            className="fixed inset-0 bg-black/60 z-40"
          />

          {/* Modal Content */}
          <motion.div
            initial={{ opacity: 0, y: 50 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 50 }}
            transition={{ type: "spring", stiffness: 300, damping: 30 }}
            className="fixed inset-0 m-auto w-full max-w-lg h-fit max-h-[90vh] bg-surface border border-border rounded-xl shadow-2xl p-6 z-50 overflow-hidden"
          >
            <div className="flex justify-between items-center mb-6 border-b border-white/5 pb-4">
              <h2 className="text-xl font-bold text-white flex items-center gap-2">
                <ShieldAlert className="text-primary" size={20} /> Nouveau Risque
              </h2>
              <button onClick={handleClose} className="text-zinc-500 hover:text-white transition-colors">
                <X size={24} />
              </button>
            </div>

            <form onSubmit={handleSubmit(onSubmit)} className="space-y-4 overflow-y-auto pr-2 max-h-[calc(90vh-140px)]">
              
              {/* Titre et Description */}
              <Input label="Titre" {...register('title')} error={errors.title?.message} />
              
              <div className="space-y-1.5">
                  <label className="text-xs font-medium text-zinc-400 uppercase tracking-wider">Description</label>
                  <textarea 
                    {...register('description')} 
                    rows={4}
                    className={`w-full bg-zinc-900 border ${errors.description ? 'border-red-500' : 'border-border'} rounded-lg p-3 text-sm text-white focus:ring-2 focus:ring-primary/50 outline-none resize-none transition-colors`}
                  />
                  {errors.description && <p className="text-xs text-red-500 mt-1">{errors.description?.message}</p>}
              </div>

              {/* Impact et Probabilité */}
              <div className="grid grid-cols-2 gap-4 pt-2">
                 {renderScoreSelector('impact', 'Impact (1-5)')}
                 {renderScoreSelector('probability', 'Probabilité (1-5)')}
              </div>

              {/* SECTION SÉLECTION ASSETS (Nouveau) */}
              <div className="space-y-2 pt-2">
                  <label className="text-xs font-medium text-zinc-400 uppercase tracking-wider flex justify-between">
                      Assets Affectés
                      <span className="text-[10px] bg-zinc-800 px-2 py-0.5 rounded-full">{selectedAssetIds.length} sélectionné(s)</span>
                  </label>
                  
                  <div className="flex flex-wrap gap-2 max-h-32 overflow-y-auto p-2 border border-border rounded-lg bg-zinc-900/30">
                      {assets.length === 0 ? (
                          <div className="text-zinc-500 text-xs w-full text-center py-2">
                            Aucun asset disponible. Veuillez en créer un dans l'Inventaire Assets.
                          </div>
                      ) : (
                          assets.map(asset => {
                              const isSelected = selectedAssetIds.includes(asset.id);
                              return (
                                  <button
                                      key={asset.id}
                                      type="button"
                                      onClick={() => toggleAsset(asset.id)}
                                      className={`flex items-center gap-2 px-3 py-1.5 rounded-md text-xs font-medium border transition-all ${
                                          isSelected 
                                          ? 'bg-blue-500/20 border-blue-500 text-blue-400 shadow-sm' 
                                          : 'bg-zinc-800 border-zinc-700 text-zinc-400 hover:border-zinc-500 hover:text-zinc-200'
                                      }`}
                                  >
                                      {getAssetIcon(asset.type)}
                                      {asset.name}
                                  </button>
                              );
                          })
                      )}
                  </div>
              </div>

              {/* Tags */}
              <Input label="Tags (séparés par des virgules)" {...register('tags')} placeholder="ex: critical, web-app, legacy" />

              {/* Footer Buttons */}
              <div className="flex justify-end gap-3 mt-6 pt-4 border-t border-white/5 sticky bottom-0 bg-surface">
                <Button type="button" variant="ghost" onClick={handleClose}>Annuler</Button>
                <Button type="submit" isLoading={isSubmitting}>Créer le Risque</Button>
              </div>
            </form>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
};