// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { motion, AnimatePresence } from 'framer-motion';
import { X, Trash2, Check } from 'lucide-react';
import { Button } from '../ui/Button';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

interface FloatingBulkBarProps {
  selectedCount: number;
  isVisible: boolean;
  onCancel: () => void;
  onConfirm: () => void;
  onAction?: (action: string) => void;
  actions?: Array<{ id: string; label: string; icon: React.ReactNode; variant?: 'default' | 'danger' }>;
  isLoading?: boolean;
  className?: string;
}

export const FloatingBulkBar = ({
  selectedCount,
  isVisible,
  onCancel,
  onConfirm,
  onAction,
  actions = [
    { id: 'delete', label: 'Supprimer', icon: <Trash2 size={16} />, variant: 'danger' as const },
  ],
  isLoading = false,
  className,
}: FloatingBulkBarProps) => {
  return (
    <AnimatePresence>
      {isVisible && selectedCount > 0 && (
        <motion.div
          initial={{ y: 100, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          exit={{ y: 100, opacity: 0 }}
          transition={{ type: 'spring', damping: 25, stiffness: 200 }}
          className={cn(
            'fixed bottom-6 left-1/2 transform -translate-x-1/2 z-40',
            'bg-gradient-to-r from-zinc-900 to-zinc-800 border border-primary/50',
            'rounded-lg shadow-2xl px-6 py-4 backdrop-blur-xl',
            className
          )}
        >
          <div className="flex items-center justify-between gap-6 max-w-2xl">
            {/* Left: Selection count and checkbox */}
            <div className="flex items-center gap-3">
              <motion.div
                initial={{ scale: 0 }}
                animate={{ scale: 1 }}
                className="flex items-center justify-center w-6 h-6 rounded-full bg-primary text-white text-xs font-bold"
              >
                {selectedCount}
              </motion.div>
              <span className="text-sm text-zinc-300">
                {selectedCount} {selectedCount === 1 ? 'risque sélectionné' : 'risques sélectionnés'}
              </span>
            </div>

            {/* Center: Quick actions */}
            <div className="flex items-center gap-2 border-l border-r border-zinc-700 px-4 py-2">
              {actions.map((action) => (
                <motion.button
                  key={action.id}
                  whileHover={{ scale: 1.05 }}
                  whileTap={{ scale: 0.95 }}
                  onClick={() => onAction?.(action.id)}
                  disabled={isLoading}
                  className={cn(
                    'flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors',
                    action.variant === 'danger'
                      ? 'hover:bg-red-500/20 text-red-400'
                      : 'hover:bg-zinc-700 text-zinc-300'
                  )}
                  title={action.label}
                >
                  {action.icon}
                  <span className="hidden sm:inline">{action.label}</span>
                </motion.button>
              ))}
            </div>

            {/* Right: Confirm/Cancel buttons */}
            <div className="flex items-center gap-2 ml-auto">
              <Button
                variant="ghost"
                size="sm"
                onClick={onCancel}
                disabled={isLoading}
                className="gap-1.5"
              >
                <X size={16} />
                <span className="hidden sm:inline">Annuler</span>
              </Button>
              <Button
                variant="secondary"
                size="sm"
                onClick={onConfirm}
                isLoading={isLoading}
                className="gap-1.5 bg-primary hover:bg-primary/90"
              >
                <Check size={16} />
                <span className="hidden sm:inline">Confirmer</span>
              </Button>
            </div>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
};
