import { useState } from 'react';
import { ShieldAlert, Database, Box, CheckCircle, AlertCircle, Loader2 } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { toast } from 'sonner';

const modules = [
  { id: 'thehive', name: 'TheHive', icon: ShieldAlert, color: 'text-yellow-500', desc: 'Incident Response Platform & Case Management.' },
  { id: 'opencti', name: 'OpenCTI', icon: Box, color: 'text-blue-500', desc: 'Cyber Threat Intelligence Knowledge Base.' },
  { id: 'openrmf', name: 'OpenRMF', icon: Database, color: 'text-emerald-500', desc: 'Risk Management Framework & Compliance.' },
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
      await new Promise(resolve => setTimeout(resolve, 2000));
      setTestResults(prev => ({ ...prev, [id]: true }));
      toast.success(`${modules.find(m => m.id === id)?.name} connection successful!`);
    } catch (error) {
      setTestResults(prev => ({ ...prev, [id]: false }));
      toast.error('Connection test failed. Please check your credentials.');
    } finally {
      setTestingModule(null);
    }
  };

  return (
    <div className="space-y-8">
       <div>
        <h3 className="text-2xl font-bold text-white mb-1">Integrations</h3>
        <p className="text-zinc-400 text-sm">Connect OpenRisk with the OpenDefender Suite.</p>
      </div>

      <div className="grid gap-4">
        {modules.map((mod) => (
            <div key={mod.id} className={`p-6 rounded-xl border transition-all ${
                enabledModules.includes(mod.id) 
                ? 'bg-surface border-primary/30 shadow-[0_0_20px_-10px_rgba(59,130,246,0.2)]' 
                : 'bg-surface/30 border-white/5 opacity-70'
            }`}>
                <div className="flex items-start justify-between">
                    <div className="flex gap-4 flex-1">
                        <div className={`p-3 rounded-lg bg-zinc-900 border border-white/10 ${mod.color}`}>
                            <mod.icon size={24} />
                        </div>
                        <div className="flex-1">
                            <h4 className="font-bold text-white flex items-center gap-2">
                                {mod.name}
                                {enabledModules.includes(mod.id) && (
                                  <span className="text-[10px] bg-emerald-500/10 text-emerald-500 px-2 py-0.5 rounded-full border border-emerald-500/20 flex items-center gap-1">
                                    <CheckCircle size={10} /> ACTIVE
                                  </span>
                                )}
                                {testResults[mod.id] === true && (
                                  <span className="text-[10px] bg-green-500/10 text-green-500 px-2 py-0.5 rounded-full border border-green-500/20 flex items-center gap-1">
                                    <CheckCircle size={10} /> VERIFIED
                                  </span>
                                )}
                                {testResults[mod.id] === false && (
                                  <span className="text-[10px] bg-red-500/10 text-red-500 px-2 py-0.5 rounded-full border border-red-500/20 flex items-center gap-1">
                                    <AlertCircle size={10} /> FAILED
                                  </span>
                                )}
                            </h4>
                            <p className="text-sm text-zinc-400 mt-1">{mod.desc}</p>
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
                          variant={enabledModules.includes(mod.id) ? "secondary" : "primary"}
                          onClick={() => toggleModule(mod.id)}
                          className="h-10"
                      >
                          {enabledModules.includes(mod.id) ? 'Configure' : 'Enable'}
                      </Button>
                    </div>
                </div>
                
                {enabledModules.includes(mod.id) && (
                    <div className="mt-6 pt-6 border-t border-white/5 grid gap-4 animate-fade-in">
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div className="space-y-1">
                                <label className="text-xs font-bold text-zinc-500 uppercase">API URL</label>
                                <input className="w-full bg-zinc-900 border border-border rounded px-3 py-2 text-sm text-white" placeholder={`https://${mod.id}.opendefender.local`} />
                            </div>
                            <div className="space-y-1">
                                <label className="text-xs font-bold text-zinc-500 uppercase">API Key</label>
                                <input type="password" className="w-full bg-zinc-900 border border-border rounded px-3 py-2 text-sm text-white" placeholder="••••••••••••••••" />
                            </div>
                        </div>
                        <div className="flex gap-2 pt-2">
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