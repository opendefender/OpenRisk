// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Bell,
  X,
  Check,
  AlertCircle,
  Info,
  AlertTriangle,
  Zap,
  Trophy,
  Target,
  Gift,
} from 'lucide-react';
import { useNotificationStore } from '../../hooks/useNotificationStore';

interface NotificationPreference {
  achievements: boolean;
  mitigations: boolean;
  risks: boolean;
  incidents: boolean;
  system: boolean;
  sound: boolean;
  desktop: boolean;
}

export const EnhancedNotificationCenter = () => {
  const {
    notifications,
    unreadCount,
    markAsRead,
    markAllAsRead,
    removeNotification,
    clearAll,
  } = useNotificationStore();

  const [isOpen, setIsOpen] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [preferences, setPreferences] = useState<NotificationPreference>({
    achievements: true,
    mitigations: true,
    risks: true,
    incidents: true,
    system: true,
    sound: true,
    desktop: false,
  });

  // Get icon and colors based on notification type
  const getNotificationConfig = (type: string) => {
    const configs: Record<
      string,
      {
        icon: any;
        color: string;
        bg: string;
        textColor: string;
        gradient: string;
      }
    > = {
      success: {
        icon: Check,
        color: 'text-success-text',
        bg: 'bg-success/10',
        textColor: 'text-success-text',
        gradient: 'from-emerald-600 to-emerald-700',
      },
      error: {
        icon: AlertCircle,
        color: 'text-danger-text',
        bg: 'bg-danger/10',
        textColor: 'text-danger-text',
        gradient: 'from-red-600 to-red-700',
      },
      warning: {
        icon: AlertTriangle,
        color: 'text-warning-text',
        bg: 'bg-warning/10',
        textColor: 'text-warning-text',
        gradient: 'from-yellow-600 to-yellow-700',
      },
      achievement: {
        icon: Trophy,
        color: 'text-purple-400',
        bg: 'bg-purple-500/10',
        textColor: 'text-purple-300',
        gradient: 'from-purple-600 to-purple-700',
      },
      milestone: {
        icon: Zap,
        color: 'text-info-text',
        bg: 'bg-accent-soft',
        textColor: 'text-info-text',
        gradient: 'from-blue-600 to-blue-700',
      },
      info: {
        icon: Info,
        color: 'text-info-text',
        bg: 'bg-accent-soft',
        textColor: 'text-info-text',
        gradient: 'from-blue-600 to-blue-700',
      },
      default: {
        icon: Info,
        color: 'text-fg-secondary',
        bg: 'bg-surface-3/10',
        textColor: 'text-fg-secondary',
        gradient: 'from-gray-600 to-gray-700',
      },
    };

    return configs[type] || configs['default'];
  };

  // Play notification sound
  const playNotificationSound = () => {
    if (preferences.sound) {
      // Using Web Audio API for a simple notification tone
      const audioContext = new (window.AudioContext || (window as any).webkitAudioContext)();
      const oscillator = audioContext.createOscillator();
      const gain = audioContext.createGain();

      oscillator.connect(gain);
      gain.connect(audioContext.destination);

      oscillator.frequency.value = 800;
      oscillator.type = 'sine';

      gain.gain.setValueAtTime(0.3, audioContext.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.01, audioContext.currentTime + 0.1);

      oscillator.start(audioContext.currentTime);
      oscillator.stop(audioContext.currentTime + 0.1);
    }
  };

  // Show desktop notification
  const showDesktopNotification = (notification: any) => {
    if (
      preferences.desktop &&
      'Notification' in window &&
      Notification.permission === 'granted'
    ) {
      new Notification(notification.title, {
        body: notification.message,
        icon: '/icon-notification.png',
        badge: '/badge-notification.png',
      });
    }
  };

  const handleNotificationClick = (notification: any) => {
    if (!notification.read) {
      markAsRead(notification.id);
      playNotificationSound();
    }
    notification.action?.onClick();
  };

  const requestNotificationPermission = async () => {
    if ('Notification' in window && Notification.permission === 'default') {
      const permission = await Notification.requestPermission();
      setPreferences((prev) => ({
        ...prev,
        desktop: permission === 'granted',
      }));
    }
  };

  return (
    <div className="relative">
      {/* Notification Bell Button */}
      <motion.button
        whileHover={{ scale: 1.1 }}
        whileTap={{ scale: 0.95 }}
        onClick={() => {
          setIsOpen(!isOpen);
          setShowSettings(false);
        }}
        className="relative text-fg-secondary hover:text-fg-primary transition-colors p-2 hover:bg-surface-1/5 rounded-full group"
      >
        <Bell size={20} />

        {/* Pulsing indicator for unread */}
        {unreadCount > 0 && (
          <>
            <span className="absolute top-1.5 right-1.5 w-4 h-4 bg-danger rounded-full border border-background text-fg-primary text-xs flex items-center justify-center font-bold">
              {unreadCount > 9 ? '9+' : unreadCount}
            </span>
            <span className="absolute top-1.5 right-1.5 w-4 h-4 bg-danger rounded-full animate-pulse" />
          </>
        )}

        {/* Hover tooltip */}
        <div className="absolute bottom-full right-0 mb-2 px-3 py-1 bg-surface-1 text-fg-primary text-xs rounded-md opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none whitespace-nowrap">
          {unreadCount} unread
        </div>
      </motion.button>

      {/* Dropdown Panel */}
      <AnimatePresence>
        {isOpen && (
          <>
            {/* Backdrop */}
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="fixed inset-0 z-40"
              onClick={() => setIsOpen(false)}
            />

            {/* Panel */}
            <motion.div
              initial={{ opacity: 0, y: -10, scale: 0.95 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: -10, scale: 0.95 }}
              transition={{ type: 'spring', damping: 20, stiffness: 300 }}
              className="absolute right-0 mt-2 w-96 max-h-[600px] bg-surface-1 border border-border-subtle rounded-2xl shadow-2xl flex flex-col z-50 overflow-hidden"
            >
              {/* Header */}
              <div className="bg-linear-to-r from-zinc-800 to-zinc-900 border-b border-border-default p-4 flex items-center justify-between shrink-0">
                <div className="flex items-center gap-2">
                  <Bell size={18} className="text-info-text" />
                  <h3 className="font-semibold text-fg-primary text-lg">Notifications</h3>
                </div>
                <div className="flex items-center gap-2">
                  {notifications.length > 0 && (
                    <>
                      <motion.button
                        whileHover={{ scale: 1.05 }}
                        onClick={markAllAsRead}
                        className="text-xs px-2 py-1 text-info-text hover:text-info-text transition-colors"
                        title="Mark all as read"
                      >
                        Mark all read
                      </motion.button>
                      <motion.button
                        whileHover={{ scale: 1.05 }}
                        onClick={() => setShowSettings(!showSettings)}
                        className="text-xs px-2 py-1 text-fg-secondary hover:text-fg-primary transition-colors"
                        title="Settings"
                      >
                        ⚙️
                      </motion.button>
                    </>
                  )}
                </div>
              </div>

              {/* Settings Panel */}
              <AnimatePresence>
                {showSettings && (
                  <motion.div
                    initial={{ opacity: 0, height: 0 }}
                    animate={{ opacity: 1, height: 'auto' }}
                    exit={{ opacity: 0, height: 0 }}
                    className="border-b border-border-default bg-surface-2/50 p-4 space-y-3"
                  >
                    <h4 className="text-sm font-semibold text-fg-primary mb-3">Notification Preferences</h4>

                    {Object.entries(preferences).map(([key, value]) => (
                      <label
                        key={key}
                        className="flex items-center gap-3 cursor-pointer hover:bg-surface-1/5 p-2 rounded-lg transition-colors"
                      >
                        <input
                          type="checkbox"
                          checked={value}
                          onChange={(e) =>
                            setPreferences((prev) => ({
                              ...prev,
                              [key]: e.target.checked,
                            }))
                          }
                          className="w-4 h-4 rounded border-border-default cursor-pointer"
                        />
                        <span className="text-sm text-fg-secondary capitalize">{key} notifications</span>
                      </label>
                    ))}

                    <motion.button
                      whileHover={{ scale: 1.02 }}
                      onClick={requestNotificationPermission}
                      className="w-full mt-4 px-3 py-2 bg-accent-solid hover:brightness-110 text-fg-on-solid text-sm rounded-lg transition-colors"
                    >
                      Enable Desktop Notifications
                    </motion.button>
                  </motion.div>
                )}
              </AnimatePresence>

              {/* Notifications List */}
              <div className="overflow-y-auto flex-1">
                {notifications.length === 0 ? (
                  <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    className="p-8 text-center text-fg-muted"
                  >
                    <Bell size={32} className="mx-auto mb-3 opacity-30" />
                    <p className="text-sm">No notifications yet</p>
                  </motion.div>
                ) : (
                  <div className="divide-y divide-border-subtle">
                    {notifications.map((notification, index) => {
                      const config = getNotificationConfig(notification.type);
                      const Icon = config.icon;

                      return (
                        <motion.div
                          key={notification.id}
                          initial={{ opacity: 0, x: 20 }}
                          animate={{ opacity: 1, x: 0 }}
                          exit={{ opacity: 0, x: -20 }}
                          transition={{ delay: index * 0.02 }}
                          onClick={() => handleNotificationClick(notification)}
                          className={`p-4 hover:bg-surface-1/5 transition-colors cursor-pointer group ${
                            !notification.read ? 'bg-surface-1/2' : ''
                          }`}
                        >
                          <div className="flex gap-3">
                            <motion.div
                              whileHover={{ scale: 1.1 }}
                              className={`p-2 rounded-full ${config.bg} shrink-0`}
                            >
                              <Icon size={16} className={config.color} />
                            </motion.div>

                            <div className="flex-1 min-w-0">
                              <div className="flex items-start justify-between gap-2">
                                <div className="flex-1">
                                  <p className="font-semibold text-fg-primary text-sm group-hover:text-info-text transition-colors">
                                    {notification.title}
                                  </p>
                                  <p className="text-xs text-fg-secondary mt-1 line-clamp-2">
                                    {notification.message}
                                  </p>
                                  <p className="text-xs text-fg-muted mt-2">
                                    {new Date(notification.timestamp).toLocaleTimeString()}
                                  </p>
                                </div>

                                {!notification.read && (
                                  <motion.div
                                    initial={{ scale: 0 }}
                                    animate={{ scale: 1 }}
                                    className="w-2 h-2 bg-accent rounded-full shrink-0 mt-1"
                                  />
                                )}
                              </div>

                              {/* Action Buttons */}
                              <div className="flex gap-2 mt-3">
                                {notification.action && (
                                  <motion.button
                                    whileHover={{ scale: 1.05 }}
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      notification.action?.onClick();
                                      markAsRead(notification.id);
                                    }}
                                    className="text-xs px-3 py-1 bg-accent-solid hover:brightness-110 text-fg-on-solid rounded transition-colors"
                                  >
                                    {notification.action.label}
                                  </motion.button>
                                )}
                                <motion.button
                                  whileHover={{ scale: 1.05 }}
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    removeNotification(notification.id);
                                  }}
                                  className="text-xs px-2 py-1 text-fg-muted hover:text-danger-text transition-colors ml-auto"
                                >
                                  <X size={14} />
                                </motion.button>
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
                <motion.div
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  className="border-t border-border-strong/5 p-3 flex gap-2 shrink-0"
                >
                  <button
                    onClick={markAllAsRead}
                    className="flex-1 text-xs px-3 py-2 bg-accent-soft hover:bg-accent-line text-info-text rounded-lg transition-colors"
                  >
                    Mark All as Read
                  </button>
                  <button
                    onClick={clearAll}
                    className="flex-1 text-xs px-3 py-2 bg-danger/20 hover:bg-danger/30 text-danger-text rounded-lg transition-colors"
                  >
                    Clear All
                  </button>
                </motion.div>
              )}
            </motion.div>
          </>
        )}
      </AnimatePresence>
    </div>
  );
};
