// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useEffect } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { X, Zap, ShieldAlert } from 'lucide-react';
import { toast } from 'sonner';
import { mitigationService } from '../../services/mitigationService';
import type { Mitigation } from '../../types/mitigation';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { useI18n } from '../../hooks/useI18n';

const createMitigationSchema = z.object({
  title: z.string().min(3, 'Le titre doit comporter au moins 3 caractères'),
  description: z.string().optional(),
  due_date: z.string().optional(),
  priority: z.enum(['critical', 'high', 'medium', 'low']).optional(),
});

type CreateMitigationForm = z.infer<typeof createMitigationSchema>;

interface CreateMitigationModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated?: (m: Mitigation) => void;
}

export const CreateMitigationModal = ({ isOpen, onClose, onCreated }: CreateMitigationModalProps) => {
  const { t } = useI18n();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<CreateMitigationForm>({
    resolver: zodResolver(createMitigationSchema),
    defaultValues: { title: '', description: '', due_date: '', priority: 'medium' },
  });

  useEffect(() => {
    if (isOpen) {
      setTimeout(() => {
        const input = document.querySelector('input[name="title"]') as HTMLInputElement | null;
        input?.focus();
      }, 50);
    }
  }, [isOpen]);

  const handleClose = () => {
    reset();
    onClose();
  };

  const onSubmit = async (values: CreateMitigationForm) => {
    try {
      const payload = {
        title: values.title,
        description: values.description,
        due_date: values.due_date || undefined,
        priority: values.priority || 'medium',
        source: 'manual',
      } as any;

      const created = await mitigationService.createMitigation(payload);
      toast.success('Plan d\'atténuation créé');
      onCreated?.(created);
      handleClose();
    } catch (err) {
      console.error(err);
      toast.error('Impossible de créer le plan', { description: 'Erreur serveur' });
    }
  };

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={handleClose}
            className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm"
          />

          <motion.div
            initial={{ opacity: 0, scale: 0.96, y: 40 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 40 }}
            transition={{ duration: 0.22, type: 'spring', stiffness: 240 }}
            className="fixed inset-x-0 top-1/2 z-50 mx-auto w-full max-w-2xl -translate-y-1/2 transform px-4"
          >
            <div className="rounded-3xl border border-zinc-800 bg-zinc-950/95 p-6 shadow-2xl shadow-black/40">
              <div className="flex items-center justify-between gap-4 mb-6">
                <div>
                  <h2 className="text-2xl font-semibold">Créer un plan d'atténuation</h2>
                  <p className="text-sm text-zinc-500">Créez un plan pour atténuer un risque</p>
                </div>
                <button type="button" onClick={handleClose} className="rounded-full p-2 text-zinc-400 hover:bg-white/10 hover:text-white transition-colors">
                  <X size={20} />
                </button>
              </div>

              <form onSubmit={handleSubmit(onSubmit)} className="space-y-6">
                <Input label="Titre" {...register('title')} error={errors.title?.message} disabled={isSubmitting} />

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">Description</label>
                  <textarea {...register('description')} rows={4} className="w-full rounded-3xl border border-zinc-800 bg-zinc-950 px-4 py-3 text-sm text-white outline-none focus:ring-2 focus:ring-primary/40" disabled={isSubmitting} />
                  {errors.description && <p className="text-xs text-red-500">{errors.description.message}</p>}
                </div>

                <div className="grid gap-4 sm:grid-cols-2">
                  <div>
                    <label className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">Deadline</label>
                    <Input type="date" {...register('due_date')} disabled={isSubmitting} />
                  </div>
                  <div>
                    <label className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">Priorité</label>
                    <select {...register('priority')} className="w-full rounded-3xl border border-zinc-800 bg-zinc-950 px-4 py-3 text-sm text-white">
                      <option value="critical">Critique</option>
                      <option value="high">Élevé</option>
                      <option value="medium">Moyen</option>
                      <option value="low">Bas</option>
                    </select>
                  </div>
                </div>

                <div className="flex justify-end gap-3">
                  <Button type="button" variant="ghost" onClick={handleClose}>Annuler</Button>
                  <Button type="submit" variant="secondary" isLoading={isSubmitting} className="gap-2"><Zap size={16} />Créer</Button>
                </div>
              </form>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
};

export default CreateMitigationModal;
