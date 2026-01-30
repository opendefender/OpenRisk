import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Building2, Plus, Trash2, Users, Settings, Search, ChevronRight, Lock } from 'lucide-react';
import { api } from '../lib/api';
import { toast } from 'sonner';
import { Button } from '../components/ui/Button';
import { useAuthStore } from '../hooks/useAuthStore';

interface Tenant {
  id: string;
  name: string;
  slug: string;
  owner_id: string;
  status: 'active' | 'suspended' | 'deleted';
  is_active: boolean;
  created_at: string;
  user_count?: number;
}

interface TenantStats {
  total_users: number;
  active_users: number;
  total_roles: number;
  created_at: string;
}

export const TenantManagement = () => {
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [selectedTenant, setSelectedTenant] = useState<Tenant | null>(null);
  const [tenantStats, setTenantStats] = useState<TenantStats | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newTenantName, setNewTenantName] = useState('');
  const [newTenantSlug, setNewTenantSlug] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  const currentUser = useAuthStore((state) => state.user);
  const isAdmin = currentUser?.role === 'admin' || currentUser?.role === 'ADMIN';

  useEffect(() => {
    if (isAdmin) {
      fetchTenants();
    }
  }, [isAdmin]);

  const fetchTenants = async () => {
    setIsLoading(true);
    try {
      const response = await api.get('/rbac/tenants');
      setTenants(response.data || []);
    } catch (err: any) {
      toast.error("We couldn't load the tenants. Please try again.");
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  const fetchTenantStats = async (tenantId: string) => {
    try {
      const response = await api.get(`/rbac/tenants/${tenantId}/stats`);
      setTenantStats(response.data);
    } catch (err: any) {
      console.error('Failed to fetch tenant stats:', err);
      setTenantStats(null);
    }
  };

  const handleSelectTenant = (tenant: Tenant) => {
    setSelectedTenant(tenant);
    fetchTenantStats(tenant.id);
  };

  const handleCreateTenant = async () => {
    if (!newTenantName.trim()) {
      toast.error('Please enter a tenant name.');
      return;
    }

    if (!newTenantSlug.trim()) {
      toast.error('Please enter a tenant slug.');
      return;
    }

    setIsCreating(true);
    try {
      await api.post('/rbac/tenants', {
        name: newTenantName,
        slug: newTenantSlug,
      });
      toast.success('Tenant created successfully');
      setNewTenantName('');
      setNewTenantSlug('');
      setShowCreateModal(false);
      await fetchTenants();
    } catch (err: any) {
      toast.error(
        err.response?.data?.message || "We couldn't create the tenant. Please check your input and try again."
      );
    } finally {
      setIsCreating(false);
    }
  };

  const handleDeleteTenant = async (tenantId: string, tenantName: string) => {
    if (!confirm(`Are you sure you want to delete the "${tenantName}" tenant? This action cannot be undone.`)) {
      return;
    }

    try {
      await api.delete(`/rbac/tenants/${tenantId}`);
      toast.success('Tenant deleted successfully');
      setSelectedTenant(null);
      setTenantStats(null);
      await fetchTenants();
    } catch (err: any) {
      toast.error(
        err.response?.data?.message || "We couldn't delete this tenant. Please try again or contact support."
      );
    }
  };

  const filteredTenants = tenants.filter((tenant) =>
    tenant.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    tenant.slug.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'active':
        return 'bg-green-500/10 text-green-400 border-green-500/20';
      case 'suspended':
        return 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20';
      case 'deleted':
        return 'bg-red-500/10 text-red-400 border-red-500/20';
      default:
        return 'bg-zinc-500/10 text-zinc-400 border-zinc-500/20';
    }
  };

  if (!isAdmin) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <Lock className="w-16 h-16 text-red-500 mx-auto mb-4" />
          <h1 className="text-2xl font-bold text-white mb-2">Access Denied</h1>
          <p className="text-zinc-400">You need administrator privileges to access tenant management.</p>
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
              <Building2 className="w-8 h-8 text-primary" />
              <div>
                <h1 className="text-2xl font-bold text-white">Tenant Management</h1>
                <p className="text-sm text-zinc-400">Manage multi-tenant organizations</p>
              </div>
            </div>
            <Button
              onClick={() => setShowCreateModal(true)}
              className="flex items-center gap-2 bg-primary hover:bg-primary/90"
            >
              <Plus size={18} />
              Create Tenant
            </Button>
          </div>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-6 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Tenants List */}
          <div className="lg:col-span-1">
            <div className="bg-surface border border-border rounded-lg p-6">
              <h2 className="text-lg font-semibold text-white mb-4">Tenants ({tenants.length})</h2>

              {/* Search */}
              <div className="relative mb-6">
                <Search size={18} className="absolute left-3 top-1/2 -translate-y-1/2 text-zinc-400" />
                <input
                  type="text"
                  placeholder="Search tenants..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="w-full pl-10 pr-4 py-2 bg-zinc-900 border border-border rounded-lg text-white placeholder-zinc-500 focus:outline-none focus:border-primary"
                />
              </div>

              {/* Tenants List */}
              <div className="space-y-2 max-h-[600px] overflow-y-auto">
                {filteredTenants.length === 0 ? (
                  <p className="text-center text-zinc-500 py-8">No tenants found</p>
                ) : (
                  filteredTenants.map((tenant) => (
                    <motion.button
                      key={tenant.id}
                      onClick={() => handleSelectTenant(tenant)}
                      whileHover={{ x: 4 }}
                      className={`w-full text-left px-4 py-3 rounded-lg transition-colors ${
                        selectedTenant?.id === tenant.id
                          ? 'bg-primary/10 border border-primary text-primary'
                          : 'bg-zinc-900/50 hover:bg-zinc-800 text-white'
                      }`}
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex-1">
                          <div className="font-medium">{tenant.name}</div>
                          <div className="text-xs text-zinc-400 font-mono">{tenant.slug}</div>
                        </div>
                        <ChevronRight size={16} className="text-zinc-400" />
                      </div>
                    </motion.button>
                  ))
                )}
              </div>
            </div>
          </div>

          {/* Tenant Details */}
          <div className="lg:col-span-2">
            {selectedTenant ? (
              <motion.div
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                className="space-y-6"
              >
                {/* Tenant Info Card */}
                <div className="bg-surface border border-border rounded-lg p-6">
                  <div className="flex items-start justify-between mb-6">
                    <div>
                      <h2 className="text-2xl font-bold text-white mb-2">{selectedTenant.name}</h2>
                      <p className="text-zinc-400 font-mono">{selectedTenant.slug}</p>
                    </div>
                    <button
                      onClick={() => handleDeleteTenant(selectedTenant.id, selectedTenant.name)}
                      className="p-2 rounded-lg bg-red-500/10 text-red-400 hover:bg-red-500/20 transition-colors"
                    >
                      <Trash2 size={18} />
                    </button>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <div className="text-sm text-zinc-400 mb-1">Status</div>
                      <div className={`inline-block px-3 py-1 rounded-full text-sm font-medium border ${getStatusBadge(
                        selectedTenant.status
                      )}`}>
                        {selectedTenant.status.charAt(0).toUpperCase() + selectedTenant.status.slice(1)}
                      </div>
                    </div>
                    <div>
                      <div className="text-sm text-zinc-400 mb-1">Created</div>
                      <div className="text-sm text-white">
                        {new Date(selectedTenant.created_at).toLocaleDateString()}
                      </div>
                    </div>
                  </div>
                </div>

                {/* Tenant Statistics */}
                {tenantStats ? (
                  <div className="grid grid-cols-2 gap-4">
                    <div className="bg-surface border border-border rounded-lg p-4">
                      <div className="flex items-center gap-2 mb-2">
                        <Users size={18} className="text-blue-400" />
                        <span className="text-sm font-medium text-zinc-400">Total Users</span>
                      </div>
                      <div className="text-3xl font-bold text-white">{tenantStats.total_users}</div>
                      <p className="text-xs text-zinc-500 mt-1">{tenantStats.active_users} active</p>
                    </div>

                    <div className="bg-surface border border-border rounded-lg p-4">
                      <div className="flex items-center gap-2 mb-2">
                        <Settings size={18} className="text-purple-400" />
                        <span className="text-sm font-medium text-zinc-400">Roles</span>
                      </div>
                      <div className="text-3xl font-bold text-white">{tenantStats.total_roles}</div>
                      <p className="text-xs text-zinc-500 mt-1">system + custom</p>
                    </div>
                  </div>
                ) : (
                  <div className="bg-surface border border-dashed border-border rounded-lg p-4 text-center text-zinc-400">
                    Loading statistics...
                  </div>
                )}

                {/* Tenant Settings */}
                <div className="bg-surface border border-border rounded-lg p-6">
                  <h3 className="text-lg font-semibold text-white mb-4">Settings</h3>
                  <div className="space-y-4">
                    <div>
                      <label className="block text-sm font-medium text-zinc-300 mb-2">Tenant Name</label>
                      <input
                        type="text"
                        value={selectedTenant.name}
                        disabled
                        className="w-full px-4 py-2 bg-zinc-900 border border-border rounded-lg text-white opacity-50 cursor-not-allowed"
                      />
                      <p className="text-xs text-zinc-500 mt-1">Read-only for now</p>
                    </div>

                    <div>
                      <label className="block text-sm font-medium text-zinc-300 mb-2">Slug</label>
                      <input
                        type="text"
                        value={selectedTenant.slug}
                        disabled
                        className="w-full px-4 py-2 bg-zinc-900 border border-border rounded-lg text-white opacity-50 cursor-not-allowed font-mono"
                      />
                    </div>

                    <div className="flex gap-3 pt-4">
                      <Button
                        disabled
                        className="flex-1 opacity-50 cursor-not-allowed"
                      >
                        Save Changes
                      </Button>
                      <Button
                        onClick={() => handleDeleteTenant(selectedTenant.id, selectedTenant.name)}
                        className="flex-1 bg-red-500 hover:bg-red-600"
                      >
                        Delete Tenant
                      </Button>
                    </div>
                  </div>
                </div>

                {/* Tenant Members */}
                <div className="bg-surface border border-border rounded-lg p-6">
                  <h3 className="text-lg font-semibold text-white mb-4">Members ({tenantStats?.total_users || 0})</h3>
                  <p className="text-sm text-zinc-400">Member management coming in next phase</p>
                </div>
              </motion.div>
            ) : (
              <div className="flex items-center justify-center h-96 bg-surface border border-dashed border-border rounded-lg">
                <div className="text-center">
                  <Building2 className="w-12 h-12 text-zinc-600 mx-auto mb-3" />
                  <p className="text-zinc-400">Select a tenant to view details</p>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Create Tenant Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            initial={{ scale: 0.95, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            className="bg-surface border border-border rounded-lg p-6 max-w-md w-full mx-4"
          >
            <h2 className="text-xl font-bold text-white mb-6">Create New Tenant</h2>

            <div className="space-y-4 mb-6">
              <div>
                <label className="block text-sm font-medium text-zinc-300 mb-2">Tenant Name</label>
                <input
                  type="text"
                  value={newTenantName}
                  onChange={(e) => setNewTenantName(e.target.value)}
                  placeholder="e.g., Acme Corporation"
                  className="w-full px-4 py-2 bg-zinc-900 border border-border rounded-lg text-white placeholder-zinc-500 focus:outline-none focus:border-primary"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-zinc-300 mb-2">Slug</label>
                <input
                  type="text"
                  value={newTenantSlug}
                  onChange={(e) => setNewTenantSlug(e.target.value.toLowerCase())}
                  placeholder="e.g., acme-corp"
                  className="w-full px-4 py-2 bg-zinc-900 border border-border rounded-lg text-white placeholder-zinc-500 focus:outline-none focus:border-primary font-mono"
                />
                <p className="text-xs text-zinc-500 mt-1">URL-friendly identifier (lowercase, hyphens only)</p>
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
                onClick={handleCreateTenant}
                disabled={isCreating}
                className="flex-1"
              >
                {isCreating ? 'Creating...' : 'Create Tenant'}
              </Button>
            </div>
          </motion.div>
        </div>
      )}
    </div>
  );
};
