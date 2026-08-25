// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Users as UsersIcon, Shield, Trash2, Lock, Unlock, Plus, Search } from 'lucide-react';
import { api } from '../lib/api';
import { toast } from 'sonner';
import { Button, Field } from '../shared/ds';
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
      await api.patch(`/users/${userId}/status`, { is_active: !isActive });
      setUsers(users.map(u => u.id === userId ? { ...u, is_active: !isActive } : u));
      toast.success(isActive ? 'User disabled' : 'User enabled');
    } catch (err) {
      toast.error("We couldn't update this user's status. Please try again.");
    }
  };

  const updateUserRole = async (userId: string, newRole: string) => {
    try {
      await api.patch(`/users/${userId}/role`, { role: newRole });
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
      await api.delete(`/users/${userId}`);
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
        return 'bg-danger/10 text-danger-text border-danger/20';
      case 'analyst':
        return 'bg-accent-500/10 text-info-text border-accent-400/20';
      case 'viewer':
        return 'bg-surface-3/10 text-text-secondary border-border-strong/20';
      default:
        return 'bg-surface-3/10 text-text-secondary border-border-strong/20';
    }
  };

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
              <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center">
                <UsersIcon className="text-text-primary" size={24} />
              </div>
              <div>
                <h1 className="text-2xl font-bold text-text-primary">User Management</h1>
                <p className="text-sm text-text-secondary">Manage users and permissions</p>
              </div>
            </div>
            <Button className="shadow-lg shadow-blue-500/20" onClick={() => setShowCreateModal(true)}>
              <Plus size={16} className="mr-2" /> Create User
            </Button>
          </div>

          {/* Filters */}
          <div className="flex flex-col sm:flex-row gap-4">
            <div className="flex-1 relative">
              <Search size={16} className="absolute left-3 top-1/2 transform -translate-y-1/2 text-text-muted" />
              <input
                type="text"
                placeholder="Search by name, email, or username..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="w-full bg-surface-1/50 border border-border rounded-lg pl-10 pr-4 py-2 text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:ring-2 focus:ring-primary/50"
              />
            </div>
            <select
              value={selectedRole}
              onChange={(e) => setSelectedRole(e.target.value)}
              className="bg-surface-1/50 border border-border rounded-lg px-4 py-2 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-primary/50"
            >
              <option value="all">All Roles</option>
              <option value="admin">Admin</option>
              <option value="analyst">Analyst</option>
              <option value="viewer">Viewer</option>
            </select>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="max-w-7xl mx-auto px-6 py-8">
        {filteredUsers.length === 0 ? (
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            className="text-center py-12"
          >
            <UsersIcon size={48} className="mx-auto text-text-muted mb-4" />
            <p className="text-text-secondary">No users found</p>
          </motion.div>
        ) : (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="grid gap-4"
          >
            {filteredUsers.map((user) => (
              <motion.div
                key={user.id}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                className="bg-surface border border-border rounded-xl p-4 hover:border-primary/50 transition-all"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4 flex-1 min-w-0">
                    <div className="w-10 h-10 rounded-full bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center flex-shrink-0">
                      <span className="text-text-primary font-bold text-sm">
                        {user.full_name.charAt(0).toUpperCase()}
                      </span>
                    </div>
                    <div className="min-w-0 flex-1">
                      <h3 className="font-medium text-text-primary truncate">{user.full_name}</h3>
                      <div className="flex items-center gap-2 text-xs text-text-muted">
                        <span className="truncate">{user.email}</span>
                        <span>•</span>
                        <span className="flex-shrink-0">@{user.username}</span>
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-3 ml-4">
                    <div className="text-right">
                      <span className={`inline-block px-3 py-1 rounded-full text-xs font-medium border ${getRoleColor(user.role)}`}>
                        <Shield size={12} className="inline-block mr-1" />
                        {user.role}
                      </span>
                      <p className="text-xs text-text-muted mt-1">
                        {user.last_login ? `Last: ${new Date(user.last_login).toLocaleDateString()}` : 'Never logged in'}
                      </p>
                    </div>

                    <div className="flex items-center gap-2 ml-4">
                      <select
                        value={user.role}
                        onChange={(e) => updateUserRole(user.id, e.target.value)}
                        disabled={user.id === currentUser?.id}
                        className="bg-surface-1/50 border border-border rounded px-2 py-1 text-xs text-text-primary focus:outline-none focus:ring-2 focus:ring-primary/50 disabled:opacity-50"
                      >
                        <option value="viewer">Viewer</option>
                        <option value="analyst">Analyst</option>
                        <option value="admin">Admin</option>
                      </select>

                      <button
                        onClick={() => toggleUserStatus(user.id, user.is_active)}
                        className="p-2 hover:bg-surface-2 rounded-lg transition-colors"
                        title={user.is_active ? 'Disable user' : 'Enable user'}
                      >
                        {user.is_active ? (
                          <Unlock size={16} className="text-success-text" />
                        ) : (
                          <Lock size={16} className="text-warning-text" />
                        )}
                      </button>

                      <button
                        onClick={() => deleteUser(user.id)}
                        disabled={user.id === currentUser?.id}
                        className="p-2 hover:bg-danger/10 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                        title="Delete user"
                      >
                        <Trash2 size={16} className="text-danger-text" />
                      </button>
                    </div>
                  </div>
                </div>
              </motion.div>
            ))}
          </motion.div>
        )}
      </div>

      {/* Create User Modal - Only visible for admins */}
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
