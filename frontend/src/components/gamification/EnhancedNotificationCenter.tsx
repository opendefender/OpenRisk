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
        color: 'text-emerald-400',
        bg: 'bg-emerald-500/10',
        textColor: 'text-emerald-300',
        gradient: 'from-emerald-600 to-emerald-700',
      },
      error: {
        icon: AlertCircle,
        color: 'text-red-400',
        bg: 'bg-red-500/10',
        textColor: 'text-red-300',
        gradient: 'from-red-600 to-red-700',
      },
      warning: {
        icon: AlertTriangle,
        color: 'text-yellow-400',
        bg: 'bg-yellow-500/10',
        textColor: 'text-yellow-300',
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
        color: 'text-blue-400',
        bg: 'bg-blue-500/10',
        textColor: 'text-blue-300',
        gradient: 'from-blue-600 to-blue-700',
      },
      info: {
        icon: Info,
        color: 'text-blue-400',
        bg: 'bg-blue-500/10',
        textColor: 'text-blue-300',
        gradient: 'from-blue-600 to-blue-700',
      },
      default: {
        icon: Info,
        color: 'text-gray-400',
        bg: 'bg-gray-500/10',
        textColor: 'text-gray-300',
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
        className="relative text-zinc-400 hover:text-white transition-colors p-2 hover:bg-white/5 rounded-full group"
      >
        <Bell size={20} />

        {/* Pulsing indicator for unread */}
        {unreadCount > 0 && (
          <>
            <span className="absolute top-1.5 right-1.5 w-4 h-4 bg-red-500 rounded-full border border-background text-white text-xs flex items-center justify-center font-bold">
              {unreadCount > 9 ? '9+' : unreadCount}
            </span>
            <span className="absolute top-1.5 right-1.5 w-4 h-4 bg-red-500 rounded-full animate-pulse" />
          </>
        )}

        {/* Hover tooltip */}
        <div className="absolute bottom-full right-0 mb-2 px-3 py-1 bg-zinc-900 text-white text-xs rounded-md opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none whitespace-nowrap">
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
              className="absolute right-0 mt-2 w-96 max-h-[600px] bg-zinc-900 border border-zinc-800 rounded-2xl shadow-2xl flex flex-col z-50 overflow-hidden"
            >
              {/* Header */}
              <div className="bg-gradient-to-r from-zinc-800 to-zinc-900 border-b border-zinc-700 p-4 flex items-center justify-between flex-shrink-0">
                <div className="flex items-center gap-2">
                  <Bell size={18} className="text-blue-400" />
                  <h3 className="font-semibold text-white text-lg">Notifications</h3>
                </div>
                <div className="flex items-center gap-2">
                  {notifications.length > 0 && (
                    <>
                      <motion.button
                        whileHover={{ scale: 1.05 }}
                        onClick={markAllAsRead}
                        className="text-xs px-2 py-1 text-blue-400 hover:text-blue-300 transition-colors"
                        title="Mark all as read"
                      >
                        Mark all read
                      </motion.button>
                      <motion.button
                        whileHover={{ scale: 1.05 }}
                        onClick={() => setShowSettings(!showSettings)}
                        className="text-xs px-2 py-1 text-zinc-400 hover:text-white transition-colors"
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
                    className="border-b border-zinc-700 bg-zinc-800/50 p-4 space-y-3"
                  >
                    <h4 className="text-sm font-semibold text-white mb-3">Notification Preferences</h4>

                    {Object.entries(preferences).map(([key, value]) => (
                      <label
                        key={key}
                        className="flex items-center gap-3 cursor-pointer hover:bg-white/5 p-2 rounded-lg transition-colors"
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
                          className="w-4 h-4 rounded border-zinc-600 cursor-pointer"
                        />
                        <span className="text-sm text-zinc-300 capitalize">{key} notifications</span>
                      </label>
                    ))}

                    <motion.button
                      whileHover={{ scale: 1.02 }}
                      onClick={requestNotificationPermission}
                      className="w-full mt-4 px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm rounded-lg transition-colors"
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
                    className="p-8 text-center text-zinc-500"
                  >
                    <Bell size={32} className="mx-auto mb-3 opacity-30" />
                    <p className="text-sm">No notifications yet</p>
                  </motion.div>
                ) : (
                  <div className="divide-y divide-white/5">
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
                          className={`p-4 hover:bg-white/5 transition-colors cursor-pointer group ${
                            !notification.read ? 'bg-white/2' : ''
                          }`}
                        >
                          <div className="flex gap-3">
                            <motion.div
                              whileHover={{ scale: 1.1 }}
                              className={`p-2 rounded-full ${config.bg} flex-shrink-0`}
                            >
                              <Icon size={16} className={config.color} />
                            </motion.div>

                            <div className="flex-1 min-w-0">
                              <div className="flex items-start justify-between gap-2">
                                <div className="flex-1">
                                  <p className="font-semibold text-white text-sm group-hover:text-blue-300 transition-colors">
                                    {notification.title}
                                  </p>
                                  <p className="text-xs text-zinc-400 mt-1 line-clamp-2">
                                    {notification.message}
                                  </p>
                                  <p className="text-xs text-zinc-600 mt-2">
                                    {new Date(notification.timestamp).toLocaleTimeString()}
                                  </p>
                                </div>

                                {!notification.read && (
                                  <motion.div
                                    initial={{ scale: 0 }}
                                    animate={{ scale: 1 }}
                                    className="w-2 h-2 bg-blue-500 rounded-full flex-shrink-0 mt-1"
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
                                    className="text-xs px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors"
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
                                  className="text-xs px-2 py-1 text-zinc-500 hover:text-red-400 transition-colors ml-auto"
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
                  className="border-t border-white/5 p-3 flex gap-2 flex-shrink-0"
                >
                  <button
                    onClick={markAllAsRead}
                    className="flex-1 text-xs px-3 py-2 bg-blue-600/20 hover:bg-blue-600/30 text-blue-300 rounded-lg transition-colors"
                  >
                    Mark All as Read
                  </button>
                  <button
                    onClick={clearAll}
                    className="flex-1 text-xs px-3 py-2 bg-red-600/20 hover:bg-red-600/30 text-red-300 rounded-lg transition-colors"
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
