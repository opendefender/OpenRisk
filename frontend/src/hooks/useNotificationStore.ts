import { create } from 'zustand';

export interface Notification {
  id: string;
  title: string;
  message: string;
  type: 'info' | 'warning' | 'error' | 'success';
  timestamp: Date;
  read: boolean;
  action?: {
    label: string;
    onClick: () => void;
  };
}

interface NotificationStore {
  notifications: Notification[];
  unreadCount: number;
  addNotification: (notification: Omit<Notification, 'id' | 'timestamp' | 'read'>) => void;
  markAsRead: (id: string) => void;
  markAllAsRead: () => void;
  removeNotification: (id: string) => void;
  clearAll: () => void;
}

export const useNotificationStore = create<NotificationStore>((set) => ({
  notifications: [],
  unreadCount: ,

  addNotification: (notification) =>
    set((state) => {
      const newNotification: Notification = {
        ...notification,
        id: ${Date.now()}-${Math.random()},
        timestamp: new Date(),
        read: false,
      };
      return {
        notifications: [newNotification, ...state.notifications],
        unreadCount: state.unreadCount + ,
      };
    }),

  markAsRead: (id) =>
    set((state) => {
      const updated = state.notifications.map((n) =>
        n.id === id ? { ...n, read: true } : n
      );
      const wasUnread = state.notifications.find((n) => n.id === id)?.read === false;
      return {
        notifications: updated,
        unreadCount: Math.max(, state.unreadCount - (wasUnread ?  : )),
      };
    }),

  markAllAsRead: () =>
    set((state) => ({
      notifications: state.notifications.map((n) => ({ ...n, read: true })),
      unreadCount: ,
    })),

  removeNotification: (id) =>
    set((state) => {
      const wasUnread = state.notifications.find((n) => n.id === id)?.read === false;
      return {
        notifications: state.notifications.filter((n) => n.id !== id),
        unreadCount: Math.max(, state.unreadCount - (wasUnread ?  : )),
      };
    }),

  clearAll: () =>
    set(() => ({
      notifications: [],
      unreadCount: ,
    })),
}));
