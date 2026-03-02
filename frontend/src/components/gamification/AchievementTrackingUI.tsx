import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Trophy, Star, Lock, Zap, Shield, Target, Award, Flame, CheckCircle2 } from 'lucide-react';
import { Card } from '../Card';

interface Achievement {
  id: string;
  name: string;
  description: string;
  icon: string;
  progress: number;
  maxProgress: number;
  unlocked: boolean;
  unlockedAt?: Date;
  category: 'risks' | 'mitigations' | 'compliance' | 'performance';
  rarity: 'common' | 'uncommon' | 'rare' | 'epic' | 'legendary';
}

interface AchievementTrackingUIProps {
  achievements: Achievement[];
  isLoading?: boolean;
}

const AchievementIcon = ({ icon, rarity }: { icon: string; rarity: string }) => {
  const iconSize = 24;
  const rarityColors = {
    common: 'text-gray-400',
    uncommon: 'text-green-400',
    rare: 'text-blue-400',
    epic: 'text-purple-400',
    legendary: 'text-yellow-400',
  };

  const iconMap: Record<string, any> = {
    trophy: Trophy,
    star: Star,
    zap: Zap,
    shield: Shield,
    target: Target,
    award: Award,
    flame: Flame,
  };

  const Icon = iconMap[icon] || Trophy;
  return <Icon size={iconSize} className={rarityColors[rarity as keyof typeof rarityColors]} />;
};

const getRarityColor = (rarity: string) => {
  const colors: Record<string, string> = {
    common: 'from-gray-600 to-gray-700',
    uncommon: 'from-green-600 to-green-700',
    rare: 'from-blue-600 to-blue-700',
    epic: 'from-purple-600 to-purple-700',
    legendary: 'from-yellow-500 to-yellow-600',
  };
  return colors[rarity] || 'from-gray-600 to-gray-700';
};

const getRarityBorder = (rarity: string) => {
  const borders: Record<string, string> = {
    common: 'border-gray-500',
    uncommon: 'border-green-500',
    rare: 'border-blue-500',
    epic: 'border-purple-500',
    legendary: 'border-yellow-500',
  };
  return borders[rarity] || 'border-gray-500';
};

const AchievementCard = ({ achievement, index }: { achievement: Achievement; index: number }) => {
  const [isHovered, setIsHovered] = useState(false);
  const progressPercent = (achievement.progress / achievement.maxProgress) * 100;

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: index * 0.05 }}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      <div
        className={`relative rounded-lg p-4 border-2 transition-all duration-300 ${
          achievement.unlocked
            ? `bg-gradient-to-br ${getRarityColor(achievement.rarity)} ${getRarityBorder(achievement.rarity)}`
            : 'bg-zinc-900 border-zinc-700 opacity-60'
        } ${isHovered && achievement.unlocked ? 'shadow-lg shadow-white/20 scale-105' : ''}`}
      >
        {/* Glow effect for unlocked achievements */}
        {achievement.unlocked && (
          <div
            className={`absolute inset-0 rounded-lg blur-xl opacity-20 ${
              achievement.rarity === 'legendary' ? 'bg-yellow-500' : 'bg-blue-500'
            }`}
          />
        )}

        <div className="relative z-10">
          {/* Header */}
          <div className="flex items-start justify-between mb-3">
            <div className="p-3 rounded-lg bg-black/30 backdrop-blur-sm">
              <AchievementIcon icon={achievement.icon} rarity={achievement.rarity} />
            </div>
            {achievement.unlocked && (
              <motion.div
                initial={{ scale: 0 }}
                animate={{ scale: 1 }}
                transition={{ type: 'spring', delay: index * 0.05 + 0.2 }}
              >
                <CheckCircle2 size={20} className="text-yellow-300" />
              </motion.div>
            )}
          </div>

          {/* Title & Description */}
          <h3 className="font-semibold text-white text-sm mb-1">{achievement.name}</h3>
          <p className="text-xs text-white/70 mb-3 line-clamp-2">{achievement.description}</p>

          {/* Progress Bar */}
          {!achievement.unlocked && (
            <div className="mb-3">
              <div className="flex items-center justify-between mb-1">
                <span className="text-xs font-medium text-white/60">
                  {achievement.progress} / {achievement.maxProgress}
                </span>
                <span className="text-xs font-medium text-white/60">{Math.round(progressPercent)}%</span>
              </div>
              <div className="w-full h-2 bg-black/30 rounded-full overflow-hidden backdrop-blur-sm">
                <motion.div
                  className="h-full bg-gradient-to-r from-blue-500 to-cyan-400"
                  initial={{ width: 0 }}
                  animate={{ width: `${progressPercent}%` }}
                  transition={{ duration: 0.6, ease: 'easeOut' }}
                />
              </div>
            </div>
          )}

          {/* Unlocked Badge */}
          {achievement.unlocked && achievement.unlockedAt && (
            <div className="text-xs text-white/60 text-center">
              Unlocked {new Date(achievement.unlockedAt).toLocaleDateString()}
            </div>
          )}

          {/* Rarity Label */}
          <div className="text-xs font-semibold uppercase text-white/80 tracking-wide text-center mt-2">
            {achievement.rarity}
          </div>
        </div>
      </div>
    </motion.div>
  );
};

