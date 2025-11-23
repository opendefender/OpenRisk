import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import * as z from 'zod'; // Validation schema
import { motion, AnimatePresence } from 'framer-motion';
import { X } from 'lucide-react';
import { toast } from 'sonner'; // Notifications pro

import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { useRiskStore } from '../../../hooks/useRiskStore';
import { api } from '../../../lib/api';

// 1. Schéma de Validation Strict (Zod)
const riskSchema = z.object({
  title: z.string().min(5, "Le titre doit faire au moins 5 caractères").max(100),
  description: z.string().min(10, "La description doit être détaillée (min 10 chars)"),
  impact: z.coerce.number().min(1).max(5), // coerce force la conversion string->number
  probability: z.coerce.number().min(1).max(5),
  tags: z.string().transform(val => val.split(',').map(t => t.trim()).filter(t => t !== '')),
});

type RiskFormData = z.infer<typeof riskSchema>;

interface CreateRiskModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export const CreateRiskModal = ({ isOpen, onClose }: CreateRiskModalProps) => {
  const { fetchRisks } = useRiskStore();
  
  const { register, handleSubmit, formState: { errors, isSubmitting }, reset } = useForm<RiskFormData>({
    resolver: zodResolver(riskSchema),
    defaultValues: {
      impact: 1,
      probability: 1,
    }
  });

  const onSubmit = async (data: RiskFormData) => {
    try {
      // API call réel
      await api.post('/risks', data);
      
      // Feedback UI immédiat
      toast.success('Risque créé avec succès', {
        description: `Le score a été calculé automatiquement.`
      });
      
      await fetchRisks(); // Refresh la liste
      reset();
      onClose();
    } catch (error) {
      toast.error("Erreur lors de la création", {
        description: "Vérifiez votre connexion ou les logs serveur."
      });
    }
  };

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Backdrop Blur */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm"
          />
          
          {/* Modal Content */}
          <motion.div
            initial={{ opacity: 0, scale: 0.95, y: 20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: 20 }}
            className="fixed left-[50%] top-[50%] z-50 w-full max-w-lg -translate-x-1/2 -translate-y-1/2 rounded-xl border border-border bg-surface p-6 shadow-2xl sm:rounded-2xl"
          >
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-xl font-semibold text-white">Nouveau Risque</h2>
              <button onClick={onClose} className="text-zinc-400 hover:text-white transition-colors">
                <X size={20} />
              </button>
            </div>

            <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
              <Input 
                label="Titre du Risque" 
                placeholder="Ex: Serveur DB non patché..." 
                {...register('title')}
                error={errors.title?.message}
                autoFocus
              />
              
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-zinc-400 uppercase tracking-wider">Description</label>
                <textarea 
                  className="flex min-h-[80px] w-full rounded-lg border border-border bg-zinc-900/50 px-3 py-2 text-sm text-white focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/50"
                  placeholder="Détails techniques et contexte..."
                  {...register('description')}
                />
                {errors.description && <p className="text-xs text-red-400">{errors.description.message}</p>}
              </div>

              <div className="grid grid-cols-2 gap-4">
                <Input 
                  label="Impact (1-5)" 
                  type="number" 
                  min={1} max={5}
                  {...register('impact')}
                  error={errors.impact?.message}
                />
                <Input 
                  label="Probabilité (1-5)" 
                  type="number" 
                  min={1} max={5}
                  {...register('probability')}
                  error={errors.probability?.message}
                />
              </div>

              <Input 
                label="Tags (séparés par virgule)" 
                placeholder="CIS, CRITICAL, OWASP" 
                {...register('tags')}
              />

              <div className="flex justify-end gap-3 mt-6 pt-4 border-t border-white/5">
                <Button type="button" variant="ghost" onClick={onClose}>Annuler</Button>
                <Button type="submit" isLoading={isSubmitting}>Créer le Risque</Button>
              </div>
            </form>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
};