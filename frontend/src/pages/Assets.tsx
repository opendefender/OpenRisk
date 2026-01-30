import { useEffect, useState } from 'react';
import { Server, Database, Laptop, Plus, HardDrive, Edit, Trash } from 'lucide-react';
import { useAssetStore } from '../hooks/useAssetStore';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { toast } from 'sonner';
import { motion } from 'framer-motion';
import { ViewToggle } from '../components/ViewToggle';

// Icons map
const TypeIcon = ({ type }: { type: string }) => {
    switch (type.toLowerCase()) {
        case 'server': return <Server size={} className="text-blue-" />;
        case 'database': return <Database size={} className="text-emerald-" />;
        case 'laptop': return <Laptop size={} className="text-zinc-" />;
        default: return <HardDrive size={} className="text-purple-" />;
    }
};

const CriticalityBadge = ({ level }: { level: string }) => {
    const colors = {
        CRITICAL: "bg-red-/ text-red- border-red-/",
        HIGH: "bg-orange-/ text-orange- border-orange-/",
        MEDIUM: "bg-yellow-/ text-yellow- border-yellow-/",
        LOW: "bg-blue-/ text-blue- border-blue-/",
    }[level] || "bg-zinc-/";

    return <span className={px- py-. rounded text-[px] font-bold border ${colors}}>{level}</span>;
};

