import { useEffect, useState } from 'react';
import { Search, Star, Download, CheckCircle, AlertCircle, Trash2, Plus, RefreshCw, Eye, Code } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

interface Connector {
  id: string;
  name: string;
  author: string;
  version: string;
  description: string;
  long_description?: string;
  icon?: string;
  category: string;
  status: 'active' | 'beta' | 'deprecated' | 'inactive';
  capabilities: string[];
  rating: number;
  install_count: number;
  documentation: string;
  support_email?: string;
}

interface InstalledApp {
  id: string;
  connector_id: string;
  name: string;
  status: 'installed' | 'pending' | 'disabled' | 'error';
  enabled: boolean;
  last_sync_at?: string;
  last_sync_status?: string;
  version: string;
  connector?: Connector;
}

export default function Marketplace() {
  const [connectors, setConnectors] = useState<Connector[]>([]);
  const [installedApps, setInstalledApps] = useState<InstalledApp[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  const [loading, setLoading] = useState(true);
  const [selectedConnector, setSelectedConnector] = useState<Connector | null>(null);
  const [showInstallModal, setShowInstallModal] = useState(false);
  const [appName, setAppName] = useState('');
  const [activeTab, setActiveTab] = useState<'connectors' | 'installed'>('connectors');

  const categories = [
    { id: 'all', name: 'All' },
    { id: 'integration', name: 'Integration' },
    { id: 'reporting', name: 'Reporting' },
    { id: 'security', name: 'Security' },
    { id: 'compliance', name: 'Compliance' },
    { id: 'notification', name: 'Notification' },
  ];

  // Fetch available connectors
  useEffect(() => {
    const fetchConnectors = async () => {
      try {
        const response = await fetch('/api/v1/marketplace/connectors', {
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        });
        const data = await response.json();
        setConnectors(data.data || []);
      } catch (error) {
        console.error('Failed to fetch connectors:', error);
      } finally {
        setLoading(false);
      }
    };

    // Fetch installed apps
    const fetchInstalledApps = async () => {
      try {
        const response = await fetch('/api/v1/marketplace/apps', {
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        });
        const data = await response.json();
        setInstalledApps(data.data || []);
      } catch (error) {
        console.error('Failed to fetch installed apps:', error);
      }
    };

    fetchConnectors();
    fetchInstalledApps();
  }, []);

  // Filter and search connectors
  const filteredConnectors = connectors.filter(connector => {
    const matchesSearch = connector.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
                        connector.description.toLowerCase().includes(searchQuery.toLowerCase());
    const matchesCategory = selectedCategory === 'all' || connector.category === selectedCategory;
    return matchesSearch && matchesCategory;
  });

  // Install connector
  const handleInstall = async (connectorId: string) => {
    if (!appName.trim()) return;

    try {
      const response = await fetch('/api/v1/marketplace/apps', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          connector_id: connectorId,
          app_name: appName,
          configuration: {}
        })
      });

      if (response.ok) {
        const app = await response.json();
        setInstalledApps([...installedApps, app]);
        setShowInstallModal(false);
        setAppName('');
        setSelectedConnector(null);
      }
    } catch (error) {
      console.error('Failed to install connector:', error);
    }
  };

  // Uninstall app
  const handleUninstall = async (appId: string) => {
    if (!confirm('Are you sure you want to uninstall this app?')) return;

    try {
      await fetch(`/api/v1/marketplace/apps/${appId}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        }
      });

      setInstalledApps(installedApps.filter(app => app.id !== appId));
    } catch (error) {
      console.error('Failed to uninstall app:', error);
    }
  };

  // Toggle app
  const handleToggleApp = async (appId: string, enabled: boolean) => {
    const endpoint = enabled ? 'disable' : 'enable';
    try {
      await fetch(`/api/v1/marketplace/apps/${appId}/${endpoint}`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        }
      });

      setInstalledApps(installedApps.map(app =>
        app.id === appId ? { ...app, enabled: !enabled } : app
      ));
    } catch (error) {
      console.error('Failed to toggle app:', error);
    }
  };

  // Trigger manual sync
  const handleSync = async (appId: string) => {
    try {
      await fetch(`/api/v1/marketplace/apps/${appId}/sync`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        }
      });

      // Refresh apps list
      const response = await fetch('/api/v1/marketplace/apps', {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        }
      });
      const data = await response.json();
      setInstalledApps(data.data || []);
    } catch (error) {
      console.error('Failed to sync app:', error);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active':
      case 'installed':
        return 'bg-green-500/20 text-green-400';
      case 'beta':
        return 'bg-blue-500/20 text-blue-400';
      case 'pending':
        return 'bg-yellow-500/20 text-yellow-400';
      case 'error':
        return 'bg-red-500/20 text-red-400';
      default:
        return 'bg-gray-500/20 text-gray-400';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'active':
      case 'installed':
        return <CheckCircle className="w-4 h-4" />;
      case 'error':
      case 'pending':
        return <AlertCircle className="w-4 h-4" />;
      default:
        return null;
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-zinc-950 via-zinc-900 to-zinc-950 p-8">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          className="mb-8"
        >
          <h1 className="text-4xl font-bold text-white mb-2">Marketplace</h1>
          <p className="text-gray-400">Discover and manage connectors to extend OpenRisk</p>
        </motion.div>

        {/* Tab Navigation */}
        <div className="flex gap-4 mb-8 border-b border-zinc-800">
          <button
            onClick={() => setActiveTab('connectors')}
            className={`pb-4 px-4 font-medium transition-colors ${
              activeTab === 'connectors'
                ? 'text-blue-400 border-b-2 border-blue-400'
                : 'text-gray-400 hover:text-gray-300'
            }`}
          >
            <div className="flex items-center gap-2">
              <Search className="w-4 h-4" />
              Browse Connectors
            </div>
          </button>
          <button
            onClick={() => setActiveTab('installed')}
            className={`pb-4 px-4 font-medium transition-colors ${
              activeTab === 'installed'
                ? 'text-blue-400 border-b-2 border-blue-400'
                : 'text-gray-400 hover:text-gray-300'
            }`}
          >
            <div className="flex items-center gap-2">
              <CheckCircle className="w-4 h-4" />
              My Applications
              <span className="bg-blue-500/20 text-blue-400 px-2 py-0.5 rounded text-sm">
                {installedApps.length}
              </span>
            </div>
          </button>
        </div>

        {/* Browse Tab */}
        <AnimatePresence mode="wait">
          {activeTab === 'connectors' && (
            <motion.div
              key="connectors"
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -20 }}
              transition={{ duration: 0.3 }}
            >
              {/* Search and Filters */}
              <div className="mb-8 space-y-4">
                <div className="relative">
                  <Search className="absolute left-4 top-3 w-5 h-5 text-gray-500" />
                  <input
                    type="text"
                    placeholder="Search connectors..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    className="w-full pl-12 pr-4 py-2 bg-zinc-800 text-white rounded-lg border border-zinc-700 focus:border-blue-500 outline-none transition-colors"
                  />
                </div>

                {/* Category Filter */}
                <div className="flex gap-2 overflow-x-auto pb-2">
                  {categories.map(cat => (
                    <button
                      key={cat.id}
                      onClick={() => setSelectedCategory(cat.id)}
                      className={`px-4 py-2 rounded-lg whitespace-nowrap transition-colors ${
                        selectedCategory === cat.id
                          ? 'bg-blue-600 text-white'
                          : 'bg-zinc-800 text-gray-400 hover:bg-zinc-700'
                      }`}
                    >
                      {cat.name}
                    </button>
                  ))}
                </div>
              </div>

              {/* Connectors Grid */}
              {loading ? (
                <div className="text-center py-12">
                  <div className="inline-block animate-spin">
                    <RefreshCw className="w-8 h-8 text-blue-400" />
                  </div>
                </div>
              ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                  {filteredConnectors.map((connector) => (
                    <motion.div
                      key={connector.id}
                      initial={{ opacity: 0, y: 20 }}
                      animate={{ opacity: 1, y: 0 }}
                      whileHover={{ y: -4 }}
                      className="group bg-gradient-to-b from-zinc-800 to-zinc-900 rounded-xl border border-zinc-700 hover:border-zinc-600 p-6 transition-all"
                    >
                      {/* Header */}
                      <div className="flex items-start justify-between mb-4">
                        <div className="flex-1">
                          <div className="flex items-center gap-2 mb-1">
                            <h3 className="text-lg font-semibold text-white">{connector.name}</h3>
                            <span className={`px-2 py-1 rounded text-xs font-medium flex items-center gap-1 ${getStatusColor(connector.status)}`}>
                              {getStatusIcon(connector.status)}
                              {connector.status}
                            </span>
                          </div>
                          <p className="text-sm text-gray-500">by {connector.author}</p>
                        </div>
                        <div className="flex items-center gap-1 text-yellow-400">
                          <Star className="w-4 h-4 fill-current" />
                          <span className="text-sm font-medium">{connector.rating.toFixed(1)}</span>
                        </div>
                      </div>

                      {/* Description */}
                      <p className="text-sm text-gray-400 mb-4 line-clamp-2">
                        {connector.description}
                      </p>

                      {/* Capabilities */}
                      <div className="flex flex-wrap gap-2 mb-4">
                        {connector.capabilities.slice(0, 3).map((cap) => (
                          <span
                            key={cap}
                            className="px-2 py-1 text-xs bg-blue-500/10 text-blue-400 rounded"
                          >
                            {cap}
                          </span>
                        ))}
                        {connector.capabilities.length > 3 && (
                          <span className="px-2 py-1 text-xs bg-zinc-700/50 text-gray-400 rounded">
                            +{connector.capabilities.length - 3} more
                          </span>
                        )}
                      </div>

                      {/* Stats */}
                      <div className="flex items-center justify-between text-sm text-gray-500 mb-4 pb-4 border-b border-zinc-700">
                        <div className="flex items-center gap-1">
                          <Download className="w-4 h-4" />
                          {connector.install_count} installs
                        </div>
                        <span className="text-xs">v{connector.version}</span>
                      </div>

                      {/* Actions */}
                      <div className="flex gap-2">
                        <button
                          onClick={() => {
                            setSelectedConnector(connector);
                            setAppName(`${connector.name} Instance`);
                            setShowInstallModal(true);
                          }}
                          className="flex-1 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-medium transition-colors flex items-center justify-center gap-2"
                        >
                          <Plus className="w-4 h-4" />
                          Install
                        </button>
                        <a
                          href={connector.documentation}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="px-4 py-2 bg-zinc-700 hover:bg-zinc-600 text-gray-300 rounded-lg transition-colors flex items-center justify-center"
                        >
                          <Code className="w-4 h-4" />
                        </a>
                      </div>
                    </motion.div>
                  ))}
                </div>
              )}

              {!loading && filteredConnectors.length === 0 && (
                <div className="text-center py-12">
                  <Search className="w-12 h-12 text-gray-600 mx-auto mb-4" />
                  <p className="text-gray-400">No connectors found</p>
                </div>
              )}
            </motion.div>
          )}

          {/* Installed Tab */}
          {activeTab === 'installed' && (
            <motion.div
              key="installed"
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -20 }}
              transition={{ duration: 0.3 }}
            >
              {installedApps.length === 0 ? (
                <div className="text-center py-12">
                  <CheckCircle className="w-12 h-12 text-gray-600 mx-auto mb-4" />
                  <p className="text-gray-400 mb-4">No installed applications yet</p>
                  <button
                    onClick={() => setActiveTab('connectors')}
                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-medium transition-colors"
                  >
                    Browse Marketplace
                  </button>
                </div>
              ) : (
                <div className="space-y-4">
                  {installedApps.map((app) => (
                    <motion.div
                      key={app.id}
                      initial={{ opacity: 0, y: 20 }}
                      animate={{ opacity: 1, y: 0 }}
                      className="bg-gradient-to-r from-zinc-800 to-zinc-900 rounded-xl border border-zinc-700 p-6"
                    >
                      <div className="flex items-start justify-between">
                        <div className="flex-1">
                          <div className="flex items-center gap-3 mb-2">
                            <h3 className="text-lg font-semibold text-white">{app.name}</h3>
                            <span className={`px-2 py-1 rounded text-xs font-medium flex items-center gap-1 ${getStatusColor(app.status)}`}>
                              {getStatusIcon(app.status)}
                              {app.status}
                            </span>
                          </div>
                          <p className="text-sm text-gray-400">{app.connector?.description}</p>
                          {app.last_sync_at && (
                            <p className="text-xs text-gray-500 mt-2">
                              Last sync: {new Date(app.last_sync_at).toLocaleString()}
                            </p>
                          )}
                        </div>

                        {/* Actions */}
                        <div className="flex gap-2 ml-4">
                          <button
                            onClick={() => handleSync(app.id)}
                            className="p-2 hover:bg-zinc-700 rounded-lg text-gray-400 hover:text-gray-300 transition-colors"
                            title="Sync now"
                          >
                            <RefreshCw className="w-5 h-5" />
                          </button>
                          <button
                            onClick={() => handleToggleApp(app.id, app.enabled)}
                            className={`p-2 rounded-lg transition-colors ${
                              app.enabled
                                ? 'bg-green-500/10 text-green-400 hover:bg-green-500/20'
                                : 'bg-gray-700/50 text-gray-400 hover:bg-gray-600/50'
                            }`}
                            title={app.enabled ? 'Disable' : 'Enable'}
                          >
                            <Eye className="w-5 h-5" />
                          </button>
                          <button
                            onClick={() => handleUninstall(app.id)}
                            className="p-2 hover:bg-red-500/10 rounded-lg text-gray-400 hover:text-red-400 transition-colors"
                            title="Uninstall"
                          >
                            <Trash2 className="w-5 h-5" />
                          </button>
                        </div>
                      </div>
                    </motion.div>
                  ))}
                </div>
              )}
            </motion.div>
          )}
        </AnimatePresence>

        {/* Install Modal */}
        <AnimatePresence>
          {showInstallModal && selectedConnector && (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50"
              onClick={() => setShowInstallModal(false)}
            >
              <motion.div
                initial={{ scale: 0.95, opacity: 0 }}
                animate={{ scale: 1, opacity: 1 }}
                exit={{ scale: 0.95, opacity: 0 }}
                onClick={(e) => e.stopPropagation()}
                className="bg-zinc-900 rounded-xl border border-zinc-700 p-8 max-w-md w-full"
              >
                <h2 className="text-2xl font-bold text-white mb-2">Install Connector</h2>
                <p className="text-gray-400 mb-6">
                  Install <span className="font-semibold text-blue-400">{selectedConnector.name}</span>
                </p>

                <div className="space-y-4 mb-6">
                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Application Name
                    </label>
                    <input
                      type="text"
                      value={appName}
                      onChange={(e) => setAppName(e.target.value)}
                      className="w-full px-4 py-2 bg-zinc-800 text-white rounded-lg border border-zinc-700 focus:border-blue-500 outline-none"
                      placeholder="e.g., My Splunk Connector"
                    />
                  </div>
                </div>

                <div className="flex gap-3">
                  <button
                    onClick={() => setShowInstallModal(false)}
                    className="flex-1 px-4 py-2 bg-zinc-800 hover:bg-zinc-700 text-gray-300 rounded-lg font-medium transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={() => handleInstall(selectedConnector.id)}
                    disabled={!appName.trim()}
                    className="flex-1 px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-700 disabled:cursor-not-allowed text-white rounded-lg font-medium transition-colors"
                  >
                    Install
                  </button>
                </div>
              </motion.div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}
