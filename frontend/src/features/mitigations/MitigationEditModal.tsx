// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useEffect, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { useForm } from 'react-hook-form';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { api } from '../../lib/api';
import { toast } from 'sonner';
import { useEscapeToClose } from '../../shared/useBackTo';
import { UserPicker } from '../../shared/UserPicker';
import { ownershipPatch, type OwnershipRole } from '../../services/ownershipService';
import { apiErrorMessage } from '../../lib/apiError';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  mitigation: any | null;
  onSaved?: () => void;
}

export const MitigationEditModal = ({ isOpen, onClose, mitigation, onSaved }: Props) => {
  // Esc closes this overlay (spec §2).
  useEscapeToClose(isOpen, onClose);
  const { register, handleSubmit, reset, setValue, formState: { isSubmitting } } = useForm();
  const [newSubTitle, setNewSubTitle] = useState('');

  // The three accountability slots. The API has accepted them since the
  // ownership work (domain.OwnershipPatch is embedded in the mitigation
  // create/update payloads) — this form simply never sent them, which is why a
  // mitigation could not be given an owner from anywhere in the product.
  const [owners, setOwners] = useState<Record<OwnershipRole, string | null>>({
    owner: null, assignee: null, reviewer: null,
  });

  useEffect(() => {
    if (isOpen && mitigation) {
      setValue('title', mitigation.title || '');
      setValue('cost', mitigation.cost || 1);
      setValue('mitigation_time', mitigation.mitigation_time || 1);
      setValue('status', mitigation.status || 'PLANNED');
      if (mitigation.due_date) setValue('due_date', new Date(mitigation.due_date).toISOString().slice(0,10));
    } else {
      reset();
    }
  }, [isOpen, mitigation, setValue, reset]);

  // Sync the ownership slots from the record by adjusting state during render —
  // React's pattern for "reset local state when the source changes". Doing it in
  // the effect above would schedule an extra render pass on every open.
  const [syncedFrom, setSyncedFrom] = useState<unknown>(undefined);
  if (mitigation !== syncedFrom) {
    setSyncedFrom(mitigation);
    setOwners(
      mitigation
        ? {
            owner: mitigation.owner_id ?? null,
            assignee: mitigation.assignee_id ?? null,
            reviewer: mitigation.reviewer_id ?? null,
          }
        : { owner: null, assignee: null, reviewer: null }
    );
  }

  const onSubmit = async (data: any) => {
    if (!mitigation) return;
    try {
      // ownershipPatch keeps the tri-state: an untouched slot is omitted rather
      // than sent as null, so saving this form never silently unassigns the
      // reviewer somebody set elsewhere.
      await api.patch(`/mitigations/${mitigation.id}`, {
        ...data,
        ...ownershipPatch({
          owner: owners.owner ?? null,
          assignee: owners.assignee ?? null,
          reviewer: owners.reviewer ?? null,
        }),
      });
      toast.success('Mitigation sauvegardée');
      onSaved?.();
      onClose();
    } catch (e) {
      const status = (e as any)?.response?.status;
      if (status === 404) {
        toast.error('La mitigation est introuvable (peut-être supprimée). Le modal va se fermer.');
        onSaved?.();
        onClose();
        return;
      }
      toast.error(apiErrorMessage(e) || 'Erreur lors de la sauvegarde');
    }
  };

  const addSubAction = async () => {
    if (!mitigation || !newSubTitle.trim()) return;
    try {
      await api.post(`/mitigations/${mitigation.id}/subactions`, { title: newSubTitle });
      setNewSubTitle('');
      toast.success('Sous-action ajoutée');
      onSaved?.();
    } catch (e) {
      const status = (e as any)?.response?.status;
      if (status === 404) {
        toast.error('La mitigation est introuvable. Le modal va se fermer.');
        onSaved?.();
        onClose();
        return;
      }
      toast.error('Impossible d\'ajouter la sous-action');
    }
  };

  const toggleSub = async (sub: any) => {
    if (!mitigation) return;
    try {
      await api.patch(`/mitigations/${mitigation.id}/subactions/${sub.id}/toggle`);
      onSaved?.();
    } catch (e) {
      const status = (e as any)?.response?.status;
      if (status === 404) {
        toast.error('Sous-action ou mitigation introuvable. Le modal va se fermer.');
        onSaved?.();
        onClose();
        return;
      }
      toast.error('Impossible de basculer la sous-action');
    }
  };

  const deleteSub = async (sub: any) => {
    if (!mitigation) return;
    try {
      await api.delete(`/mitigations/${mitigation.id}/subactions/${sub.id}`);
      toast.success('Sous-action supprimée');
      onSaved?.();
    } catch (e) {
      const status = (e as any)?.response?.status;
      if (status === 404) {
        toast.error('Sous-action ou mitigation introuvable. Le modal va se fermer.');
        onSaved?.();
        onClose();
        return;
      }
      toast.error('Impossible de supprimer la sous-action');
    }
  };

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={onClose} className="fixed inset-0 bg-surface-overlay z-40" />
          <motion.div initial={{ opacity: 0, y: 30 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: 30 }} className="fixed inset-0 m-auto w-full max-w-md h-fit max-h-[90vh] bg-surface border border-border rounded-xl shadow-2xl p-6 z-50 overflow-auto">
            <h3 className="text-lg font-semibold text-text-primary mb-4">Modifier la mitigation</h3>
            <form onSubmit={handleSubmit(onSubmit)} className="space-y-3">
              <Input label="Titre" {...register('title')} />

              <div className="space-y-2">
                <label className="text-xs font-medium text-text-secondary uppercase tracking-wider">
                  Responsabilités
                </label>
                <div className="grid gap-2 sm:grid-cols-3">
                  {(['owner', 'assignee', 'reviewer'] as const).map((role) => (
                    <UserPicker
                      key={role}
                      role={role}
                      value={owners[role]}
                      onChange={(id) => setOwners((prev) => ({ ...prev, [role]: id }))}
                      permission="mitigations:update"
                      size="sm"
                    />
                  ))}
                </div>
              </div>

              <div className="grid grid-cols-2 gap-2">
                {/* Progress is COMPUTED server-side from the sub-actions below
                    (domain.ComputeMitigationProgress) and no endpoint accepts
                    it. It used to be an editable input, which is a control that
                    cannot do what it appears to do — shown read-only instead. */}
                <div className="space-y-1.5">
                  <label className="text-xs font-medium text-text-secondary uppercase tracking-wider">
                    Avancement
                  </label>
                  <div className="h-10 flex items-center px-3 rounded-lg border border-border bg-surface-1 text-sm text-text-secondary">
                    {mitigation?.progress ?? 0}%
                    <span className="ml-2 text-xs text-text-muted">(calculé)</span>
                  </div>
                </div>
                <Input type="number" label="Temps (jours)" {...register('mitigation_time')} />
              </div>
              <div className="grid grid-cols-2 gap-2">
                <Input type="number" label="Coût (1-3)" {...register('cost')} />
                <Input type="date" label="Due date" {...register('due_date')} />
              </div>
              {/* Sub-actions checklist */}
              <div className="mt-3">
                <h4 className="text-sm font-medium text-text-primary mb-2">Checklist</h4>
                <div className="space-y-2">
                  {mitigation?.sub_actions?.length ? mitigation.sub_actions.map((s: any) => (
                    <div key={s.id} className="flex items-center justify-between bg-muted p-2 rounded">
                      <div className="flex items-center gap-2">
                        <input type="checkbox" checked={s.completed} onChange={() => toggleSub(s)} />
                        <span className={s.completed ? 'line-through text-muted-foreground' : ''}>{s.title}</span>
                      </div>
                      <button type="button" className="text-sm text-danger-text" onClick={() => deleteSub(s)}>Supprimer</button>
                    </div>
                  )) : <div className="text-xs text-muted-foreground">Aucune sous-action</div>}
                </div>

                <div className="flex gap-2 mt-2">
                  <Input value={newSubTitle} onChange={(e:any) => setNewSubTitle(e.target.value)} placeholder="Nouvelle sous-action" />
                  <Button type="button" onClick={addSubAction}>Ajouter</Button>
                </div>
              </div>
              <div className="flex justify-end gap-2 mt-4">
                <Button variant="ghost" type="button" onClick={onClose}>Annuler</Button>
                <Button type="submit" isLoading={isSubmitting}>Sauvegarder</Button>
              </div>
            </form>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
};