export const Assets = () => {
    const { assets, fetchAssets, createAsset } = useAssetStore();
    const [isCreating, setIsCreating] = useState(false);
    const [view, setView] = useState<'table' | 'card'>(() => {
        const saved = localStorage.getItem('assetView');
        return (saved as 'table' | 'card') || 'table';
    });
    
    // Quick Form State
    const [newName, setNewName] = useState('');
    const [newType, setNewType] = useState('Server');

    useEffect(() => { fetchAssets(); }, []);
    useEffect(() => { localStorage.setItem('assetView', view); }, [view]);

    const handleCreate = async (e: React.FormEvent) => {
        e.preventDefault();
        try {
            await createAsset({ name: newName, type: newType, criticality: 'MEDIUM', owner: 'IT Dept' });
            toast.success("Asset added to inventory");
            setIsCreating(false);
            setNewName('');
        } catch(e) { toast.error("Failed to create asset"); }
    };

    return (
        <div className="p- space-y-">
            <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-">
                <div>
                    <h className="text-xl font-bold text-white">Assets Inventory</h>
                    <p className="text-zinc- text-sm">Manage infrastructure and linked risks.</p>
                </div>
                <div className="flex items-center gap-">
                    <ViewToggle view={view} onViewChange={setView} />
                    <Button onClick={() => setIsCreating(!isCreating)}>
                        <Plus size={} className="mr-" /> Add Asset
                    </Button>
                </div>
            </div>

            {/ Quick Create Form (Inline) /}
            {isCreating && (
                <motion.form 
                    initial={{ opacity: , height:  }} animate={{ opacity: , height: 'auto' }}
                    onSubmit={handleCreate} 
                    className="bg-surface border border-border p- rounded-xl flex gap- items-end"
                >
                    <div className="flex-"><Input label="Asset Name" placeholder="Ex: Production-DB-" value={newName} onChange={e => setNewName(e.target.value)} autoFocus /></div>
                    <div className="w-">
                         <label className="text-xs font-medium text-zinc- uppercase tracking-wider mb-. block">Type</label>
                         <select 
                            className="w-full h- bg-zinc- border border-border rounded-lg px- text-sm text-white focus:ring- focus:ring-primary/ outline-none"
                            value={newType} onChange={e => setNewType(e.target.value)}
                        >
                            <option>Server</option><option>Database</option><option>Laptop</option><option>SaaS</option>
                         </select>
                    </div>
                    <Button type="submit">Save</Button>
                </motion.form>
            )}

            {/ Data Table / Cards /}
            {view === 'table' && (
                <div className="bg-surface border border-border rounded-xl overflow-hidden shadow-sm">
                    <table className="w-full text-left text-sm">
                        <thead className="bg-white/ text-zinc- font-medium uppercase text-xs">
                            <tr>
                                <th className="px- py-">Name</th>
                                <th className="px- py-">Type</th>
                                <th className="px- py-">Criticality</th>
                                <th className="px- py-">Active Risks</th>
                                <th className="px- py- text-right">Source</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-white/">
                            {assets.map((asset) => (
                                <tr key={asset.id} className="hover:bg-white/ transition-colors group cursor-pointer">
                                    <td className="px- py- font-medium text-white">{asset.name}</td>
                                    <td className="px- py- text-zinc- flex items-center gap-">
                                        <TypeIcon type={asset.type} /> {asset.type}
                                    </td>
                                    <td className="px- py-"><CriticalityBadge level={asset.criticality} /></td>
                                    <td className="px- py-">
                                        {asset.risks && asset.risks.length >  ? (
                                            <span className="text-red- font-bold flex items-center gap-">
                                                {asset.risks.length} <span className="w- h- rounded-full bg-red- animate-pulse"/>
                                            </span>
                                        ) : <span className="text-zinc-">-</span>}
                                    </td>
                                    <td className="px- py- text-right text-xs text-zinc- font-mono">
                                        {asset.source}
                                    </td>
                                </tr>
                            ))}
                            {assets.length ===  && !isCreating && (
                                <tr><td colSpan={} className="px- py- text-center text-zinc-">No assets found. Add one or sync OpenAsset.</td></tr>
                            )}
                        </tbody>
                    </table>
                </div>
            )}

            {view === 'card' && (
                <div className="grid grid-cols- md:grid-cols- lg:grid-cols- gap-">
                    {assets.map((asset) => (
                        <motion.div
                            key={asset.id}
                            initial={{ opacity: , y:  }}
                            animate={{ opacity: , y:  }}
                            whileHover={{ y: - }}
                            className="bg-surface border border-border rounded-lg p- hover:border-primary/ transition-all cursor-pointer group"
                        >
                            <div className="flex items-start justify-between mb-">
                                <div className="flex items-center gap-">
                                    <div className="p- rounded-lg bg-primary/">
                                        <TypeIcon type={asset.type} />
                                    </div>
                                    <div>
                                        <h className="font-semibold text-white group-hover:text-primary transition-colors">
                                            {asset.name}
                                        </h>
                                        <p className="text-xs text-zinc-">{asset.type}</p>
                                    </div>
                                </div>
                            </div>

                            <div className="space-y- mb- border-t border-border pt-">
                                <div className="flex items-center justify-between">
                                    <span className="text-xs text-zinc-">Criticality</span>
                                    <CriticalityBadge level={asset.criticality} />
                                </div>
                                <div className="flex items-center justify-between">
                                    <span className="text-xs text-zinc-">Active Risks</span>
                                    {asset.risks && asset.risks.length >  ? (
                                        <span className="text-red- font-bold flex items-center gap-">
                                            {asset.risks.length} <span className="w- h- rounded-full bg-red- animate-pulse"/>
                                        </span>
                                    ) : <span className="text-zinc-">-</span>}
                                </div>
                                <div className="flex items-center justify-between">
                                    <span className="text-xs text-zinc-">Source</span>
                                    <span className="text-xs font-mono text-zinc-">{asset.source}</span>
                                </div>
                            </div>

                            <div className="flex gap- pt- border-t border-border">
                                <Button variant="ghost" className="flex- text-xs flex items-center justify-center gap-">
                                    <Edit size={} /> Edit
                                </Button>
                                <Button variant="ghost" className="flex- text-xs flex items-center justify-center gap-">
                                    <Trash size={} /> Delete
                                </Button>
                            </div>
                        </motion.div>
                    ))}
                    {assets.length ===  && !isCreating && (
                        <div className="col-span-full text-center py-">
                            <p className="text-zinc-">No assets found. Add one or sync OpenAsset.</p>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
};