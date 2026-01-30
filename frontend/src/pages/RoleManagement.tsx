import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Shield, Plus, Trash2, Edit2, Lock, Unlock, Search, ChevronRight } from 'lucide-react';
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
  0: { name: 'Viewer', color: 'bg-zinc-500', badge: 'text-zinc-300' },
  3: { name: 'Analyst', color: 'bg-blue-500', badge: 'text-blue-300' },
  6: { name: 'Manager', color: 'bg-purple-500', badge: 'text-purple-300' },
  9: { name: 'Admin', color: 'bg-red-500', badge: 'text-red-300' },
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
  const [newRoleLevel, setNewRoleLevel] = useState(3);
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
      const response = await api.get(`/rbac/roles/${roleId}`);
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
      setNewRoleLevel(3);
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
    if (!confirm(`Are you sure you want to delete the "${roleName}" role? This action cannot be undone.`)) {
      return;
    }

    try {
      await api.delete(`/rbac/roles/${roleId}`);
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
      await api.post(`/rbac/roles/${roleId}/permissions`, {
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
      await api.delete(`/rbac/roles/${roleId}/permissions/${permissionId}`);
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
      reports: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
      audit: 'bg-orange-500/10 text-orange-400 border-orange-500/20',
      connector: 'bg-purple-500/10 text-purple-400 border-purple-500/20',
      user: 'bg-green-500/10 text-green-400 border-green-500/20',
      role: 'bg-pink-500/10 text-pink-400 border-pink-500/20',
    };
    return colors[resource] || 'bg-zinc-500/10 text-zinc-400 border-zinc-500/20';
  };

  if (!isAdmin) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <Lock className="w-16 h-16 text-red-500 mx-auto mb-4" />
          <h1 className="text-2xl font-bold text-white mb-2">Access Denied</h1>
          <p className="text-zinc-400">You need administrator privileges to access role management.</p>
        </div>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-primary"></div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <div className="border-b border-border bg-surface/50 backdrop-blur-md sticky top-0 z-10">
        <div className="max-w-7xl mx-auto px-6 py-6">
          <div className="flex items-center justify-between mb-6">
            <div className="flex items-center gap-3">
              <Shield className="w-8 h-8 text-primary" />
              <div>
                <h1 className="text-2xl font-bold text-white">Role Management</h1>
                <p className="text-sm text-zinc-400">Manage roles and permissions</p>
              </div>
            </div>
            <Button
              onClick={() => setShowCreateModal(true)}
              className="flex items-center gap-2 bg-primary hover:bg-primary/90"
            >
              <Plus size={18} />
              Create Role
            </Button>
          </div>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-6 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Roles List */}
          <div className="lg:col-span-1">
            <div className="bg-surface border border-border rounded-lg p-6">
              <h2 className="text-lg font-semibold text-white mb-4">Roles</h2>

              {/* Search */}
              <div className="relative mb-6">
                <Search size={18} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-400" />
                <input
                  type="text"
                  placeholder="Search roles..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="w-full pl-10 pr-4 py-2 bg-zinc-900 border border-border rounded-lg text-white placeholder-zinc-500 focus:outline-none focus:border-primary"
                />
              </div>

              {/* Roles List */}
              <div className="space-y-2 max-h-[600px] overflow-y-auto">
                {filteredRoles.length === 0 ? (
                  <p className="text-center text-zinc-500 py-8">No roles found</p>
                ) : (
                  filteredRoles.map((role) => (
                    <motion.button
                      key={role.id}
                      onClick={() => fetchRoleWithPermissions(role.id)}
                      whileHover={{ x: 4 }}
                      className={`w-full text-left px-4 py-3 rounded-lg transition-colors ${
                        selectedRole?.id === role.id
                          ? 'bg-primary/10 border border-primary text-primary'
                          : 'bg-zinc-900/50 hover:bg-zinc-800 text-white'
                      }`}
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex-1">
                          <div className="font-medium">{role.name}</div>
                          <div className="text-xs text-zinc-400">
                            Level: {levelLabels[role.level as keyof typeof levelLabels]?.name || 'Custom'}
                          </div>
                        </div>
                        <ChevronRight size={16} className="text-zinc-400" />
                      </div>
                    </motion.button>
                  ))
                )}
              </div>
            </div>
          </div>

          {/* Role Details and Permissions */}
          <div className="lg:col-span-2">
            {selectedRole ? (
              <motion.div
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                className="space-y-6"
              >
                {/* Role Info Card */}
                <div className="bg-surface border border-border rounded-lg p-6">
                  <div className="flex items-start justify-between mb-6">
                    <div>
                      <h2 className="text-2xl font-bold text-white mb-2">{selectedRole.name}</h2>
                      <p className="text-zinc-400">{selectedRole.description}</p>
                    </div>
                    {!selectedRole.is_predefined && (
                      <button
                        onClick={() => handleDeleteRole(selectedRole.id, selectedRole.name)}
                        className="p-2 rounded-lg bg-red-500/10 text-red-400 hover:bg-red-500/20 transition-colors"
                      >
                        <Trash2 size={18} />
                      </button>
                    )}
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <div className="text-sm text-zinc-400 mb-1">Level</div>
                      <div className={`inline-block px-3 py-1 rounded-full text-sm font-medium ${
                        levelLabels[selectedRole.level as keyof typeof levelLabels]?.color || 'bg-zinc-500'
                      }`}>
                        {levelLabels[selectedRole.level as keyof typeof levelLabels]?.name || 'Custom'}
                      </div>
                    </div>
                    <div>
                      <div className="text-sm text-zinc-400 mb-1">Status</div>
                      <div className={`inline-block px-3 py-1 rounded-full text-sm font-medium ${
                        selectedRole.is_predefined
                          ? 'bg-purple-500/10 text-purple-400'
                          : 'bg-green-500/10 text-green-400'
                      }`}>
                        {selectedRole.is_predefined ? 'System Role' : 'Custom Role'}
                      </div>
                    </div>
                  </div>
                </div>

                {/* Permissions Matrix */}
                <div className="bg-surface border border-border rounded-lg p-6">
                  <div className="flex items-center justify-between mb-6">
                    <h3 className="text-lg font-semibold text-white">Permissions ({selectedRole.permissions.length})</h3>
                    <button
                      onClick={() => setShowPermissionMatrix(!showPermissionMatrix)}
                      className="text-sm text-primary hover:text-primary/80 font-medium"
                    >
                      {showPermissionMatrix ? 'Hide Matrix' : 'Show Matrix'}
                    </button>
                  </div>

                  {showPermissionMatrix ? (
                    // Permission Matrix View
                    <div className="space-y-4 max-h-[400px] overflow-y-auto">
                      {/* Group by resource */}
                      {Array.from(new Set(permissions.map((p) => p.resource))).map((resource) => (
                        <div key={resource} className="border border-border rounded-lg p-4">
                          <h4 className={`text-sm font-semibold mb-3 px-3 py-1 rounded-full inline-block ${getResourceColor(
                            resource
                          )}`}>
                            {resource.charAt(0).toUpperCase() + resource.slice(1)}
                          </h4>
                          <div className="space-y-2">
                            {permissions
                              .filter((p) => p.resource === resource)
                              .map((permission) => {
                                const isAssigned = selectedRole.permissions.some(
                                  (p) => p.id === permission.id
                                );
                                return (
                                  <div
                                    key={permission.id}
                                    className="flex items-center justify-between p-3 rounded-lg bg-zinc-900/50 hover:bg-zinc-800 transition-colors"
                                  >
                                    <div className="flex-1">
                                      <div className="text-sm font-medium text-white">
                                        {resource}:{permission.action}
                                      </div>
                                      <div className="text-xs text-zinc-400">{permission.description}</div>
                                    </div>
                                    <button
                                      onClick={() =>
                                        isAssigned
                                          ? handleRemovePermission(selectedRole.id, permission.id)
                                          : handleAssignPermission(selectedRole.id, permission.id)
                                      }
                                      className={`px-3 py-1 rounded-lg text-sm font-medium transition-colors ${
                                        isAssigned
                                          ? 'bg-green-500/10 text-green-400 hover:bg-green-500/20'
                                          : 'bg-zinc-700 text-zinc-300 hover:bg-zinc-600'
                                      }`}
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
                    <div className="space-y-2 max-h-[300px] overflow-y-auto">
                      {selectedRole.permissions.length === 0 ? (
                        <p className="text-center text-zinc-500 py-8">No permissions assigned</p>
                      ) : (
                        selectedRole.permissions.map((permission) => (
                          <div
                            key={permission.id}
                            className="flex items-center justify-between p-3 rounded-lg bg-zinc-900/50"
                          >
                            <div>
                              <div className="text-sm font-medium text-white">
                                {permission.resource}:{permission.action}
                              </div>
                              <div className="text-xs text-zinc-400">{permission.description}</div>
                            </div>
                            <button
                              onClick={() => handleRemovePermission(selectedRole.id, permission.id)}
                              className="p-1 rounded-lg text-zinc-400 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                            >
                              <Trash2 size={16} />
                            </button>
                          </div>
                        ))
                      )}
                    </div>
                  )}
                </div>
              </motion.div>
            ) : (
              <div className="flex items-center justify-center h-96 bg-surface border border-dashed border-border rounded-lg">
                <div className="text-center">
                  <Shield className="w-12 h-12 text-zinc-600 mx-auto mb-3" />
                  <p className="text-zinc-400">Select a role to view and manage permissions</p>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Create Role Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            initial={{ scale: 0.95, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            className="bg-surface border border-border rounded-lg p-6 max-w-md w-full mx-4"
          >
            <h2 className="text-xl font-bold text-white mb-6">Create New Role</h2>

            <div className="space-y-4 mb-6">
              <div>
                <label className="block text-sm font-medium text-zinc-300 mb-2">Role Name</label>
                <input
                  type="text"
                  value={newRoleName}
                  onChange={(e) => setNewRoleName(e.target.value)}
                  placeholder="e.g., Security Officer"
                  className="w-full px-4 py-2 bg-zinc-900 border border-border rounded-lg text-white placeholder-zinc-500 focus:outline-none focus:border-primary"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-zinc-300 mb-2">Description</label>
                <input
                  type="text"
                  value={newRoleDescription}
                  onChange={(e) => setNewRoleDescription(e.target.value)}
                  placeholder="Brief description of this role"
                  className="w-full px-4 py-2 bg-zinc-900 border border-border rounded-lg text-white placeholder-zinc-500 focus:outline-none focus:border-primary"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-zinc-300 mb-2">Level</label>
                <select
                  value={newRoleLevel}
                  onChange={(e) => setNewRoleLevel(Number(e.target.value))}
                  className="w-full px-4 py-2 bg-zinc-900 border border-border rounded-lg text-white focus:outline-none focus:border-primary"
                >
                  <option value={0}>Viewer (Level 0)</option>
                  <option value={3}>Analyst (Level 3)</option>
                  <option value={6}>Manager (Level 6)</option>
                  <option value={9}>Admin (Level 9)</option>
                </select>
              </div>
            </div>

            <div className="flex gap-3">
              <Button
                onClick={() => setShowCreateModal(false)}
                variant="ghost"
                className="flex-1"
              >
                Cancel
              </Button>
              <Button
                onClick={handleCreateRole}
                disabled={isCreating}
                className="flex-1"
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
