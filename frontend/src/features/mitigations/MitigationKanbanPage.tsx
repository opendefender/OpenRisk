// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useEffect, useState, useCallback, useMemo } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  DragDropContext,
  Droppable,
  Draggable,
  type DropResult,
} from '@hello-pangea/dnd';
import { Plus, AlertCircle, Zap } from 'lucide-react';
import { toast } from 'sonner';
import type { Mitigation, MitigationStatus } from '../../types/mitigation';
import { mitigationService } from '../../services/mitigationService';
import { useMitigationStore } from './store';
import { MitigationCard } from './MitigationCard';
import { MitigationDetailDrawer } from './MitigationDetailDrawer';
import { ViewSwitcher } from './ViewSwitcher';
import { MitigationTableView } from './MitigationTableView';
import { MitigationGanttView } from './MitigationGanttView';
import { CreateMitigationModal } from './CreateMitigationModal';
import { Button } from '../../components/ui/Button';
import { EmptyState } from '../../components/shared/EmptyState';
import { useI18n } from '../../hooks/useI18n';
import { cn } from '../../utils/cn';

type KanbanColumn = MitigationStatus;

const KANBAN_COLUMNS: Array<{ id: KanbanColumn; label: string; color: string }> = [
  { id: 'TODO', label: 'À faire', color: 'bg-slate-500/20' },
  { id: 'IN_PROGRESS', label: 'En cours', color: 'bg-blue-500/20' },
  { id: 'REVIEW', label: 'Vérification', color: 'bg-orange-500/20' },
  { id: 'DONE', label: 'Complété', color: 'bg-emerald-500/20' },
];

interface OptimisticUpdate {
  mitigationId: string;
  previousStatus: MitigationStatus;
  newStatus: MitigationStatus;
}

