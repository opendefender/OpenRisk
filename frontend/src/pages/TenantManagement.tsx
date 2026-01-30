import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Building, Plus, Trash, Users, Settings, Search, ChevronRight, Lock } from 'lucide-react';
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
      const response = await api.get(/rbac/tenants/${tenantId}/stats);
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
    if (!confirm(Are you sure you want to delete the "${tenantName}" tenant? This action cannot be undone.)) {
      return;
    }

    try {
      await api.delete(/rbac/tenants/${tenantId});
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
        return 'bg-green-/ text-green- border-green-/';
      case 'suspended':
        return 'bg-yellow-/ text-yellow- border-yellow-/';
      case 'deleted':
        return 'bg-red-/ text-red- border-red-/';
      default:
        return 'bg-zinc-/ text-zinc- border-zinc-/';
    }
  };

  if (!isAdmin) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <Lock className="w- h- text-red- mx-auto mb-" />
          <h className="text-xl font-bold text-white mb-">Access Denied</h>
          <p className="text-zinc-">You need administrator privileges to access tenant management.</p>
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
              <Building className="w- h- text-primary" />
              <div>
                <h className="text-xl font-bold text-white">Tenant Management</h>
                <p className="text-sm text-zinc-">Manage multi-tenant organizations</p>
              </div>
            </div>
            <Button
              onClick={() => setShowCreateModal(true)}
              className="flex items-center gap- bg-primary hover:bg-primary/"
            >
              <Plus size={} />
              Create Tenant
            </Button>
          </div>
        </div>
      </div>

      <div className="max-w-xl mx-auto px- py-">
        <div className="grid grid-cols- lg:grid-cols- gap-">
          {/ Tenants List /}
          <div className="lg:col-span-">
            <div className="bg-surface border border-border rounded-lg p-">
              <h className="text-lg font-semibold text-white mb-">Tenants ({tenants.length})</h>

              {/ Search /}
              <div className="relative mb-">
                <Search size={} className="absolute left- top-/ -translate-y-/ text-zinc-" />
                <input
                  type="text"
                  placeholder="Search tenants..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="w-full pl- pr- py- bg-zinc- border border-border rounded-lg text-white placeholder-zinc- focus:outline-none focus:border-primary"
                />
              </div>

              {/ Tenants List /}
              <div className="space-y- max-h-[px] overflow-y-auto">
                {filteredTenants.length ===  ? (
                  <p className="text-center text-zinc- py-">No tenants found</p>
                ) : (
                  filteredTenants.map((tenant) => (
                    <motion.button
                      key={tenant.id}
                      onClick={() => handleSelectTenant(tenant)}
                      whileHover={{ x:  }}
                      className={w-full text-left px- py- rounded-lg transition-colors ${
                        selectedTenant?.id === tenant.id
                          ? 'bg-primary/ border border-primary text-primary'
                          : 'bg-zinc-/ hover:bg-zinc- text-white'
                      }}
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex-">
                          <div className="font-medium">{tenant.name}</div>
                          <div className="text-xs text-zinc- font-mono">{tenant.slug}</div>
                        </div>
                        <ChevronRight size={} className="text-zinc-" />
                      </div>
                    </motion.button>
                  ))
                )}
              </div>
            </div>
          </div>

          {/ Tenant Details /}
          <div className="lg:col-span-">
            {selectedTenant ? (
              <motion.div
                initial={{ opacity: , y:  }}
                animate={{ opacity: , y:  }}
                className="space-y-"
              >
                {/ Tenant Info Card /}
                <div className="bg-surface border border-border rounded-lg p-">
                  <div className="flex items-start justify-between mb-">
                    <div>
                      <h className="text-xl font-bold text-white mb-">{selectedTenant.name}</h>
                      <p className="text-zinc- font-mono">{selectedTenant.slug}</p>
                    </div>
                    <button
                      onClick={() => handleDeleteTenant(selectedTenant.id, selectedTenant.name)}
                      className="p- rounded-lg bg-red-/ text-red- hover:bg-red-/ transition-colors"
                    >
                      <Trash size={} />
                    </button>
                  </div>

                  <div className="grid grid-cols- gap-">
                    <div>
                      <div className="text-sm text-zinc- mb-">Status</div>
                      <div className={inline-block px- py- rounded-full text-sm font-medium border ${getStatusBadge(
                        selectedTenant.status
                      )}}>
                        {selectedTenant.status.charAt().toUpperCase() + selectedTenant.status.slice()}
                      </div>
                    </div>
                    <div>
                      <div className="text-sm text-zinc- mb-">Created</div>
                      <div className="text-sm text-white">
                        {new Date(selectedTenant.created_at).toLocaleDateString()}
                      </div>
                    </div>
                  </div>
                </div>

                {/ Tenant Statistics /}
                {tenantStats ? (
                  <div className="grid grid-cols- gap-">
                    <div className="bg-surface border border-border rounded-lg p-">
                      <div className="flex items-center gap- mb-">
                        <Users size={} className="text-blue-" />
                        <span className="text-sm font-medium text-zinc-">Total Users</span>
                      </div>
                      <div className="text-xl font-bold text-white">{tenantStats.total_users}</div>
                      <p className="text-xs text-zinc- mt-">{tenantStats.active_users} active</p>
                    </div>

                    <div className="bg-surface border border-border rounded-lg p-">
                      <div className="flex items-center gap- mb-">
                        <Settings size={} className="text-purple-" />
                        <span className="text-sm font-medium text-zinc-">Roles</span>
                      </div>
                      <div className="text-xl font-bold text-white">{tenantStats.total_roles}</div>
                      <p className="text-xs text-zinc- mt-">system + custom</p>
                    </div>
                  </div>
                ) : (
                  <div className="bg-surface border border-dashed border-border rounded-lg p- text-center text-zinc-">
                    Loading statistics...
                  </div>
                )}

                {/ Tenant Settings /}
                <div className="bg-surface border border-border rounded-lg p-">
                  <h className="text-lg font-semibold text-white mb-">Settings</h>
                  <div className="space-y-">
                    <div>
                      <label className="block text-sm font-medium text-zinc- mb-">Tenant Name</label>
                      <input
                        type="text"
                        value={selectedTenant.name}
                        disabled
                        className="w-full px- py- bg-zinc- border border-border rounded-lg text-white opacity- cursor-not-allowed"
                      />
                      <p className="text-xs text-zinc- mt-">Read-only for now</p>
                    </div>

                    <div>
                      <label className="block text-sm font-medium text-zinc- mb-">Slug</label>
                      <input
                        type="text"
                        value={selectedTenant.slug}
                        disabled
                        className="w-full px- py- bg-zinc- border border-border rounded-lg text-white opacity- cursor-not-allowed font-mono"
                      />
                    </div>

                    <div className="flex gap- pt-">
                      <Button
                        disabled
                        className="flex- opacity- cursor-not-allowed"
                      >
                        Save Changes
                      </Button>
                      <Button
                        onClick={() => handleDeleteTenant(selectedTenant.id, selectedTenant.name)}
                        className="flex- bg-red- hover:bg-red-"
                      >
                        Delete Tenant
                      </Button>
                    </div>
                  </div>
                </div>

                {/ Tenant Members /}
                <div className="bg-surface border border-border rounded-lg p-">
                  <h className="text-lg font-semibold text-white mb-">Members ({tenantStats?.total_users || })</h>
                  <p className="text-sm text-zinc-">Member management coming in next phase</p>
                </div>
              </motion.div>
            ) : (
              <div className="flex items-center justify-center h- bg-surface border border-dashed border-border rounded-lg">
                <div className="text-center">
                  <Building className="w- h- text-zinc- mx-auto mb-" />
                  <p className="text-zinc-">Select a tenant to view details</p>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/ Create Tenant Modal /}
      {showCreateModal && (
        <div className="fixed inset- bg-black/ flex items-center justify-center z-">
          <motion.div
            initial={{ scale: ., opacity:  }}
            animate={{ scale: , opacity:  }}
            className="bg-surface border border-border rounded-lg p- max-w-md w-full mx-"
          >
            <h className="text-xl font-bold text-white mb-">Create New Tenant</h>

            <div className="space-y- mb-">
              <div>
                <label className="block text-sm font-medium text-zinc- mb-">Tenant Name</label>
                <input
                  type="text"
                  value={newTenantName}
                  onChange={(e) => setNewTenantName(e.target.value)}
                  placeholder="e.g., Acme Corporation"
                  className="w-full px- py- bg-zinc- border border-border rounded-lg text-white placeholder-zinc- focus:outline-none focus:border-primary"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-zinc- mb-">Slug</label>
                <input
                  type="text"
                  value={newTenantSlug}
                  onChange={(e) => setNewTenantSlug(e.target.value.toLowerCase())}
                  placeholder="e.g., acme-corp"
                  className="w-full px- py- bg-zinc- border border-border rounded-lg text-white placeholder-zinc- focus:outline-none focus:border-primary font-mono"
                />
                <p className="text-xs text-zinc- mt-">URL-friendly identifier (lowercase, hyphens only)</p>
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
                onClick={handleCreateTenant}
                disabled={isCreating}
                className="flex-"
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
