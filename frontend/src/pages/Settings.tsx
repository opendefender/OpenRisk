import { useState } from 'react';
import { motion } from 'framer-motion';
import { User, Shield, Globe, Key, Lock } from 'lucide-react';
import { GeneralTab } from '../features/settings/GeneralTab';
import { IntegrationsTab } from '../features/settings/IntegrationsTab';
import { TeamTab } from '../features/settings/TeamTab';
import { RBACTab } from '../features/settings/RBACTab';
import { useAuthStore } from '../hooks/useAuthStore';

const tabs = [
  { id: 'general', label: 'General', icon: User },
  { id: 'team', label: 'Team & Members', icon: Shield },
  { id: 'integrations', label: 'Integrations', icon: Globe },
  { id: 'rbac', label: 'Access Control', icon: Lock },
  { id: 'security', label: 'Security', icon: Key },
];

export const Settings = () => {
  const [activeTab, setActiveTab] = useState('general');
  useAuthStore((state) => state.user);

  return (
    <div className="flex h-screen bg-background overflow-hidden">
      {/* Settings Sidebar */}
      <div className="w-64 border-r border-border bg-surface/30 p-6 flex flex-col">
        <h2 className="text-lg font-bold text-white mb-6 px-3">Settings</h2>
        <nav className="space-y-1">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                activeTab === tab.id 
                  ? 'bg-primary/10 text-primary' 
                  : 'text-zinc-400 hover:text-zinc-100 hover:bg-white/5'
              }`}
            >
              <tab.icon size={18} />
              {tab.label}
            </button>
          ))}
        </nav>
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-y-auto p-12">
        <div className="max-w-3xl mx-auto">
          <motion.div
            key={activeTab}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.2 }}
          >
            {activeTab === 'general' && <GeneralTab />}
            {activeTab === 'integrations' && <IntegrationsTab />}
            {activeTab === 'team' && <TeamTab />}
            {activeTab === 'rbac' && <RBACTab />}
            {activeTab === 'security' && <div className="text-zinc-500">Security Audit Logs (Coming Soon)</div>}
          </motion.div>
        </div>
      </div>
    </div>
  );
};