import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Shield, Plus, Trash, Edit, Lock, Unlock, Search, ChevronRight } from 'lucide-react';
import { api } from '../lib/api';
import { toast } from 'sonner';
import { Button } from '../components/ui/Button';
import { useAuthStore } from '../hooks/useAuthStore';

interface Permission {
  id: string;
  resource: string;
  action: string;
  description: string;
  is_system: boolean;
}

interface Role {
  id: string;
  name: string;
  description: string;
  level: number;
  is_predefined: boolean;
  is_active: boolean;
  created_at: string;
}

interface RoleWithPermissions extends Role {
  permissions: Permission[];
}

const levelLabels: Record<number, { name: string; color: string; badge: string }> = {
  : { name: 'Viewer', color: 'bg-zinc-', badge: 'text-zinc-' },
  : { name: 'Analyst', color: 'bg-blue-', badge: 'text-blue-' },
  : { name: 'Manager', color: 'bg-purple-', badge: 'text-purple-' },
  : { name: 'Admin', color: 'bg-red-', badge: 'text-red-' },
};

export const RoleManagement = () => {
  const [roles, setRoles] = useState<Role[]>([]);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedRole, setSelectedRole] = useState<RoleWithPermissions | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showPermissionMatrix, setShowPermissionMatrix] = useState(false);
  const [newRoleName, setNewRoleName] = useState('');
  const [newRoleDescription, setNewRoleDescription] = useState('');
  const [newRoleLevel, setNewRoleLevel] = useState();
  const [selectedPermissions, setSelectedPermissions] = useState<string[]>([]);
  const [isCreating, setIsCreating] = useState(false);
  const currentUser = useAuthStore((state) => state.user);
  const isAdmin = currentUser?.role === 'admin' || currentUser?.role === 'ADMIN';

  useEffect(() => {
    if (isAdmin) {
      fetchRolesAndPermissions();
    }
  }, [isAdmin]);

  const fetchRolesAndPermissions = async () => {
    setIsLoading(true);
    try {
      const [rolesRes, permsRes] = await Promise.all([
        api.get('/rbac/roles'),
        api.get('/rbac/permissions'),
      ]);
      setRoles(rolesRes.data?.roles || []);
      setPermissions(permsRes.data || []);
    } catch (err: any) {
      toast.error("We couldn't load roles and permissions. Please try again.");
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  const fetchRoleWithPermissions = async (roleId: string) => {
    try {
      const response = await api.get(/rbac/roles/${roleId});
      setSelectedRole(response.data);
      setSelectedPermissions(response.data.permissions.map((p: Permission) => p.id));
    } catch (err: any) {
      toast.error("We couldn't load this role's permissions. Please try again.");
    }
  };

  const handleCreateRole = async () => {
    if (!newRoleName.trim()) {
      toast.error('Please enter a role name.');
      return;
    }

    setIsCreating(true);
    try {
      await api.post('/rbac/roles', {
        name: newRoleName,
        description: newRoleDescription,
        level: newRoleLevel,
      });
      toast.success('Role created successfully');
      setNewRoleName('');
      setNewRoleDescription('');
      setNewRoleLevel();
      setShowCreateModal(false);
      await fetchRolesAndPermissions();
    } catch (err: any) {
      toast.error(
        err.response?.data?.message || "We couldn't create the role. Please check your input and try again."
      );
    } finally {
      setIsCreating(false);
    }
  };

  const handleDeleteRole = async (roleId: string, roleName: string) => {
    if (!confirm(Are you sure you want to delete the "${roleName}" role? This action cannot be undone.)) {
      return;
    }

    try {
      await api.delete(/rbac/roles/${roleId});
      toast.success('Role deleted successfully');
      setSelectedRole(null);
      await fetchRolesAndPermissions();
    } catch (err: any) {
      toast.error(
        err.response?.data?.message || "We couldn't delete this role. Please try again or contact support."
      );
    }
  };

  const handleAssignPermission = async (roleId: string, permissionId: string) => {
    try {
      await api.post(/rbac/roles/${roleId}/permissions, {
        permission_id: permissionId,
      });
      toast.success('Permission assigned successfully');
      await fetchRoleWithPermissions(roleId);
    } catch (err: any) {
      toast.error("We couldn't assign this permission. Please try again.");
    }
  };

  const handleRemovePermission = async (roleId: string, permissionId: string) => {
    try {
      await api.delete(/rbac/roles/${roleId}/permissions/${permissionId});
      toast.success('Permission removed successfully');
      await fetchRoleWithPermissions(roleId);
    } catch (err: any) {
      toast.error("We couldn't remove this permission. Please try again.");
    }
  };

  const filteredRoles = roles.filter((role) =>
    role.name.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const getResourceColor = (resource: string) => {
    const colors: Record<string, string> = {
      reports: 'bg-blue-/ text-blue- border-blue-/',
      audit: 'bg-orange-/ text-orange- border-orange-/',
      connector: 'bg-purple-/ text-purple- border-purple-/',
      user: 'bg-green-/ text-green- border-green-/',
      role: 'bg-pink-/ text-pink- border-pink-/',
    };
    return colors[resource] || 'bg-zinc-/ text-zinc- border-zinc-/';
  };

  if (!isAdmin) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <Lock className="w- h- text-red- mx-auto mb-" />
          <h className="text-xl font-bold text-white mb-">Access Denied</h>
          <p className="text-zinc-">You need administrator privileges to access role management.</p>
        </div>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h- w- border-t- border-b- border-primary"></div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-background">
      {/ Header /}
      <div className="border-b border-border bg-surface/ backdrop-blur-md sticky top- z-">
        <div className="max-w-xl mx-auto px- py-">
          <div className="flex items-center justify-between mb-">
            <div className="flex items-center gap-">
              <Shield className="w- h- text-primary" />
              <div>
                <h className="text-xl font-bold text-white">Role Management</h>
                <p className="text-sm text-zinc-">Manage roles and permissions</p>
              </div>
            </div>
            <Button
              onClick={() => setShowCreateModal(true)}
              className="flex items-center gap- bg-primary hover:bg-primary/"
            >
              <Plus size={} />
              Create Role
            </Button>
          </div>
        </div>
      </div>

      <div className="max-w-xl mx-auto px- py-">
        <div className="grid grid-cols- lg:grid-cols- gap-">
          {/ Roles List /}
          <div className="lg:col-span-">
            <div className="bg-surface border border-border rounded-lg p-">
              <h className="text-lg font-semibold text-white mb-">Roles</h>

              {/ Search /}
              <div className="relative mb-">
                <Search size={} className="absolute left- top-/ -translate-y-/ text-zinc-" />
                <input
                  type="text"
                  placeholder="Search roles..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="w-full pl- pr- py- bg-zinc- border border-border rounded-lg text-white placeholder-zinc- focus:outline-none focus:border-primary"
                />
              </div>

              {/ Roles List /}
              <div className="space-y- max-h-[px] overflow-y-auto">
                {filteredRoles.length ===  ? (
                  <p className="text-center text-zinc- py-">No roles found</p>
                ) : (
                  filteredRoles.map((role) => (
                    <motion.button
                      key={role.id}
                      onClick={() => fetchRoleWithPermissions(role.id)}
                      whileHover={{ x:  }}
                      className={w-full text-left px- py- rounded-lg transition-colors ${
                        selectedRole?.id === role.id
                          ? 'bg-primary/ border border-primary text-primary'
                          : 'bg-zinc-/ hover:bg-zinc- text-white'
                      }}
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex-">
                          <div className="font-medium">{role.name}</div>
                          <div className="text-xs text-zinc-">
                            Level: {levelLabels[role.level as keyof typeof levelLabels]?.name || 'Custom'}
                          </div>
                        </div>
                        <ChevronRight size={} className="text-zinc-" />
                      </div>
                    </motion.button>
                  ))
                )}
              </div>
            </div>
          </div>

          {/ Role Details and Permissions /}
          <div className="lg:col-span-">
            {selectedRole ? (
              <motion.div
                initial={{ opacity: , y:  }}
                animate={{ opacity: , y:  }}
                className="space-y-"
              >
                {/ Role Info Card /}
                <div className="bg-surface border border-border rounded-lg p-">
                  <div className="flex items-start justify-between mb-">
                    <div>
                      <h className="text-xl font-bold text-white mb-">{selectedRole.name}</h>
                      <p className="text-zinc-">{selectedRole.description}</p>
                    </div>
                    {!selectedRole.is_predefined && (
                      <button
                        onClick={() => handleDeleteRole(selectedRole.id, selectedRole.name)}
                        className="p- rounded-lg bg-red-/ text-red- hover:bg-red-/ transition-colors"
                      >
                        <Trash size={} />
                      </button>
                    )}
                  </div>

                  <div className="grid grid-cols- gap-">
                    <div>
                      <div className="text-sm text-zinc- mb-">Level</div>
                      <div className={inline-block px- py- rounded-full text-sm font-medium ${
                        levelLabels[selectedRole.level as keyof typeof levelLabels]?.color || 'bg-zinc-'
                      }}>
                        {levelLabels[selectedRole.level as keyof typeof levelLabels]?.name || 'Custom'}
                      </div>
                    </div>
                    <div>
                      <div className="text-sm text-zinc- mb-">Status</div>
                      <div className={inline-block px- py- rounded-full text-sm font-medium ${
                        selectedRole.is_predefined
                          ? 'bg-purple-/ text-purple-'
                          : 'bg-green-/ text-green-'
                      }}>
                        {selectedRole.is_predefined ? 'System Role' : 'Custom Role'}
                      </div>
                    </div>
                  </div>
                </div>

                {/ Permissions Matrix /}
                <div className="bg-surface border border-border rounded-lg p-">
                  <div className="flex items-center justify-between mb-">
                    <h className="text-lg font-semibold text-white">Permissions ({selectedRole.permissions.length})</h>
                    <button
                      onClick={() => setShowPermissionMatrix(!showPermissionMatrix)}
                      className="text-sm text-primary hover:text-primary/ font-medium"
                    >
                      {showPermissionMatrix ? 'Hide Matrix' : 'Show Matrix'}
                    </button>
                  </div>

                  {showPermissionMatrix ? (
                    // Permission Matrix View
                    <div className="space-y- max-h-[px] overflow-y-auto">
                      {/ Group by resource /}
                      {Array.from(new Set(permissions.map((p) => p.resource))).map((resource) => (
                        <div key={resource} className="border border-border rounded-lg p-">
                          <h className={text-sm font-semibold mb- px- py- rounded-full inline-block ${getResourceColor(
                            resource
                          )}}>
                            {resource.charAt().toUpperCase() + resource.slice()}
                          </h>
                          <div className="space-y-">
                            {permissions
                              .filter((p) => p.resource === resource)
                              .map((permission) => {
                                const isAssigned = selectedRole.permissions.some(
                                  (p) => p.id === permission.id
                                );
                                return (
                                  <div
                                    key={permission.id}
                                    className="flex items-center justify-between p- rounded-lg bg-zinc-/ hover:bg-zinc- transition-colors"
                                  >
                                    <div className="flex-">
                                      <div className="text-sm font-medium text-white">
                                        {resource}:{permission.action}
                                      </div>
                                      <div className="text-xs text-zinc-">{permission.description}</div>
                                    </div>
                                    <button
                                      onClick={() =>
                                        isAssigned
                                          ? handleRemovePermission(selectedRole.id, permission.id)
                                          : handleAssignPermission(selectedRole.id, permission.id)
                                      }
                                      className={px- py- rounded-lg text-sm font-medium transition-colors ${
                                        isAssigned
                                          ? 'bg-green-/ text-green- hover:bg-green-/'
                                          : 'bg-zinc- text-zinc- hover:bg-zinc-'
                                      }}
                                    >
                                      {isAssigned ? 'Assigned' : 'Assign'}
                                    </button>
                                  </div>
                                );
                              })}
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    // Compact Permission List
                    <div className="space-y- max-h-[px] overflow-y-auto">
                      {selectedRole.permissions.length ===  ? (
                        <p className="text-center text-zinc- py-">No permissions assigned</p>
                      ) : (
                        selectedRole.permissions.map((permission) => (
                          <div
                            key={permission.id}
                            className="flex items-center justify-between p- rounded-lg bg-zinc-/"
                          >
                            <div>
                              <div className="text-sm font-medium text-white">
                                {permission.resource}:{permission.action}
                              </div>
                              <div className="text-xs text-zinc-">{permission.description}</div>
                            </div>
                            <button
                              onClick={() => handleRemovePermission(selectedRole.id, permission.id)}
                              className="p- rounded-lg text-zinc- hover:text-red- hover:bg-red-/ transition-colors"
                            >
                              <Trash size={} />
                            </button>
                          </div>
                        ))
                      )}
                    </div>
                  )}
                </div>
              </motion.div>
            ) : (
              <div className="flex items-center justify-center h- bg-surface border border-dashed border-border rounded-lg">
                <div className="text-center">
                  <Shield className="w- h- text-zinc- mx-auto mb-" />
                  <p className="text-zinc-">Select a role to view and manage permissions</p>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/ Create Role Modal /}
      {showCreateModal && (
        <div className="fixed inset- bg-black/ flex items-center justify-center z-">
          <motion.div
            initial={{ scale: ., opacity:  }}
            animate={{ scale: , opacity:  }}
            className="bg-surface border border-border rounded-lg p- max-w-md w-full mx-"
          >
            <h className="text-xl font-bold text-white mb-">Create New Role</h>

            <div className="space-y- mb-">
              <div>
                <label className="block text-sm font-medium text-zinc- mb-">Role Name</label>
                <input
                  type="text"
                  value={newRoleName}
                  onChange={(e) => setNewRoleName(e.target.value)}
                  placeholder="e.g., Security Officer"
                  className="w-full px- py- bg-zinc- border border-border rounded-lg text-white placeholder-zinc- focus:outline-none focus:border-primary"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-zinc- mb-">Description</label>
                <input
                  type="text"
                  value={newRoleDescription}
                  onChange={(e) => setNewRoleDescription(e.target.value)}
                  placeholder="Brief description of this role"
                  className="w-full px- py- bg-zinc- border border-border rounded-lg text-white placeholder-zinc- focus:outline-none focus:border-primary"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-zinc- mb-">Level</label>
                <select
                  value={newRoleLevel}
                  onChange={(e) => setNewRoleLevel(Number(e.target.value))}
                  className="w-full px- py- bg-zinc- border border-border rounded-lg text-white focus:outline-none focus:border-primary"
                >
                  <option value={}>Viewer (Level )</option>
                  <option value={}>Analyst (Level )</option>
                  <option value={}>Manager (Level )</option>
                  <option value={}>Admin (Level )</option>
                </select>
              </div>
            </div>

            <div className="flex gap-">
              <Button
                onClick={() => setShowCreateModal(false)}
                variant="outline"
                className="flex-"
              >
                Cancel
              </Button>
              <Button
                onClick={handleCreateRole}
                disabled={isCreating}
                className="flex-"
              >
                {isCreating ? 'Creating...' : 'Create Role'}
              </Button>
            </div>
          </motion.div>
        </div>
      )}
    </div>
  );
};
