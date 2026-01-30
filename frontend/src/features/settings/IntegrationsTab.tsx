import { useState } from 'react';
import { ShieldAlert, Database, Box, CheckCircle, AlertCircle, Loader } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { toast } from 'sonner';

const modules = [
  { id: 'thehive', name: 'TheHive', icon: ShieldAlert, color: 'text-yellow-', desc: 'Incident Response Platform & Case Management.' },
  { id: 'opencti', name: 'OpenCTI', icon: Box, color: 'text-blue-', desc: 'Cyber Threat Intelligence Knowledge Base.' },
  { id: 'openrmf', name: 'OpenRMF', icon: Database, color: 'text-emerald-', desc: 'Risk Management Framework & Compliance.' },
];

export const IntegrationsTab = () => {
  const [enabledModules, setEnabledModules] = useState<string[]>(['thehive']);
  const [testingModule, setTestingModule] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<string, boolean>>({});

  const toggleModule = (id: string) => {
    if (enabledModules.includes(id)) {
      setEnabledModules(enabledModules.filter(m => m !== id));
    } else {
      setEnabledModules([...enabledModules, id]);
    }
  };

  const testIntegration = async (id: string) => {
    setTestingModule(id);
    try {
      // Simulate API call to test integration
      await new Promise(resolve => setTimeout(resolve, ));
      setTestResults(prev => ({ ...prev, [id]: true }));
      toast.success(${modules.find(m => m.id === id)?.name} connection successful!);
    } catch (error) {
      setTestResults(prev => ({ ...prev, [id]: false }));
      toast.error('Connection test failed. Please check your credentials and try again.');
    } finally {
      setTestingModule(null);
    }
  };

  return (
    <div className="space-y-">
       <div>
        <h className="text-xl font-bold text-white mb-">Integrations</h>
        <p className="text-zinc- text-sm">Connect OpenRisk with the OpenDefender Suite.</p>
      </div>

      <div className="grid gap-">
        {modules.map((mod) => (
            <div key={mod.id} className={p- rounded-xl border transition-all ${
                enabledModules.includes(mod.id) 
                ? 'bg-surface border-primary/ shadow-[__px_-px_rgba(,,,.)]' 
                : 'bg-surface/ border-white/ opacity-'
            }}>
                <div className="flex items-start justify-between">
                    <div className="flex gap- flex-">
                        <div className={p- rounded-lg bg-zinc- border border-white/ ${mod.color}}>
                            <mod.icon size={} />
                        </div>
                        <div className="flex-">
                            <h className="font-bold text-white flex items-center gap-">
                                {mod.name}
                                {enabledModules.includes(mod.id) && (
                                  <span className="text-[px] bg-emerald-/ text-emerald- px- py-. rounded-full border border-emerald-/ flex items-center gap-">
                                    <CheckCircle size={} /> ACTIVE
                                  </span>
                                )}
                                {testResults[mod.id] === true && (
                                  <span className="text-[px] bg-green-/ text-green- px- py-. rounded-full border border-green-/ flex items-center gap-">
                                    <CheckCircle size={} /> VERIFIED
                                  </span>
                                )}
                                {testResults[mod.id] === false && (
                                  <span className="text-[px] bg-red-/ text-red- px- py-. rounded-full border border-red-/ flex items-center gap-">
                                    <AlertCircle size={} /> FAILED
                                  </span>
                                )}
                            </h>
                            <p className="text-sm text-zinc- mt-">{mod.desc}</p>
                        </div>
                    </div>
                    <div className="flex gap- ml-">
                      <Button 
                          variant="ghost"
                          onClick={() => testIntegration(mod.id)}
                          disabled={!enabledModules.includes(mod.id) || testingModule === mod.id}
                          className="text-sm"
                      >
                          {testingModule === mod.id ? (
                            <>
                              <Loader size={} className="mr- animate-spin" />
                              Testing...
                            </>
                          ) : (
                            'Test'
                          )}
                      </Button>
                      <Button 
                          variant={enabledModules.includes(mod.id) ? "secondary" : "primary"}
                          onClick={() => toggleModule(mod.id)}
                          className="h-"
                      >
                          {enabledModules.includes(mod.id) ? 'Configure' : 'Enable'}
                      </Button>
                    </div>
                </div>
                
                {enabledModules.includes(mod.id) && (
                    <div className="mt- pt- border-t border-white/ grid gap- animate-fade-in">
                        <div className="grid grid-cols- md:grid-cols- gap-">
                            <div className="space-y-">
                                <label className="text-xs font-bold text-zinc- uppercase">API URL</label>
                                <input className="w-full bg-zinc- border border-border rounded px- py- text-sm text-white" placeholder={https://${mod.id}.opendefender.local} />
                            </div>
                            <div className="space-y-">
                                <label className="text-xs font-bold text-zinc- uppercase">API Key</label>
                                <input type="password" className="w-full bg-zinc- border border-border rounded px- py- text-sm text-white" placeholder="••••••••••••••••" />
                            </div>
                        </div>
                        <div className="flex gap- pt-">
                          <Button className="text-sm">Save Configuration</Button>
                          <Button variant="ghost" className="text-sm">Reset</Button>
                        </div>
                    </div>
                )}
            </div>
        ))}
      </div>
    </div>
  );
};