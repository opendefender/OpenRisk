import { useEffect, useState } from 'react';
import { Server, Database, Laptop, Plus, HardDrive, Edit2, Trash2 } from 'lucide-react';
import { useAssetStore } from '../hooks/useAssetStore';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { toast } from 'sonner';
import { motion } from 'framer-motion';
import { ViewToggle } from '../components/ViewToggle';

// Icons map
const TypeIcon = ({ type }: { type: string }) => {
    switch (type.toLowerCase()) {
        case 'server': return <Server size={16} className="text-blue-400" />;
        case 'database': return <Database size={16} className="text-emerald-400" />;
        case 'laptop': return <Laptop size={16} className="text-zinc-400" />;
        default: return <HardDrive size={16} className="text-purple-400" />;
    }
};

const CriticalityBadge = ({ level }: { level: string }) => {
    const colors = {
        CRITICAL: "bg-red-500/10 text-red-500 border-red-500/20",
        HIGH: "bg-orange-500/10 text-orange-500 border-orange-500/20",
        MEDIUM: "bg-yellow-500/10 text-yellow-500 border-yellow-500/20",
        LOW: "bg-blue-500/10 text-blue-500 border-blue-500/20",
    }[level] || "bg-zinc-500/10";

    return <span className={`px-2 py-0.5 rounded text-[10px] font-bold border ${colors}`}>{level}</span>;
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
        <div className="p-8 space-y-6">
            <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
                <div>
                    <h1 className="text-2xl font-bold text-white">Assets Inventory</h1>
                    <p className="text-zinc-400 text-sm">Manage infrastructure and linked risks.</p>
                </div>
                <div className="flex items-center gap-4">
                    <ViewToggle view={view} onViewChange={setView} />
                    <Button onClick={() => setIsCreating(!isCreating)}>
                        <Plus size={16} className="mr-2" /> Add Asset
                    </Button>
                </div>
            </div>

            {/* Quick Create Form (Inline) */}
            {isCreating && (
                <motion.form 
                    initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: 'auto' }}
                    onSubmit={handleCreate} 
                    className="bg-surface border border-border p-4 rounded-xl flex gap-4 items-end"
                >
                    <div className="flex-1"><Input label="Asset Name" placeholder="Ex: Production-DB-01" value={newName} onChange={e => setNewName(e.target.value)} autoFocus /></div>
                    <div className="w-48">
                         <label className="text-xs font-medium text-zinc-400 uppercase tracking-wider mb-1.5 block">Type</label>
                         <select 
                            className="w-full h-10 bg-zinc-900 border border-border rounded-lg px-3 text-sm text-white focus:ring-2 focus:ring-primary/50 outline-none"
                            value={newType} onChange={e => setNewType(e.target.value)}
                        >
                            <option>Server</option><option>Database</option><option>Laptop</option><option>SaaS</option>
                         </select>
                    </div>
                    <Button type="submit">Save</Button>
                </motion.form>
            )}

            {/* Data Table / Cards */}
            {view === 'table' && (
                <div className="bg-surface border border-border rounded-xl overflow-hidden shadow-sm">
                    <table className="w-full text-left text-sm">
                        <thead className="bg-white/5 text-zinc-400 font-medium uppercase text-xs">
                            <tr>
                                <th className="px-6 py-4">Name</th>
                                <th className="px-6 py-4">Type</th>
                                <th className="px-6 py-4">Criticality</th>
                                <th className="px-6 py-4">Active Risks</th>
                                <th className="px-6 py-4 text-right">Source</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-white/5">
                            {assets.map((asset) => (
                                <tr key={asset.id} className="hover:bg-white/5 transition-colors group cursor-pointer">
                                    <td className="px-6 py-4 font-medium text-white">{asset.name}</td>
                                    <td className="px-6 py-4 text-zinc-400 flex items-center gap-2">
                                        <TypeIcon type={asset.type} /> {asset.type}
                                    </td>
                                    <td className="px-6 py-4"><CriticalityBadge level={asset.criticality} /></td>
                                    <td className="px-6 py-4">
                                        {asset.risks && asset.risks.length > 0 ? (
                                            <span className="text-red-400 font-bold flex items-center gap-1">
                                                {asset.risks.length} <span className="w-2 h-2 rounded-full bg-red-500 animate-pulse"/>
                                            </span>
                                        ) : <span className="text-zinc-600">-</span>}
                                    </td>
                                    <td className="px-6 py-4 text-right text-xs text-zinc-500 font-mono">
                                        {asset.source}
                                    </td>
                                </tr>
                            ))}
                            {assets.length === 0 && !isCreating && (
                                <tr><td colSpan={5} className="px-6 py-8 text-center text-zinc-500">No assets found. Add one or sync OpenAsset.</td></tr>
                            )}
                        </tbody>
                    </table>
                </div>
            )}

            {view === 'card' && (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                    {assets.map((asset) => (
                        <motion.div
                            key={asset.id}
                            initial={{ opacity: 0, y: 20 }}
                            animate={{ opacity: 1, y: 0 }}
                            whileHover={{ y: -4 }}
                            className="bg-surface border border-border rounded-lg p-6 hover:border-primary/50 transition-all cursor-pointer group"
                        >
                            <div className="flex items-start justify-between mb-4">
                                <div className="flex items-center gap-3">
                                    <div className="p-2 rounded-lg bg-primary/10">
                                        <TypeIcon type={asset.type} />
                                    </div>
                                    <div>
                                        <h3 className="font-semibold text-white group-hover:text-primary transition-colors">
                                            {asset.name}
                                        </h3>
                                        <p className="text-xs text-zinc-500">{asset.type}</p>
                                    </div>
                                </div>
                            </div>

                            <div className="space-y-3 mb-4 border-t border-border pt-4">
                                <div className="flex items-center justify-between">
                                    <span className="text-xs text-zinc-400">Criticality</span>
                                    <CriticalityBadge level={asset.criticality} />
                                </div>
                                <div className="flex items-center justify-between">
                                    <span className="text-xs text-zinc-400">Active Risks</span>
                                    {asset.risks && asset.risks.length > 0 ? (
                                        <span className="text-red-400 font-bold flex items-center gap-1">
                                            {asset.risks.length} <span className="w-2 h-2 rounded-full bg-red-500 animate-pulse"/>
                                        </span>
                                    ) : <span className="text-zinc-600">-</span>}
                                </div>
                                <div className="flex items-center justify-between">
                                    <span className="text-xs text-zinc-400">Source</span>
                                    <span className="text-xs font-mono text-zinc-500">{asset.source}</span>
                                </div>
                            </div>

                            <div className="flex gap-2 pt-4 border-t border-border">
                                <Button variant="ghost" className="flex-1 text-xs flex items-center justify-center gap-1">
                                    <Edit2 size={14} /> Edit
                                </Button>
                                <Button variant="ghost" className="flex-1 text-xs flex items-center justify-center gap-1">
                                    <Trash2 size={14} /> Delete
                                </Button>
                            </div>
                        </motion.div>
                    ))}
                    {assets.length === 0 && !isCreating && (
                        <div className="col-span-full text-center py-12">
                            <p className="text-zinc-500">No assets found. Add one or sync OpenAsset.</p>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
};