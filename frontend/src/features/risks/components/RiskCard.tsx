import { Database, ShieldAlert, Box } from 'lucide-react';


const SourceIcon = ({ source }: { source: string }) => {
    switch (source) {
        case 'THEHIVE': return <ShieldAlert size={12} className="text-yellow-500" />;
        case 'OPENRMF': return <Database size={12} className="text-blue-500" />;
        case 'OPENCTI': return <Box size={12} className="text-purple-500" />;
        default: return <User size={12} className="text-zinc-500" />; // Manual
    }
};


<div className="flex items-center gap-1 text-[10px] font-bold border border-white/10 px-2 py-1 rounded bg-zinc-900">
    <SourceIcon source={risk.source} />
    <span className="text-zinc-400">{risk.source}</span>
</div>