export const MitigationKanbanPage = () => {
  const { t } = useI18n();
  const [mitigations, setMitigations] = useState<Mitigation[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [optimisticUpdates, setOptimisticUpdates] = useState<Map<string, OptimisticUpdate>>(new Map());

  const store = useMitigationStore();
  const { isDrawerOpen, selectedMitigationId, viewMode } = store;

  // Load mitigations
  const loadMitigations = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const response = await mitigationService.listMitigations({
        ...store.filters,
        per_page: 100,
      });
      setMitigations(response.items);
      store.setMitigations(response.items);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Erreur lors du chargement';
      setError(message);
      toast.error(message);
    } finally {
      setIsLoading(false);
    }
  }, [store.filters, store]);

  useEffect(() => {
    loadMitigations();
  }, [loadMitigations]);

  // Group mitigations by status (for Kanban view)
  const groupedMitigations = useMemo(() => {
    const result: Record<KanbanColumn, Mitigation[]> = {
      TODO: [],
      IN_PROGRESS: [],
      REVIEW: [],
      DONE: [],
    };

    const displayMitigations = mitigations.map((m) => {
      const update = optimisticUpdates.get(m.id);
      return update ? { ...m, status: update.newStatus } : m;
    });

    displayMitigations.forEach((m) => {
      result[m.status as KanbanColumn]?.push(m);
    });

    return result;
  }, [mitigations, optimisticUpdates]);

  // Count items pending review or overdue
  const reviewPendingCount = useMemo(() => {
    const reviewItems = groupedMitigations.REVIEW;
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    return reviewItems.filter((m) => {
      const deadline = new Date(m.due_date);
      deadline.setHours(0, 0, 0, 0);
      return deadline < today;
    }).length;
  }, [groupedMitigations]);

  // Handle drag end
  const onDragEnd = useCallback(async (result: DropResult) => {
    const { source, destination, draggableId } = result;

    if (!destination) return;
    if (source.droppableId === destination.droppableId && source.index === destination.index) {
      return;
    }

    const mitigationId = draggableId;
    const mitigation = mitigations.find((m) => m.id === mitigationId);

    if (!mitigation) return;

    const newStatus = destination.droppableId as MitigationStatus;
    const previousStatus = mitigation.status as MitigationStatus;

    // Optimistic update
    setOptimisticUpdates((prev) => {
      const next = new Map(prev);
      next.set(mitigationId, { mitigationId, previousStatus, newStatus });
      return next;
    });

    try {
      await mitigationService.updateMitigation(mitigationId, { status: newStatus });
      toast.success(`Mitigation déplacée vers ${newStatus}`);
      
      // Update local state
      setMitigations((prev) =>
        prev.map((m) => (m.id === mitigationId ? { ...m, status: newStatus } : m))
      );

      // Clear optimistic update
      setOptimisticUpdates((prev) => {
        const next = new Map(prev);
        next.delete(mitigationId);
        return next;
      });
    } catch (err) {
      // Rollback
      toast.error('Erreur lors du déplacement. Tentative annulée.');
      setOptimisticUpdates((prev) => {
        const next = new Map(prev);
        next.delete(mitigationId);
        return next;
      });
    }
  }, [mitigations]);

  const selectedMitigation = useMemo(
    () => mitigations.find((m) => m.id === selectedMitigationId) || null,
    [mitigations, selectedMitigationId]
  );

  if (error) {
    return (
      <div className="flex-1 overflow-auto p-6">
        <EmptyState
          icon={<AlertCircle size={48} />}
          title="Erreur lors du chargement"
          description={error}
          action={{
            label: 'Réessayer',
            onClick: () => window.location.reload(),
          }}
        />
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {/* Header */}
      <div className="shrink-0 border-b border-border bg-background/80 backdrop-blur-md px-6 py-4 flex items-center justify-between gap-4">
        <div className="flex-1">
          <h1 className="text-2xl font-bold text-white">Plans d'atténuation</h1>
          <p className="text-sm text-zinc-400 mt-1">Gérez et suivez vos plans d'atténuation des risques</p>
        </div>
        <div className="flex items-center gap-3">
          <ViewSwitcher />
          <Button onClick={() => setIsCreateOpen(true)}>
            <Plus size={16} />
            <span className="hidden sm:inline ml-2">Nouveau plan</span>
          </Button>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto">
        {isLoading ? (
          <div className="flex items-center justify-center h-full">
            <motion.div animate={{ rotate: 360 }} transition={{ duration: 2, repeat: Infinity }}>
              <Zap className="text-blue-500" size={32} />
            </motion.div>
          </div>
        ) : mitigations.length === 0 ? (
          <div className="flex items-center justify-center h-full">
            <EmptyState
              icon="📋"
              title="Aucun plan d'atténuation"
              description="Créez votre premier plan pour commencer à gérer vos risques"
              action={{
                label: 'Créer un plan',
                onClick: () => setIsCreateOpen(true),
              }}
            />
          </div>
        ) : viewMode === 'kanban' ? (
          // KANBAN VIEW
          <DragDropContext onDragEnd={onDragEnd}>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 p-6 auto-cols-max">
              {KANBAN_COLUMNS.map((column) => {
                const items = groupedMitigations[column.id] || [];
                const isReviewColumn = column.id === 'REVIEW';
                const hasReviewPending = isReviewColumn && reviewPendingCount > 0;

                return (
                  <motion.div
                    key={column.id}
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: KANBAN_COLUMNS.indexOf(column) * 0.1 }}
                    className="flex flex-col h-full min-w-72 rounded-lg overflow-hidden"
                  >
                    {/* Column Header */}
                    <div className={cn(
                      'px-4 py-3 border-b border-zinc-700',
                      isReviewColumn ? 'bg-orange-500/20' : 'bg-zinc-800/50'
                    )}>
                      <div className="flex items-center justify-between">
                        <h2 className={cn(
                          'font-semibold text-sm',
                          isReviewColumn ? 'text-orange-400' : 'text-white'
                        )}>
                          {column.label}
                        </h2>
                        <div className="flex items-center gap-2">
                          <span className={cn(
                            'text-xs font-medium px-2 py-1 rounded-full',
                            isReviewColumn ? 'bg-orange-500/30 text-orange-300' : 'bg-zinc-700 text-zinc-300'
                          )}>
                            {items.length}
                          </span>
                          {hasReviewPending && (
                            <motion.span
                              animate={{ scale: [1, 1.1, 1] }}
                              transition={{ duration: 1.5, repeat: Infinity }}
                              className="text-xs font-bold px-2 py-1 rounded-full bg-red-500/30 text-red-400"
                            >
                              {reviewPendingCount} en retard
                            </motion.span>
                          )}
                        </div>
                      </div>
                    </div>

                    {/* Drop Zone */}
                    <Droppable droppableId={column.id}>
                      {(provided, snapshot) => (
                        <div
                          ref={provided.innerRef}
                          {...provided.droppableProps}
                          className={cn(
                            'flex-1 p-3 space-y-3 overflow-y-auto',
                            snapshot.isDraggingOver ? 'bg-zinc-800/30' : 'bg-zinc-900/20'
                          )}
                        >
                          <AnimatePresence>
                            {items.map((mitigation, index) => (
                              <Draggable
                                key={mitigation.id}
                                draggableId={mitigation.id}
                                index={index}
                              >
                                {(provided, snapshot) => (
                                  <div
                                    ref={provided.innerRef}
                                    {...provided.draggableProps}
                                    {...provided.dragHandleProps}
                                  >
                                    <MitigationCard
                                      mitigation={mitigation}
                                      isDragging={snapshot.isDragging}
                                      isSelected={selectedMitigationId === mitigation.id}
                                      onClick={() => store.openDrawer(mitigation.id)}
                                    />
                                  </div>
                                )}
                              </Draggable>
                            ))}
                          </AnimatePresence>

                          {items.length === 0 && (
                            <div className="text-center py-8 text-zinc-500 text-sm">
                              Glissez des plans ici
                            </div>
                          )}

                          {provided.placeholder}
                        </div>
                      )}
                    </Droppable>
                  </motion.div>
                );
              })}
            </div>
          </DragDropContext>
        ) : viewMode === 'table' ? (
          // TABLE VIEW
          <div className="p-6">
            <MitigationTableView
              mitigations={mitigations}
              isLoading={isLoading}
              onRowClick={(m) => store.openDrawer(m.id)}
            />
          </div>
        ) : (
          // GANTT VIEW
          <div className="p-6">
            <MitigationGanttView
              mitigations={mitigations}
              isLoading={isLoading}
              onRowClick={(m) => store.openDrawer(m.id)}
            />
          </div>
        )}
      </div>

      {/* Detail Drawer */}
      <MitigationDetailDrawer
        isOpen={isDrawerOpen}
        mitigation={selectedMitigation}
        onClose={() => store.closeDrawer()}
      />
      <CreateMitigationModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        onCreated={() => loadMitigations()}
      />
    </div>
  );
};
