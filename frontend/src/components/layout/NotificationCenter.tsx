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
        return { icon: Check, color: 'text-emerald-', bg: 'bg-emerald-/' };
      case 'error':
        return { icon: AlertCircle, color: 'text-red-', bg: 'bg-red-/' };
      case 'warning':
        return { icon: AlertTriangle, color: 'text-yellow-', bg: 'bg-yellow-/' };
      case 'info':
      default:
        return { icon: Info, color: 'text-blue-', bg: 'bg-blue-/' };
    }
  };

  return (
    <div className="relative">
      {/ Notification Bell Button /}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="relative text-zinc- hover:text-white transition-colors p- hover:bg-white/ rounded-full"
      >
        <Bell size={} />
        {unreadCount >  && (
          <span className="absolute top-. right-. w- h- bg-red- rounded-full animate-pulse border border-background text-white text-xs flex items-center justify-center font-bold">
            {unreadCount >  ? '+' : unreadCount}
          </span>
        )}
      </button>

      {/ Dropdown Panel /}
      <AnimatePresence>
        {isOpen && (
          <>
            {/ Backdrop /}
            <div
              className="fixed inset- z-"
              onClick={() => setIsOpen(false)}
            />

            {/ Notification Panel /}
            <motion.div
              initial={{ opacity: , y: - }}
              animate={{ opacity: , y:  }}
              exit={{ opacity: , y: - }}
              className="absolute right- top-full mt- w- bg-surface border border-border rounded-lg shadow-xl z- max-h-[px] flex flex-col"
            >
              {/ Header /}
              <div className="border-b border-white/ p- flex items-center justify-between">
                <div>
                  <h className="font-bold text-white">Notifications</h>
                  {unreadCount >  && (
                    <p className="text-xs text-zinc-">{unreadCount} unread</p>
                  )}
                </div>
                <div className="flex gap-">
                  {notifications.length >  && (
                    <>
                      <button
                        onClick={markAllAsRead}
                        className="text-xs px- py- text-zinc- hover:text-white transition-colors"
                      >
                        Mark all read
                      </button>
                      <button
                        onClick={clearAll}
                        className="text-xs px- py- text-zinc- hover:text-red- transition-colors"
                      >
                        Clear all
                      </button>
                    </>
                  )}
                </div>
              </div>

              {/ Notifications List /}
              <div className="overflow-y-auto flex-">
                {notifications.length ===  ? (
                  <div className="p- text-center text-zinc-">
                    <Bell size={} className="mx-auto mb- opacity-" />
                    <p>No notifications</p>
                  </div>
                ) : (
                  <div className="divide-y divide-white/">
                    {notifications.map((notification) => {
                      const { icon: Icon, color, bg } = getIconAndColor(notification.type);
                      return (
                        <motion.div
                          key={notification.id}
                          initial={{ opacity: , x:  }}
                          animate={{ opacity: , x:  }}
                          exit={{ opacity: , x: - }}
                          className={p- hover:bg-white/ transition-colors ${!notification.read ? 'bg-white/' : ''}}
                        >
                          <div className="flex gap-">
                            <div className={p- rounded-full ${bg} flex-shrink-}>
                              <Icon size={} className={color} />
                            </div>
                            <div className="flex- min-w-">
                              <div className="flex items-start justify-between gap-">
                                <div className="flex-">
                                  <p className="font-semibold text-white text-sm">
                                    {notification.title}
                                  </p>
                                  <p className="text-xs text-zinc- mt-">
                                    {notification.message}
                                  </p>
                                  <p className="text-xs text-zinc- mt-">
                                    {new Date(notification.timestamp).toLocaleString()}
                                  </p>
                                </div>
                                {!notification.read && (
                                  <div className="w- h- bg-blue- rounded-full flex-shrink- mt-" />
                                )}
                              </div>
                              <div className="flex gap- mt-">
                                {notification.action && (
                                  <button
                                    onClick={() => {
                                      notification.action?.onClick();
                                      markAsRead(notification.id);
                                    }}
                                    className="text-xs text-blue- hover:text-blue- transition-colors"
                                  >
                                    {notification.action.label}
                                  </button>
                                )}
                                <button
                                  onClick={() => {
                                    markAsRead(notification.id);
                                  }}
                                  className="text-xs text-zinc- hover:text-zinc- transition-colors"
                                >
                                  {notification.read ? 'Mark unread' : 'Mark read'}
                                </button>
                                <button
                                  onClick={() => removeNotification(notification.id)}
                                  className="text-xs text-zinc- hover:text-red- transition-colors ml-auto"
                                >
                                  <X size={} />
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

              {/ Footer /}
              {notifications.length >  && (
                <div className="border-t border-white/ p-">
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
