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
  title: z.string().min().max(),
  description: z.string().min(),
  impact: z.number().min().max(),
  probability: z.number().min().max(),
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
      impact: ,
      probability: ,
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
      setValue('impact', risk.impact || );
      setValue('probability', risk.probability || );
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
    }, );
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

  const frameworksList = ['ISO', 'CIS', 'NIST', 'OWASP'];
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
          <motion.div initial={{ opacity:  }} animate={{ opacity:  }} exit={{ opacity:  }} onClick={handleClose} className="fixed inset- bg-black/ z-" />

          <motion.div initial={{ opacity: , y:  }} animate={{ opacity: , y:  }} exit={{ opacity: , y:  }} transition={{ type: "spring", stiffness: , damping:  }} className="fixed inset- m-auto w-full max-w-lg h-fit max-h-[vh] bg-surface border border-border rounded-xl shadow-xl p- z- overflow-hidden">
            <div className="flex justify-between items-center mb- border-b border-white/ pb-">
              <h className="text-xl font-bold text-white flex items-center gap-">
                <ShieldAlert className="text-primary" size={} /> Modifier le Risque
              </h>
              <button onClick={handleClose} className="text-zinc- hover:text-white transition-colors"><X size={} /></button>
            </div>

            <form onSubmit={handleSubmit((data: any) => onSubmit(data))} className="space-y- overflow-y-auto pr- max-h-[calc(vh-px)]">
              <Input label="Titre" {...register('title')} error={errors.title?.message} disabled={isLoading} />

              <div className="space-y-.">
                <label className="text-xs font-medium text-zinc- uppercase tracking-wider">Description</label>
                <textarea {...register('description')} rows={} disabled={isLoading} className={w-full bg-zinc- border ${errors.description ? 'border-red-' : 'border-border'} rounded-lg p- text-sm text-white focus:ring- focus:ring-primary/ outline-none resize-none transition-colors ${isLoading ? 'opacity-' : ''}} />
                {errors.description && <p className="text-xs text-red- mt-">{errors.description?.message}</p>}
              </div>

              <div className="grid grid-cols- gap- pt-">
                <div className="space-y-.">
                  <label className="text-xs font-medium text-zinc- uppercase tracking-wider">Impact (-)</label>
                  <div className="flex bg-zinc- border border-border rounded-lg p-">
                    {[,,,,].map(n => (
                      <button key={n} type="button" onClick={() => setValue('impact', n as any)} disabled={isLoading} className={flex- text-center py- text-sm font-medium rounded-md transition-colors ${watch('impact') === n ? 'bg-primary text-white' : 'text-zinc- hover:bg-zinc-'} ${isLoading ? 'opacity-' : ''}}>
                        {n}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="space-y-.">
                  <label className="text-xs font-medium text-zinc- uppercase tracking-wider">Probabilit (-)</label>
                  <div className="flex bg-zinc- border border-border rounded-lg p-">
                    {[,,,,].map(n => (
                      <button key={n} type="button" onClick={() => setValue('probability', n as any)} disabled={isLoading} className={flex- text-center py- text-sm font-medium rounded-md transition-colors ${watch('probability') === n ? 'bg-primary text-white' : 'text-zinc- hover:bg-zinc-'} ${isLoading ? 'opacity-' : ''}}>
                        {n}
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              {/ Assets selector /}
              <div className="space-y- pt-">
                <label className="text-xs font-medium text-zinc- uppercase tracking-wider flex justify-between">
                  Assets Affects
                  <span className="text-[px] bg-zinc- px- py-. rounded-full">{selectedAssetIds.length} slectionn(s)</span>
                </label>
                <div className="flex flex-wrap gap- max-h- overflow-y-auto p- border border-border rounded-lg bg-zinc-/">
                  {assets.length ===  ? <div className="text-zinc- text-xs w-full text-center py-">Aucun asset.</div> : assets.map(a => (
                    <button key={a.id} type="button" onClick={() => toggleAsset(a.id)} disabled={isLoading} className={flex items-center gap- px- py-. rounded-md text-xs font-medium border transition-all ${selectedAssetIds.includes(a.id) ? 'bg-blue-/ border-blue- text-blue-' : 'bg-zinc- border-zinc- text-zinc-'} ${isLoading ? 'opacity-' : ''}}>
                      {a.name}
                    </button>
                  ))}
                </div>
              </div>

              <Input label="Tags (spars par des virgules)" {...register('tags')} placeholder="ex: critical, web-app, legacy" disabled={isLoading} />

              {/ Frameworks selector /}
              <div className="space-y- pt-">
                <label className="text-xs font-medium text-zinc- uppercase tracking-wider flex justify-between">
                  Frameworks
                  <span className="text-[px] bg-zinc- px- py-. rounded-full">{selectedFrameworks.length} slectionn(s)</span>
                </label>
                <div className="flex flex-wrap gap- p-">
                  {frameworksList.map(f => (
                    <button key={f} type="button" onClick={() => toggleFramework(f)} disabled={isLoading} className={px- py-. rounded-md text-xs font-medium border transition-all ${selectedFrameworks.includes(f) ? 'bg-emerald-/ border-emerald- text-emerald-' : 'bg-zinc- border-zinc- text-zinc- hover:border-zinc-'}}>
                      {f}
                    </button>
                  ))}
                </div>
              </div>

              <div className="flex justify-end gap- mt- pt- border-t border-white/ sticky bottom- bg-surface">
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
