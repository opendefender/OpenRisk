import { useEffect, useState } from 'react';
import { Shield, Users, Zap } from 'lucide-react';
import { api } from '../../lib/api';
import { useAuthStore } from '../../hooks/useAuthStore';

interface RoleInfo {
  name: string;
  level: number;
  permissions_count: number;
}

interface TeamStats {
  total_users: number;
  active_users: number;
  pending_invites: number;
  teams_count: number;
}

const levelColors: Record<number, { bg: string; text: string; icon: string }> = {
  0: { bg: 'bg-zinc-500/10', text: 'text-zinc-400', icon: '👁️' },
  3: { bg: 'bg-blue-500/10', text: 'text-blue-400', icon: '🔍' },
  6: { bg: 'bg-purple-500/10', text: 'text-purple-400', icon: '👨‍💼' },
  9: { bg: 'bg-red-500/10', text: 'text-red-400', icon: '👑' },
};

export const RBACDashboardWidget = () => {
  const [roleInfo, setRoleInfo] = useState<RoleInfo | null>(null);
  const [teamStats, setTeamStats] = useState<TeamStats | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const currentUser = useAuthStore((state) => state.user);

  useEffect(() => {
    fetchRBACData();
  }, []);

  const fetchRBACData = async () => {
    try {
      // Fetch current user's role info
      if (currentUser?.role) {
        const roleRes = await api.get(`/rbac/roles/name/${currentUser.role}`);
        setRoleInfo(roleRes.data);
      }

      // Fetch team statistics
      try {
        const statsRes = await api.get('/rbac/users/stats');
        setTeamStats(statsRes.data);
      } catch {
        // Stats might not be available
      }
    } catch (err) {
      console.error('Failed to fetch RBAC data:', err);
    } finally {
      setIsLoading(false);
    }
  };

  if (isLoading || !roleInfo) {
    return null;
  }

  const colors = levelColors[roleInfo.level as keyof typeof levelColors] || levelColors[0];

  return (
    <div className="space-y-4">
      {/* Current Role Card */}
      <div className={`rounded-lg border border-border p-4 ${colors.bg} backdrop-blur-sm`}>
        <div className="flex items-start justify-between">
          <div>
            <div className="text-sm font-medium text-zinc-400 mb-1">Your Role</div>
            <h3 className={`text-lg font-semibold ${colors.text}`}>{roleInfo.name}</h3>
            <p className="text-xs text-zinc-500 mt-1">
              Level {roleInfo.level} • {roleInfo.permissions_count} permissions
            </p>
          </div>
          <div className="text-2xl">{colors.icon}</div>
        </div>
        <div className="mt-3 w-full h-1.5 bg-zinc-900/50 rounded-full overflow-hidden">
          <div
            className={`h-full transition-all ${colors.bg}`}
            style={{ width: `${(roleInfo.level / 9) * 100}%` }}
          />
        </div>
      </div>

      {/* Team Statistics */}
      {teamStats && (
        <div className="grid grid-cols-2 gap-3">
          <div className="rounded-lg border border-border bg-surface/30 p-3 backdrop-blur-sm">
            <div className="flex items-center gap-2 mb-2">
              <Users size={16} className="text-blue-400" />
              <span className="text-xs font-medium text-zinc-400">Team Members</span>
            </div>
            <div className="text-2xl font-bold text-white">{teamStats.active_users}</div>
            <p className="text-xs text-zinc-500 mt-1">{teamStats.total_users} total</p>
          </div>

          <div className="rounded-lg border border-border bg-surface/30 p-3 backdrop-blur-sm">
            <div className="flex items-center gap-2 mb-2">
              <Zap size={16} className="text-purple-400" />
              <span className="text-xs font-medium text-zinc-400">Teams</span>
            </div>
            <div className="text-2xl font-bold text-white">{teamStats.teams_count}</div>
            <p className="text-xs text-zinc-500 mt-1">
              {teamStats.pending_invites > 0 ? `${teamStats.pending_invites} pending` : 'All accepted'}
            </p>
          </div>
        </div>
      )}

      {/* Quick Actions */}
      <div className="rounded-lg border border-dashed border-border bg-surface/20 p-3 text-center">
        <Shield size={16} className="inline-block mb-2 text-primary" />
        <p className="text-xs text-zinc-400">
          Your access level determines what actions you can perform. 
          <a href="/settings?tab=rbac" className="text-primary hover:underline ml-1">
            View permissions →
          </a>
        </p>
      </div>
    </div>
  );
};
