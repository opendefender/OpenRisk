import { useEffect } from 'react';
import { Trophy, Star, Target, Crown, AlertCircle } from 'lucide-react';
import { motion } from 'framer-motion';
import { useGamificationStore } from '../../hooks/useGamificationStore';

const LEVEL_COLORS = {
  : 'from-green- to-teal-',
  : 'from-blue- to-cyan-',
  : 'from-purple- to-indigo-',
  : 'from-pink- to-rose-',
  : 'from-orange- to-red-',
};

const getBadgeIcon = (iconName: string) => {
  const icons: Record<string, React.ReactNode> = {
    Flag: <Target className="w- h-" />,
    ShieldCheck: <Trophy className="w- h-" />,
    Brain: <Star className="w- h-" />,
    Crown: <Crown className="w- h-" />,
  };
  return icons[iconName] || <Star className="w- h-" />;
};

const getLevelColor = (level: number): string => {
  if (level >= ) return LEVEL_COLORS[];
  if (level >= ) return LEVEL_COLORS[];
  if (level >= ) return LEVEL_COLORS[];
  if (level >= ) return LEVEL_COLORS[];
  return LEVEL_COLORS[];
};

export const UserLevelCard = () => {
  const { stats, loading, error, fetchStats } = useGamificationStore();

  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  if (loading) {
    return (
      <div className="bg-white/ backdrop-blur-xl border border-white/ rounded-xl p- animate-pulse">
        <div className="h- bg-white/ rounded-lg" />
      </div>
    );
  }

  if (error || !stats) {
    return (
      <div className="bg-white/ backdrop-blur-xl border border-white/ rounded-xl p-">
        <div className="flex items-center gap- text-orange-">
          <AlertCircle className="w- h-" />
          <p className="text-sm">{error || 'Impossible de charger les donn√es'}</p>
        </div>
      </div>
    );
  }

  const levelGradient = getLevelColor(stats.level);

  return (
    <div className="space-y-">
      {/ Main Level Card /}
      <motion.div
        initial={{ opacity: , y:  }}
        animate={{ opacity: , y:  }}
        transition={{ duration: . }}
        className="bg-gradient-to-br from-white/ to-white/ backdrop-blur-xl border border-white/ rounded-xl overflow-hidden shadow-xl"
      >
        <div className={h- bg-gradient-to-r ${levelGradient} opacity-} />

        <div className="px- pb- -mt- relative z-">
          {/ Level Badge /}
          <motion.div
            initial={{ scale:  }}
            animate={{ scale:  }}
            transition={{ delay: ., type: 'spring' }}
            className={w- h- rounded-full bg-gradient-to-br ${levelGradient} flex items-center justify-center shadow-glow border- border-white/ mb-}
          >
            <div className="text-center">
              <div className="text-xl font-black text-white">{stats.level}</div>
              <div className="text-xs uppercase tracking-wider text-white/ mt-">Level</div>
            </div>
          </motion.div>

          {/ Stats Summary /}
          <div className="space-y-">
            <div>
              <div className="flex justify-between items-center mb-">
                <h className="text-lg font-bold text-white">Progression XP</h>
                <span className="text-sm text-zinc-">
                  {stats.total_xp.toLocaleString()} / {stats.next_level_xp.toLocaleString()} XP
                </span>
              </div>
              <div className="w-full bg-white/ rounded-full h- overflow-hidden border border-white/">
                <motion.div
                  initial={{ width:  }}
                  animate={{ width: ${stats.progress_percent}% }}
                  transition={{ delay: ., duration: ., ease: 'easeOut' }}
                  className={h-full bg-gradient-to-r ${levelGradient} rounded-full}
                />
              </div>
              <p className="text-xs text-zinc- mt-">
                {Math.round(stats.progress_percent)}% vers le niveau {stats.level + }
              </p>
            </div>

            {/ Achievement Stats /}
            <div className="grid grid-cols- gap- pt-">
              <motion.div
                initial={{ opacity: , x: - }}
                animate={{ opacity: , x:  }}
                transition={{ delay: . }}
                className="bg-white/ border border-white/ rounded-lg p- text-center"
              >
                <div className="text-xl font-bold text-blue-">{stats.risks_managed}</div>
                <div className="text-xs text-zinc- uppercase tracking-wide mt-">Risques G√r√s</div>
              </motion.div>

              <motion.div
                initial={{ opacity: , x:  }}
                animate={{ opacity: , x:  }}
                transition={{ delay: . }}
                className="bg-white/ border border-white/ rounded-lg p- text-center"
              >
                <div className="text-xl font-bold text-green-">{stats.mitigations_done}</div>
                <div className="text-xs text-zinc- uppercase tracking-wide mt-">Att√nuations</div>
              </motion.div>
            </div>
          </div>
        </div>
      </motion.div>

      {/ Badges Section /}
      {stats.badges && stats.badges.length >  && (
        <motion.div
          initial={{ opacity: , y:  }}
          animate={{ opacity: , y:  }}
          transition={{ delay: ., duration: . }}
          className="bg-white/ backdrop-blur-xl border border-white/ rounded-xl p-"
        >
          <h className="text-lg font-bold text-white mb- flex items-center gap-">
            <Trophy className="w- h- text-yellow-" />
            Badges D√bloqu√s
          </h>

          <div className="grid grid-cols- md:grid-cols- gap-">
            {stats.badges.map((badge, idx) => (
              <motion.div
                key={badge.id}
                initial={{ opacity: , scale: . }}
                animate={{ opacity: , scale:  }}
                transition={{ delay: . + idx  . }}
                className={relative group cursor-pointer}
              >
                <div
                  className={
                    aspect-square rounded-xl border- flex flex-col items-center justify-center gap- p- text-center
                    transition-all duration-
                    ${
                      badge.unlocked
                        ? 'bg-gradient-to-br from-yellow-/ to-orange-/ border-yellow-/ shadow-glow'
                        : 'bg-white/ border-white/ opacity-'
                    }
                  }
                >
                  <div className={badge.unlocked ? 'text-yellow-' : 'text-zinc-'}>
                    {getBadgeIcon(badge.icon)}
                  </div>
                  <div className="text-xs font-semibold text-white line-clamp-">{badge.name}</div>
                  {badge.unlocked && (
                    <Star className="w- h- text-yellow- absolute top- right- fill-current" />
                  )}
                </div>

                {/ Tooltip /}
                <div
                  className={
                    absolute bottom-full left-/ -translate-x-/ mb- px- py- bg-zinc- border border-white/ rounded-lg text-xs text-white whitespace-nowrap
                    opacity- pointer-events-none group-hover:opacity- transition-opacity z-
                    ${badge.unlocked ? '' : 'grayscale'}
                  }
                >
                  {badge.description}
                </div>
              </motion.div>
            ))}
          </div>

          {stats.badges.filter((b) => !b.unlocked).length >  && (
            <p className="text-xs text-zinc- mt-">
              {stats.badges.filter((b) => !b.unlocked).length} badge(s) √† d√verrouiller
            </p>
          )}
        </motion.div>
      )}
    </div>
  );
};