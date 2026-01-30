import { useEffect, useState } from 'react';
import { Search, Star, Download, CheckCircle, AlertCircle, Trash, Plus, RefreshCw, Eye, Code } from 'lucide-react';
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
        const response = await fetch('/api/v/marketplace/connectors', {
          headers: {
            'Authorization': Bearer ${localStorage.getItem('token')}
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
        const response = await fetch('/api/v/marketplace/apps', {
          headers: {
            'Authorization': Bearer ${localStorage.getItem('token')}
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
      const response = await fetch('/api/v/marketplace/apps', {
        method: 'POST',
        headers: {
          'Authorization': Bearer ${localStorage.getItem('token')},
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
      await fetch(/api/v/marketplace/apps/${appId}, {
        method: 'DELETE',
        headers: {
          'Authorization': Bearer ${localStorage.getItem('token')}
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
      await fetch(/api/v/marketplace/apps/${appId}/${endpoint}, {
        method: 'POST',
        headers: {
          'Authorization': Bearer ${localStorage.getItem('token')}
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
      await fetch(/api/v/marketplace/apps/${appId}/sync, {
        method: 'POST',
        headers: {
          'Authorization': Bearer ${localStorage.getItem('token')}
        }
      });

      // Refresh apps list
      const response = await fetch('/api/v/marketplace/apps', {
        headers: {
          'Authorization': Bearer ${localStorage.getItem('token')}
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
        return 'bg-green-/ text-green-';
      case 'beta':
        return 'bg-blue-/ text-blue-';
      case 'pending':
        return 'bg-yellow-/ text-yellow-';
      case 'error':
        return 'bg-red-/ text-red-';
      default:
        return 'bg-gray-/ text-gray-';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'active':
      case 'installed':
        return <CheckCircle className="w- h-" />;
      case 'error':
      case 'pending':
        return <AlertCircle className="w- h-" />;
      default:
        return null;
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-zinc- via-zinc- to-zinc- p-">
      <div className="max-w-xl mx-auto">
        {/ Header /}
        <motion.div
          initial={{ opacity: , y: - }}
          animate={{ opacity: , y:  }}
          className="mb-"
        >
          <h className="text-xl font-bold text-white mb-">Marketplace</h>
          <p className="text-gray-">Discover and manage connectors to extend OpenRisk</p>
        </motion.div>

        {/ Tab Navigation /}
        <div className="flex gap- mb- border-b border-zinc-">
          <button
            onClick={() => setActiveTab('connectors')}
            className={pb- px- font-medium transition-colors ${
              activeTab === 'connectors'
                ? 'text-blue- border-b- border-blue-'
                : 'text-gray- hover:text-gray-'
            }}
          >
            <div className="flex items-center gap-">
              <Search className="w- h-" />
              Browse Connectors
            </div>
          </button>
          <button
            onClick={() => setActiveTab('installed')}
            className={pb- px- font-medium transition-colors ${
              activeTab === 'installed'
                ? 'text-blue- border-b- border-blue-'
                : 'text-gray- hover:text-gray-'
            }}
          >
            <div className="flex items-center gap-">
              <CheckCircle className="w- h-" />
              My Applications
              <span className="bg-blue-/ text-blue- px- py-. rounded text-sm">
                {installedApps.length}
              </span>
            </div>
          </button>
        </div>

        {/ Browse Tab /}
        <AnimatePresence mode="wait">
          {activeTab === 'connectors' && (
            <motion.div
              key="connectors"
              initial={{ opacity: , y:  }}
              animate={{ opacity: , y:  }}
              exit={{ opacity: , y: - }}
              transition={{ duration: . }}
            >
              {/ Search and Filters /}
              <div className="mb- space-y-">
                <div className="relative">
                  <Search className="absolute left- top- w- h- text-gray-" />
                  <input
                    type="text"
                    placeholder="Search connectors..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    className="w-full pl- pr- py- bg-zinc- text-white rounded-lg border border-zinc- focus:border-blue- outline-none transition-colors"
                  />
                </div>

                {/ Category Filter /}
                <div className="flex gap- overflow-x-auto pb-">
                  {categories.map(cat => (
                    <button
                      key={cat.id}
                      onClick={() => setSelectedCategory(cat.id)}
                      className={px- py- rounded-lg whitespace-nowrap transition-colors ${
                        selectedCategory === cat.id
                          ? 'bg-blue- text-white'
                          : 'bg-zinc- text-gray- hover:bg-zinc-'
                      }}
                    >
                      {cat.name}
                    </button>
                  ))}
                </div>
              </div>

              {/ Connectors Grid /}
              {loading ? (
                <div className="text-center py-">
                  <div className="inline-block animate-spin">
                    <RefreshCw className="w- h- text-blue-" />
                  </div>
                </div>
              ) : (
                <div className="grid grid-cols- md:grid-cols- lg:grid-cols- gap-">
                  {filteredConnectors.map((connector) => (
                    <motion.div
                      key={connector.id}
                      initial={{ opacity: , y:  }}
                      animate={{ opacity: , y:  }}
                      whileHover={{ y: - }}
                      className="group bg-gradient-to-b from-zinc- to-zinc- rounded-xl border border-zinc- hover:border-zinc- p- transition-all"
                    >
                      {/ Header /}
                      <div className="flex items-start justify-between mb-">
                        <div className="flex-">
                          <div className="flex items-center gap- mb-">
                            <h className="text-lg font-semibold text-white">{connector.name}</h>
                            <span className={px- py- rounded text-xs font-medium flex items-center gap- ${getStatusColor(connector.status)}}>
                              {getStatusIcon(connector.status)}
                              {connector.status}
                            </span>
                          </div>
                          <p className="text-sm text-gray-">by {connector.author}</p>
                        </div>
                        <div className="flex items-center gap- text-yellow-">
                          <Star className="w- h- fill-current" />
                          <span className="text-sm font-medium">{connector.rating.toFixed()}</span>
                        </div>
                      </div>

                      {/ Description /}
                      <p className="text-sm text-gray- mb- line-clamp-">
                        {connector.description}
                      </p>

                      {/ Capabilities /}
                      <div className="flex flex-wrap gap- mb-">
                        {connector.capabilities.slice(, ).map((cap) => (
                          <span
                            key={cap}
                            className="px- py- text-xs bg-blue-/ text-blue- rounded"
                          >
                            {cap}
                          </span>
                        ))}
                        {connector.capabilities.length >  && (
                          <span className="px- py- text-xs bg-zinc-/ text-gray- rounded">
                            +{connector.capabilities.length - } more
                          </span>
                        )}
                      </div>

                      {/ Stats /}
                      <div className="flex items-center justify-between text-sm text-gray- mb- pb- border-b border-zinc-">
                        <div className="flex items-center gap-">
                          <Download className="w- h-" />
                          {connector.install_count} installs
                        </div>
                        <span className="text-xs">v{connector.version}</span>
                      </div>

                      {/ Actions /}
                      <div className="flex gap-">
                        <button
                          onClick={() => {
                            setSelectedConnector(connector);
                            setAppName(${connector.name} Instance);
                            setShowInstallModal(true);
                          }}
                          className="flex- px- py- bg-blue- hover:bg-blue- text-white rounded-lg font-medium transition-colors flex items-center justify-center gap-"
                        >
                          <Plus className="w- h-" />
                          Install
                        </button>
                        <a
                          href={connector.documentation}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="px- py- bg-zinc- hover:bg-zinc- text-gray- rounded-lg transition-colors flex items-center justify-center"
                        >
                          <Code className="w- h-" />
                        </a>
                      </div>
                    </motion.div>
                  ))}
                </div>
              )}

              {!loading && filteredConnectors.length ===  && (
                <div className="text-center py-">
                  <Search className="w- h- text-gray- mx-auto mb-" />
                  <p className="text-gray-">No connectors found</p>
                </div>
              )}
            </motion.div>
          )}

          {/ Installed Tab /}
          {activeTab === 'installed' && (
            <motion.div
              key="installed"
              initial={{ opacity: , y:  }}
              animate={{ opacity: , y:  }}
              exit={{ opacity: , y: - }}
              transition={{ duration: . }}
            >
              {installedApps.length ===  ? (
                <div className="text-center py-">
                  <CheckCircle className="w- h- text-gray- mx-auto mb-" />
                  <p className="text-gray- mb-">No installed applications yet</p>
                  <button
                    onClick={() => setActiveTab('connectors')}
                    className="px- py- bg-blue- hover:bg-blue- text-white rounded-lg font-medium transition-colors"
                  >
                    Browse Marketplace
                  </button>
                </div>
              ) : (
                <div className="space-y-">
                  {installedApps.map((app) => (
                    <motion.div
                      key={app.id}
                      initial={{ opacity: , y:  }}
                      animate={{ opacity: , y:  }}
                      className="bg-gradient-to-r from-zinc- to-zinc- rounded-xl border border-zinc- p-"
                    >
                      <div className="flex items-start justify-between">
                        <div className="flex-">
                          <div className="flex items-center gap- mb-">
                            <h className="text-lg font-semibold text-white">{app.name}</h>
                            <span className={px- py- rounded text-xs font-medium flex items-center gap- ${getStatusColor(app.status)}}>
                              {getStatusIcon(app.status)}
                              {app.status}
                            </span>
                          </div>
                          <p className="text-sm text-gray-">{app.connector?.description}</p>
                          {app.last_sync_at && (
                            <p className="text-xs text-gray- mt-">
                              Last sync: {new Date(app.last_sync_at).toLocaleString()}
                            </p>
                          )}
                        </div>

                        {/ Actions /}
                        <div className="flex gap- ml-">
                          <button
                            onClick={() => handleSync(app.id)}
                            className="p- hover:bg-zinc- rounded-lg text-gray- hover:text-gray- transition-colors"
                            title="Sync now"
                          >
                            <RefreshCw className="w- h-" />
                          </button>
                          <button
                            onClick={() => handleToggleApp(app.id, app.enabled)}
                            className={p- rounded-lg transition-colors ${
                              app.enabled
                                ? 'bg-green-/ text-green- hover:bg-green-/'
                                : 'bg-gray-/ text-gray- hover:bg-gray-/'
                            }}
                            title={app.enabled ? 'Disable' : 'Enable'}
                          >
                            <Eye className="w- h-" />
                          </button>
                          <button
                            onClick={() => handleUninstall(app.id)}
                            className="p- hover:bg-red-/ rounded-lg text-gray- hover:text-red- transition-colors"
                            title="Uninstall"
                          >
                            <Trash className="w- h-" />
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

        {/ Install Modal /}
        <AnimatePresence>
          {showInstallModal && selectedConnector && (
            <motion.div
              initial={{ opacity:  }}
              animate={{ opacity:  }}
              exit={{ opacity:  }}
              className="fixed inset- bg-black/ flex items-center justify-center p- z-"
              onClick={() => setShowInstallModal(false)}
            >
              <motion.div
                initial={{ scale: ., opacity:  }}
                animate={{ scale: , opacity:  }}
                exit={{ scale: ., opacity:  }}
                onClick={(e) => e.stopPropagation()}
                className="bg-zinc- rounded-xl border border-zinc- p- max-w-md w-full"
              >
                <h className="text-xl font-bold text-white mb-">Install Connector</h>
                <p className="text-gray- mb-">
                  Install <span className="font-semibold text-blue-">{selectedConnector.name}</span>
                </p>

                <div className="space-y- mb-">
                  <div>
                    <label className="block text-sm font-medium text-gray- mb-">
                      Application Name
                    </label>
                    <input
                      type="text"
                      value={appName}
                      onChange={(e) => setAppName(e.target.value)}
                      className="w-full px- py- bg-zinc- text-white rounded-lg border border-zinc- focus:border-blue- outline-none"
                      placeholder="e.g., My Splunk Connector"
                    />
                  </div>
                </div>

                <div className="flex gap-">
                  <button
                    onClick={() => setShowInstallModal(false)}
                    className="flex- px- py- bg-zinc- hover:bg-zinc- text-gray- rounded-lg font-medium transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={() => handleInstall(selectedConnector.id)}
                    disabled={!appName.trim()}
                    className="flex- px- py- bg-blue- hover:bg-blue- disabled:bg-gray- disabled:cursor-not-allowed text-white rounded-lg font-medium transition-colors"
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
