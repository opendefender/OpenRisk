import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Shield, Lock, Unlock, AlertCircle, Users, Check } from 'lucide-react';
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
  : { name: 'Viewer', description: 'Read-only access to data' },
  : { name: 'Analyst', description: 'Can create and manage risks' },
  : { name: 'Manager', description: 'Can manage resources and users' },
  : { name: 'Admin', description: 'Full access and control' },
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
      <div className="flex items-center justify-center py-">
        <div className="animate-spin rounded-full h- w- border-t- border-b- border-primary"></div>
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
    <div className="space-y-">
      {/ Header /}
      <div className="flex items-center gap- mb-">
        <Shield className="w- h- text-primary" />
        <div>
          <h className="text-xl font-bold text-white">Access Control</h>
          <p className="text-sm text-zinc-">View your roles and permissions</p>
        </div>
      </div>

      {/ Tab Navigation /}
      <div className="flex gap- border-b border-border">
        <button
          onClick={() => setActiveTab('roles')}
          className={px- py- font-medium transition-colors border-b- ${
            activeTab === 'roles'
              ? 'border-primary text-primary'
              : 'border-transparent text-zinc- hover:text-white'
          }}
        >
          <div className="flex items-center gap-">
            <Users size={} />
            My Roles
          </div>
        </button>
        <button
          onClick={() => setActiveTab('permissions')}
          className={px- py- font-medium transition-colors border-b- ${
            activeTab === 'permissions'
              ? 'border-primary text-primary'
              : 'border-transparent text-zinc- hover:text-white'
          }}
        >
          <div className="flex items-center gap-">
            <Lock size={} />
            My Permissions
          </div>
        </button>
      </div>

      {/ Roles Tab /}
      {activeTab === 'roles' && (
        <motion.div
          initial={{ opacity: , y:  }}
          animate={{ opacity: , y:  }}
          className="space-y-"
        >
          {userRoles.length ===  ? (
            <div className="flex items-center gap- p- rounded-lg bg-yellow-/ border border-yellow-/ text-yellow-">
              <AlertCircle size={} />
              <p>You don't have any roles assigned. Contact your administrator.</p>
            </div>
          ) : (
            <div className="space-y-">
              {userRoles.map((role) => (
                <motion.div
                  key={role.id}
                  initial={{ opacity: , x: - }}
                  animate={{ opacity: , x:  }}
                  className="p- rounded-lg border border-border bg-surface/ hover:border-primary/ transition-colors"
                >
                  <div className="flex items-start justify-between mb-">
                    <div>
                      <h className="text-lg font-semibold text-white">{role.name}</h>
                      <p className="text-sm text-zinc-">{role.description}</p>
                    </div>
                    <div className="flex items-center gap-">
                      {role.is_predefined ? (
                        <span className="px- py- rounded-full text-xs font-medium bg-purple-/ text-purple-">
                          System
                        </span>
                      ) : (
                        <span className="px- py- rounded-full text-xs font-medium bg-green-/ text-green-">
                          Custom
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-">
                    <div className="w- h- bg-zinc- rounded-full overflow-hidden">
                      <div
                        className="h-full bg-primary transition-all"
                        style={{ width: ${(role.level / )  }% }}
                      />
                    </div>
                    <span className="text-xs font-medium text-zinc-">
                      Level {role.level}: {levelLabels[role.level as keyof typeof levelLabels]?.name || 'Unknown'}
                    </span>
                  </div>
                </motion.div>
              ))}
            </div>
          )}

          {isAdmin && allRoles.length >  && (
            <div className="mt- p- rounded-lg bg-blue-/ border border-blue-/">
              <h className="text-sm font-semibold text-blue- mb-">Admin - All Available Roles</h>
              <div className="space-y-">
                {allRoles.map((role) => (
                  <div key={role.id} className="flex items-center justify-between p- rounded bg-zinc-/">
                    <span className="text-sm text-zinc-">{role.name}</span>
                    <span className="text-xs text-zinc-">Level {role.level}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </motion.div>
      )}

      {/ Permissions Tab /}
      {activeTab === 'permissions' && (
        <motion.div
          initial={{ opacity: , y:  }}
          animate={{ opacity: , y:  }}
          className="space-y-"
        >
          {Object.keys(permissionsByResource).length ===  ? (
            <div className="flex items-center gap- p- rounded-lg bg-yellow-/ border border-yellow-/ text-yellow-">
              <AlertCircle size={} />
              <p>You don't have any permissions assigned.</p>
            </div>
          ) : (
            <div className="space-y-">
              {Object.entries(permissionsByResource).map(([resource, actions]) => (
                <div key={resource} className="border border-border rounded-lg p- bg-surface/">
                  <h className="text-lg font-semibold text-white mb- capitalize">{resource}</h>
                  <div className="grid grid-cols- gap-">
                    {actions.map((action) => (
                      <div
                        key={${resource}:${action}}
                        className="flex items-center gap- p- rounded-lg bg-zinc-/ border border-border"
                      >
                        <Check size={} className="text-green-" />
                        <span className="text-sm font-medium text-zinc-">
                          {resource}:{action}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}

          {/ Permission Legend /}
          <div className="mt- p- rounded-lg bg-zinc- border border-zinc-">
            <h className="text-sm font-semibold text-white mb-">Permission Format</h>
            <p className="text-xs text-zinc-">
              Permissions are formatted as <code className="text-primary">resource:action</code>. For example:
            </p>
            <ul className="text-xs text-zinc- mt- space-y- ml- list-disc">
              <li><code className="text-primary">reports:read</code> - Ability to view reports</li>
              <li><code className="text-primary">audit:manage</code> - Ability to manage audit logs</li>
              <li><code className="text-primary">user:create</code> - Ability to create users</li>
              <li><code className="text-primary"></code> - Full access (admin only)</li>
            </ul>
          </div>
        </motion.div>
      )}

      {/ Info Box /}
      <div className="mt- p- rounded-lg bg-blue-/ border border-blue-/">
        <div className="flex gap-">
          <Shield className="w- h- text-blue- flex-shrink- mt-." />
          <div className="text-sm text-blue-">
            <p className="font-medium mb-">About Your Access</p>
            <p>Your roles and permissions determine what actions you can perform in OpenRisk. If you need additional access, contact your administrator.</p>
          </div>
        </div>
      </div>
    </div>
  );
};
