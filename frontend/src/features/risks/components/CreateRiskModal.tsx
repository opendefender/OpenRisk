import { useEffect } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { X, Server, Database, Laptop, HardDrive, ShieldAlert, Zap } from 'lucide-react';
import { toast } from 'sonner';
import { useRiskStore } from '../../../hooks/useRiskStore';
import { useAssetStore } from '../../../hooks/useAssetStore'; // Import Assets Store
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';

// --- . Schma de Validation Zod ---
const riskSchema = z.object({
  title: z.string().min(, "Titre requis (min  chars)").max(),
  description: z.string().min(, "Description requise (min  chars)"),
  impact: z.number().min().max(),
  probability: z.number().min().max(),
  tags: z.array(z.string()),
  asset_ids: z.array(z.string()).optional(), // Nouveau champ pour les UUIDs des Assets
  frameworks: z.array(z.string()).optional(),
});

type RiskFormData = z.infer<typeof riskSchema>;

interface CreateRiskModalProps {
  isOpen: boolean;
  onClose: () => void;
}

// Helper pour icne Asset (pour le slecteur visuel)
const getAssetIcon = (type: string) => {
    switch (type.toLowerCase()) {
        case 'server': return <Server size={} />;
        case 'database': return <Database size={} />;
        case 'laptop': return <Laptop size={} />;
        default: return <HardDrive size={} />;
    }
};