export const AchievementTrackingUI = ({ achievements, isLoading }: AchievementTrackingUIProps) => {
  const [filter, setFilter] = useState<'all' | 'unlocked' | 'locked'>('all');

  const filtered = achievements.filter((a) => {
    if (filter === 'all') return true;
    if (filter === 'unlocked') return a.unlocked;
    return !a.unlocked;
  });

  const unlockedCount = achievements.filter((a) => a.unlocked).length;
  const totalCount = achievements.length;

  return (
    <div className="space-y-6">
      {/* Header Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card className="bg-gradient-to-br from-purple-900/30 to-purple-800/20 border-purple-700/50">
          <div className="p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-zinc-400 text-sm">Total Achievements</p>
                <p className="text-3xl font-bold text-white mt-1">{totalCount}</p>
              </div>
              <Trophy size={32} className="text-purple-400 opacity-60" />
            </div>
          </div>
        </Card>

        <Card className="bg-gradient-to-br from-emerald-900/30 to-emerald-800/20 border-emerald-700/50">
          <div className="p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-zinc-400 text-sm">Unlocked</p>
                <div className="flex items-baseline gap-2 mt-1">
                  <p className="text-3xl font-bold text-white">{unlockedCount}</p>
                  <p className="text-sm text-zinc-400">({Math.round((unlockedCount / totalCount) * 100)}%)</p>
                </div>
              </div>
              <CheckCircle2 size={32} className="text-emerald-400 opacity-60" />
            </div>
          </div>
        </Card>

        <Card className="bg-gradient-to-br from-yellow-900/30 to-yellow-800/20 border-yellow-700/50">
          <div className="p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-zinc-400 text-sm">Completion</p>
                <p className="text-3xl font-bold text-white mt-1">{Math.round((unlockedCount / totalCount) * 100)}%</p>
              </div>
              <Flame size={32} className="text-yellow-400 opacity-60" />
            </div>
          </div>
        </Card>
      </div>

      {/* Filter Tabs */}
      <div className="flex gap-3">
        {(['all', 'unlocked', 'locked'] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setFilter(tab)}
            className={`px-4 py-2 rounded-lg font-medium text-sm transition-all ${
              filter === tab
                ? 'bg-blue-600 text-white shadow-lg shadow-blue-500/50'
                : 'bg-zinc-900 text-zinc-400 hover:bg-zinc-800'
            }`}
          >
            {tab.charAt(0).toUpperCase() + tab.slice(1)}
          </button>
        ))}
      </div>

      {/* Achievements Grid */}
      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[...Array(6)].map((_, i) => (
            <div key={i} className="h-32 bg-zinc-900 rounded-lg animate-pulse" />
          ))}
        </div>
      ) : (
        <AnimatePresence>
          {filtered.length === 0 ? (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="text-center py-12"
            >
              <Star size={48} className="mx-auto text-zinc-600 mb-3 opacity-50" />
              <p className="text-zinc-400">No achievements to display</p>
            </motion.div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {filtered.map((achievement, index) => (
                <AchievementCard key={achievement.id} achievement={achievement} index={index} />
              ))}
            </div>
          )}
        </AnimatePresence>
      )}

      {/* Category Breakdown */}
      <Card className="bg-zinc-900/50 border-zinc-800">
        <div className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4">Achievements by Category</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            {['risks', 'mitigations', 'compliance', 'performance'].map((category) => {
              const categoryAchievements = achievements.filter((a) => a.category === category);
              const unlockedInCategory = categoryAchievements.filter((a) => a.unlocked).length;
              return (
                <div key={category} className="bg-zinc-800/50 p-4 rounded-lg">
                  <p className="text-zinc-400 text-sm capitalize mb-2">{category}</p>
                  <div className="flex items-baseline gap-2">
                    <p className="text-2xl font-bold text-white">{unlockedInCategory}</p>
                    <p className="text-sm text-zinc-400">/ {categoryAchievements.length}</p>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </Card>
    </div>
  );
};
