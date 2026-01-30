import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Shield, Lock, AlertCircle, Users, Check } from 'lucide-react';
import { api } from '../../lib/api';
import { toast } from 'sonner';
import { useAuthStore } from '../../hooks/useAuthStore';

interface Role {
  id: string;
  name: string;
  description: string;
  level: number;
  is_predefined: boolean;
  user_count?: number;
}

interface UserPermissions {
  id: string;
  resource: string;
  action: string;
}

const levelLabels: Record<number, { name: string; description: string }> = {
  0: { name: 'Viewer', description: 'Read-only access to data' },
  3: { name: 'Analyst', description: 'Can create and manage risks' },
  6: { name: 'Manager', description: 'Can manage resources and users' },
  9: { name: 'Admin', description: 'Full access and control' },
};

export const RBACTab = () => {
  const [userRoles, setUserRoles] = useState<Role[]>([]);
  const [userPermissions, setUserPermissions] = useState<UserPermissions[]>([]);
  const [allRoles, setAllRoles] = useState<Role[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'roles' | 'permissions'>('roles');
  const currentUser = useAuthStore((state) => state.user);
  const isAdmin = currentUser?.role === 'admin' || currentUser?.role === 'ADMIN';

  useEffect(() => {
    fetchRBACData();
  }, []);

  const fetchRBACData = async () => {
    setIsLoading(true);
    try {
      const [userRolesRes, userPermsRes] = await Promise.all([
        api.get('/rbac/users/roles'),
        api.get('/rbac/users/permissions'),
      ]);
      setUserRoles(userRolesRes.data || []);
      setUserPermissions(userPermsRes.data || []);

      // If admin, also fetch all roles
      if (isAdmin) {
        const allRolesRes = await api.get('/rbac/roles');
        setAllRoles(allRolesRes.data?.roles || []);
      }
    } catch (err: any) {
      console.error('Failed to fetch RBAC data:', err);
      toast.error("We couldn't load your RBAC information. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-primary"></div>
      </div>
    );
  }

  // Group permissions by resource
  const permissionsByResource = userPermissions.reduce((acc, perm) => {
    if (!acc[perm.resource]) {
      acc[perm.resource] = [];
    }
    acc[perm.resource].push(perm.action);
    return acc;
  }, {} as Record<string, string[]>);

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center gap-3 mb-6">
        <Shield className="w-6 h-6 text-primary" />
        <div>
          <h2 className="text-2xl font-bold text-white">Access Control</h2>
          <p className="text-sm text-zinc-400">View your roles and permissions</p>
        </div>
      </div>

      {/* Tab Navigation */}
      <div className="flex gap-4 border-b border-border">
        <button
          onClick={() => setActiveTab('roles')}
          className={`px-4 py-2 font-medium transition-colors border-b-2 ${
            activeTab === 'roles'
              ? 'border-primary text-primary'
              : 'border-transparent text-zinc-400 hover:text-white'
          }`}
        >
          <div className="flex items-center gap-2">
            <Users size={18} />
            My Roles
          </div>
        </button>
        <button
          onClick={() => setActiveTab('permissions')}
          className={`px-4 py-2 font-medium transition-colors border-b-2 ${
            activeTab === 'permissions'
              ? 'border-primary text-primary'
              : 'border-transparent text-zinc-400 hover:text-white'
          }`}
        >
          <div className="flex items-center gap-2">
            <Lock size={18} />
            My Permissions
          </div>
        </button>
      </div>

      {/* Roles Tab */}
      {activeTab === 'roles' && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="space-y-4"
        >
          {userRoles.length === 0 ? (
            <div className="flex items-center gap-3 p-4 rounded-lg bg-yellow-500/10 border border-yellow-500/20 text-yellow-300">
              <AlertCircle size={20} />
              <p>You don't have any roles assigned. Contact your administrator.</p>
            </div>
          ) : (
            <div className="space-y-4">
              {userRoles.map((role) => (
                <motion.div
                  key={role.id}
                  initial={{ opacity: 0, x: -10 }}
                  animate={{ opacity: 1, x: 0 }}
                  className="p-4 rounded-lg border border-border bg-surface/50 hover:border-primary/50 transition-colors"
                >
                  <div className="flex items-start justify-between mb-3">
                    <div>
                      <h3 className="text-lg font-semibold text-white">{role.name}</h3>
                      <p className="text-sm text-zinc-400">{role.description}</p>
                    </div>
                    <div className="flex items-center gap-2">
                      {role.is_predefined ? (
                        <span className="px-3 py-1 rounded-full text-xs font-medium bg-purple-500/10 text-purple-300">
                          System
                        </span>
                      ) : (
                        <span className="px-3 py-1 rounded-full text-xs font-medium bg-green-500/10 text-green-300">
                          Custom
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <div className="w-24 h-2 bg-zinc-800 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-primary transition-all"
                        style={{ width: `${(role.level / 9) * 100}%` }}
                      />
                    </div>
                    <span className="text-xs font-medium text-zinc-400">
                      Level {role.level}: {levelLabels[role.level as keyof typeof levelLabels]?.name || 'Unknown'}
                    </span>
                  </div>
                </motion.div>
              ))}
            </div>
          )}

          {isAdmin && allRoles.length > 0 && (
            <div className="mt-8 p-4 rounded-lg bg-blue-500/10 border border-blue-500/20">
              <h4 className="text-sm font-semibold text-blue-300 mb-3">Admin - All Available Roles</h4>
              <div className="space-y-2">
                {allRoles.map((role) => (
                  <div key={role.id} className="flex items-center justify-between p-3 rounded bg-zinc-900/50">
                    <span className="text-sm text-zinc-300">{role.name}</span>
                    <span className="text-xs text-zinc-500">Level {role.level}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </motion.div>
      )}

      {/* Permissions Tab */}
      {activeTab === 'permissions' && (
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="space-y-6"
        >
          {Object.keys(permissionsByResource).length === 0 ? (
            <div className="flex items-center gap-3 p-4 rounded-lg bg-yellow-500/10 border border-yellow-500/20 text-yellow-300">
              <AlertCircle size={20} />
              <p>You don't have any permissions assigned.</p>
            </div>
          ) : (
            <div className="space-y-6">
              {Object.entries(permissionsByResource).map(([resource, actions]) => (
                <div key={resource} className="border border-border rounded-lg p-6 bg-surface/50">
                  <h3 className="text-lg font-semibold text-white mb-4 capitalize">{resource}</h3>
                  <div className="grid grid-cols-2 gap-3">
                    {actions.map((action) => (
                      <div
                        key={`${resource}:${action}`}
                        className="flex items-center gap-2 p-3 rounded-lg bg-zinc-900/50 border border-border"
                      >
                        <Check size={16} className="text-green-400" />
                        <span className="text-sm font-medium text-zinc-300">
                          {resource}:{action}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Permission Legend */}
          <div className="mt-8 p-4 rounded-lg bg-zinc-900 border border-zinc-800">
            <h4 className="text-sm font-semibold text-white mb-3">Permission Format</h4>
            <p className="text-xs text-zinc-400">
              Permissions are formatted as <code className="text-primary">resource:action</code>. For example:
            </p>
            <ul className="text-xs text-zinc-400 mt-2 space-y-1 ml-4 list-disc">
              <li><code className="text-primary">reports:read</code> - Ability to view reports</li>
              <li><code className="text-primary">audit:manage</code> - Ability to manage audit logs</li>
              <li><code className="text-primary">user:create</code> - Ability to create users</li>
              <li><code className="text-primary">*</code> - Full access (admin only)</li>
            </ul>
          </div>
        </motion.div>
      )}

      {/* Info Box */}
      <div className="mt-8 p-4 rounded-lg bg-blue-500/5 border border-blue-500/20">
        <div className="flex gap-3">
          <Shield className="w-5 h-5 text-blue-400 flex-shrink-0 mt-0.5" />
          <div className="text-sm text-blue-300">
            <p className="font-medium mb-1">About Your Access</p>
            <p>Your roles and permissions determine what actions you can perform in OpenRisk. If you need additional access, contact your administrator.</p>
          </div>
        </div>
      </div>
    </div>
  );
};
