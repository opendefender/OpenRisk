import { useState, useEffect } from 'react';
import { Bell, X, Check, AlertCircle, Info, AlertTriangle } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { useNotificationStore } from '../../hooks/useNotificationStore';
import { Button } from '../ui/Button';

export const NotificationCenter = () => {
  const { notifications, unreadCount, markAsRead, markAllAsRead, removeNotification, clearAll } = useNotificationStore();
  const [isOpen, setIsOpen] = useState(false);

  const getIconAndColor = (type: string) => {
    switch (type) {
      case 'success':
        return { icon: Check, color: 'text-emerald-400', bg: 'bg-emerald-500/10' };
      case 'error':
        return { icon: AlertCircle, color: 'text-red-400', bg: 'bg-red-500/10' };
      case 'warning':
        return { icon: AlertTriangle, color: 'text-yellow-400', bg: 'bg-yellow-500/10' };
      case 'info':
      default:
        return { icon: Info, color: 'text-blue-400', bg: 'bg-blue-500/10' };
    }
  };

  return (
    <div className="relative">
      {/* Notification Bell Button */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="relative text-zinc-400 hover:text-white transition-colors p-2 hover:bg-white/5 rounded-full"
      >
        <Bell size={20} />
        {unreadCount > 0 && (
          <span className="absolute top-1.5 right-1.5 w-4 h-4 bg-red-500 rounded-full animate-pulse border border-background text-white text-xs flex items-center justify-center font-bold">
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
              <div className="border-b border-white/5 p-4 flex items-center justify-between">
                <div>
                  <h3 className="font-bold text-white">Notifications</h3>
                  {unreadCount > 0 && (
                    <p className="text-xs text-zinc-400">{unreadCount} unread</p>
                  )}
                </div>
                <div className="flex gap-2">
                  {notifications.length > 0 && (
                    <>
                      <button
                        onClick={markAllAsRead}
                        className="text-xs px-2 py-1 text-zinc-400 hover:text-white transition-colors"
                      >
                        Mark all read
                      </button>
                      <button
                        onClick={clearAll}
                        className="text-xs px-2 py-1 text-zinc-400 hover:text-red-400 transition-colors"
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
                  <div className="p-8 text-center text-zinc-500">
                    <Bell size={32} className="mx-auto mb-2 opacity-50" />
                    <p>No notifications</p>
                  </div>
                ) : (
                  <div className="divide-y divide-white/5">
                    {notifications.map((notification) => {
                      const { icon: Icon, color, bg } = getIconAndColor(notification.type);
                      return (
                        <motion.div
                          key={notification.id}
                          initial={{ opacity: 0, x: 20 }}
                          animate={{ opacity: 1, x: 0 }}
                          exit={{ opacity: 0, x: -20 }}
                          className={`p-4 hover:bg-white/5 transition-colors ${!notification.read ? 'bg-white/2' : ''}`}
                        >
                          <div className="flex gap-3">
                            <div className={`p-2 rounded-full ${bg} flex-shrink-0`}>
                              <Icon size={16} className={color} />
                            </div>
                            <div className="flex-1 min-w-0">
                              <div className="flex items-start justify-between gap-2">
                                <div className="flex-1">
                                  <p className="font-semibold text-white text-sm">
                                    {notification.title}
                                  </p>
                                  <p className="text-xs text-zinc-400 mt-1">
                                    {notification.message}
                                  </p>
                                  <p className="text-xs text-zinc-600 mt-2">
                                    {new Date(notification.timestamp).toLocaleString()}
                                  </p>
                                </div>
                                {!notification.read && (
                                  <div className="w-2 h-2 bg-blue-500 rounded-full flex-shrink-0 mt-1" />
                                )}
                              </div>
                              <div className="flex gap-2 mt-3">
                                {notification.action && (
                                  <button
                                    onClick={() => {
                                      notification.action?.onClick();
                                      markAsRead(notification.id);
                                    }}
                                    className="text-xs text-blue-400 hover:text-blue-300 transition-colors"
                                  >
                                    {notification.action.label}
                                  </button>
                                )}
                                <button
                                  onClick={() => {
                                    markAsRead(notification.id);
                                  }}
                                  className="text-xs text-zinc-500 hover:text-zinc-400 transition-colors"
                                >
                                  {notification.read ? 'Mark unread' : 'Mark read'}
                                </button>
                                <button
                                  onClick={() => removeNotification(notification.id)}
                                  className="text-xs text-zinc-500 hover:text-red-400 transition-colors ml-auto"
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
                <div className="border-t border-white/5 p-4">
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
