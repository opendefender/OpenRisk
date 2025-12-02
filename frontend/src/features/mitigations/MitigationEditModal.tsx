import { useEffect, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { useForm } from 'react-hook-form';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';
import { api } from '../../lib/api';
import { toast } from 'sonner';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  mitigation: any | null;
  onSaved?: () => void;
}

export const MitigationEditModal = ({ isOpen, onClose, mitigation, onSaved }: Props) => {
  const { register, handleSubmit, reset, setValue, formState: { isSubmitting } } = useForm();
  const [newSubTitle, setNewSubTitle] = useState('');

  useEffect(() => {
    if (isOpen && mitigation) {
      setValue('title', mitigation.title || '');
      setValue('assignee', mitigation.assignee || '');
      setValue('progress', mitigation.progress || 0);
      setValue('cost', mitigation.cost || 1);
      setValue('mitigation_time', mitigation.mitigation_time || 1);
      setValue('status', mitigation.status || 'PLANNED');
      if (mitigation.due_date) setValue('due_date', new Date(mitigation.due_date).toISOString().slice(0,10));
    } else {
      reset();
    }
  }, [isOpen, mitigation, setValue, reset]);

  const onSubmit = async (data: any) => {
    if (!mitigation) return;
    try {
      await api.patch(`/mitigations/${mitigation.id}`, data);
      toast.success('Mitigation sauvegardée');
      onSaved?.();
      onClose();
    } catch (e) {
      toast.error('Erreur lors de la sauvegarde');
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
      toast.error('Impossible d\'ajouter la sous-action');
    }
  };

  const toggleSub = async (sub: any) => {
    if (!mitigation) return;
    try {
      await api.patch(`/mitigations/${mitigation.id}/subactions/${sub.id}/toggle`);
      onSaved?.();
    } catch (e) {
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
      toast.error('Impossible de supprimer la sous-action');
    }
  };

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={onClose} className="fixed inset-0 bg-black/60 z-40" />
          <motion.div initial={{ opacity: 0, y: 30 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: 30 }} className="fixed inset-0 m-auto w-full max-w-md h-fit max-h-[90vh] bg-surface border border-border rounded-xl shadow-2xl p-6 z-50 overflow-auto">
            <h3 className="text-lg font-semibold text-white mb-4">Modifier la mitigation</h3>
            <form onSubmit={handleSubmit(onSubmit)} className="space-y-3">
              <Input label="Titre" {...register('title')} />
              <Input label="Assigné à" {...register('assignee')} />
              <div className="grid grid-cols-2 gap-2">
                <Input type="number" label="Progress (%)" {...register('progress')} />
                <Input type="number" label="Temps (jours)" {...register('mitigation_time')} />
              </div>
              <div className="grid grid-cols-2 gap-2">
                <Input type="number" label="Coût (1-3)" {...register('cost')} />
                <Input type="date" label="Due date" {...register('due_date')} />
              </div>
              {/* Sub-actions checklist */}
              <div className="mt-3">
                <h4 className="text-sm font-medium text-white mb-2">Checklist</h4>
                <div className="space-y-2">
                  {mitigation?.sub_actions?.length ? mitigation.sub_actions.map((s: any) => (
                    <div key={s.id} className="flex items-center justify-between bg-muted p-2 rounded">
                      <div className="flex items-center gap-2">
                        <input type="checkbox" checked={s.completed} onChange={() => toggleSub(s)} />
                        <span className={s.completed ? 'line-through text-muted-foreground' : ''}>{s.title}</span>
                      </div>
                      <button type="button" className="text-sm text-red-400" onClick={() => deleteSub(s)}>Supprimer</button>
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
