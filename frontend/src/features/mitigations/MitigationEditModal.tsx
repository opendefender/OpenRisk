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
      setValue('progress', mitigation.progress || );
      setValue('cost', mitigation.cost || );
      setValue('mitigation_time', mitigation.mitigation_time || );
      setValue('status', mitigation.status || 'PLANNED');
      if (mitigation.due_date) setValue('due_date', new Date(mitigation.due_date).toISOString().slice(,));
    } else {
      reset();
    }
  }, [isOpen, mitigation, setValue, reset]);

  const onSubmit = async (data: any) => {
    if (!mitigation) return;
    try {
      await api.patch(/mitigations/${mitigation.id}, data);
      toast.success('Mitigation sauvegarde');
      onSaved?.();
      onClose();
    } catch (e) {
      const status = (e as any)?.response?.status;
      if (status === ) {
        toast.error('La mitigation est introuvable (peut-être supprime). Le modal va se fermer.');
        onSaved?.();
        onClose();
        return;
      }
      toast.error('Erreur lors de la sauvegarde');
    }
  };

  const addSubAction = async () => {
    if (!mitigation || !newSubTitle.trim()) return;
    try {
      await api.post(/mitigations/${mitigation.id}/subactions, { title: newSubTitle });
      setNewSubTitle('');
      toast.success('Sous-action ajoute');
      onSaved?.();
    } catch (e) {
      const status = (e as any)?.response?.status;
      if (status === ) {
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
      await api.patch(/mitigations/${mitigation.id}/subactions/${sub.id}/toggle);
      onSaved?.();
    } catch (e) {
      const status = (e as any)?.response?.status;
      if (status === ) {
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
      await api.delete(/mitigations/${mitigation.id}/subactions/${sub.id});
      toast.success('Sous-action supprime');
      onSaved?.();
    } catch (e) {
      const status = (e as any)?.response?.status;
      if (status === ) {
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
          <motion.div initial={{ opacity:  }} animate={{ opacity:  }} exit={{ opacity:  }} onClick={onClose} className="fixed inset- bg-black/ z-" />
          <motion.div initial={{ opacity: , y:  }} animate={{ opacity: , y:  }} exit={{ opacity: , y:  }} className="fixed inset- m-auto w-full max-w-md h-fit max-h-[vh] bg-surface border border-border rounded-xl shadow-xl p- z- overflow-auto">
            <h className="text-lg font-semibold text-white mb-">Modifier la mitigation</h>
            <form onSubmit={handleSubmit(onSubmit)} className="space-y-">
              <Input label="Titre" {...register('title')} />
              <Input label="Assign à" {...register('assignee')} />
              <div className="grid grid-cols- gap-">
                <Input type="number" label="Progress (%)" {...register('progress')} />
                <Input type="number" label="Temps (jours)" {...register('mitigation_time')} />
              </div>
              <div className="grid grid-cols- gap-">
                <Input type="number" label="Coût (-)" {...register('cost')} />
                <Input type="date" label="Due date" {...register('due_date')} />
              </div>
              {/ Sub-actions checklist /}
              <div className="mt-">
                <h className="text-sm font-medium text-white mb-">Checklist</h>
                <div className="space-y-">
                  {mitigation?.sub_actions?.length ? mitigation.sub_actions.map((s: any) => (
                    <div key={s.id} className="flex items-center justify-between bg-muted p- rounded">
                      <div className="flex items-center gap-">
                        <input type="checkbox" checked={s.completed} onChange={() => toggleSub(s)} />
                        <span className={s.completed ? 'line-through text-muted-foreground' : ''}>{s.title}</span>
                      </div>
                      <button type="button" className="text-sm text-red-" onClick={() => deleteSub(s)}>Supprimer</button>
                    </div>
                  )) : <div className="text-xs text-muted-foreground">Aucune sous-action</div>}
                </div>

                <div className="flex gap- mt-">
                  <Input value={newSubTitle} onChange={(e:any) => setNewSubTitle(e.target.value)} placeholder="Nouvelle sous-action" />
                  <Button type="button" onClick={addSubAction}>Ajouter</Button>
                </div>
              </div>
              <div className="flex justify-end gap- mt-">
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
