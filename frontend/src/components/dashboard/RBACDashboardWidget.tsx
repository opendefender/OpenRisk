import { useEffect, useState } from 'react';
import { Shield, Users, Lock, Zap } from 'lucide-react';
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
  : { bg: 'bg-zinc-/', text: 'text-zinc-', icon: '' },
  : { bg: 'bg-blue-/', text: 'text-blue-', icon: '' },
  : { bg: 'bg-purple-/', text: 'text-purple-', icon: '' },
  : { bg: 'bg-red-/', text: 'text-red-', icon: '' },
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
        const roleRes = await api.get(/rbac/roles/name/${currentUser.role});
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

  const levelName = ['Viewer', 'Analyst', 'Manager', 'Admin'][Math.min(Math.floor(roleInfo.level / ), )];
  const colors = levelColors[roleInfo.level as keyof typeof levelColors] || levelColors[];

  return (
    <div className="space-y-">
      {/ Current Role Card /}
      <div className={rounded-lg border border-border p- ${colors.bg} backdrop-blur-sm}>
        <div className="flex items-start justify-between">
          <div>
            <div className="text-sm font-medium text-zinc- mb-">Your Role</div>
            <h className={text-lg font-semibold ${colors.text}}>{roleInfo.name}</h>
            <p className="text-xs text-zinc- mt-">
              Level {roleInfo.level} • {roleInfo.permissions_count} permissions
            </p>
          </div>
          <div className="text-xl">{colors.icon}</div>
        </div>
        <div className="mt- w-full h-. bg-zinc-/ rounded-full overflow-hidden">
          <div
            className={h-full transition-all ${colors.bg}}
            style={{ width: ${(roleInfo.level / )  }% }}
          />
        </div>
      </div>

      {/ Team Statistics /}
      {teamStats && (
        <div className="grid grid-cols- gap-">
          <div className="rounded-lg border border-border bg-surface/ p- backdrop-blur-sm">
            <div className="flex items-center gap- mb-">
              <Users size={} className="text-blue-" />
              <span className="text-xs font-medium text-zinc-">Team Members</span>
            </div>
            <div className="text-xl font-bold text-white">{teamStats.active_users}</div>
            <p className="text-xs text-zinc- mt-">{teamStats.total_users} total</p>
          </div>

          <div className="rounded-lg border border-border bg-surface/ p- backdrop-blur-sm">
            <div className="flex items-center gap- mb-">
              <Zap size={} className="text-purple-" />
              <span className="text-xs font-medium text-zinc-">Teams</span>
            </div>
            <div className="text-xl font-bold text-white">{teamStats.teams_count}</div>
            <p className="text-xs text-zinc- mt-">
              {teamStats.pending_invites >  ? ${teamStats.pending_invites} pending : 'All accepted'}
            </p>
          </div>
        </div>
      )}

      {/ Quick Actions /}
      <div className="rounded-lg border border-dashed border-border bg-surface/ p- text-center">
        <Shield size={} className="inline-block mb- text-primary" />
        <p className="text-xs text-zinc-">
          Your access level determines what actions you can perform. 
          <a href="/settings?tab=rbac" className="text-primary hover:underline ml-">
            View permissions →
          </a>
        </p>
      </div>
    </div>
  );
};
