// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useEffect } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { X, ShieldAlert } from 'lucide-react';
import { toast } from 'sonner';

import { useRiskStore } from '../../../hooks/useRiskStore';
import { useAssetStore } from '../../../hooks/useAssetStore';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';

const riskSchema = z.object({
  title: z.string().min(5).max(100),
  description: z.string().min(10),
  impact: z.number().min(1).max(5),
  probability: z.number().min(1).max(5),
  tags: z.array(z.string()),
  asset_ids: z.array(z.string()).optional(),
  frameworks: z.array(z.string()).optional(),
});

type RiskFormData = z.infer<typeof riskSchema>;

interface EditRiskModalProps {
  isOpen: boolean;
  onClose: () => void;
  risk: any | null;
  onSuccess?: () => void;
}

export const EditRiskModal = ({ isOpen, onClose, risk, onSuccess }: EditRiskModalProps) => {
  const { updateRisk, isLoading } = useRiskStore();
  const { assets, fetchAssets } = useAssetStore();

  const { register, handleSubmit, setValue, watch, formState: { errors, isSubmitting }, reset } = useForm<RiskFormData>({
    resolver: zodResolver(riskSchema),
    defaultValues: {
      impact: 3,
      probability: 3,
      asset_ids: [],
      tags: [],
      frameworks: [],
    }
  });

  useEffect(() => {
    if (isOpen) fetchAssets();
    if (risk) {
      setValue('title', risk.title || '');
      setValue('description', risk.description || '');
      setValue('impact', risk.impact || 3);
      setValue('probability', risk.probability || 3);
      setValue('tags', (risk.tags || []).join(','));
      setValue('asset_ids', (risk.assets || []).map((a: any) => a.id));
      setValue('frameworks', risk.frameworks || []);
    } else {
      reset();
    }
  }, [isOpen, risk, setValue, reset, fetchAssets]);

  // Accessibility: focus title input when modal opens
  useEffect(() => {
    if (!isOpen) return;
    const t = window.setTimeout(() => {
      const el = document.querySelector('input[name="title"]') as HTMLInputElement | null;
      el?.focus();
    }, 40);
    return () => window.clearTimeout(t);
  }, [isOpen]);

  // Close on Escape
  useEffect(() => {
    if (!isOpen) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [isOpen, onClose]);

  const selectedAssetIds = watch('asset_ids') || [];
  const selectedFrameworks = watch('frameworks') || [];
  const toggleAsset = (assetId: string) => {
    const current = selectedAssetIds;
    if (current.includes(assetId)) setValue('asset_ids', current.filter((id: string) => id !== assetId), { shouldValidate: true });
    else setValue('asset_ids', [...current, assetId], { shouldValidate: true });
  };

  const frameworksList = ['ISO27001', 'CIS', 'NIST', 'OWASP'];
  const toggleFramework = (f: string) => {
    const current = selectedFrameworks;
    if (current.includes(f)) setValue('frameworks', current.filter((v: string) => v !== f), { shouldValidate: true });
    else setValue('frameworks', [...current, f], { shouldValidate: true });
  };

  const onSubmit = async (data: RiskFormData) => {
    if (!risk) return;
    try {
      await updateRisk(risk.id, data);
      toast.success('Risque mis à jour');
      onClose();
      if (typeof ({} as any) !== 'undefined') {
        // call onSuccess if provided
      }
      // call provided onSuccess from props
      try { (onSuccess as any)?.(); } catch (_) {}
    } catch (err) {
      toast.error('Erreur lors de la mise à jour');
    }
  };

  const handleClose = () => { reset(); onClose(); };

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={handleClose} className="fixed inset-0 bg-black/60 z-[80]" />

          <motion.div initial={{ opacity: 0, y: 50 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: 50 }} transition={{ type: "spring", stiffness: 300, damping: 30 }} className="fixed inset-0 m-auto w-full max-w-lg h-fit max-h-[90vh] bg-surface border border-border rounded-xl shadow-2xl p-6 z-[90] overflow-hidden">
            <div className="flex justify-between items-center mb-6 border-b border-white/5 pb-4">
              <h2 className="text-xl font-bold text-white flex items-center gap-2">
                <ShieldAlert className="text-primary" size={20} /> Modifier le Risque
              </h2>
              <button onClick={handleClose} className="text-zinc-500 hover:text-white transition-colors"><X size={24} /></button>
            </div>

            <form onSubmit={handleSubmit((data: any) => onSubmit(data))} className="space-y-4 overflow-y-auto pr-2 max-h-[calc(90vh-140px)]">
              <Input label="Titre" {...register('title')} error={errors.title?.message} disabled={isLoading} />

              <div className="space-y-1.5">
                <label className="text-xs font-medium text-zinc-400 uppercase tracking-wider">Description</label>
                <textarea {...register('description')} rows={4} disabled={isLoading} className={`w-full bg-zinc-900 border ${errors.description ? 'border-red-500' : 'border-border'} rounded-lg p-3 text-sm text-white focus:ring-2 focus:ring-primary/50 outline-none resize-none transition-colors ${isLoading ? 'opacity-70' : ''}`} />
                {errors.description && <p className="text-xs text-red-500 mt-1">{errors.description?.message}</p>}
              </div>

              <div className="grid grid-cols-2 gap-4 pt-2">
                <div className="space-y-1.5">
                  <label className="text-xs font-medium text-zinc-400 uppercase tracking-wider">Impact (1-5)</label>
                  <div className="flex bg-zinc-900 border border-border rounded-lg p-1">
                    {[1,2,3,4,5].map(n => (
                      <button key={n} type="button" onClick={() => setValue('impact', n as any)} disabled={isLoading} className={`flex-1 text-center py-2 text-sm font-medium rounded-md transition-colors ${watch('impact') === n ? 'bg-primary text-white' : 'text-zinc-400 hover:bg-zinc-800'} ${isLoading ? 'opacity-70' : ''}`}>
                        {n}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-medium text-zinc-400 uppercase tracking-wider">Probabilité (1-5)</label>
                  <div className="flex bg-zinc-900 border border-border rounded-lg p-1">
                    {[1,2,3,4,5].map(n => (
                      <button key={n} type="button" onClick={() => setValue('probability', n as any)} disabled={isLoading} className={`flex-1 text-center py-2 text-sm font-medium rounded-md transition-colors ${watch('probability') === n ? 'bg-primary text-white' : 'text-zinc-400 hover:bg-zinc-800'} ${isLoading ? 'opacity-70' : ''}`}>
                        {n}
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              {/* Assets selector */}
              <div className="space-y-2 pt-2">
                <label className="text-xs font-medium text-zinc-400 uppercase tracking-wider flex justify-between">
                  Assets Affectés
                  <span className="text-[10px] bg-zinc-800 px-2 py-0.5 rounded-full">{selectedAssetIds.length} sélectionné(s)</span>
                </label>
                <div className="flex flex-wrap gap-2 max-h-32 overflow-y-auto p-2 border border-border rounded-lg bg-zinc-900/30">
                  {assets.length === 0 ? <div className="text-zinc-500 text-xs w-full text-center py-2">Aucun asset.</div> : assets.map(a => (
                    <button key={a.id} type="button" onClick={() => toggleAsset(a.id)} disabled={isLoading} className={`flex items-center gap-2 px-3 py-1.5 rounded-md text-xs font-medium border transition-all ${selectedAssetIds.includes(a.id) ? 'bg-blue-500/20 border-blue-500 text-blue-400' : 'bg-zinc-800 border-zinc-700 text-zinc-400'} ${isLoading ? 'opacity-70' : ''}`}>
                      {a.name}
                    </button>
                  ))}
                </div>
              </div>

              <Input label="Tags (séparés par des virgules)" {...register('tags')} placeholder="ex: critical, web-app, legacy" disabled={isLoading} />

              {/* Frameworks selector */}
              <div className="space-y-2 pt-2">
                <label className="text-xs font-medium text-zinc-400 uppercase tracking-wider flex justify-between">
                  Frameworks
                  <span className="text-[10px] bg-zinc-800 px-2 py-0.5 rounded-full">{selectedFrameworks.length} sélectionné(s)</span>
                </label>
                <div className="flex flex-wrap gap-2 p-2">
                  {frameworksList.map(f => (
                    <button key={f} type="button" onClick={() => toggleFramework(f)} disabled={isLoading} className={`px-3 py-1.5 rounded-md text-xs font-medium border transition-all ${selectedFrameworks.includes(f) ? 'bg-emerald-600/20 border-emerald-500 text-emerald-300' : 'bg-zinc-800 border-zinc-700 text-zinc-400 hover:border-zinc-500'}`}>
                      {f}
                    </button>
                  ))}
                </div>
              </div>

              <div className="flex justify-end gap-3 mt-6 pt-4 border-t border-white/5 sticky bottom-0 bg-surface">
                <Button type="button" variant="ghost" onClick={handleClose} disabled={isLoading}>Annuler</Button>
                <Button type="submit" isLoading={isLoading || isSubmitting}>Enregistrer</Button>
              </div>
            </form>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
};
