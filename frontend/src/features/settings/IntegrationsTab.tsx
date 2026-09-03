// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useState } from 'react';
import { ShieldAlert, Database, Box, CheckCircle, AlertCircle, Loader2 } from 'lucide-react';
import { Button } from '../../shared/ds';
import { toast } from 'sonner';

const modules = [
  {
    id: 'thehive',
    name: 'TheHive',
    icon: ShieldAlert,
    color: 'text-warning-text',
    desc: 'Incident Response Platform & Case Management.',
  },
  {
    id: 'opencti',
    name: 'OpenCTI',
    icon: Box,
    color: 'text-info-text',
    desc: 'Cyber Threat Intelligence Knowledge Base.',
  },
  {
    id: 'openrmf',
    name: 'OpenRMF',
    icon: Database,
    color: 'text-success-text',
    desc: 'Risk Management Framework & Compliance.',
  },
];

export const IntegrationsTab = () => {
  const [enabledModules, setEnabledModules] = useState<string[]>(['thehive']);
  const [testingModule, setTestingModule] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<string, boolean>>({});

  const toggleModule = (id: string) => {
    if (enabledModules.includes(id)) {
      setEnabledModules(enabledModules.filter((m) => m !== id));
    } else {
      setEnabledModules([...enabledModules, id]);
    }
  };

  const testIntegration = async (id: string) => {
    setTestingModule(id);
    try {
      // Simulate API call to test integration
      await new Promise((resolve) => setTimeout(resolve, 2000));
      setTestResults((prev) => ({ ...prev, [id]: true }));
      toast.success(`${modules.find((m) => m.id === id)?.name} connection successful!`);
    } catch (error) {
      setTestResults((prev) => ({ ...prev, [id]: false }));
      toast.error('Connection test failed. Please check your credentials and try again.');
    } finally {
      setTestingModule(null);
    }
  };

  return (
    <div className="space-y-8">
      <div>
        <h3 className="text-2xl font-bold text-fg-primary mb-1">Integrations</h3>
        <p className="text-fg-secondary text-sm">Connect OpenRisk with the OpenDefender Suite.</p>
      </div>

      <div className="grid gap-4">
        {modules.map((mod) => (
          <div
            key={mod.id}
            className={`p-6 rounded-xl border transition-all ${
              enabledModules.includes(mod.id)
                ? 'bg-surface border-primary/30 shadow-[0_0_20px_-10px_rgba(59,130,246,0.2)]'
                : 'bg-surface/30 border-border-strong/5 opacity-70'
            }`}
          >
            <div className="flex items-start justify-between">
              <div className="flex gap-4 flex-1">
                <div
                  className={`p-3 rounded-lg bg-surface-1 border border-border-strong/10 ${mod.color}`}
                >
                  <mod.icon size={24} />
                </div>
                <div className="flex-1">
                  <h4 className="font-bold text-fg-primary flex items-center gap-2">
                    {mod.name}
                    {enabledModules.includes(mod.id) && (
                      <span className="text-[10px] bg-success/10 text-success-text px-2 py-0.5 rounded-full border border-success/20 flex items-center gap-1">
                        <CheckCircle size={10} /> ACTIVE
                      </span>
                    )}
                    {testResults[mod.id] === true && (
                      <span className="text-[10px] bg-success/10 text-success-text px-2 py-0.5 rounded-full border border-success/20 flex items-center gap-1">
                        <CheckCircle size={10} /> VERIFIED
                      </span>
                    )}
                    {testResults[mod.id] === false && (
                      <span className="text-[10px] bg-danger/10 text-danger-text px-2 py-0.5 rounded-full border border-danger/20 flex items-center gap-1">
                        <AlertCircle size={10} /> FAILED
                      </span>
                    )}
                  </h4>
                  <p className="text-sm text-fg-secondary mt-1">{mod.desc}</p>
                </div>
              </div>
              <div className="flex gap-2 ml-4">
                <Button
                  variant="ghost"
                  onClick={() => testIntegration(mod.id)}
                  disabled={!enabledModules.includes(mod.id) || testingModule === mod.id}
                  className="text-sm"
                >
                  {testingModule === mod.id ? (
                    <>
                      <Loader2 size={16} className="mr-2 animate-spin" />
                      Testing...
                    </>
                  ) : (
                    'Test'
                  )}
                </Button>
                <Button
                  variant={enabledModules.includes(mod.id) ? 'secondary' : 'primary'}
                  onClick={() => toggleModule(mod.id)}
                  className="h-10"
                >
                  {enabledModules.includes(mod.id) ? 'Configure' : 'Enable'}
                </Button>
              </div>
            </div>

            {enabledModules.includes(mod.id) && (
              <div className="mt-6 pt-6 border-t border-border-strong/5 grid gap-4 animate-fade-in">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="space-y-1">
                    <label className="text-xs font-bold text-fg-muted uppercase">API URL</label>
                    <input
                      className="w-full bg-surface-1 border border-border rounded px-3 py-2 text-sm text-fg-primary"
                      placeholder={`https://${mod.id}.opendefender.local`}
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs font-bold text-fg-muted uppercase">API Key</label>
                    <input
                      type="password"
                      className="w-full bg-surface-1 border border-border rounded px-3 py-2 text-sm text-fg-primary"
                      placeholder="••••••••••••••••"
                    />
                  </div>
                </div>
                <div className="flex gap-2 pt-2">
                  <Button variant="primary" className="text-sm">
                    Save Configuration
                  </Button>
                  <Button variant="ghost" className="text-sm">
                    Reset
                  </Button>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};
