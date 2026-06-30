// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useState, useEffect, useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { X, FileText, MessageCircle } from 'lucide-react';
import { toast } from 'sonner';
import type { Mitigation, SubAction } from '../../types/mitigation';
import { mitigationService } from '../../services/mitigationService';
import { SubActionTable } from './SubActionTable';
import { Button } from '../../components/ui/Button';
import { ProgressBar } from '../../components/shared/ProgressBar';
import { useI18n } from '../../hooks/useI18n';
import { cn } from '../../utils/cn';

interface MitigationDetailDrawerProps {
  isOpen: boolean;
  mitigation: Mitigation | null;
  onClose: () => void;
}

const getStatusColor = (status: string) => {
  switch (status) {
    case 'TODO':
      return 'bg-slate-500';
    case 'IN_PROGRESS':
      return 'bg-blue-500';
    case 'REVIEW':
      return 'bg-orange-500';
    case 'DONE':
      return 'bg-emerald-500';
    default:
      return 'bg-zinc-500';
  }
};

export const MitigationDetailDrawer = ({
  isOpen,
  mitigation,
  onClose,
}: MitigationDetailDrawerProps) => {
  const { t } = useI18n();
  const [activeTab, setActiveTab] = useState<'overview' | 'sub-actions' | 'evidence' | 'timeline' | 'ai'>('overview');
  const [isEditingDescription, setIsEditingDescription] = useState(false);
  const [descriptionValue, setDescriptionValue] = useState('');
  const [isUpdating, setIsUpdating] = useState(false);
  const [subActions, setSubActions] = useState<SubAction[]>([]);

  useEffect(() => {
    if (mitigation?.sub_actions) {
      setSubActions(mitigation.sub_actions);
      setDescriptionValue(mitigation.description || '');
    }
  }, [mitigation?.id, mitigation?.sub_actions, mitigation?.description]);

  const handleSaveDescription = async () => {
    if (!mitigation) return;

    setIsUpdating(true);
    try {
      await mitigationService.updateMitigation(mitigation.id, {
        description: descriptionValue,
      });
      toast.success('Description mise à jour');
      setIsEditingDescription(false);
    } catch (err) {
      toast.error('Erreur lors de la mise à jour');
      setDescriptionValue(mitigation.description || '');
    } finally {
      setIsUpdating(false);
    }
  };

  const progress = useMemo(() => {
    if (!subActions.length) return 0;
    return Math.round((subActions.filter((sa: SubAction) => sa.status === 'DONE').length / subActions.length) * 100);
  }, [subActions]);

  const autoDetectedCount = useMemo(() => {
    return subActions.filter((sa: SubAction) => sa.completed_source === 'scanner').length;
  }, [subActions]);

  const completedCount = useMemo(() => {
    return subActions.filter((sa: SubAction) => sa.status === 'DONE').length;
  }, [subActions]);

  if (!isOpen || !mitigation) return null;

  return (
    <AnimatePresence>
      <div className="fixed inset-0 z-50 flex pointer-events-none">
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          onClick={onClose}
          className="absolute inset-0 bg-black/40 pointer-events-auto"
        />

        <motion.div
          initial={{ x: 620 }}
          animate={{ x: 0 }}
          exit={{ x: 620 }}
          transition={{ type: 'spring', damping: 25, stiffness: 200 }}
          className="ml-auto w-full max-w-[620px] h-full bg-zinc-900 border-l border-zinc-700 flex flex-col pointer-events-auto overflow-hidden"
        >
          <div className="shrink-0 border-b border-zinc-700 px-6 py-4 flex items-center justify-between">
            <h2 className="text-lg font-bold text-white">Détails du plan</h2>
            <button
              onClick={onClose}
              className="p-1 hover:bg-zinc-700 rounded transition-colors"
            >
              <X size={20} className="text-zinc-400" />
            </button>
          </div>

          <div className="shrink-0 border-b border-zinc-700 px-6 flex gap-1 overflow-x-auto">
            {(['overview', 'sub-actions', 'evidence', 'timeline', 'ai'] as const).map((tab) => (
              <button
                key={tab}
                onClick={() => setActiveTab(tab)}
                className={cn(
                  'px-4 py-3 text-sm font-medium whitespace-nowrap border-b-2 transition-colors',
                  activeTab === tab
                    ? 'border-blue-500 text-white'
                    : 'border-transparent text-zinc-400 hover:text-zinc-300'
                )}
              >
                {tab === 'overview' && 'Aperçu'}
                {tab === 'sub-actions' && 'Sous-actions'}
                {tab === 'evidence' && 'Preuves'}
                {tab === 'timeline' && 'Historique'}
                {tab === 'ai' && 'IA'}
              </button>
            ))}
          </div>

          <div className="flex-1 overflow-y-auto px-6 py-4">
            <div className="space-y-6">
              {activeTab === 'overview' && (
                <div className="space-y-4">
                  <div className="space-y-2">
                    <h3 className="text-xl font-bold text-white">{mitigation.title}</h3>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="text-xs text-zinc-500 mb-1">Statut</p>
                      <div className="flex items-center gap-2">
                        <div className={cn('w-2 h-2 rounded-full', getStatusColor(mitigation.status))} />
                        <span className="text-sm text-white capitalize">
                          {t(`mitigations.status.${mitigation.status.toLowerCase()}`)}
                        </span>
                      </div>
                    </div>
                    <div>
                      <p className="text-xs text-zinc-500 mb-1">Priorité</p>
                      <span className="text-sm text-white capitalize">
                        {t(`mitigations.priority.${mitigation.priority.toLowerCase()}`)}
                      </span>
                    </div>
                  </div>

                  <div>
                    <p className="text-xs text-zinc-500 mb-1">Échéance</p>
                    <p className="text-sm text-white">
                      {new Date(mitigation.due_date).toLocaleDateString('fr-FR', {
                        year: 'numeric',
                        month: 'long',
                        day: 'numeric',
                      })}
                    </p>
                  </div>

                  <div>
                    <p className="text-xs text-zinc-500 mb-2">Progression</p>
                    <ProgressBar
                      value={progress}
                      max={100}
                      showPercentage
                      animated
                    />
                    <p className="text-xs text-zinc-400 mt-2">
                      {completedCount}/{subActions.length} actions
                      {autoDetectedCount > 0 && ` (${autoDetectedCount} auto-détectées)`}
                    </p>
                  </div>

                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <p className="text-xs text-zinc-500">Description</p>
                      {!isEditingDescription && (
                        <button
                          onClick={() => setIsEditingDescription(true)}
                          className="text-xs text-blue-400 hover:text-blue-300"
                        >
                          Modifier
                        </button>
                      )}
                    </div>

                    {isEditingDescription ? (
                      <div className="space-y-2">
                        <textarea
                          value={descriptionValue}
                          onChange={(e) => setDescriptionValue(e.currentTarget.value)}
                          placeholder="Description..."
                          className="w-full min-h-[100px] px-3 py-2 bg-zinc-800 border border-zinc-700 rounded text-sm text-white placeholder:text-zinc-500 focus:outline-none focus:border-blue-500"
                        />
                        <div className="flex gap-2">
                          <Button
                            onClick={handleSaveDescription}
                            disabled={isUpdating}
                          >
                            Enregistrer
                          </Button>
                          <Button
                            variant="secondary"
                            onClick={() => {
                              setIsEditingDescription(false);
                              setDescriptionValue(mitigation.description || '');
                            }}
                          >
                            Annuler
                          </Button>
                        </div>
                      </div>
                    ) : (
                      <p className="text-sm text-zinc-300">
                        {mitigation.description || 'Aucune description'}
                      </p>
                    )}
                  </div>

                  <div>
                    <p className="text-xs text-zinc-500 mb-2">Assignés</p>
                    <div className="flex gap-2">
                      {mitigation.assigned_to_user ? (
                        <div
                          className="w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center text-xs text-white font-medium"
                          title={mitigation.assigned_to_user.name}
                        >
                          {mitigation.assigned_to_user.name.slice(0, 1).toUpperCase()}
                        </div>
                      ) : (
                        <p className="text-sm text-zinc-500">Aucun assigné</p>
                      )}
                    </div>
                  </div>
                </div>
              )}

              {activeTab === 'sub-actions' && (
                <SubActionTable
                  mitigationId={mitigation.id}
                  subActions={subActions}
                  onUpdate={setSubActions}
                />
              )}

              {activeTab === 'evidence' && (
                <div className="space-y-3">
                  <p className="text-sm text-zinc-500 text-center py-8">
                    Aucune preuve ajoutée
                  </p>
                </div>
              )}

              {activeTab === 'timeline' && (
                <div className="space-y-3">
                  <p className="text-sm text-zinc-500 text-center py-8">
                    Aucun événement
                  </p>
                </div>
              )}

              {activeTab === 'ai' && (
                <div className="space-y-4">
                  <div className="p-4 rounded-lg bg-blue-500/10 border border-blue-500/30">
                    <div className="flex items-start gap-3">
                      <MessageCircle size={16} className="text-blue-400 flex-shrink-0 mt-1" />
                      <div>
                        <p className="text-sm font-medium text-white mb-2">
                          Suggestions de l'IA
                        </p>
                        <p className="text-xs text-zinc-300">
                          L'assistant IA peut vous proposer des actions pour optimiser ce plan d'atténuation.
                        </p>
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>
        </motion.div>
      </div>
    </AnimatePresence>
  );
};
