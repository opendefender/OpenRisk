import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Users as UsersIcon, Shield, Trash, Lock, Unlock, Plus, Search } from 'lucide-react';
import { api } from '../lib/api';
import { toast } from 'sonner';
import { Button } from '../components/ui/Button';
import { useAuthStore } from '../hooks/useAuthStore';
import { CreateUserModal } from '../features/users/CreateUserModal';

interface User {
  id: string;
  email: string;
  username: string;
  full_name: string;
  role: string;
  is_active: boolean;
  created_at: string;
  last_login?: string;
}

export const Users = () => {
  const [users, setUsers] = useState<User[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedRole, setSelectedRole] = useState<string>('all');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const currentUser = useAuthStore((state) => state.user);
  const isAdmin = currentUser?.role === 'admin' || currentUser?.role === 'ADMIN';

  useEffect(() => {
    fetchUsers();
  }, []);

  const fetchUsers = async () => {
    setIsLoading(true);
    try {
      const response = await api.get('/users');
      setUsers(response.data || []);
    } catch (err: any) {
      toast.error("We couldn't load the user list. Please refresh the page and try again.");
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  const toggleUserStatus = async (userId: string, isActive: boolean) => {
    try {
      await api.patch(/users/${userId}/status, { is_active: !isActive });
      setUsers(users.map(u => u.id === userId ? { ...u, is_active: !isActive } : u));
      toast.success(isActive ? 'User disabled' : 'User enabled');
    } catch (err) {
      toast.error("We couldn't update this user's status. Please try again.");
    }
  };

  const updateUserRole = async (userId: string, newRole: string) => {
    try {
      await api.patch(/users/${userId}/role, { role: newRole });
      setUsers(users.map(u => u.id === userId ? { ...u, role: newRole } : u));
      toast.success('User role updated');
    } catch (err) {
      toast.error("Couldn't change the user's role. Please verify the selection and try again.");
    }
  };

  const deleteUser = async (userId: string) => {
    if (userId === currentUser?.id) {
      toast.error('Cannot delete your own account');
      return;
    }

    if (!confirm('Are you sure you want to delete this user?')) {
      return;
    }

    try {
      await api.delete(/users/${userId});
      setUsers(users.filter(u => u.id !== userId));
      toast.success('User deleted');
    } catch (err) {
      toast.error("We couldn't delete this user. Please try again or contact support if the problem persists.");
    }
  };

  const filteredUsers = users.filter(user => {
    const matchesSearch = user.email.toLowerCase().includes(searchTerm.toLowerCase()) ||
                          user.username.toLowerCase().includes(searchTerm.toLowerCase()) ||
                          user.full_name.toLowerCase().includes(searchTerm.toLowerCase());
    const matchesRole = selectedRole === 'all' || user.role.toLowerCase() === selectedRole.toLowerCase();
    return matchesSearch && matchesRole;
  });

  const getRoleColor = (role: string) => {
    switch (role.toLowerCase()) {
      case 'admin':
        return 'bg-red-/ text-red- border-red-/';
      case 'analyst':
        return 'bg-blue-/ text-blue- border-blue-/';
      case 'viewer':
        return 'bg-zinc-/ text-zinc- border-zinc-/';
      default:
        return 'bg-gray-/ text-gray- border-gray-/';
    }
  };

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
              <div className="w- h- rounded-lg bg-gradient-to-br from-blue- to-purple- flex items-center justify-center">
                <UsersIcon className="text-white" size={} />
              </div>
              <div>
                <h className="text-xl font-bold text-white">User Management</h>
                <p className="text-sm text-zinc-">Manage users and permissions</p>
              </div>
            </div>
            <Button className="shadow-lg shadow-blue-/" onClick={() => setShowCreateModal(true)}>
              <Plus size={} className="mr-" /> Create User
            </Button>
          </div>

          {/ Filters /}
          <div className="flex flex-col sm:flex-row gap-">
            <div className="flex- relative">
              <Search size={} className="absolute left- top-/ transform -translate-y-/ text-zinc-" />
              <input
                type="text"
                placeholder="Search by name, email, or username..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="w-full bg-zinc-/ border border-border rounded-lg pl- pr- py- text-sm text-white placeholder:text-zinc- focus:outline-none focus:ring- focus:ring-primary/"
              />
            </div>
            <select
              value={selectedRole}
              onChange={(e) => setSelectedRole(e.target.value)}
              className="bg-zinc-/ border border-border rounded-lg px- py- text-sm text-white focus:outline-none focus:ring- focus:ring-primary/"
            >
              <option value="all">All Roles</option>
              <option value="admin">Admin</option>
              <option value="analyst">Analyst</option>
              <option value="viewer">Viewer</option>
            </select>
          </div>
        </div>
      </div>

      {/ Content /}
      <div className="max-w-xl mx-auto px- py-">
        {filteredUsers.length ===  ? (
          <motion.div
            initial={{ opacity: , y:  }}
            animate={{ opacity: , y:  }}
            className="text-center py-"
          >
            <UsersIcon size={} className="mx-auto text-zinc- mb-" />
            <p className="text-zinc-">No users found</p>
          </motion.div>
        ) : (
          <motion.div
            initial={{ opacity:  }}
            animate={{ opacity:  }}
            className="grid gap-"
          >
            {filteredUsers.map((user) => (
              <motion.div
                key={user.id}
                initial={{ opacity: , y:  }}
                animate={{ opacity: , y:  }}
                className="bg-surface border border-border rounded-xl p- hover:border-primary/ transition-all"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap- flex- min-w-">
                    <div className="w- h- rounded-full bg-gradient-to-br from-blue- to-purple- flex items-center justify-center flex-shrink-">
                      <span className="text-white font-bold text-sm">
                        {user.full_name.charAt().toUpperCase()}
                      </span>
                    </div>
                    <div className="min-w- flex-">
                      <h className="font-medium text-white truncate">{user.full_name}</h>
                      <div className="flex items-center gap- text-xs text-zinc-">
                        <span className="truncate">{user.email}</span>
                        <span>•</span>
                        <span className="flex-shrink-">@{user.username}</span>
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap- ml-">
                    <div className="text-right">
                      <span className={inline-block px- py- rounded-full text-xs font-medium border ${getRoleColor(user.role)}}>
                        <Shield size={} className="inline-block mr-" />
                        {user.role}
                      </span>
                      <p className="text-xs text-zinc- mt-">
                        {user.last_login ? Last: ${new Date(user.last_login).toLocaleDateString()} : 'Never logged in'}
                      </p>
                    </div>

                    <div className="flex items-center gap- ml-">
                      <select
                        value={user.role}
                        onChange={(e) => updateUserRole(user.id, e.target.value)}
                        disabled={user.id === currentUser?.id}
                        className="bg-zinc-/ border border-border rounded px- py- text-xs text-white focus:outline-none focus:ring- focus:ring-primary/ disabled:opacity-"
                      >
                        <option value="viewer">Viewer</option>
                        <option value="analyst">Analyst</option>
                        <option value="admin">Admin</option>
                      </select>

                      <button
                        onClick={() => toggleUserStatus(user.id, user.is_active)}
                        className="p- hover:bg-zinc- rounded-lg transition-colors"
                        title={user.is_active ? 'Disable user' : 'Enable user'}
                      >
                        {user.is_active ? (
                          <Unlock size={} className="text-green-" />
                        ) : (
                          <Lock size={} className="text-yellow-" />
                        )}
                      </button>

                      <button
                        onClick={() => deleteUser(user.id)}
                        disabled={user.id === currentUser?.id}
                        className="p- hover:bg-red-/ rounded-lg transition-colors disabled:opacity- disabled:cursor-not-allowed"
                        title="Delete user"
                      >
                        <Trash size={} className="text-red-" />
                      </button>
                    </div>
                  </div>
                </div>
              </motion.div>
            ))}
          </motion.div>
        )}
      </div>

      {/ Create User Modal - Only visible for admins /}
      {isAdmin && (
        <CreateUserModal
          isOpen={showCreateModal}
          onClose={() => setShowCreateModal(false)}
          onSuccess={fetchUsers}
        />
      )}
    </div>
  );
};

export default Users;
