import { useState } from 'react';
import { motion } from 'framer-motion';
import { LayoutDashboard, ShieldAlert, Activity, Map, FileText, Settings, ChevronLeft, ChevronRight, Zap, Server, Sparkles, Users, Clock, Key, BarChart, Store, Shield, Building, PieChart } from 'lucide-react';
import { cn } from '../ui/Button';
import { useNavigate, useLocation } from 'react-router-dom';

const menuItems = [
  { icon: LayoutDashboard, label: 'Overview', path: '/'},
  { icon: ShieldAlert, label: 'Risks', path: '/risks' },
  { icon: BarChart, label: 'Analytics', path: '/analytics' },
  { icon: Activity, label: 'Incidents', path: '/incidents' },
  { icon: Map, label: 'Threat Map', path: '/threat-map' },
  { icon: FileText, label: 'Reports', path: '/reports' },
  { icon: Store, label: 'Marketplace', path: '/marketplace' },
  { icon: Settings, label: 'Settings', path: '/settings'},
  { icon: Users, label: 'Users', path: '/users'},
  { icon: Shield, label: 'Roles', path: '/roles'},
  { icon: Building, label: 'Tenants', path: '/tenants'},
  { icon: PieChart, label: 'Permissions', path: '/analytics/permissions'},
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
      animate={{ width: isCollapsed ?  :  }}
      className="h-screen bg-surface border-r border-border flex flex-col relative shrink- transition-all duration- ease-in-out"
    >
      {/ Logo Area /}
      <div className="p- flex items-center gap- overflow-hidden whitespace-nowrap">
        <div className="w- h- rounded-lg bg-gradient-to-br from-blue- to-purple- flex items-center justify-center shrink- shadow-glow">
            <Zap size={} className="text-white" fill="currentColor" />
        </div>
        <motion.span 
          animate={{ opacity: isCollapsed ?  :  }}
          className="font-bold text-xl tracking-tight bg-gradient-to-r from-white to-zinc- bg-clip-text text-transparent"
        >
          OpenRisk
        </motion.span>
      </div>

      {/ Navigation /}
      <nav className="flex- px- py- space-y-">
        {menuItems.map((item) => {
          const isActive = item.path === location.pathname;
          return (
          <button
            key={item.label}
            onClick={() => item.path && navigate(item.path)}
            className={cn(
              "w-full flex items-center gap- px- py-. rounded-lg transition-all duration- group relative",
              isActive 
                ? "bg-primary/ text-primary" 
                : "text-zinc- hover:bg-white/ hover:text-zinc-"
            )}
          >
            <item.icon size={} className={cn("shrink-", isActive && "text-primary drop-shadow-[__px_rgba(,,,.)]")} />
            
            {!isCollapsed && (
              <span className="font-medium text-sm">{item.label}</span>
            )}
            
            {/ Active Indicator /}
            {isActive && (
              <div className="absolute left- top-/ -translate-y-/ w- h- bg-primary rounded-r-full shadow-[__px_rgba(,,,.)]" />
            )}
          </button>
        );
        })}
      </nav>

      {/ Collapse Button /}
      <button 
        onClick={() => setIsCollapsed(!isCollapsed)}
        // onClick={() => setIsCollapsed(!isCollapsed)}
        className="absolute -right- top- w- h- bg-zinc- border border-border rounded-full flex items-center justify-center text-zinc- hover:text-white hover:border-primary transition-colors z- shadow-lg"
      >
        {isCollapsed ? <ChevronRight size={} /> : <ChevronLeft size={} />}
      </button>

      {/ User Profile (Bottom) /}
      <div className="p- border-t border-border">
        <div className="flex items-center gap-">
          <div className="w- h- rounded-full bg-gradient-to-r from-emerald- to-teal- flex items-center justify-center text-xs font-bold text-white shrink-">
            JD
          </div>
          {!isCollapsed && (
            <div className="overflow-hidden">
              <p className="text-sm font-medium text-white truncate">John Doe</p>
              <p className="text-xs text-zinc- truncate">CISO Admin</p>
            </div>
          )}
        </div>
      </div>
    </motion.div>
  );
};