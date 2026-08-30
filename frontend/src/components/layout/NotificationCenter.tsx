// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useState } from 'react';
import { Bell, X, Check, AlertCircle, Info, AlertTriangle } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { useNotificationStore } from '../../hooks/useNotificationStore';
import { Button } from '../../shared/ds';

export const NotificationCenter = () => {
  const { notifications, unreadCount, markAsRead, markAllAsRead, removeNotification, clearAll } = useNotificationStore();
  const [isOpen, setIsOpen] = useState(false);

  const getIconAndColor = (type: string) => {
    switch (type) {
      case 'success':
        return { icon: Check, color: 'text-success-text', bg: 'bg-success/10' };
      case 'error':
        return { icon: AlertCircle, color: 'text-danger-text', bg: 'bg-danger/10' };
      case 'warning':
        return { icon: AlertTriangle, color: 'text-warning-text', bg: 'bg-warning/10' };
      case 'info':
      default:
        return { icon: Info, color: 'text-info-text', bg: 'bg-accent-soft' };
    }
  };

  return (
    <div className="relative">
      {/* Notification Bell Button */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="relative text-text-secondary hover:text-text-primary transition-colors p-2 hover:bg-surface-1/5 rounded-full"
      >
        <Bell size={20} />
        {unreadCount > 0 && (
          <span className="absolute top-1.5 right-1.5 w-4 h-4 bg-danger rounded-full animate-pulse border border-background text-text-primary text-xs flex items-center justify-center font-bold">
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        )}
      </button>

      {/* Dropdown Panel */}
      <AnimatePresence>
        {isOpen && (
          <>
            {/* Backdrop */}
            <div
              className="fixed inset-0 z-40"
              onClick={() => setIsOpen(false)}
            />

            {/* Notification Panel */}
            <motion.div
              initial={{ opacity: 0, y: -10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -10 }}
              className="absolute right-0 top-full mt-2 w-96 bg-surface border border-border rounded-lg shadow-xl z-50 max-h-[600px] flex flex-col"
            >
              {/* Header */}
              <div className="border-b border-border-strong/5 p-4 flex items-center justify-between">
                <div>
                  <h3 className="font-bold text-text-primary">Notifications</h3>
                  {unreadCount > 0 && (
                    <p className="text-xs text-text-secondary">{unreadCount} unread</p>
                  )}
                </div>
                <div className="flex gap-2">
                  {notifications.length > 0 && (
                    <>
                      <button
                        onClick={markAllAsRead}
                        className="text-xs px-2 py-1 text-text-secondary hover:text-text-primary transition-colors"
                      >
                        Mark all read
                      </button>
                      <button
                        onClick={clearAll}
                        className="text-xs px-2 py-1 text-text-secondary hover:text-danger-text transition-colors"
                      >
                        Clear all
                      </button>
                    </>
                  )}
                </div>
              </div>

              {/* Notifications List */}
              <div className="overflow-y-auto flex-1">
                {notifications.length === 0 ? (
                  <div className="p-8 text-center text-text-muted">
                    <Bell size={32} className="mx-auto mb-2 opacity-50" />
                    <p>No notifications</p>
                  </div>
                ) : (
                  <div className="divide-y divide-border-subtle">
                    {notifications.map((notification) => {
                      const { icon: Icon, color, bg } = getIconAndColor(notification.type);
                      return (
                        <motion.div
                          key={notification.id}
                          initial={{ opacity: 0, x: 20 }}
                          animate={{ opacity: 1, x: 0 }}
                          exit={{ opacity: 0, x: -20 }}
                          className={`p-4 hover:bg-surface-1/5 transition-colors ${!notification.read ? 'bg-surface-1/2' : ''}`}
                        >
                          <div className="flex gap-3">
                            <div className={`p-2 rounded-full ${bg} flex-shrink-0`}>
                              <Icon size={16} className={color} />
                            </div>
                            <div className="flex-1 min-w-0">
                              <div className="flex items-start justify-between gap-2">
                                <div className="flex-1">
                                  <p className="font-semibold text-text-primary text-sm">
                                    {notification.title}
                                  </p>
                                  <p className="text-xs text-text-secondary mt-1">
                                    {notification.message}
                                  </p>
                                  <p className="text-xs text-text-muted mt-2">
                                    {new Date(notification.timestamp).toLocaleString()}
                                  </p>
                                </div>
                                {!notification.read && (
                                  <div className="w-2 h-2 bg-accent rounded-full flex-shrink-0 mt-1" />
                                )}
                              </div>
                              <div className="flex gap-2 mt-3">
                                {notification.action && (
                                  <button
                                    onClick={() => {
                                      notification.action?.onClick();
                                      markAsRead(notification.id);
                                    }}
                                    className="text-xs text-info-text hover:text-info-text transition-colors"
                                  >
                                    {notification.action.label}
                                  </button>
                                )}
                                <button
                                  onClick={() => {
                                    markAsRead(notification.id);
                                  }}
                                  className="text-xs text-text-muted hover:text-text-secondary transition-colors"
                                >
                                  {notification.read ? 'Mark unread' : 'Mark read'}
                                </button>
                                <button
                                  onClick={() => removeNotification(notification.id)}
                                  className="text-xs text-text-muted hover:text-danger-text transition-colors ml-auto"
                                >
                                  <X size={14} />
                                </button>
                              </div>
                            </div>
                          </div>
                        </motion.div>
                      );
                    })}
                  </div>
                )}
              </div>

              {/* Footer */}
              {notifications.length > 0 && (
                <div className="border-t border-border-strong/5 p-4">
                  <Button variant="ghost" className="w-full text-sm">
                    View all notifications
                  </Button>
                </div>
              )}
            </motion.div>
          </>
        )}
      </AnimatePresence>
    </div>
  );
};
