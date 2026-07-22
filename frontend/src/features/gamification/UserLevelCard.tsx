// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useEffect } from 'react';
import { Trophy, Star, Target, Crown, AlertCircle } from 'lucide-react';
import { motion } from 'framer-motion';
import { useGamificationStore } from '../../hooks/useGamificationStore';

const LEVEL_COLORS = {
  1: 'from-green-500 to-teal-600',
  2: 'from-blue-500 to-cyan-600',
  3: 'from-purple-500 to-indigo-600',
  4: 'from-pink-500 to-rose-600',
  5: 'from-orange-500 to-red-600',
};

const getBadgeIcon = (iconName: string) => {
  const icons: Record<string, React.ReactNode> = {
    Flag: <Target className="w-5 h-5" />,
    ShieldCheck: <Trophy className="w-5 h-5" />,
    Brain: <Star className="w-5 h-5" />,
    Crown: <Crown className="w-5 h-5" />,
  };
  return icons[iconName] || <Star className="w-5 h-5" />;
};

const getLevelColor = (level: number): string => {
  if (level >= 5) return LEVEL_COLORS[5];
  if (level >= 4) return LEVEL_COLORS[4];
  if (level >= 3) return LEVEL_COLORS[3];
  if (level >= 2) return LEVEL_COLORS[2];
  return LEVEL_COLORS[1];
};

export const UserLevelCard = () => {
  const { stats, loading, error, fetchStats } = useGamificationStore();

  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  if (loading) {
    return (
      <div className="bg-white/5 backdrop-blur-xl border border-white/10 rounded-xl p-8 animate-pulse">
        <div className="h-48 bg-white/10 rounded-lg" />
      </div>
    );
  }

  if (error || !stats) {
    return (
      <div className="bg-white/5 backdrop-blur-xl border border-white/10 rounded-xl p-8">
        <div className="flex items-center gap-3 text-orange-400">
          <AlertCircle className="w-5 h-5" />
          <p className="text-sm">{error || 'Impossible de charger les données'}</p>
        </div>
      </div>
    );
  }

  const levelGradient = getLevelColor(stats.level);

  return (
    <div className="space-y-6">
      {/* Main Level Card */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3 }}
        className="bg-gradient-to-br from-white/10 to-white/5 backdrop-blur-xl border border-white/20 rounded-2xl overflow-hidden shadow-2xl"
      >
        <div className={`h-32 bg-gradient-to-r ${levelGradient} opacity-20`} />

        <div className="px-8 pb-8 -mt-16 relative z-10">
          {/* Level Badge */}
          <motion.div
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
            transition={{ delay: 0.2, type: 'spring' }}
            className={`w-32 h-32 rounded-full bg-gradient-to-br ${levelGradient} flex items-center justify-center shadow-glow border-4 border-white/20 mb-6`}
          >
            <div className="text-center">
              <div className="text-5xl font-black text-white">{stats.level}</div>
              <div className="text-xs uppercase tracking-wider text-white/80 mt-1">Level</div>
            </div>
          </motion.div>

          {/* Stats Summary */}
          <div className="space-y-4">
            <div>
              <div className="flex justify-between items-center mb-2">
                <h3 className="text-lg font-bold text-white">Progression XP</h3>
                <span className="text-sm text-zinc-400">
                  {stats.total_xp.toLocaleString()} / {stats.next_level_xp.toLocaleString()} XP
                </span>
              </div>
              <div className="w-full bg-white/10 rounded-full h-3 overflow-hidden border border-white/20">
                <motion.div
                  initial={{ width: 0 }}
                  animate={{ width: `${stats.progress_percent}%` }}
                  transition={{ delay: 0.4, duration: 0.8, ease: 'easeOut' }}
                  className={`h-full bg-gradient-to-r ${levelGradient} rounded-full`}
                />
              </div>
              <p className="text-xs text-zinc-500 mt-2">
                {Math.round(stats.progress_percent)}% vers le niveau {stats.level + 1}
              </p>
            </div>

            {/* Achievement Stats */}
            <div className="grid grid-cols-2 gap-4 pt-4">
              <motion.div
                initial={{ opacity: 0, x: -20 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ delay: 0.3 }}
                className="bg-white/5 border border-white/10 rounded-lg p-4 text-center"
              >
                <div className="text-2xl font-bold text-blue-400">{stats.risks_managed}</div>
                <div className="text-xs text-zinc-400 uppercase tracking-wide mt-1">Risques Gérés</div>
              </motion.div>

              <motion.div
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ delay: 0.3 }}
                className="bg-white/5 border border-white/10 rounded-lg p-4 text-center"
              >
                <div className="text-2xl font-bold text-green-400">{stats.mitigations_done}</div>
                <div className="text-xs text-zinc-400 uppercase tracking-wide mt-1">Atténuations</div>
              </motion.div>
            </div>
          </div>
        </div>
      </motion.div>

      {/* Badges Section */}
      {stats.badges && stats.badges.length > 0 && (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.5, duration: 0.3 }}
          className="bg-white/5 backdrop-blur-xl border border-white/10 rounded-2xl p-6"
        >
          <h3 className="text-lg font-bold text-white mb-4 flex items-center gap-2">
            <Trophy className="w-5 h-5 text-yellow-400" />
            Badges Débloqués
          </h3>

          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            {stats.badges.map((badge, idx) => (
              <motion.div
                key={badge.id}
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ delay: 0.6 + idx * 0.1 }}
                className={`relative group cursor-pointer`}
              >
                <div
                  className={`
                    aspect-square rounded-xl border-2 flex flex-col items-center justify-center gap-2 p-3 text-center
                    transition-all duration-300
                    ${
                      badge.unlocked
                        ? 'bg-gradient-to-br from-yellow-500/20 to-orange-500/20 border-yellow-400/50 shadow-glow'
                        : 'bg-white/5 border-white/10 opacity-50'
                    }
                  `}
                >
                  <div className={badge.unlocked ? 'text-yellow-400' : 'text-zinc-600'}>
                    {getBadgeIcon(badge.icon)}
                  </div>
                  <div className="text-xs font-semibold text-white line-clamp-2">{badge.name}</div>
                  {badge.unlocked && (
                    <Star className="w-3 h-3 text-yellow-400 absolute top-1 right-1 fill-current" />
                  )}
                </div>

                {/* Tooltip */}
                <div
                  className={`
                    absolute bottom-full left-1/2 -translate-x-1/2 mb-2 px-3 py-2 bg-zinc-900 border border-white/20 rounded-lg text-xs text-white whitespace-nowrap
                    opacity-0 pointer-events-none group-hover:opacity-100 transition-opacity z-50
                    ${badge.unlocked ? '' : 'grayscale'}
                  `}
                >
                  {badge.description}
                </div>
              </motion.div>
            ))}
          </div>

          {stats.badges.filter((b) => !b.unlocked).length > 0 && (
            <p className="text-xs text-zinc-500 mt-4">
              {stats.badges.filter((b) => !b.unlocked).length} badge(s) à déverrouiller
            </p>
          )}
        </motion.div>
      )}
    </div>
  );
};