export const CreateRiskModal = ({ isOpen, onClose }: CreateRiskModalProps) => {
  const { fetchRisks, createRisk, isLoading } = useRiskStore();
  const { assets, fetchAssets } = useAssetStore(); // Store pour les Assets
  
  // Charger les assets ds que le modal est ouvert
  useEffect(() => {
      if (isOpen) {
          fetchAssets();
      }
  }, [isOpen, fetchAssets]);

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

  // Pour grer la slection visuelle des assets
  const selectedAssetIds = watch('asset_ids') || [];
  const selectedFrameworks = watch('frameworks') || [];

  const toggleAsset = (assetId: string) => {
      const current = selectedAssetIds;
      if (current.includes(assetId)) {
          // Dslectionner
          setValue('asset_ids', current.filter(id => id !== assetId), { shouldValidate: true });
      } else {
          // Slectionner
          setValue('asset_ids', [...current, assetId], { shouldValidate: true });
      }
  };

  const frameworksList = ['ISO', 'CIS', 'NIST', 'OWASP'];
  const toggleFramework = (f: string) => {
      const current = selectedFrameworks;
      if (current.includes(f)) setValue('frameworks', current.filter((v: string) => v !== f), { shouldValidate: true });
      else setValue('frameworks', [...current, f], { shouldValidate: true });
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onSubmit = async (data: RiskFormData) => {
    try {
      await createRisk(data);
      toast.success('Risque cr avec succs !', {
        description: 'Le risque a t enregistr et le score calcul.',
        icon: <Zap className="w- h- text-primary" />,
      });
      await fetchRisks();
      handleClose();
    } catch (error) {
      console.error(error);
      toast.error("Erreur de cration", {
          description: "Veuillez vrifier les champs et l'tat du serveur.",
          icon: <ShieldAlert className="w- h- text-red-" />,
      });
    }
  };

  // UI helpers for impact/probability selection (Linear style)
  const renderScoreSelector = (field: keyof RiskFormData, label: string) => (
    <div className="space-y-.">
        <label className="text-xs font-medium text-zinc- uppercase tracking-wider">{label}</label>
        <div className="flex bg-zinc- border border-border rounded-lg p-">
            {[, , , , ].map(score => (
              <motion.button
                key={score}
                type="button"
                onClick={() => setValue(field, score, { shouldValidate: true })}
                disabled={isLoading}
                className={flex- text-center py- text-sm font-medium rounded-md transition-colors ${
                  watch(field) === score ? 'bg-primary text-white' : 'text-zinc- hover:bg-zinc-'
                } ${isLoading ? 'opacity-' : ''}}
              >
                {score}
              </motion.button>
            ))}
        </div>
        {errors[field] && <p className="text-xs text-red- mt-">{errors[field]?.message as string}</p>}
    </div>
  );

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/ Backdrop /}
          <motion.div
            initial={{ opacity:  }}
            animate={{ opacity:  }}
            exit={{ opacity:  }}
            onClick={handleClose}
            className="fixed inset- bg-black/ z-"
          />

          {/ Modal Content /}
          <motion.div
            initial={{ opacity: , y:  }}
            animate={{ opacity: , y:  }}
            exit={{ opacity: , y:  }}
            transition={{ type: "spring", stiffness: , damping:  }}
            className="fixed inset- m-auto w-full max-w-lg h-fit max-h-[vh] bg-surface border border-border rounded-xl shadow-xl p- z- overflow-hidden"
          >
            <div className="flex justify-between items-center mb- border-b border-white/ pb-">
              <h className="text-xl font-bold text-white flex items-center gap-">
                <ShieldAlert className="text-primary" size={} /> Nouveau Risque
              </h>
              <button onClick={handleClose} className="text-zinc- hover:text-white transition-colors">
                <X size={} />
              </button>
            </div>

            <form onSubmit={handleSubmit((data: any) => onSubmit(data))} className="space-y- overflow-y-auto pr- max-h-[calc(vh-px)]">
              
              {/ Titre et Description /}
              <Input label="Titre" {...register('title')} error={errors.title?.message} disabled={isLoading} />
              
              <div className="space-y-.">
                  <label className="text-xs font-medium text-zinc- uppercase tracking-wider">Description</label>
                  <textarea 
                    {...register('description')} 
                    rows={}
                    className={w-full bg-zinc- border ${errors.description ? 'border-red-' : 'border-border'} rounded-lg p- text-sm text-white focus:ring- focus:ring-primary/ outline-none resize-none transition-colors}
                  />
                  {errors.description && <p className="text-xs text-red- mt-">{errors.description?.message}</p>}
              </div>

              {/ Impact et Probabilit /}
              <div className="grid grid-cols- gap- pt-">
                 {renderScoreSelector('impact', 'Impact (-)')}
                 {renderScoreSelector('probability', 'Probabilit (-)')}
              </div>

              {/ SECTION SÉLECTION ASSETS (Nouveau) /}
              <div className="space-y- pt-">
                  <label className="text-xs font-medium text-zinc- uppercase tracking-wider flex justify-between">
                      Assets Affects
                      <span className="text-[px] bg-zinc- px- py-. rounded-full">{selectedAssetIds.length} slectionn(s)</span>
                  </label>
                  
                  <div className="flex flex-wrap gap- max-h- overflow-y-auto p- border border-border rounded-lg bg-zinc-/">
                      {assets.length ===  ? (
                          <div className="text-zinc- text-xs w-full text-center py-">
                            Aucun asset disponible. Veuillez en crer un dans l'Inventaire Assets.
                          </div>
                      ) : (
                          assets.map(asset => {
                              const isSelected = selectedAssetIds.includes(asset.id);
                              return (
                                    <button
                                      key={asset.id}
                                      type="button"
                                      onClick={() => toggleAsset(asset.id)}
                                      disabled={isLoading}
                                      className={flex items-center gap- px- py-. rounded-md text-xs font-medium border transition-all ${
                                        isSelected 
                                        ? 'bg-blue-/ border-blue- text-blue- shadow-sm' 
                                        : 'bg-zinc- border-zinc- text-zinc- hover:border-zinc- hover:text-zinc-'
                                      } ${isLoading ? 'opacity-' : ''}}
                                    >
                                      {getAssetIcon(asset.type)}
                                      {asset.name}
                                  </button>
                              );
                          })
                      )}
                  </div>
              </div>

              {/ Tags /}
              <Input label="Tags (spars par des virgules)" {...register('tags')} placeholder="ex: critical, web-app, legacy" disabled={isLoading} />

                {/ Frameworks /}
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

              {/ Footer Buttons /}
              <div className="flex justify-end gap- mt- pt- border-t border-white/ sticky bottom- bg-surface">
                <Button type="button" variant="ghost" onClick={handleClose} disabled={isLoading}>Annuler</Button>
                <Button type="submit" isLoading={isLoading || isSubmitting}>Crer le Risque</Button>
              </div>
            </form>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
};