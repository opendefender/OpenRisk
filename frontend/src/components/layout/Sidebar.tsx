import { useState } from 'react';
import { motion } from 'framer-motion';
import { LayoutDashboard, ShieldAlert, Activity, Map, FileText, Settings, ChevronLeft, ChevronRight, Zap, Server, Sparkles, Users, Clock, Key, BarChart3, Store, Shield, Building2 } from 'lucide-react';
import { cn } from '../ui/Button';
import { useNavigate, useLocation } from 'react-router-dom';

const menuItems = [
  { icon: LayoutDashboard, label: 'Overview', path: '/'},
  { icon: ShieldAlert, label: 'Risks', path: '/risks' },
  { icon: BarChart3, label: 'Analytics', path: '/analytics' },
  { icon: Activity, label: 'Incidents', path: '/incidents' },
  { icon: Map, label: 'Threat Map', path: '/threat-map' },
  { icon: FileText, label: 'Reports', path: '/reports' },
  { icon: Store, label: 'Marketplace', path: '/marketplace' },
  { icon: Settings, label: 'Settings', path: '/settings'},
  { icon: Users, label: 'Users', path: '/users'},
  { icon: Shield, label: 'Roles', path: '/roles'},
  { icon: Building2, label: 'Tenants', path: '/tenants'},
  { icon: Clock, label: 'Audit Logs', path: '/audit-logs'},
  { icon: Key, label: 'API Tokens', path: '/tokens'},
  { icon: Server,  label: 'Assets', path: '/assets' },
  { icon: Sparkles, label: 'Intelligence', path: '/recommendations' },
];

export const Sidebar = () => {
  const [isCollapsed, setIsCollapsed] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  return (
    <motion.div 
      animate={{ width: isCollapsed ? 80 : 260 }}
      className="h-screen bg-surface border-r border-border flex flex-col relative shrink-0 transition-all duration-300 ease-in-out"
    >
      {/* Logo Area */}
      <div className="p-6 flex items-center gap-3 overflow-hidden whitespace-nowrap">
        <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center shrink-0 shadow-glow">
            <Zap size={18} className="text-white" fill="currentColor" />
        </div>
        <motion.span 
          animate={{ opacity: isCollapsed ? 0 : 1 }}
          className="font-bold text-xl tracking-tight bg-gradient-to-r from-white to-zinc-400 bg-clip-text text-transparent"
        >
          OpenRisk
        </motion.span>
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-3 py-4 space-y-1">
        {menuItems.map((item) => {
          const isActive = item.path === location.pathname;
          return (
          <button
            key={item.label}
            onClick={() => item.path && navigate(item.path)}
            className={cn(
              "w-full flex items-center gap-3 px-3 py-2.5 rounded-lg transition-all duration-200 group relative",
              isActive 
                ? "bg-primary/10 text-primary" 
                : "text-zinc-400 hover:bg-white/5 hover:text-zinc-100"
            )}
          >
            <item.icon size={20} className={cn("shrink-0", isActive && "text-primary drop-shadow-[0_0_8px_rgba(59,130,246,0.5)]")} />
            
            {!isCollapsed && (
              <span className="font-medium text-sm">{item.label}</span>
            )}
            
            {/* Active Indicator */}
            {isActive && (
              <div className="absolute left-0 top-1/2 -translate-y-1/2 w-1 h-6 bg-primary rounded-r-full shadow-[0_0_10px_rgba(59,130,246,0.8)]" />
            )}
          </button>
        );
        })}
      </nav>

      {/* Collapse Button */}
      <button 
        onClick={() => setIsCollapsed(!isCollapsed)}
        // onClick={() => setIsCollapsed(!isCollapsed)}
        className="absolute -right-3 top-10 w-6 h-6 bg-zinc-900 border border-border rounded-full flex items-center justify-center text-zinc-400 hover:text-white hover:border-primary transition-colors z-20 shadow-lg"
      >
        {isCollapsed ? <ChevronRight size={14} /> : <ChevronLeft size={14} />}
      </button>

      {/* User Profile (Bottom) */}
      <div className="p-4 border-t border-border">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-full bg-gradient-to-r from-emerald-500 to-teal-500 flex items-center justify-center text-xs font-bold text-white shrink-0">
            JD
          </div>
          {!isCollapsed && (
            <div className="overflow-hidden">
              <p className="text-sm font-medium text-white truncate">John Doe</p>
              <p className="text-xs text-zinc-500 truncate">CISO Admin</p>
            </div>
          )}
        </div>
      </div>
    </motion.div>
  );
};