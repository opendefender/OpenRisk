// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { LayoutDashboard, ShieldAlert, ShieldCheck, Activity, Map, FileText, Settings, ChevronLeft, ChevronRight, Zap, Server, Sparkles, Users, Clock, Key, BarChart3, Store, Shield, Building2, PieChart, AlertCircle, Sliders, Zap as ZapIcon, Wrench, X } from 'lucide-react';
import { cn } from '../ui/Button';
import { useNavigate, useLocation } from 'react-router-dom';

const menuItems = [
  { icon: LayoutDashboard, label: 'Overview', path: '/'},
  { icon: ShieldAlert, label: 'Risks', path: '/risks' },
  { icon: AlertCircle, label: 'Risk Management', path: '/risk-management' },
  { icon: Wrench, label: 'Mitigations', path: '/mitigations' },
  { icon: ShieldCheck, label: 'Compliance', path: '/compliance' },
  { icon: BarChart3, label: 'Analytics', path: '/analytics' },
  { icon: Activity, label: 'Incidents', path: '/incidents' },
  { icon: Map, label: 'Threat Map', path: '/threat-map' },
  { icon: FileText, label: 'Reports', path: '/reports' },
  { icon: Store, label: 'Marketplace', path: '/marketplace' },
  { icon: Sliders, label: 'Custom Fields', path: '/custom-fields' },
  { icon: ZapIcon, label: 'Bulk Ops', path: '/bulk-operations' },
  { icon: Settings, label: 'Settings', path: '/settings'},
  { icon: Users, label: 'Users', path: '/users'},
  { icon: Shield, label: 'Roles', path: '/roles'},
  { icon: Building2, label: 'Tenants', path: '/tenants'},
  { icon: PieChart, label: 'Permissions', path: '/analytics/permissions'},
  { icon: Clock, label: 'Audit Logs', path: '/audit-logs'},
  { icon: Key, label: 'API Tokens', path: '/tokens'},
  { icon: Server,  label: 'Assets', path: '/assets' },
  { icon: Sparkles, label: 'Intelligence', path: '/recommendations' },
];

interface SidebarProps {
  /** Whether the off-canvas drawer is open on mobile (< lg). Ignored on desktop, where the
   *  sidebar is always in the layout flow. */
  mobileOpen?: boolean;
  /** Called when the mobile drawer should close (backdrop click, close button, navigation). */
  onMobileClose?: () => void;
}

export const Sidebar = ({ mobileOpen = false, onMobileClose }: SidebarProps) => {
  const [isCollapsed, setIsCollapsed] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  // Close the mobile drawer whenever the route changes so a tap-through never leaves it open.
  useEffect(() => {
    onMobileClose?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname]);

  return (
    <>
      {/* Backdrop — mobile only, behind the drawer. */}
      {mobileOpen && (
        <div
          onClick={onMobileClose}
          className="lg:hidden fixed inset-0 bg-black/60 backdrop-blur-sm z-40"
          aria-hidden="true"
        />
      )}

      <aside
        className={cn(
          'h-screen bg-surface border-r border-border flex flex-col z-50 transition-all duration-300 ease-in-out',
          // Desktop: part of the flex layout, collapsible width.
          'lg:static lg:shrink-0 lg:translate-x-0',
          isCollapsed ? 'lg:w-20' : 'lg:w-[260px]',
          // Mobile: fixed off-canvas drawer, slides in from the left.
          'fixed inset-y-0 left-0 w-[260px] max-w-[82vw]',
          mobileOpen ? 'translate-x-0 shadow-2xl' : '-translate-x-full lg:translate-x-0'
        )}
      >
        {/* Logo Area */}
        <div className="p-6 flex items-center gap-3 overflow-hidden whitespace-nowrap">
          <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center shrink-0 shadow-glow">
            <Zap size={18} className="text-white" fill="currentColor" />
          </div>
          <motion.span
            animate={{ opacity: isCollapsed ? 0 : 1 }}
            className="font-bold text-xl tracking-tight bg-gradient-to-r from-white to-zinc-400 bg-clip-text text-transparent lg:inline"
          >
            OpenRisk
          </motion.span>

          {/* Close button — mobile only. */}
          <button
            onClick={onMobileClose}
            className="lg:hidden ml-auto text-zinc-400 hover:text-white"
            aria-label="Close menu"
          >
            <X size={20} />
          </button>
        </div>

        {/* Navigation */}
        <nav className="flex-1 min-h-0 overflow-y-auto px-3 py-4 space-y-1 scrollbar-thin">
          {menuItems.map((item) => {
            const isActive = item.path === location.pathname;
            return (
              <button
                key={item.label}
                onClick={() => item.path && navigate(item.path)}
                className={cn(
                  'w-full flex items-center gap-3 px-3 py-2.5 rounded-lg transition-all duration-200 group relative',
                  isActive
                    ? 'bg-primary/10 text-primary'
                    : 'text-zinc-400 hover:bg-white/5 hover:text-zinc-100'
                )}
              >
                <item.icon size={20} className={cn('shrink-0', isActive && 'text-primary drop-shadow-[0_0_8px_rgba(59,130,246,0.5)]')} />

                {/* Label hides only when collapsed on desktop; always shown in the mobile drawer. */}
                <span className={cn('font-medium text-sm', isCollapsed && 'lg:hidden')}>{item.label}</span>

                {/* Active Indicator */}
                {isActive && (
                  <div className="absolute left-0 top-1/2 -translate-y-1/2 w-1 h-6 bg-primary rounded-r-full shadow-[0_0_10px_rgba(59,130,246,0.8)]" />
                )}
              </button>
            );
          })}
        </nav>

        {/* Collapse Button — desktop only. */}
        <button
          onClick={() => setIsCollapsed(!isCollapsed)}
          className="hidden lg:flex absolute -right-3 top-10 w-6 h-6 bg-zinc-900 border border-border rounded-full items-center justify-center text-zinc-400 hover:text-white hover:border-primary transition-colors z-20 shadow-lg"
          aria-label={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          {isCollapsed ? <ChevronRight size={14} /> : <ChevronLeft size={14} />}
        </button>

        {/* User Profile (Bottom) */}
        <div className="p-4 border-t border-border">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-full bg-gradient-to-r from-emerald-500 to-teal-500 flex items-center justify-center text-xs font-bold text-white shrink-0">
              JD
            </div>
            <div className={cn('overflow-hidden', isCollapsed && 'lg:hidden')}>
              <p className="text-sm font-medium text-white truncate">John Doe</p>
              <p className="text-xs text-zinc-500 truncate">CISO Admin</p>
            </div>
          </div>
        </div>
      </aside>
    </>
  );
};
