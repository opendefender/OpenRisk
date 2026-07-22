// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useState, useCallback, useMemo } from 'react';
import {
  DragDropContext,
  Droppable,
  Draggable,
  type DropResult,
} from '@hello-pangea/dnd';
import { motion } from 'framer-motion';
import { AlertCircle, RotateCcw, Zap, CheckCircle2 } from 'lucide-react';
import { toast } from 'sonner';
import type { SubAction, SubActionStatus } from '../../types/mitigation';
import { mitigationService } from '../../services/mitigationService';
import { Button } from '../../components/ui/Button';
import { StatusDot } from '../../components/shared/StatusDot';
import { AutoDetectedBadge } from '../../components/shared/AutoDetectedBadge';
import { useI18n } from '../../hooks/useI18n';
import { cn } from '../../utils/cn';

interface SubActionTableProps {
  mitigationId: string;
  subActions: SubAction[];
  onUpdate: (subActions: SubAction[]) => void;
}

export const SubActionTable = ({ mitigationId, subActions, onUpdate }: SubActionTableProps) => {
  const { t } = useI18n();
  const [optimisticUpdates, setOptimisticUpdates] = useState<Map<string, SubAction>>(new Map());
  const [revertingIds, setRevertingIds] = useState<Set<string>>(new Set());

  // Apply optimistic updates to display
  const displaySubActions = useMemo(() => {
    return subActions.map((sa) => optimisticUpdates.get(sa.id) || sa);
  }, [subActions, optimisticUpdates]);

  // Check dependency violations when reordering
  const canReorderTo = useCallback((action: SubAction, newIndex: number): boolean => {
    if (!action.depends_on?.length) return true;

    // Find the new position in the reordered list
    const beforeReorder = displaySubActions.slice(0, newIndex);
    const dependencyIds = action.depends_on;

    // Check if all dependencies are before this action
    for (const depId of dependencyIds) {
      const depIndex = beforeReorder.findIndex((sa) => sa.id === depId);
      if (depIndex === -1) return false;
    }

    return true;
  }, [displaySubActions]);

  // Handle drag end (reordering)
  const onDragEnd = useCallback(async (result: DropResult) => {
    const { source, destination, draggableId } = result;

    if (!destination) return;
    if (source.index === destination.index) return;

    const subAction = displaySubActions[source.index];
    if (!subAction) return;

    // Validate dependencies
    if (!canReorderTo(subAction, destination.index)) {
      toast.error('❌ Violation de dépendance détectée. Cette sous-action dépend d\'autres qui seraient après elle.');
      return;
    }

    // Reorder in local state
    const reordered = Array.from(displaySubActions);
    reordered.splice(source.index, 1);
    reordered.splice(destination.index, 0, subAction);

    // Apply optimistic update
    onUpdate(reordered);

    try {
      // Call API to update order
      await mitigationService.reorderSubActions(mitigationId, reordered.map((sa) => sa.id));
      toast.success('Sous-actions réorganisées');
    } catch (err) {
      // Rollback
      onUpdate(subActions);
      toast.error('Erreur lors de la réorganisation');
    }
  }, [mitigationId, displaySubActions, subActions, canReorderTo, onUpdate]);

  // Handle toggle completed
  const handleToggleCompleted = useCallback(
    async (subAction: SubAction) => {
      const newStatus: SubActionStatus = subAction.status === 'DONE' ? 'TODO' : 'DONE';

      // Optimistic update
      setOptimisticUpdates((prev) => {
        const next = new Map(prev);
        next.set(subAction.id, { ...subAction, status: newStatus });
        return next;
      });

      try {
        await mitigationService.updateSubAction(mitigationId, subAction.id, {
          status: newStatus,
        });

        // Update parent
        const updated = displaySubActions.map((sa) =>
          sa.id === subAction.id ? { ...sa, status: newStatus } : sa
        );
        onUpdate(updated);

        // Clear optimistic
        setOptimisticUpdates((prev) => {
          const next = new Map(prev);
          next.delete(subAction.id);
          return next;
        });

        toast.success(
          newStatus === 'DONE'
            ? 'Sous-action marquée comme complétée'
            : 'Sous-action marquée comme en attente'
        );
      } catch (err) {
        // Rollback
        setOptimisticUpdates((prev) => {
          const next = new Map(prev);
          next.delete(subAction.id);
          return next;
        });
        toast.error('Erreur lors de la mise à jour');
      }
    },
    [mitigationId, displaySubActions, onUpdate]
  );

  // Handle revert (auto-detected only)
  const handleRevert = useCallback(
    async (subAction: SubAction) => {
      if (subAction.completed_source !== 'scanner') {
        toast.error('Cette action ne peut pas être annulée');
        return;
      }

      setRevertingIds((prev) => new Set([...prev, subAction.id]));

      try {
        await mitigationService.revertSubAction(mitigationId, subAction.id);

        // Update parent
        const updated = displaySubActions.filter((sa) => sa.id !== subAction.id);
        onUpdate(updated);

        toast.success('Auto-détection annulée');
      } catch (err) {
        toast.error('Erreur lors de l\'annulation');
      } finally {
        setRevertingIds((prev) => {
          const next = new Set(prev);
          next.delete(subAction.id);
          return next;
        });
      }
    },
    [mitigationId, displaySubActions, onUpdate]
  );

  return (
    <DragDropContext onDragEnd={onDragEnd}>
      <Droppable droppableId="sub-actions">
        {(provided, snapshot) => (
          <div
            ref={provided.innerRef}
            {...provided.droppableProps}
            className={cn('space-y-2', snapshot.isDraggingOver && 'bg-blue-500/10 rounded-lg p-2')}
          >
            {displaySubActions.length === 0 ? (
              <div className="text-center py-8 text-zinc-500 text-sm">
                Aucune sous-action
              </div>
            ) : (
              displaySubActions.map((subAction, index) => {
                const isCompleted = subAction.status === 'DONE';
                const isAutoDetected = subAction.completed_source === 'scanner';
                const isReverting = revertingIds.has(subAction.id);

                return (
                  <Draggable
                    key={subAction.id}
                    draggableId={subAction.id}
                    index={index}
                  >
                    {(provided, snapshot) => (
                      <div
                        ref={provided.innerRef}
                        {...provided.draggableProps}
                        {...provided.dragHandleProps}
                        className={cn(
                          'flex items-center gap-3 p-3 rounded-lg border border-zinc-700 bg-zinc-800/40 transition-all',
                          snapshot.isDragging && 'bg-blue-500/20 shadow-lg',
                          (subAction.depends_on?.length || 0) > 0 && 'border-l-2 border-l-yellow-500',
                          isReverting && 'opacity-50'
                        )}
                      >
                        {/* Checkbox */}
                        <button
                          onClick={() => handleToggleCompleted(subAction)}
                          className="flex-shrink-0 p-1 hover:bg-zinc-700 rounded transition-colors"
                        >
                          {isCompleted ? (
                            <CheckCircle2 size={20} className="text-emerald-500" />
                          ) : (
                            <div className="w-5 h-5 border-2 border-zinc-600 rounded-full hover:border-zinc-500" />
                          )}
                        </button>

                        {/* Title & Dependencies */}
                        <div className="flex-1 min-w-0">
                          <p className={cn('text-sm font-medium truncate', isCompleted && 'line-through text-zinc-500')}>
                            {subAction.title}
                          </p>
                          {(subAction.depends_on?.length || 0) > 0 && (
                            <p className="text-xs text-yellow-600 mt-1">
                              ⚠️ Dépend de {subAction.depends_on?.length || 0} autre(s)
                            </p>
                          )}
                        </div>

                        {/* Source Badge */}
                        <div className="flex-shrink-0">
                          {isAutoDetected ? (
                            <AutoDetectedBadge
                              detectedAt={subAction.completed_at || new Date().toISOString()}
                              scanId={subAction.scanner_details?.scan_id || 'unknown'}
                            />
                          ) : (
                            <span className="text-xs px-2 py-1 rounded-full bg-blue-500/20 text-blue-400">
                              Manuel
                            </span>
                          )}
                        </div>

                        {/* Revert Button (auto-detected only) */}
                        {isAutoDetected && (
                          <Button
                            
                            variant="ghost"
                            onClick={() => handleRevert(subAction)}
                            disabled={isReverting}
                            title="Annuler cette auto-détection"
                          >
                            <RotateCcw size={16} />
                          </Button>
                        )}

                        {/* Evidence Link */}
                        {(subAction.evidence_ids?.length || 0) > 0 && (
                          <a
                            href={`#evidence-${subAction.evidence_ids?.[0] || ''}`}
                            className="text-xs text-blue-400 hover:text-blue-300"
                          >
                            Preuve
                          </a>
                        )}
                      </div>
                    )}
                  </Draggable>
                );
              })
            )}

            {provided.placeholder}
          </div>
        )}
      </Droppable>
    </DragDropContext>
  );
};
