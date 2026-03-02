import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Crown, TrendingUp, Zap, Target, Medal, Users } from 'lucide-react';
import { Card } from '../Card';
import { AchievementTrackingUI } from './AchievementTrackingUI';

interface UserRanking {
  rank: number;
  userId: string;
  username: string;
  totalXP: number;
  level: number;
  avatar?: string;
  risksManaged: number;
}

interface GamificationDashboardProps {
  userStats: {
    totalXP: number;
    level: number;
    nextLevelXP: number;
    progressPercent: number;
    risksManaged: number;
    mitigationsDone: number;
    achievements: any[];
  };
  rankings?: UserRanking[];
  isLoading?: boolean;
}

export const GamificationDashboard = ({
  userStats,
  rankings = [],
  isLoading = false,
}: GamificationDashboardProps) => {
  const [activeTab, setActiveTab] = useState<'overview' | 'achievements' | 'leaderboard'>('overview');
  const [userRank, setUserRank] = useState(1);

  useEffect(() => {
    // Calculate user rank
    if (rankings.length > 0) {
      const myRank = rankings.findIndex((r) => r.userId === 'current-user') + 1;
      setUserRank(myRank || rankings.length);
    }
  }, [rankings]);

  const getLevelColor = (level: number) => {
    if (level < 5) return 'from-green-600 to-green-700';
    if (level < 10) return 'from-blue-600 to-blue-700';
    if (level < 20) return 'from-purple-600 to-purple-700';
    return 'from-yellow-500 to-yellow-600';
  };

  return (
    <div className="space-y-6">
      {/* User Level Card */}
      <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }}>
        <Card className={`bg-gradient-to-br ${getLevelColor(userStats.level)} border-2 border-white/20`}>
          <div className="p-8">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
              {/* Level Circle */}
              <div className="flex items-center justify-center">
                <div className="relative w-32 h-32">
                  <svg className="absolute inset-0 transform -rotate-90" viewBox="0 0 100 100">
                    <circle cx="50" cy="50" r="45" fill="none" stroke="rgba(255,255,255,0.1)" strokeWidth="4" />
                    <motion.circle
                      cx="50"
                      cy="50"
                      r="45"
                      fill="none"
                      stroke="white"
                      strokeWidth="4"
                      strokeLinecap="round"
                      initial={{ strokeDasharray: '283' }}
                      animate={{
                        strokeDasharray: `${(userStats.progressPercent / 100) * 283} 283`,
                      }}
                      transition={{ duration: 0.8, ease: 'easeOut' }}
                    />
                  </svg>
                  <div className="absolute inset-0 flex items-center justify-center">
                    <div className="text-center">
                      <div className="text-4xl font-bold text-white">{userStats.level}</div>
                      <div className="text-xs text-white/70">Level</div>
                    </div>
                  </div>
                </div>
              </div>

              {/* XP Info */}
              <div className="space-y-4">
                <div>
                  <p className="text-white/80 text-sm mb-2">Total Experience</p>
                  <p className="text-3xl font-bold text-white">{userStats.totalXP.toLocaleString()}</p>
                  <p className="text-xs text-white/60 mt-1">XP</p>
                </div>
                <div>
                  <p className="text-white/80 text-sm mb-2">Next Level</p>
                  <p className="text-lg font-semibold text-white">{userStats.nextLevelXP.toLocaleString()} XP</p>
                </div>
              </div>

              {/* Stats */}
              <div className="space-y-4">
                <div className="bg-white/10 backdrop-blur-sm rounded-lg p-4">
                  <div className="flex items-center gap-2 mb-2">
                    <Target size={16} className="text-white/70" />
                    <p className="text-white/80 text-sm">Risks Managed</p>
                  </div>
                  <p className="text-2xl font-bold text-white">{userStats.risksManaged}</p>
                </div>
                <div className="bg-white/10 backdrop-blur-sm rounded-lg p-4">
                  <div className="flex items-center gap-2 mb-2">
                    <Zap size={16} className="text-white/70" />
                    <p className="text-white/80 text-sm">Mitigations Done</p>
                  </div>
                  <p className="text-2xl font-bold text-white">{userStats.mitigationsDone}</p>
                </div>
              </div>
            </div>

            {/* Progress Bar */}
            <div className="mt-6">
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm text-white/80">Progress to Level {userStats.level + 1}</span>
                <span className="text-sm font-semibold text-white">{userStats.progressPercent}%</span>
              </div>
              <div className="w-full h-3 bg-black/30 rounded-full overflow-hidden backdrop-blur-sm">
                <motion.div
                  className="h-full bg-white/80"
                  initial={{ width: 0 }}
                  animate={{ width: `${userStats.progressPercent}%` }}
                  transition={{ duration: 0.8, ease: 'easeOut' }}
                />
              </div>
            </div>
          </div>
        </Card>
      </motion.div>

      {/* Navigation Tabs */}
      <div className="flex gap-3 border-b border-zinc-800">
        {(['overview', 'achievements', 'leaderboard'] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-6 py-3 font-medium text-sm transition-all border-b-2 ${
              activeTab === tab
                ? 'text-blue-400 border-blue-400'
                : 'text-zinc-400 border-transparent hover:text-zinc-300'
            }`}
          >
            {tab === 'overview' && '📊 Overview'}
            {tab === 'achievements' && '🏆 Achievements'}
            {tab === 'leaderboard' && '👑 Leaderboard'}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      {activeTab === 'overview' && (
        <motion.div
          key="overview"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="space-y-6"
        >
          {/* Quick Stats */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <Card className="bg-zinc-900/50 border-zinc-800 hover:border-blue-600/50 transition-colors">
              <div className="p-6">
                <div className="flex items-center justify-between mb-3">
                  <p className="text-zinc-400 text-sm">Your Rank</p>
                  <Crown size={20} className="text-yellow-400 opacity-60" />
                </div>
                <p className="text-3xl font-bold text-white">#{userRank}</p>
                <p className="text-xs text-zinc-500 mt-2">Global Ranking</p>
              </div>
            </Card>

            <Card className="bg-zinc-900/50 border-zinc-800 hover:border-emerald-600/50 transition-colors">
              <div className="p-6">
                <div className="flex items-center justify-between mb-3">
                  <p className="text-zinc-400 text-sm">Achievements</p>
                  <Medal size={20} className="text-emerald-400 opacity-60" />
                </div>
                <p className="text-3xl font-bold text-white">{userStats.achievements.length}</p>
                <p className="text-xs text-zinc-500 mt-2">Unlocked</p>
              </div>
            </Card>

            <Card className="bg-zinc-900/50 border-zinc-800 hover:border-purple-600/50 transition-colors">
              <div className="p-6">
                <div className="flex items-center justify-between mb-3">
                  <p className="text-zinc-400 text-sm">Win Streak</p>
                  <TrendingUp size={20} className="text-purple-400 opacity-60" />
                </div>
                <p className="text-3xl font-bold text-white">7</p>
                <p className="text-xs text-zinc-500 mt-2">Days Active</p>
              </div>
            </Card>

            <Card className="bg-zinc-900/50 border-zinc-800 hover:border-orange-600/50 transition-colors">
              <div className="p-6">
                <div className="flex items-center justify-between mb-3">
                  <p className="text-zinc-400 text-sm">Badges</p>
                  <Zap size={20} className="text-orange-400 opacity-60" />
                </div>
                <p className="text-3xl font-bold text-white">12</p>
                <p className="text-xs text-zinc-500 mt-2">Special</p>
              </div>
            </Card>
          </div>

          {/* Achievements Summary */}
          <Card className="bg-zinc-900/50 border-zinc-800">
            <div className="p-6">
              <h3 className="text-lg font-semibold text-white mb-4">Recent Achievements</h3>
              <div className="space-y-3">
                {userStats.achievements.slice(0, 5).map((achievement, index) => (
                  <motion.div
                    key={achievement.id}
                    initial={{ opacity: 0, x: -20 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ delay: index * 0.1 }}
                    className="flex items-center gap-4 p-3 bg-zinc-800/50 rounded-lg hover:bg-zinc-700/50 transition-colors"
                  >
                    <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-blue-500 to-purple-500 flex items-center justify-center flex-shrink-0">
                      <Medal size={20} className="text-white" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-white font-medium text-sm">{achievement.name}</p>
                      <p className="text-xs text-zinc-400">{achievement.description}</p>
                    </div>
                  </motion.div>
                ))}
              </div>
            </div>
          </Card>
        </motion.div>
      )}

      {activeTab === 'achievements' && (
        <motion.div
          key="achievements"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
        >
          <AchievementTrackingUI achievements={userStats.achievements} isLoading={isLoading} />
        </motion.div>
      )}

      {activeTab === 'leaderboard' && (
        <motion.div
          key="leaderboard"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
        >
          <Card className="bg-zinc-900/50 border-zinc-800">
            <div className="p-6">
              <div className="flex items-center gap-2 mb-6">
                <Users size={24} className="text-blue-400" />
                <h3 className="text-lg font-semibold text-white">Global Leaderboard</h3>
              </div>

              {isLoading ? (
                <div className="space-y-3">
                  {[...Array(5)].map((_, i) => (
                    <div key={i} className="h-12 bg-zinc-800/50 rounded-lg animate-pulse" />
                  ))}
                </div>
              ) : (
                <div className="space-y-3">
                  {rankings.slice(0, 10).map((user, index) => {
                    const isCurrent = user.userId === 'current-user';
                    return (
                      <motion.div
                        key={user.userId}
                        initial={{ opacity: 0, y: 10 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ delay: index * 0.05 }}
                        className={`flex items-center gap-4 p-4 rounded-lg transition-colors ${
                          isCurrent
                            ? 'bg-gradient-to-r from-blue-900/30 to-purple-900/30 border border-blue-600/50'
                            : 'bg-zinc-800/30 hover:bg-zinc-800/50'
                        }`}
                      >
                        {/* Rank */}
                        <div className="flex-shrink-0 w-8">
                          {user.rank === 1 && (
                            <div className="text-2xl">🥇</div>
                          )}
                          {user.rank === 2 && (
                            <div className="text-2xl">🥈</div>
                          )}
                          {user.rank === 3 && (
                            <div className="text-2xl">🥉</div>
                          )}
                          {user.rank > 3 && (
                            <p className="text-lg font-bold text-zinc-400">#{user.rank}</p>
                          )}
                        </div>

                        {/* User Info */}
                        <div className="flex items-center gap-3 flex-1 min-w-0">
                          <div
                            className="w-10 h-10 rounded-full bg-gradient-to-br from-blue-500 to-purple-500 flex items-center justify-center flex-shrink-0 text-white font-semibold text-sm"
                          >
                            {user.username.charAt(0).toUpperCase()}
                          </div>
                          <div className="flex-1 min-w-0">
                            <p className={`font-medium ${isCurrent ? 'text-blue-400' : 'text-white'}`}>
                              {user.username}
                              {isCurrent && <span className="text-xs text-blue-300 ml-2">(You)</span>}
                            </p>
                            <p className="text-xs text-zinc-400">Level {user.level}</p>
                          </div>
                        </div>

                        {/* XP */}
                        <div className="text-right flex-shrink-0">
                          <p className="text-lg font-bold text-white">{user.totalXP.toLocaleString()}</p>
                          <p className="text-xs text-zinc-400">XP</p>
                        </div>
                      </motion.div>
                    );
                  })}
                </div>
              )}
            </div>
          </Card>
        </motion.div>
      )}
    </div>
  );
};
