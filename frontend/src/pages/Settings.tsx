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
  const currentUser = useAuthStore((state) => state.user);

  return (
    <div className="flex h-screen bg-background overflow-hidden">
      {/ Settings Sidebar /}
      <div className="w- border-r border-border bg-surface/ p- flex flex-col">
        <h className="text-lg font-bold text-white mb- px-">Settings</h>
        <nav className="space-y-">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={w-full flex items-center gap- px- py- rounded-lg text-sm font-medium transition-colors ${
                activeTab === tab.id 
                  ? 'bg-primary/ text-primary' 
                  : 'text-zinc- hover:text-zinc- hover:bg-white/'
              }}
            >
              <tab.icon size={} />
              {tab.label}
            </button>
          ))}
        </nav>
      </div>

      {/ Content Area /}
      <div className="flex- overflow-y-auto p-">
        <div className="max-w-xl mx-auto">
          <motion.div
            key={activeTab}
            initial={{ opacity: , y:  }}
            animate={{ opacity: , y:  }}
            transition={{ duration: . }}
          >
            {activeTab === 'general' && <GeneralTab />}
            {activeTab === 'integrations' && <IntegrationsTab />}
            {activeTab === 'team' && <TeamTab />}
            {activeTab === 'rbac' && <RBACTab />}
            {activeTab === 'security' && <div className="text-zinc-">Security Audit Logs (Coming Soon)</div>}
          </motion.div>
        </div>
      </div>
    </div>
  );
};